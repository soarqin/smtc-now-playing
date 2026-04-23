package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lxzan/gws"
	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/domain"
	"smtc-now-playing/internal/smtc"
	"smtc-now-playing/internal/wsproto"
)

type SMTCService interface {
	Subscribe(bufSize int) <-chan smtc.Event
	Unsubscribe(ch <-chan smtc.Event)
	GetSessions() []smtc.SessionInfo
	SelectDevice(appID string)
	GetCapabilities() smtc.ControlCapabilities
	Play() error
	Pause() error
	StopPlayback() error
	TogglePlayPause() error
	SkipNext() error
	SkipPrevious() error
	SeekTo(positionMs int64) error
	SetShuffle(active bool) error
	SetRepeat(mode int) error
}

type stateSnapshot struct {
	info         *domain.InfoData
	progress     *domain.ProgressData
	infoJSON     []byte
	progressJSON []byte
	albumArtHash string
	albumArtData []byte
	albumArtCT   string
}

type Server struct {
	cfg     *config.Config
	svc     SMTCService
	hub     *hub
	httpSrv *http.Server

	state atomic.Pointer[stateSnapshot]
}

func New(cfg *config.Config, smtcSvc SMTCService) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("server: nil config")
	}
	if smtcSvc == nil {
		return nil, errors.New("server: nil SMTC service")
	}

	s := &Server{
		cfg: cfg,
		svc: smtcSvc,
		hub: newHub(),
	}
	s.state.Store(&stateSnapshot{})

	mux := s.setupRoutes()
	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: mux,
	}

	return s, nil
}

func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/now-playing", s.handleNowPlaying)
	mux.HandleFunc("GET /api/devices", s.handleSessions)
	mux.HandleFunc("GET /api/sessions", s.handleSessions)
	mux.HandleFunc("GET /api/capabilities", s.handleCapabilities)
	mux.HandleFunc("POST /api/control/{action}", localhostOnly(s.handleControl, s.cfg.Server.AllowRemote))
	mux.HandleFunc("GET /albumArt/{hash}", s.handleAlbumArt)
	mux.HandleFunc("GET /script/{file}", s.handleScript)
	mux.HandleFunc("GET /ws", s.handleWebSocket)
	mux.HandleFunc("GET /", s.handleTheme)
	return mux
}

func (s *Server) Run(ctx context.Context) error {
	go s.hub.Run(ctx)

	eventCh := s.svc.Subscribe(64)
	defer s.svc.Unsubscribe(eventCh)

	go s.processEvents(ctx, eventCh)

	watcherErrCh := make(chan error, 1)
	if s.cfg.Server.HotReload {
		go s.runHotReload(ctx, watcherErrCh)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		s.hub.Shutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-watcherErrCh:
		s.hub.Shutdown()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.httpSrv.Shutdown(shutdownCtx)
		return err
	case err := <-errCh:
		s.hub.Shutdown()
		return err
	}
}

func (s *Server) processEvents(ctx context.Context, eventCh <-chan smtc.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			s.handleEvent(ev)
		}
	}
}

func (s *Server) handleEvent(ev smtc.Event) {
	switch e := ev.(type) {
	case smtc.InfoEvent:
		s.handleInfoEvent(e.Data)
	case smtc.ProgressEvent:
		s.handleProgressEvent(e.Data)
	case smtc.SessionsChangedEvent:
		s.broadcastEnvelope(wsproto.NewSessions(e.Sessions))
	case smtc.DeviceChangedEvent:
		slog.Debug("active SMTC device changed", "appID", e.AppID)
	}
}

func (s *Server) handleInfoEvent(data domain.InfoData) {
	prev := s.snapshot()
	next := s.cloneState(prev)

	infoCopy := cloneInfoData(data)
	next.info = infoCopy

	if len(data.ThumbnailData) > 0 {
		thumb := append([]byte(nil), data.ThumbnailData...)
		next.albumArtData = thumb
		next.albumArtCT = data.ThumbnailContentType
		checksum := sha256.Sum256(thumb)
		next.albumArtHash = hex.EncodeToString(checksum[:])
	} else {
		next.albumArtData = nil
		next.albumArtCT = ""
		next.albumArtHash = ""
	}

	env := wsproto.NewInfo(*infoCopy, s.albumArtURL(next.albumArtHash))
	msg, err := json.Marshal(env)
	if err != nil {
		slog.Warn("failed to marshal info update", "err", err)
		return
	}

	if bytesEqual(prev.infoJSON, msg) {
		return
	}
	next.infoJSON = msg
	s.state.Store(next)
	s.hub.Broadcast(msg)
}

func (s *Server) handleProgressEvent(data domain.ProgressData) {
	prev := s.snapshot()
	next := s.cloneState(prev)
	progressCopy := cloneProgressData(data)
	next.progress = progressCopy

	env := wsproto.NewProgress(*progressCopy)
	msg, err := json.Marshal(env)
	if err != nil {
		slog.Warn("failed to marshal progress update", "err", err)
		return
	}

	if bytesEqual(prev.progressJSON, msg) {
		return
	}
	next.progressJSON = msg
	s.state.Store(next)
	s.hub.Broadcast(msg)
}

func (s *Server) runHotReload(ctx context.Context, errCh chan<- error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		select {
		case errCh <- err:
		default:
		}
		return
	}
	defer watcher.Close()

	themePath := filepath.Join("themes", s.cfg.UI.Theme)
	if err := watcher.Add(themePath); err != nil {
		select {
		case errCh <- err:
		default:
		}
		return
	}

	msg, err := json.Marshal(wsproto.NewReload())
	if err != nil {
		select {
		case errCh <- err:
		default:
		}
		return
	}

	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	defer debounce.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
				debounce.Reset(500 * time.Millisecond)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("hot reload watcher error", "err", err)
		case <-debounce.C:
			s.hub.Broadcast(msg)
		}
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	handler := &wsHandler{srv: s}
	upgrader := gws.NewUpgrader(handler, &gws.ServerOption{
		ParallelEnabled: true,
		ParallelGolimit: 10,
		Authorize: func(r *http.Request, session gws.SessionStorage) bool {
			return true
		},
	})

	conn, err := upgrader.Upgrade(w, r)
	if err != nil {
		return
	}

	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("websocket ReadLoop panic", "err", recovered)
				s.hub.Remove(conn)
				_ = conn.WriteClose(1011, nil)
			}
		}()
		conn.ReadLoop()
	}()
}

type wsHandler struct {
	srv *Server
}

func (h *wsHandler) OnOpen(socket *gws.Conn) {
	h.srv.hub.Add(socket)
	state := h.srv.snapshot()
	if len(state.infoJSON) > 0 {
		_ = socket.WriteMessage(gws.OpcodeText, state.infoJSON)
	}
	if len(state.progressJSON) > 0 {
		_ = socket.WriteMessage(gws.OpcodeText, state.progressJSON)
	}
	if sessions := h.srv.svc.GetSessions(); len(sessions) > 0 {
		if msg, err := json.Marshal(wsproto.NewSessions(sessionInfosToDomain(sessions))); err == nil {
			_ = socket.WriteMessage(gws.OpcodeText, msg)
		}
	}
	slog.Info("WS client connected")
}

func (h *wsHandler) OnClose(socket *gws.Conn, err error) {
	h.srv.hub.Remove(socket)
	slog.Info("WS client disconnected")
}

func (h *wsHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (h *wsHandler) OnPong(socket *gws.Conn, payload []byte) {}

func (h *wsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	message.Close()
}

func (s *Server) handleAlbumArt(w http.ResponseWriter, r *http.Request) {
	state := s.snapshot()
	if state.albumArtHash == "" || r.PathValue("hash") != state.albumArtHash || len(state.albumArtData) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	contentType := state.albumArtCT
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(state.albumArtData)
}

func safeThemePath(theme, urlPath string) (string, bool) {
	return safeJoin(filepath.Join("themes", theme), urlPath)
}

func safeJoin(base, rel string) (string, bool) {
	if base == "" {
		return "", false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", false
	}
	rel = strings.TrimLeft(rel, "/")
	if strings.ContainsRune(rel, '\\') {
		return "", false
	}
	joined := filepath.Join(absBase, filepath.FromSlash(rel))
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", false
	}
	relPath, err := filepath.Rel(absBase, absJoined)
	if err != nil {
		return "", false
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return "", false
	}
	return absJoined, true
}

func (s *Server) handleTheme(w http.ResponseWriter, r *http.Request) {
	path, ok := safeThemePath(s.cfg.UI.Theme, r.URL.Path)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, path)
}

func (s *Server) handleScript(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/script")
	path, ok := safeJoin("script", rel)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, path)
}

func (s *Server) Address() string {
	if s.httpSrv == nil {
		return ""
	}
	return s.httpSrv.Addr
}

func (s *Server) SetTheme(theme string) {
	s.cfg.UI.Theme = theme
}

func (s *Server) GetSessions() []smtc.SessionInfo {
	return s.svc.GetSessions()
}

func (s *Server) SelectDevice(appID string) {
	s.svc.SelectDevice(appID)
}

func (s *Server) snapshot() *stateSnapshot {
	state := s.state.Load()
	if state == nil {
		return &stateSnapshot{}
	}
	return state
}

func (s *Server) cloneState(prev *stateSnapshot) *stateSnapshot {
	if prev == nil {
		return &stateSnapshot{}
	}
	next := *prev
	next.infoJSON = append([]byte(nil), prev.infoJSON...)
	next.progressJSON = append([]byte(nil), prev.progressJSON...)
	next.albumArtData = append([]byte(nil), prev.albumArtData...)
	return &next
}

func (s *Server) albumArtURL(hash string) string {
	if hash == "" {
		return ""
	}
	return "/albumArt/" + hash
}

func (s *Server) broadcastEnvelope(env wsproto.Envelope) {
	msg, err := json.Marshal(env)
	if err != nil {
		slog.Warn("failed to marshal websocket message", "err", err)
		return
	}
	s.hub.Broadcast(msg)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func cloneInfoData(data domain.InfoData) *domain.InfoData {
	copyData := data
	copyData.ThumbnailData = append([]byte(nil), data.ThumbnailData...)
	return &copyData
}

func cloneProgressData(data domain.ProgressData) *domain.ProgressData {
	copyData := data
	if data.IsShuffleActive != nil {
		value := *data.IsShuffleActive
		copyData.IsShuffleActive = &value
	}
	return &copyData
}

func sessionInfosToDomain(sessions []smtc.SessionInfo) []domain.SessionInfo {
	if len(sessions) == 0 {
		return nil
	}
	out := make([]domain.SessionInfo, len(sessions))
	for i, session := range sessions {
		out[i] = domain.SessionInfo{
			AppID:       session.AppID,
			Name:        session.Name,
			SourceAppID: session.SourceAppID,
		}
	}
	return out
}

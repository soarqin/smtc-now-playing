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
	"smtc-now-playing/internal/version"
	"smtc-now-playing/internal/wsproto"
)

var log = slog.With("subsystem", "server")

// Sentinel errors for server operations.
var (
	ErrServerShutdown = errors.New("server: shut down")
)

// ControlError represents an error during media control execution.
type ControlError struct {
	Action string
	Err    error
}

// Error implements the error interface.
func (e *ControlError) Error() string {
	return fmt.Sprintf("server: control %s failed: %v", e.Action, e.Err)
}

// Unwrap returns the underlying error.
func (e *ControlError) Unwrap() error {
	return e.Err
}

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

const heartbeatStateKey = "heartbeatState"

type heartbeatState struct {
	lastPongUnixMilli atomic.Int64
}

type wsControlSeekArgs struct {
	Position int64 `json:"position"`
}

type wsControlShuffleArgs struct {
	Active bool `json:"active"`
}

type wsControlRepeatArgs struct {
	Mode int `json:"mode"`
}

func newHeartbeatState(now time.Time) *heartbeatState {
	state := &heartbeatState{}
	state.Touch(now)
	return state
}

func (s *heartbeatState) Touch(now time.Time) {
	s.lastPongUnixMilli.Store(now.UnixMilli())
}

func (s *heartbeatState) LastPong() time.Time {
	return time.UnixMilli(s.lastPongUnixMilli.Load())
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
	s.httpSrv = s.newHTTPServer(s.setupRoutes())

	return s, nil
}

func (s *Server) setupRoutes() http.Handler {
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
	return accessLog(mux, s.cfg.Logging.Debug)
}

func (s *Server) newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.Server.Port),
		Handler:           handler,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		ReadHeaderTimeout: 3 * time.Second,
	}
}

func (s *Server) shutdownHTTPServer() {
	if s.httpSrv == nil {
		return
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	defer cancel()
	_ = s.httpSrv.Shutdown(shutdownCtx)
}

func (s *Server) Run(ctx context.Context) error {
	go s.hub.Run(ctx)

	eventCh := s.svc.Subscribe(subscribeBufSize)
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
		s.shutdownHTTPServer()
		return ctx.Err()
	case err := <-watcherErrCh:
		s.hub.Shutdown()
		s.shutdownHTTPServer()
		return err
	case err := <-errCh:
		s.hub.Shutdown()
		s.shutdownHTTPServer()
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
				debounce.Reset(hotReloadDebounce)
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
	heartbeat := newHeartbeatState(time.Now())
	socket.Session().Store(heartbeatStateKey, heartbeat)

	if msg, err := json.Marshal(wsproto.NewHello(version.Version, h.srv.capabilitiesToMap())); err == nil {
		_ = socket.WriteMessage(gws.OpcodeText, msg)
	}

	snapshot := h.srv.snapshot()
	if len(snapshot.infoJSON) > 0 {
		_ = socket.WriteMessage(gws.OpcodeText, snapshot.infoJSON)
	}
	if len(snapshot.progressJSON) > 0 {
		_ = socket.WriteMessage(gws.OpcodeText, snapshot.progressJSON)
	}
	if sessions := h.srv.svc.GetSessions(); len(sessions) > 0 {
		if msg, err := json.Marshal(wsproto.NewSessions(sessionInfosToDomain(sessions))); err == nil {
			_ = socket.WriteMessage(gws.OpcodeText, msg)
		}
	}
	go h.srv.runHeartbeat(socket)
	slog.Info("WS client connected")
}

func (h *wsHandler) OnClose(socket *gws.Conn, err error) {
	h.srv.hub.Remove(socket)
	slog.Info("WS client disconnected")
}

func (h *wsHandler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.WritePong(payload)
}

func (h *wsHandler) OnPong(socket *gws.Conn, payload []byte) {
	h.srv.touchHeartbeat(socket)
}

func (h *wsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	env, err := wsproto.ParseEnvelope(message.Bytes())
	if err != nil {
		h.srv.closeUnsupportedProtocol(socket)
		return
	}

	switch env.Type {
	case wsproto.MsgPing:
		msg, err := json.Marshal(wsproto.NewPong(env.TS))
		if err != nil {
			slog.Warn("failed to marshal websocket pong", "err", err)
			return
		}
		_ = socket.WriteMessage(gws.OpcodeText, msg)
	case wsproto.MsgPong:
		h.srv.touchHeartbeat(socket)
	case wsproto.MsgControl:
		ctrl, err := env.ParseControl()
		if err != nil || ctrl.Action == "" {
			h.srv.writeControlAck(socket, env.ID, errors.New("invalid control payload"))
			return
		}
		go h.srv.handleWSControl(socket, env.ID, ctrl.Action, ctrl.Args)
	default:
		h.srv.handleUnknownMessage(socket, env)
	}
}

func (s *Server) handleUnknownMessage(conn *gws.Conn, env wsproto.Envelope) {
	if env.ID != "" {
		s.writeControlAck(conn, env.ID, fmt.Errorf("unsupported message type: %s", env.Type))
		return
	}
	s.closeUnsupportedProtocol(conn)
}

func (s *Server) runHeartbeat(conn *gws.Conn) {
	ticker := time.NewTicker(wsHeartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		lastPong := s.lastHeartbeat(conn)
		if !lastPong.IsZero() && time.Since(lastPong) > wsHeartbeatTimeout {
			_ = conn.WriteClose(4001, []byte("heartbeat timeout"))
			if netConn := conn.NetConn(); netConn != nil {
				_ = netConn.Close()
			}
			return
		}

		msg, err := json.Marshal(wsproto.NewPing())
		if err != nil {
			slog.Warn("failed to marshal websocket ping", "err", err)
			return
		}
		if err := conn.WriteMessage(gws.OpcodeText, msg); err != nil {
			return
		}
	}
}

func (s *Server) closeUnsupportedProtocol(conn *gws.Conn) {
	_ = conn.WriteClose(4002, []byte("unsupported protocol version"))
	if netConn := conn.NetConn(); netConn != nil {
		_ = netConn.Close()
	}
}

func (s *Server) touchHeartbeat(conn *gws.Conn) {
	if state := s.heartbeatState(conn); state != nil {
		state.Touch(time.Now())
	}
}

func (s *Server) lastHeartbeat(conn *gws.Conn) time.Time {
	if state := s.heartbeatState(conn); state != nil {
		return state.LastPong()
	}
	return time.Time{}
}

func (s *Server) heartbeatState(conn *gws.Conn) *heartbeatState {
	value, ok := conn.Session().Load(heartbeatStateKey)
	if !ok {
		return nil
	}
	state, _ := value.(*heartbeatState)
	return state
}

func (s *Server) handleWSControl(conn *gws.Conn, id string, action string, args json.RawMessage) {
	err := s.executeWSControl(action, args)
	s.writeControlAck(conn, id, err)
}

func (s *Server) executeWSControl(action string, args json.RawMessage) error {
	switch action {
	case "play":
		return s.svc.Play()
	case "pause":
		return s.svc.Pause()
	case "stop":
		return s.svc.StopPlayback()
	case "toggle":
		return s.svc.TogglePlayPause()
	case "next":
		return s.svc.SkipNext()
	case "previous":
		return s.svc.SkipPrevious()
	case "seek":
		var body wsControlSeekArgs
		if err := json.Unmarshal(args, &body); err != nil {
			return err
		}
		return s.svc.SeekTo(body.Position)
	case "shuffle":
		var body wsControlShuffleArgs
		if err := json.Unmarshal(args, &body); err != nil {
			return err
		}
		return s.svc.SetShuffle(body.Active)
	case "repeat":
		var body wsControlRepeatArgs
		if err := json.Unmarshal(args, &body); err != nil {
			return err
		}
		return s.svc.SetRepeat(body.Mode)
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

func (s *Server) writeControlAck(conn *gws.Conn, id string, controlErr error) {
	msg, err := json.Marshal(wsproto.NewAck(id, controlErr))
	if err != nil {
		slog.Warn("failed to marshal websocket ack", "err", err)
		return
	}
	if err := conn.WriteMessage(gws.OpcodeText, msg); err != nil {
		slog.Debug("failed to write websocket ack", "err", err)
	}
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

func (s *Server) capabilitiesToMap() map[string]bool {
	caps := s.svc.GetCapabilities()
	return map[string]bool{
		"control":          true,
		"heartbeat":        true,
		"play":             caps.IsPlayEnabled,
		"pause":            caps.IsPauseEnabled,
		"stop":             caps.IsStopEnabled,
		"next":             caps.IsNextEnabled,
		"previous":         caps.IsPreviousEnabled,
		"seek":             caps.IsSeekEnabled,
		"shuffle":          caps.IsShuffleEnabled,
		"repeat":           caps.IsRepeatEnabled,
		"sessions":         true,
		"reload":           s.cfg.Server.HotReload,
		"albumArtEndpoint": true,
	}
}

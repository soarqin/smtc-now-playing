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
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/lxzan/gws"
	"smtc-now-playing/internal/smtc"
)

type WebServer struct {
	httpSrv *http.Server
	smtc    *smtc.Smtc

	currentTheme string

	currentMutex     sync.Mutex
	currentInfo      string
	currentProgress  string
	currentSourceApp string

	errorChan           chan error
	waitGroup           sync.WaitGroup
	smtcCancel          context.CancelFunc
	wsConnections       map[*gws.Conn]struct{}
	wsConnectionsMutex  sync.Mutex
	albumArtContentType string
	albumArtData        []byte

	// Hot-reload file watcher
	hotReload   bool
	watcher     *fsnotify.Watcher
	hotReloadCh chan struct{} // closed to signal the watcher goroutine to exit
	hotReloadWG sync.WaitGroup
	hotReloadMu sync.Mutex // protects watcher / hotReloadCh during Start/Stop

	// Device selection
	sessionsMutex          sync.Mutex
	sessions               []smtc.SessionInfo
	onSessionsChanged      func()
	onSelectedDeviceChange func(string)

	// shutdown flag — once true, broadcast / connection handling no-ops so
	// late events from SMTC or pending goroutines can't touch torn-down state.
	shuttingDown bool
	shutdownMu   sync.RWMutex
}

type infoDetail struct {
	Title        string `json:"title"`
	Artist       string `json:"artist"`
	AlbumArt     string `json:"albumArt"`
	AlbumTitle   string `json:"albumTitle"`
	AlbumArtist  string `json:"albumArtist"`
	PlaybackType int    `json:"playbackType"`
	SourceApp    string `json:"sourceApp"`
}

type progressDetail struct {
	Position        int     `json:"position"`
	Duration        int     `json:"duration"`
	Status          int     `json:"status"`
	PlaybackRate    float64 `json:"playbackRate"`
	IsShuffleActive *bool   `json:"isShuffleActive"`
	AutoRepeatMode  int     `json:"autoRepeatMode"`
	LastUpdatedTime int64   `json:"lastUpdatedTime"`
}

func New(host string, port string, theme string, selectedDevice string, hotReload bool) *WebServer {
	mux := http.NewServeMux()
	srv := &WebServer{
		httpSrv: &http.Server{
			Addr:    fmt.Sprintf("%s:%s", host, port),
			Handler: mux,
		},
		currentTheme:  theme,
		wsConnections: make(map[*gws.Conn]struct{}),
		hotReload:     hotReload,
	}

	srv.smtc = smtc.New(smtc.Options{
		OnInfo: func(data smtc.InfoData) {
			srv.handleInfoUpdate(data)
		},
		OnProgress: func(data smtc.ProgressData) {
			srv.handleProgressUpdate(data)
		},
		OnSessionsChanged: func(sessions []smtc.SessionInfo) {
			srv.sessionsMutex.Lock()
			srv.sessions = sessions
			srv.sessionsMutex.Unlock()
			if srv.onSessionsChanged != nil {
				srv.onSessionsChanged()
			}
		},
		OnSelectedDeviceChange: func(appID string) {
			srv.currentMutex.Lock()
			srv.currentSourceApp = appID
			srv.currentMutex.Unlock()
			if srv.onSelectedDeviceChange != nil {
				srv.onSelectedDeviceChange(appID)
			}
		},
		InitialDevice: selectedDevice,
	})

	mux.HandleFunc("/ws", srv.handleWebSocket)
	mux.HandleFunc("/albumArt/", srv.handleAlbumArt)
	mux.HandleFunc("/script/", srv.handleScript)
	mux.HandleFunc("GET /api/now-playing", srv.handleNowPlaying)
	mux.HandleFunc("GET /api/devices", srv.handleDevices)
	mux.HandleFunc("GET /api/sessions", srv.handleDevices)
	mux.HandleFunc("GET /api/capabilities", srv.handleCapabilities)
	mux.HandleFunc("POST /api/control/play", srv.handleControlPlay)
	mux.HandleFunc("POST /api/control/pause", srv.handleControlPause)
	mux.HandleFunc("POST /api/control/stop", srv.handleControlStop)
	mux.HandleFunc("POST /api/control/toggle", srv.handleControlToggle)
	mux.HandleFunc("POST /api/control/next", srv.handleControlNext)
	mux.HandleFunc("POST /api/control/previous", srv.handleControlPrevious)
	mux.HandleFunc("POST /api/control/seek", srv.handleControlSeek)
	mux.HandleFunc("POST /api/control/shuffle", srv.handleControlShuffle)
	mux.HandleFunc("POST /api/control/repeat", srv.handleControlRepeat)
	mux.HandleFunc("/", srv.handleStatic)

	return srv
}

func (srv *WebServer) handleInfoUpdate(data smtc.InfoData) {
	var info infoDetail
	info.Artist = data.Artist
	info.Title = data.Title
	info.AlbumTitle = data.AlbumTitle
	info.AlbumArtist = data.AlbumArtist
	info.PlaybackType = data.PlaybackType
	srv.currentMutex.Lock()
	info.SourceApp = srv.currentSourceApp
	if len(data.ThumbnailData) > 0 {
		srv.albumArtContentType = data.ThumbnailContentType
		srv.albumArtData = data.ThumbnailData
		checksum := sha256.Sum256(data.ThumbnailData)
		info.AlbumArt = "/albumArt/" + hex.EncodeToString(checksum[:])
	} else {
		srv.albumArtContentType = ""
		srv.albumArtData = nil
		info.AlbumArt = ""
	}
	srv.currentMutex.Unlock()
	j, err := json.Marshal(&struct {
		Type string      `json:"type"`
		Data *infoDetail `json:"data"`
	}{Type: "info", Data: &info})
	if err != nil {
		slog.Warn("failed to marshal info update", "err", err)
		return
	}
	infoStr := string(j)
	srv.currentMutex.Lock()
	if infoStr != srv.currentInfo {
		srv.currentInfo = infoStr
		srv.currentMutex.Unlock()
		srv.broadcastMessage(j)
	} else {
		srv.currentMutex.Unlock()
	}
}

func (srv *WebServer) handleProgressUpdate(data smtc.ProgressData) {
	progress := progressDetail{
		Position:        data.Position,
		Duration:        data.Duration,
		Status:          data.Status,
		PlaybackRate:    data.PlaybackRate,
		IsShuffleActive: data.IsShuffleActive,
		AutoRepeatMode:  data.AutoRepeatMode,
		LastUpdatedTime: data.LastUpdatedTime,
	}
	j, err := json.Marshal(&struct {
		Type string          `json:"type"`
		Data *progressDetail `json:"data"`
	}{Type: "progress", Data: &progress})
	if err != nil {
		slog.Warn("failed to marshal progress update", "err", err)
		return
	}
	progressStr := string(j)
	srv.currentMutex.Lock()
	if progressStr != srv.currentProgress {
		srv.currentProgress = progressStr
		srv.currentMutex.Unlock()
		srv.broadcastMessage(j)
	} else {
		srv.currentMutex.Unlock()
	}
}

func (srv *WebServer) addWebSocketConnection(conn *gws.Conn) {
	srv.wsConnectionsMutex.Lock()
	defer srv.wsConnectionsMutex.Unlock()
	srv.wsConnections[conn] = struct{}{}
}

func (srv *WebServer) removeWebSocketConnection(conn *gws.Conn) {
	srv.wsConnectionsMutex.Lock()
	defer srv.wsConnectionsMutex.Unlock()
	delete(srv.wsConnections, conn)
}

// broadcastMessage sends data to every currently-open WebSocket client.
// It takes a *snapshot* of the connection set under the mutex, then writes
// to each connection with the mutex released. This prevents a slow client
// from blocking broadcasts to every other client and avoids the classic
// "OnClose can't acquire the mutex while broadcast holds it" deadlock.
func (srv *WebServer) broadcastMessage(data []byte) {
	// Cheap shutdown short-circuit — avoid writes to connections that are
	// being torn down.
	srv.shutdownMu.RLock()
	down := srv.shuttingDown
	srv.shutdownMu.RUnlock()
	if down {
		return
	}

	srv.wsConnectionsMutex.Lock()
	conns := make([]*gws.Conn, 0, len(srv.wsConnections))
	for conn := range srv.wsConnections {
		conns = append(conns, conn)
	}
	srv.wsConnectionsMutex.Unlock()

	for _, conn := range conns {
		// gws.Conn.WriteMessage itself serializes concurrent writers on
		// a single connection, so calling it from our broadcaster is
		// safe even if another broadcaster goroutine is active.
		if err := conn.WriteMessage(gws.OpcodeText, data); err != nil {
			slog.Debug("websocket write failed", "err", err)
		}
	}
}

type wsHandler struct {
	srv *WebServer
}

func (h *wsHandler) OnOpen(socket *gws.Conn) {
	h.srv.addWebSocketConnection(socket)
	slog.Info("WS client connected")
	h.srv.currentMutex.Lock()
	info := h.srv.currentInfo
	progress := h.srv.currentProgress
	h.srv.currentMutex.Unlock()
	if info != "" {
		socket.WriteMessage(gws.OpcodeText, []byte(info))
	}
	if progress != "" {
		socket.WriteMessage(gws.OpcodeText, []byte(progress))
	}
}

func (h *wsHandler) OnClose(socket *gws.Conn, err error) {
	h.srv.removeWebSocketConnection(socket)
	slog.Info("WS client disconnected")
}

func (h *wsHandler) OnPing(socket *gws.Conn, payload []byte) {
	socket.WritePong(nil)
}

func (h *wsHandler) OnPong(socket *gws.Conn, payload []byte) {
}

func (h *wsHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	message.Close()
}

func (srv *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	handler := &wsHandler{srv: srv}
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
	// Wrap ReadLoop in a panic-recovery goroutine: a crash in the gws
	// parser (e.g. on a malformed frame) must not take down the process
	// or orphan this connection in wsConnections.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("websocket ReadLoop panic", "err", r)
				srv.removeWebSocketConnection(conn)
				_ = conn.WriteClose(1011, nil)
			}
		}()
		conn.ReadLoop()
	}()
}

func (srv *WebServer) handleAlbumArt(w http.ResponseWriter, r *http.Request) {
	srv.currentMutex.Lock()
	data := srv.albumArtData
	ct := srv.albumArtContentType
	srv.currentMutex.Unlock()
	if len(data) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if ct == "" {
		// Defensive: we have bytes but no content-type. Guess something
		// reasonable rather than serve an empty header.
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}

// safeThemePath resolves a request path against the theme directory while
// rejecting attempts to escape via ".." / absolute paths / backslashes.
// Returns the resolved absolute path and a boolean indicating validity.
func safeThemePath(theme, urlPath string) (string, bool) {
	return safeJoin(filepath.Join("themes", theme), urlPath)
}

// safeJoin joins base and rel, guaranteeing the resolved absolute path is
// contained within the absolute form of base. Rejects empty bases.
func safeJoin(base, rel string) (string, bool) {
	if base == "" {
		return "", false
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", false
	}
	// Strip a leading slash so filepath.Join treats the input as relative.
	rel = strings.TrimLeft(rel, "/")
	// Reject backslashes to avoid Windows-specific path traversal tricks
	// like "..\foo" surviving URL-path cleaning.
	if strings.ContainsRune(rel, '\\') {
		return "", false
	}
	joined := filepath.Join(absBase, filepath.FromSlash(rel))
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", false
	}
	// Ensure absJoined is either equal to absBase or a descendant of it.
	// filepath.Rel returns ".." components only if the target escapes.
	rp, err := filepath.Rel(absBase, absJoined)
	if err != nil {
		return "", false
	}
	if rp == ".." || strings.HasPrefix(rp, ".."+string(filepath.Separator)) {
		return "", false
	}
	return absJoined, true
}

func (srv *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	path, ok := safeThemePath(srv.currentTheme, r.URL.Path)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, path)
}

func (srv *WebServer) handleScript(w http.ResponseWriter, r *http.Request) {
	// r.URL.Path is of the form "/script/<relative>"; strip the "/script"
	// prefix and resolve inside the script directory only.
	rel := strings.TrimPrefix(r.URL.Path, "/script")
	path, ok := safeJoin("script", rel)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, path)
}

// startHotReload watches the given theme directory for file changes and
// broadcasts a reload WebSocket message to all clients on each change.
func (srv *WebServer) startHotReload(themePath string) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create file watcher", "err", err)
		return
	}
	if err := w.Add(themePath); err != nil {
		slog.Error("failed to watch theme dir", "err", err, "path", themePath)
		w.Close()
		return
	}

	srv.hotReloadMu.Lock()
	srv.watcher = w
	srv.hotReloadCh = make(chan struct{})
	stopCh := srv.hotReloadCh
	srv.hotReloadMu.Unlock()

	srv.hotReloadWG.Add(1)
	go func() {
		defer srv.hotReloadWG.Done()
		debounce := time.NewTimer(0)
		<-debounce.C // drain initial tick
		debounce.Stop()
		for {
			select {
			case <-stopCh:
				// Shutdown requested — exit cleanly, dropping any pending
				// debounce tick. We deliberately do NOT broadcast a final
				// reload on the way out.
				return
			case event, ok := <-w.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					debounce.Reset(500 * time.Millisecond)
				}
			case _, ok := <-w.Errors:
				if !ok {
					return
				}
			case <-debounce.C:
				// Re-check shutdown flag under the lock so we don't
				// broadcast against a connection set that Stop() is
				// actively draining.
				srv.shutdownMu.RLock()
				down := srv.shuttingDown
				srv.shutdownMu.RUnlock()
				if !down {
					srv.broadcastMessage([]byte(`{"type":"reload"}`))
					slog.Info("theme reload triggered")
				}
			}
		}
	}()
}

func (srv *WebServer) Start() {
	srv.errorChan = make(chan error, 1)
	srv.waitGroup.Add(1)
	go func() {
		err := srv.httpSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			srv.errorChan <- err
		}
		srv.waitGroup.Done()
	}()
	slog.Info("server started", "port", srv.httpSrv.Addr)
	smtcCtx, cancel := context.WithCancel(context.Background())
	srv.smtcCancel = cancel
	srv.waitGroup.Add(1)
	go func() {
		defer srv.waitGroup.Done()
		if err := srv.smtc.Run(smtcCtx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("smtc loop exited", "err", err)
		}
	}()
	if srv.hotReload {
		srv.startHotReload(fmt.Sprintf("themes/%s", srv.currentTheme))
	}
}

func (srv *WebServer) Stop() {
	// Flip the shutdown flag first so in-flight SMTC callbacks can
	// short-circuit before we start tearing down state they depend on.
	srv.shutdownMu.Lock()
	srv.shuttingDown = true
	srv.shutdownMu.Unlock()

	srv.currentMutex.Lock()
	srv.currentInfo = ""
	srv.currentProgress = ""
	srv.currentSourceApp = ""
	srv.currentMutex.Unlock()

	// Stop hot-reload BEFORE closing connections — that way the debounce
	// tick can't race with connection teardown.
	srv.hotReloadMu.Lock()
	stopCh := srv.hotReloadCh
	watcher := srv.watcher
	srv.hotReloadCh = nil
	srv.watcher = nil
	srv.hotReloadMu.Unlock()
	if stopCh != nil {
		close(stopCh)
	}
	if watcher != nil {
		watcher.Close()
	}
	srv.hotReloadWG.Wait()

	// Send close frames, then Close() each connection so ReadLoop exits.
	srv.wsConnectionsMutex.Lock()
	conns := make([]*gws.Conn, 0, len(srv.wsConnections))
	for conn := range srv.wsConnections {
		conns = append(conns, conn)
	}
	srv.wsConnectionsMutex.Unlock()
	for _, conn := range conns {
		_ = conn.WriteClose(1000, nil)
		// NetConn().Close() forces ReadLoop to return with an error.
		if nc := conn.NetConn(); nc != nil {
			_ = nc.Close()
		}
	}

	cancel := srv.smtcCancel
	srv.smtcCancel = nil
	if cancel != nil {
		cancel()
	}
	srv.httpSrv.Close()
	srv.waitGroup.Wait()
	slog.Info("server stopped")
}

func (srv *WebServer) Address() string {
	return srv.httpSrv.Addr
}

func (srv *WebServer) Error() <-chan error {
	return srv.errorChan
}

func (srv *WebServer) SetTheme(theme string) {
	srv.currentTheme = theme
}

// GetSessions returns a copy of the current list of available SMTC sessions.
func (srv *WebServer) GetSessions() []smtc.SessionInfo {
	srv.sessionsMutex.Lock()
	defer srv.sessionsMutex.Unlock()
	if len(srv.sessions) == 0 {
		return nil
	}
	result := make([]smtc.SessionInfo, len(srv.sessions))
	copy(result, srv.sessions)
	return result
}

// SelectDevice selects the SMTC session identified by appID for monitoring.
func (srv *WebServer) SelectDevice(appID string) {
	srv.smtc.SelectDevice(appID)
}

// SetOnSessionsChanged sets the callback invoked when the session list changes.
func (srv *WebServer) SetOnSessionsChanged(callback func()) {
	srv.onSessionsChanged = callback
}

// SetOnSelectedDeviceChange sets the callback invoked when the monitored device changes.
func (srv *WebServer) SetOnSelectedDeviceChange(callback func(string)) {
	srv.onSelectedDeviceChange = callback
}

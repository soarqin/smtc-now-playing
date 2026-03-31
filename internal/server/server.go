package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/lxzan/gws"
	"smtc-now-playing/internal/smtc"
)

type WebServer struct {
	httpSrv *http.Server
	smtc    *smtc.Smtc

	currentTheme string

	currentMutex    sync.Mutex
	currentInfo     string
	currentProgress string

	errorChan           chan error
	waitGroup           sync.WaitGroup
	wsConnections       map[*gws.Conn]struct{}
	wsConnectionsMutex  sync.Mutex
	albumArtContentType string
	albumArtData        []byte

	// Device selection
	sessionsMutex          sync.Mutex
	sessions               []smtc.SessionInfo
	onSessionsChanged      func()
	onSelectedDeviceChange func(string)
}

type infoDetail struct {
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	AlbumArt string `json:"albumArt"`
}

type progressDetail struct {
	Position int `json:"position"`
	Duration int `json:"duration"`
	Status   int `json:"status"`
}

func New(host string, port string, theme string, selectedDevice string) *WebServer {
	mux := http.NewServeMux()
	srv := &WebServer{
		httpSrv: &http.Server{
			Addr:    fmt.Sprintf("%s:%s", host, port),
			Handler: mux,
		},
		currentTheme:  theme,
		wsConnections: make(map[*gws.Conn]struct{}),
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
			if srv.onSelectedDeviceChange != nil {
				srv.onSelectedDeviceChange(appID)
			}
		},
		InitialDevice: selectedDevice,
	})

	mux.HandleFunc("/ws", srv.handleWebSocket)
	mux.HandleFunc("/albumArt/", srv.handleAlbumArt)
	mux.HandleFunc("/script/", srv.handleScript)
	mux.HandleFunc("/", srv.handleStatic)

	return srv
}

func (srv *WebServer) handleInfoUpdate(data smtc.InfoData) {
	var info infoDetail
	info.Artist = data.Artist
	info.Title = data.Title
	srv.currentMutex.Lock()
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
	if err == nil {
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
}

func (srv *WebServer) handleProgressUpdate(data smtc.ProgressData) {
	progress := progressDetail{
		Position: data.Position,
		Duration: data.Duration,
		Status:   data.Status,
	}
	j, err := json.Marshal(&struct {
		Type string          `json:"type"`
		Data *progressDetail `json:"data"`
	}{Type: "progress", Data: &progress})
	if err == nil {
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

func (srv *WebServer) broadcastMessage(data []byte) {
	srv.wsConnectionsMutex.Lock()
	defer srv.wsConnectionsMutex.Unlock()
	for conn := range srv.wsConnections {
		conn.WriteMessage(gws.OpcodeText, data)
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
	go conn.ReadLoop()
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
	w.Header().Set("Content-Type", ct)
	w.Write(data)
}

func (srv *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, fmt.Sprintf("themes/%s/%s", srv.currentTheme, r.URL.Path[1:]))
}

func (srv *WebServer) handleScript(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
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
	srv.smtc.Start()
}

func (srv *WebServer) Stop() {
	srv.currentMutex.Lock()
	srv.currentInfo = ""
	srv.currentProgress = ""
	srv.currentMutex.Unlock()
	srv.wsConnectionsMutex.Lock()
	for conn := range srv.wsConnections {
		conn.WriteClose(1000, nil)
	}
	srv.wsConnectionsMutex.Unlock()
	srv.smtc.Stop()
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

package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

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
	quitChan            chan struct{}
	waitGroup           sync.WaitGroup
	wsConnections       map[*gws.Conn]struct{}
	wsConnectionsMutex  sync.Mutex
	albumArtContentType string
	albumArtData        []byte
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

func New(host string, port string, theme string) *WebServer {
	smtc := smtc.New()
	if smtc.Init() != 0 {
		return nil
	}

	mux := http.NewServeMux()
	srv := &WebServer{
		httpSrv: &http.Server{
			Addr:    fmt.Sprintf("%s:%s", host, port),
			Handler: mux,
		},
		smtc: smtc,

		currentTheme: theme,

		wsConnections: make(map[*gws.Conn]struct{}),
	}

	mux.HandleFunc("/ws", srv.handleWebSocket)
	mux.HandleFunc("/albumArt/", srv.handleAlbumArt)
	mux.HandleFunc("/script/", srv.handleScript)
	mux.HandleFunc("/", srv.handleStatic)

	return srv
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
	if len(srv.albumArtData) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", srv.albumArtContentType)
	w.Write(srv.albumArtData)
}

func (srv *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, fmt.Sprintf("themes/%s/%s", srv.currentTheme, r.URL.Path[1:]))
}

func (srv *WebServer) handleScript(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, r.URL.Path[1:])
}

func (srv *WebServer) Start() {
	srv.errorChan = make(chan error, 1)
	srv.quitChan = make(chan struct{}, 1)
	srv.waitGroup.Add(2)
	go func() {
		err := srv.httpSrv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			srv.errorChan <- err
		}
		srv.waitGroup.Done()
	}()
	go func() {
		var info infoDetail
		var progress progressDetail

		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-srv.quitChan:
				srv.waitGroup.Done()
				return
			case <-ticker.C:
				srv.smtc.Update()
				dirty := srv.smtc.RetrieveDirtyData(&info.Artist, &info.Title, &srv.albumArtContentType, &srv.albumArtData, &progress.Position, &progress.Duration, &progress.Status)
				if dirty&1 != 0 {
					if len(srv.albumArtData) > 0 {
						checksum := sha256.Sum256(srv.albumArtData)
						info.AlbumArt = "/albumArt/" + hex.EncodeToString(checksum[:])
					} else {
						info.AlbumArt = ""
					}
					j, err := json.Marshal(&struct {
						Type string      `json:"type"`
						Data *infoDetail `json:"data"`
					}{
						Type: "info",
						Data: &info,
					})
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
				if dirty&2 != 0 {
					j, err := json.Marshal(&struct {
						Type string          `json:"type"`
						Data *progressDetail `json:"data"`
					}{
						Type: "progress",
						Data: &progress,
					})
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
			}
		}
	}()
}

func (srv *WebServer) Stop() {
	srv.currentInfo = ""
	srv.currentProgress = ""
	srv.wsConnectionsMutex.Lock()
	for conn := range srv.wsConnections {
		conn.WriteClose(1000, nil)
	}
	srv.wsConnectionsMutex.Unlock()
	close(srv.quitChan)
	srv.httpSrv.Close()
	srv.waitGroup.Wait()
	srv.smtc.Destroy()
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

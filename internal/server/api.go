package server

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/smtc"
)

// isLocalhost checks if the request comes from localhost.
func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// handleNowPlaying returns current media info and progress as JSON.
// Returns 404 if no active session exists.
func (srv *WebServer) handleNowPlaying(w http.ResponseWriter, r *http.Request) {
	srv.currentMutex.Lock()
	infoStr := srv.currentInfo
	progressStr := srv.currentProgress
	srv.currentMutex.Unlock()

	if infoStr == "" {
		http.Error(w, "no active session", http.StatusNotFound)
		return
	}

	var infoMsg struct {
		Data infoDetail `json:"data"`
	}
	var progressMsg struct {
		Data progressDetail `json:"data"`
	}

	if err := json.Unmarshal([]byte(infoStr), &infoMsg); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if progressStr != "" {
		_ = json.Unmarshal([]byte(progressStr), &progressMsg)
	}

	response := struct {
		Info     infoDetail     `json:"info"`
		Progress progressDetail `json:"progress"`
	}{
		Info:     infoMsg.Data,
		Progress: progressMsg.Data,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleDevices returns a JSON array of available SMTC sessions.
// Returns an empty array when no sessions are available.
func (srv *WebServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	srv.sessionsMutex.Lock()
	sessions := srv.sessions
	srv.sessionsMutex.Unlock()

	if sessions == nil {
		sessions = []smtc.SessionInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sessions)
}

// handleCapabilities returns the current session's control capabilities as JSON.
// Uses a 50ms timeout so the handler is non-blocking when no SMTC session is active
// (e.g. in tests or before the first session is established).
func (srv *WebServer) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	capsChan := make(chan smtc.ControlCapabilities, 1)
	go func() { capsChan <- srv.smtc.GetCapabilities() }()

	var caps smtc.ControlCapabilities
	select {
	case caps = <-capsChan:
	case <-time.After(50 * time.Millisecond):
		// No active session or SMTC goroutine not running; return zero capabilities.
	}

	_ = json.NewEncoder(w).Encode(caps)
}

// handleControl checks the localhost guard and executes action, returning a JSON
// success/error response. All media control endpoints delegate to this helper.
func (srv *WebServer) handleControl(w http.ResponseWriter, r *http.Request, action func() error) {
	if !config.Get().ControlAllowRemote && !isLocalhost(r) {
		http.Error(w, `{"success":false,"error":"control requires localhost"}`, http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := action(); err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// handleControlPlay handles POST /api/control/play.
func (srv *WebServer) handleControlPlay(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.Play)
}

// handleControlPause handles POST /api/control/pause.
func (srv *WebServer) handleControlPause(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.Pause)
}

// handleControlStop handles POST /api/control/stop.
func (srv *WebServer) handleControlStop(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.StopPlayback)
}

// handleControlToggle handles POST /api/control/toggle (toggle play/pause).
func (srv *WebServer) handleControlToggle(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.TogglePlayPause)
}

// handleControlNext handles POST /api/control/next.
func (srv *WebServer) handleControlNext(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.SkipNext)
}

// handleControlPrevious handles POST /api/control/previous.
func (srv *WebServer) handleControlPrevious(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.SkipPrevious)
}

// handleControlSeek handles POST /api/control/seek.
// Body: {"position": 12345} where position is the target position in milliseconds.
func (srv *WebServer) handleControlSeek(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Position int64 `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	srv.handleControl(w, r, func() error { return srv.smtc.Seek(body.Position) })
}

// handleControlShuffle handles POST /api/control/shuffle.
// Body: {"active": true} to enable shuffle, {"active": false} to disable.
func (srv *WebServer) handleControlShuffle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	srv.handleControl(w, r, func() error { return srv.smtc.SetShuffle(body.Active) })
}

// handleControlRepeat handles POST /api/control/repeat.
// Body: {"mode": 0} for None, {"mode": 1} for Track, {"mode": 2} for List.
func (srv *WebServer) handleControlRepeat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode int `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	srv.handleControl(w, r, func() error { return srv.smtc.SetRepeat(body.Mode) })
}

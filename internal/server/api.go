package server

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/smtc"
)

// isLocalhost reports whether r originated from a loopback address.
// Uses net.IP.IsLoopback() so every loopback form is recognised —
// 127.0.0.0/8, ::1, and IPv6 addresses with zone identifiers such as
// "fe80::1%eth0" or "[::1]:12345". Falls back to a raw-host compare
// only if SplitHostPort fails AND the raw RemoteAddr isn't parseable
// (some tests use synthetic RemoteAddr values like "pipe").
func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	// Strip any IPv6 zone suffix ("%zone") before parsing so
	// ParseIP accepts link-local loopback forms too.
	if i := indexByte(host, '%'); i >= 0 {
		host = host[:i]
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	// Last-resort literal compare for odd RemoteAddr values.
	return host == "localhost"
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// writeJSON writes v as JSON to w, logging any encode error. Intended for
// all API responses so we have one place to fix MIME / error handling.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 && status != http.StatusOK {
		w.WriteHeader(status)
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("api: failed to encode response", "err", err)
	}
}

// handleNowPlaying returns current media info and progress as JSON.
// Returns 404 (as JSON) when no active session exists.
func (srv *WebServer) handleNowPlaying(w http.ResponseWriter, r *http.Request) {
	srv.currentMutex.Lock()
	infoStr := srv.currentInfo
	progressStr := srv.currentProgress
	srv.currentMutex.Unlock()

	if infoStr == "" {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"success": false,
			"error":   "no active session",
		})
		return
	}

	var infoMsg struct {
		Data infoDetail `json:"data"`
	}
	var progressMsg struct {
		Data progressDetail `json:"data"`
	}

	if err := json.Unmarshal([]byte(infoStr), &infoMsg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"success": false,
			"error":   "internal error",
		})
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

	writeJSON(w, http.StatusOK, response)
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

	writeJSON(w, http.StatusOK, sessions)
}

// handleCapabilities returns the current session's control capabilities as JSON.
// Uses a 50ms timeout so the handler is non-blocking when no SMTC session is active
// (e.g. in tests or before the first session is established).
func (srv *WebServer) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	capsChan := make(chan smtc.ControlCapabilities, 1)
	go func() { capsChan <- srv.smtc.GetCapabilities() }()

	var caps smtc.ControlCapabilities
	select {
	case caps = <-capsChan:
	case <-time.After(50 * time.Millisecond):
		// No active session or SMTC goroutine not running; return zero capabilities.
	}

	writeJSON(w, http.StatusOK, caps)
}

// handleControl checks the localhost guard and executes action, returning a JSON
// success/error response. All media control endpoints delegate to this helper.
func (srv *WebServer) handleControl(w http.ResponseWriter, r *http.Request, action func() error) {
	if !config.Get().ControlAllowRemote && !isLocalhost(r) {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"success": false,
			"error":   "control requires localhost",
		})
		return
	}

	if err := action(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (srv *WebServer) handleControlPlay(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.Play)
}

func (srv *WebServer) handleControlPause(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.Pause)
}

func (srv *WebServer) handleControlStop(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.StopPlayback)
}

func (srv *WebServer) handleControlToggle(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.TogglePlayPause)
}

func (srv *WebServer) handleControlNext(w http.ResponseWriter, r *http.Request) {
	srv.handleControl(w, r, srv.smtc.SkipNext)
}

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
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	srv.handleControl(w, r, func() error { return srv.smtc.SeekTo(body.Position) })
}

// handleControlShuffle handles POST /api/control/shuffle.
// Body: {"active": true} to enable shuffle, {"active": false} to disable.
func (srv *WebServer) handleControlShuffle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
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
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
		return
	}
	srv.handleControl(w, r, func() error { return srv.smtc.SetRepeat(body.Mode) })
}

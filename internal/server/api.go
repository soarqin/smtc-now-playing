package server

import (
	"encoding/json"
	"net/http"

	"smtc-now-playing/internal/smtc"
)

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

// handleCapabilities returns server capabilities as a JSON object.
// ControlCapabilities will be extended in a future task (T12).
func (srv *WebServer) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct{}{})
}

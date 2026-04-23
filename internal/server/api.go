package server

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"

	"smtc-now-playing/internal/smtc"
	"smtc-now-playing/internal/wsproto"
)

func isLocalhost(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if i := indexByte(host, '%'); i >= 0 {
		host = host[:i]
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 && status != http.StatusOK {
		w.WriteHeader(status)
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("api: failed to encode response", "err", err)
	}
}

func (s *Server) handleNowPlaying(w http.ResponseWriter, r *http.Request) {
	state := s.snapshot()
	if state.info == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "no active session"})
		return
	}

	response := struct {
		Info     wsproto.InfoPayload     `json:"info"`
		Progress wsproto.ProgressPayload `json:"progress"`
	}{
		Info: wsproto.InfoPayload{
			Artist:       state.info.Artist,
			Title:        state.info.Title,
			AlbumTitle:   state.info.AlbumTitle,
			AlbumArtist:  state.info.AlbumArtist,
			PlaybackType: state.info.PlaybackType,
			SourceApp:    state.info.SourceApp,
			AlbumArt:     s.albumArtURL(state.albumArtHash),
		},
	}
	if state.progress != nil {
		response.Progress = wsproto.ProgressPayload{
			Position:        state.progress.Position,
			Duration:        state.progress.Duration,
			Status:          state.progress.Status,
			PlaybackRate:    state.progress.PlaybackRate,
			IsShuffleActive: state.progress.IsShuffleActive,
			AutoRepeatMode:  state.progress.AutoRepeatMode,
			LastUpdatedTime: state.progress.LastUpdatedTime,
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.svc.GetSessions()
	if sessions == nil {
		sessions = []smtc.SessionInfo{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.GetCapabilities())
}

func (s *Server) handleControl(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")

	var err error
	switch action {
	case "play":
		err = s.svc.Play()
	case "pause":
		err = s.svc.Pause()
	case "stop":
		err = s.svc.StopPlayback()
	case "toggle":
		err = s.svc.TogglePlayPause()
	case "next":
		err = s.svc.SkipNext()
	case "previous":
		err = s.svc.SkipPrevious()
	case "seek":
		var body struct {
			Position int64 `json:"position"`
		}
		if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
			return
		}
		err = s.svc.SeekTo(body.Position)
	case "shuffle":
		var body struct {
			Active bool `json:"active"`
		}
		if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
			return
		}
		err = s.svc.SetShuffle(body.Active)
	case "repeat":
		var body struct {
			Mode int `json:"mode"`
		}
		if decodeErr := json.NewDecoder(r.Body).Decode(&body); decodeErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
			return
		}
		err = s.svc.SetRepeat(body.Mode)
	default:
		writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": "unknown action"})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

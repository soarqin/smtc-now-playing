package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/smtc"
)

// TestHandleNowPlaying_NoSession_404 verifies that /api/now-playing returns
// 404 when no active session exists (currentInfo is empty).
func TestHandleNowPlaying_NoSession_404(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/now-playing", nil)
	w := httptest.NewRecorder()

	srv.handleNowPlaying(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("handleNowPlaying with no session: got HTTP %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestHandleNowPlaying_WithData_200 verifies that /api/now-playing returns 200
// with info and progress JSON fields when media info is set.
func TestHandleNowPlaying_WithData_200(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	// Inject info in the same wire format as handleInfoUpdate produces.
	info := infoDetail{Title: "Test Track", Artist: "Test Artist"}
	j, err := json.Marshal(struct {
		Type string      `json:"type"`
		Data *infoDetail `json:"data"`
	}{Type: "info", Data: &info})
	if err != nil {
		t.Fatalf("failed to marshal info: %v", err)
	}
	srv.currentMutex.Lock()
	srv.currentInfo = string(j)
	srv.currentMutex.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/now-playing", nil)
	w := httptest.NewRecorder()

	srv.handleNowPlaying(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleNowPlaying with data: got HTTP %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want \"application/json\"", ct)
	}

	var result struct {
		Info struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
		} `json:"info"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result.Info.Title != "Test Track" {
		t.Errorf("info.title: got %q, want \"Test Track\"", result.Info.Title)
	}
	if result.Info.Artist != "Test Artist" {
		t.Errorf("info.artist: got %q, want \"Test Artist\"", result.Info.Artist)
	}
}

// TestHandleDevices_Empty_ReturnsArray verifies that /api/devices returns a
// JSON array (not null) even when no sessions are available.
func TestHandleDevices_Empty_ReturnsArray(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()

	srv.handleDevices(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleDevices empty: got HTTP %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want \"application/json\"", ct)
	}

	var result []smtc.SessionInfo
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result == nil {
		t.Error("expected empty JSON array, got null/nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// TestHandleDevices_WithSessions_200 verifies that /api/devices returns the
// full session list when sessions are available.
func TestHandleDevices_WithSessions_200(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.sessionsMutex.Lock()
	srv.sessions = []smtc.SessionInfo{
		{AppID: "com.example.player", Name: "Example Player", SourceAppID: "example"},
	}
	srv.sessionsMutex.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()

	srv.handleDevices(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleDevices with sessions: got HTTP %d, want %d", w.Code, http.StatusOK)
	}

	var result []smtc.SessionInfo
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 session, got %d", len(result))
	}
	if result[0].AppID != "com.example.player" {
		t.Errorf("AppID: got %q, want \"com.example.player\"", result[0].AppID)
	}
	if result[0].Name != "Example Player" {
		t.Errorf("Name: got %q, want \"Example Player\"", result[0].Name)
	}
}

// TestHandleCapabilities_200 verifies that /api/capabilities returns 200 with
// a valid JSON object body.
func TestHandleCapabilities_200(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	w := httptest.NewRecorder()

	srv.handleCapabilities(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleCapabilities: got HTTP %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want \"application/json\"", ct)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("response body is not a valid JSON object: %v", err)
	}
}

// TestHandleControlPlay_LocalhostAllowed verifies that a localhost request
// is allowed through the localhost guard and returns HTTP 200 with success:true.
// handleControl is tested directly with a stub action to avoid deadlock from
// smtc.Play requiring a running SMTC event-loop goroutine (not started in tests).
func TestHandleControlPlay_LocalhostAllowed(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()

	// Stub action returns nil (success) — isolates the localhost-guard logic.
	srv.handleControl(w, req, func() error { return nil })

	if w.Code != http.StatusOK {
		t.Errorf("handleControl localhost: got HTTP %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want \"application/json\"", ct)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if result["success"] != true {
		t.Errorf("success: got %v, want true", result["success"])
	}
}

// TestHandleControlPlay_RemoteForbidden verifies that a non-localhost request
// is rejected with HTTP 403 when controlAllowRemote is false (the default).
// The localhost guard fires before smtc.Play is called, so no deadlock occurs.
func TestHandleControlPlay_RemoteForbidden(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	// Guard fires before smtc.Play — no deadlock.
	srv.handleControlPlay(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("handleControlPlay remote: got HTTP %d, want %d", w.Code, http.StatusForbidden)
	}
}

// TestHandleCapabilities_ReturnsShape verifies that /api/capabilities returns
// a JSON object containing all expected ControlCapabilities fields.
// handleCapabilities uses a 50ms timeout; in tests (no SMTC session) it returns
// zero-value capabilities after the timeout.
func TestHandleCapabilities_ReturnsShape(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	w := httptest.NewRecorder()

	srv.handleCapabilities(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleCapabilities: got HTTP %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("response not valid JSON object: %v", err)
	}

	expectedFields := []string{
		"isPlayEnabled", "isPauseEnabled", "isStopEnabled",
		"isNextEnabled", "isPreviousEnabled", "isSeekEnabled",
		"isShuffleEnabled", "isRepeatEnabled",
	}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Errorf("capabilities response missing field %q", field)
		}
	}
}

// TestHandleControlSeek_ValidBody verifies that handleControlSeek returns HTTP 200
// (body parsing succeeded) for a valid JSON body. The smtc call will return an
// error (no session), but that results in 200 with success:false — NOT 400.
func TestHandleControlSeek_ValidBody(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)
	// Start the smtc goroutine so sendControl can complete with ErrNoSession.
	_ = srv.smtc.Start()
	defer srv.smtc.Stop()

	req := httptest.NewRequest(http.MethodPost, "/api/control/seek",
		strings.NewReader(`{"position": 5000}`))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	srv.handleControlSeek(w, req)

	if w.Code == http.StatusBadRequest {
		t.Errorf("handleControlSeek valid body: got 400, want 200 (body parsed ok, smtc failure is 200+error)")
	}
	if w.Code != http.StatusOK {
		t.Errorf("handleControlSeek valid body: got HTTP %d, want 200", w.Code)
	}
	// smtc returns ErrNoSession — body must be success:false with an error message.
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if result["success"] != false {
		t.Errorf("success: got %v, want false (smtc has no session)", result["success"])
	}
	if result["error"] == "" || result["error"] == nil {
		t.Errorf("error field: got %v, want non-empty error message", result["error"])
	}
}

// TestHandleControlSeek_InvalidBody verifies that handleControlSeek returns
// HTTP 400 with success:false and error message for invalid JSON body.
func TestHandleControlSeek_InvalidBody(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodPost, "/api/control/seek",
		strings.NewReader(`not json`))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	srv.handleControlSeek(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleControlSeek invalid body: got HTTP %d, want 400", w.Code)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if result["success"] != false {
		t.Errorf("success: got %v, want false", result["success"])
	}
}

// TestHandleControlShuffle_ValidBody verifies that handleControlShuffle returns
// HTTP 200 for a valid JSON body.
func TestHandleControlShuffle_ValidBody(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)
	// Start the smtc goroutine so sendControl can complete with ErrNoSession.
	_ = srv.smtc.Start()
	defer srv.smtc.Stop()

	req := httptest.NewRequest(http.MethodPost, "/api/control/shuffle",
		strings.NewReader(`{"active": true}`))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	srv.handleControlShuffle(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleControlShuffle valid body: got HTTP %d, want 200", w.Code)
	}
}

// TestHandleControlShuffle_InvalidBody verifies that handleControlShuffle returns
// HTTP 400 for an invalid JSON body.
func TestHandleControlShuffle_InvalidBody(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodPost, "/api/control/shuffle",
		strings.NewReader(`bad`))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	srv.handleControlShuffle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleControlShuffle invalid body: got HTTP %d, want 400", w.Code)
	}
}

// TestHandleControlRepeat_ValidBody verifies that handleControlRepeat returns
// HTTP 200 for a valid JSON body.
func TestHandleControlRepeat_ValidBody(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)
	// Start the smtc goroutine so sendControl can complete with ErrNoSession.
	_ = srv.smtc.Start()
	defer srv.smtc.Stop()

	req := httptest.NewRequest(http.MethodPost, "/api/control/repeat",
		strings.NewReader(`{"mode": 1}`))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	srv.handleControlRepeat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleControlRepeat valid body: got HTTP %d, want 200", w.Code)
	}
}

// TestHandleControlRepeat_InvalidBody verifies that handleControlRepeat returns
// HTTP 400 for an invalid JSON body.
func TestHandleControlRepeat_InvalidBody(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodPost, "/api/control/repeat",
		strings.NewReader(`bad`))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()

	srv.handleControlRepeat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("handleControlRepeat invalid body: got HTTP %d, want 400", w.Code)
	}
}

// TestHandleControl_AllowRemote verifies that when ControlAllowRemote is true,
// requests from non-localhost are accepted (HTTP 200, not 403).
func TestHandleControl_AllowRemote(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	// Temporarily enable remote control.
	saved := config.Get().ControlAllowRemote
	config.Get().ControlAllowRemote = true
	defer func() { config.Get().ControlAllowRemote = saved }()

	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.RemoteAddr = "192.168.1.100:1234"
	w := httptest.NewRecorder()

	// Use stub action to isolate the guard logic from smtc internals.
	srv.handleControl(w, req, func() error { return nil })

	if w.Code != http.StatusOK {
		t.Errorf("handleControl AllowRemote: got HTTP %d, want 200", w.Code)
	}
}

// TestWriteJSON_ContentType verifies that writeJSON sets Content-Type to
// application/json.
func TestWriteJSON_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "val"})

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q, want \"application/json\"", ct)
	}
}

// TestWriteJSON_NonOKStatus verifies that writeJSON writes the supplied
// non-200 status code to the response.
func TestWriteJSON_NonOKStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want 404", w.Code)
	}
}

// TestWriteJSON_OKStatus verifies that writeJSON with status 200 produces
// a 200 response (ResponseRecorder default, not double-written).
func TestWriteJSON_OKStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", w.Code)
	}
}

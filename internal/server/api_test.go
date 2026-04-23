package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smtc-now-playing/internal/domain"
	"smtc-now-playing/internal/smtc"
)

func TestHandleNowPlaying_NoSession_404(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/now-playing", nil)
	w := httptest.NewRecorder()
	srv.handleNowPlaying(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleNowPlaying_WithData_200(t *testing.T) {
	srv, _, _ := newTestServer(t)
	srv.handleInfoEvent(domain.InfoData{Title: "Test Track", Artist: "Test Artist"})
	req := httptest.NewRequest(http.MethodGet, "/api/now-playing", nil)
	w := httptest.NewRecorder()
	srv.handleNowPlaying(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var result struct {
		Info struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
		} `json:"info"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Info.Title != "Test Track" || result.Info.Artist != "Test Artist" {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestHandleDevices_Empty_ReturnsArray(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()
	srv.handleSessions(w, req)
	var result []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result == nil || len(result) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHandleDevices_WithSessions_200(t *testing.T) {
	srv, svc, _ := newTestServer(t)
	svc.sessions = []smtc.SessionInfo{{AppID: "com.example.player", Name: "Example Player", SourceAppID: "example"}}
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	w := httptest.NewRecorder()
	srv.handleSessions(w, req)
	var result []smtc.SessionInfo
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].AppID != "com.example.player" || result[0].Name != "Example Player" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestHandleCapabilities_200(t *testing.T) {
	srv, svc, _ := newTestServer(t)
	svc.capabilities = smtc.ControlCapabilities{IsPlayEnabled: true}
	req := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	w := httptest.NewRecorder()
	srv.handleCapabilities(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if _, ok := result["isPlayEnabled"]; !ok {
		t.Fatalf("missing isPlayEnabled field: %+v", result)
	}
}

func TestHandleControlPlay_LocalhostAllowed(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.SetPathValue("action", "play")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["success"] != true {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestHandleControlPlay_RemoteForbidden(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	req.SetPathValue("action", "play")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleCapabilities_ReturnsShape(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/capabilities", nil)
	w := httptest.NewRecorder()
	srv.handleCapabilities(w, req)
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"isPlayEnabled", "isPauseEnabled", "isStopEnabled", "isNextEnabled", "isPreviousEnabled", "isSeekEnabled", "isShuffleEnabled", "isRepeatEnabled"} {
		if _, ok := result[field]; !ok {
			t.Fatalf("missing %q in %+v", field, result)
		}
	}
}

func TestHandleControlSeek_ValidBody(t *testing.T) {
	srv, svc, _ := newTestServer(t)
	svc.seekErr = smtc.ErrNoSession
	req := httptest.NewRequest(http.MethodPost, "/api/control/seek", strings.NewReader(`{"position": 5000}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.SetPathValue("action", "seek")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
	if len(svc.seekCalls) != 1 || svc.seekCalls[0] != 5000 {
		t.Fatalf("seekCalls = %v, want [5000]", svc.seekCalls)
	}
}

func TestHandleControlSeek_InvalidBody(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/seek", strings.NewReader(`not json`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.SetPathValue("action", "seek")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleControlShuffle_ValidBody(t *testing.T) {
	srv, svc, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/shuffle", strings.NewReader(`{"active": true}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.SetPathValue("action", "shuffle")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
	if len(svc.shuffleCalls) != 1 || !svc.shuffleCalls[0] {
		t.Fatalf("shuffleCalls = %v, want [true]", svc.shuffleCalls)
	}
}

func TestHandleControlShuffle_InvalidBody(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/shuffle", strings.NewReader(`bad`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.SetPathValue("action", "shuffle")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleControlRepeat_ValidBody(t *testing.T) {
	srv, svc, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/repeat", strings.NewReader(`{"mode": 1}`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.SetPathValue("action", "repeat")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
	if len(svc.repeatCalls) != 1 || svc.repeatCalls[0] != 1 {
		t.Fatalf("repeatCalls = %v, want [1]", svc.repeatCalls)
	}
}

func TestHandleControlRepeat_InvalidBody(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/repeat", strings.NewReader(`bad`))
	req.RemoteAddr = "127.0.0.1:1234"
	req.SetPathValue("action", "repeat")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleControl_AllowRemote(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.RemoteAddr = "192.168.1.100:1234"
	req.SetPathValue("action", "play")
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, true)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
}

func TestWriteJSON_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "val"})
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q", ct)
	}
}

func TestWriteJSON_NonOKStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", w.Code, http.StatusNotFound)
	}
}

func TestWriteJSON_OKStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
}

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smtc-now-playing/internal/config"
	"smtc-now-playing/internal/domain"
	"smtc-now-playing/internal/smtc"
	"smtc-now-playing/internal/wsproto"
)

func boolPtr(b bool) *bool { return &b }

type mockSMTCService struct {
	events             chan smtc.Event
	sessions           []smtc.SessionInfo
	capabilities       smtc.ControlCapabilities
	playErr            error
	pauseErr           error
	stopErr            error
	toggleErr          error
	nextErr            error
	previousErr        error
	seekErr            error
	shuffleErr         error
	repeatErr          error
	selectedDevice     string
	seekPosition       int64
	shuffleActive      bool
	repeatMode         int
	unsubscribedCalled bool
}

func newTestServer(t *testing.T) (*Server, *mockSMTCService) {
	t.Helper()
	svc := &mockSMTCService{events: make(chan smtc.Event, 16)}
	srv, err := New(&config.Config{Server: config.ServerConfig{Port: 11451}, UI: config.UIConfig{Theme: "default"}}, svc)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return srv, svc
}

func (m *mockSMTCService) Subscribe(int) <-chan smtc.Event { return m.events }
func (m *mockSMTCService) Unsubscribe(<-chan smtc.Event)   { m.unsubscribedCalled = true }
func (m *mockSMTCService) GetSessions() []smtc.SessionInfo {
	if len(m.sessions) == 0 {
		return nil
	}
	out := make([]smtc.SessionInfo, len(m.sessions))
	copy(out, m.sessions)
	return out
}
func (m *mockSMTCService) SelectDevice(appID string)                 { m.selectedDevice = appID }
func (m *mockSMTCService) GetCapabilities() smtc.ControlCapabilities { return m.capabilities }
func (m *mockSMTCService) Play() error                               { return m.playErr }
func (m *mockSMTCService) Pause() error                              { return m.pauseErr }
func (m *mockSMTCService) StopPlayback() error                       { return m.stopErr }
func (m *mockSMTCService) TogglePlayPause() error                    { return m.toggleErr }
func (m *mockSMTCService) SkipNext() error                           { return m.nextErr }
func (m *mockSMTCService) SkipPrevious() error                       { return m.previousErr }
func (m *mockSMTCService) SeekTo(positionMs int64) error {
	m.seekPosition = positionMs
	return m.seekErr
}
func (m *mockSMTCService) SetShuffle(active bool) error {
	m.shuffleActive = active
	return m.shuffleErr
}
func (m *mockSMTCService) SetRepeat(mode int) error {
	m.repeatMode = mode
	return m.repeatErr
}

func TestNew_NoPanic(t *testing.T) {
	srv, _ := newTestServer(t)
	if srv == nil {
		t.Fatal("New returned nil")
	}
}

func TestHandleAlbumArt_NoData_Returns404(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/albumArt/abc123", nil)
	req.SetPathValue("hash", "abc123")
	w := httptest.NewRecorder()
	srv.handleAlbumArt(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("got %d want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleAlbumArt_WithData_ReturnsContent(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleInfoEvent(domain.InfoData{Title: "Song", ThumbnailContentType: "image/jpeg", ThumbnailData: []byte{0xFF, 0xD8, 0xFF}})
	state := srv.snapshot()
	req := httptest.NewRequest(http.MethodGet, "/albumArt/"+state.albumArtHash, nil)
	req.SetPathValue("hash", state.albumArtHash)
	w := httptest.NewRecorder()
	srv.handleAlbumArt(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestGetSessions_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	if got := srv.GetSessions(); got != nil {
		t.Fatalf("GetSessions() = %#v, want nil", got)
	}
}

func TestAddress(t *testing.T) {
	svc := &mockSMTCService{events: make(chan smtc.Event, 1)}
	srv, err := New(&config.Config{Server: config.ServerConfig{Port: 9999}, UI: config.UIConfig{Theme: "default"}}, svc)
	if err != nil {
		t.Fatal(err)
	}
	if got := srv.Address(); got != ":9999" {
		t.Fatalf("Address() = %q", got)
	}
}

func TestHandleInfoEvent_MessageFormatting(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleInfoEvent(domain.InfoData{Artist: "TestArtist", Title: "TestTitle"})
	state := srv.snapshot()
	if len(state.infoJSON) == 0 {
		t.Fatal("infoJSON is empty")
	}
	var msg struct {
		Type string              `json:"type"`
		Data wsproto.InfoPayload `json:"data"`
	}
	if err := json.Unmarshal(state.infoJSON, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if msg.Type != string(wsproto.MsgInfo) || msg.Data.Artist != "TestArtist" || msg.Data.Title != "TestTitle" {
		t.Fatalf("unexpected payload: %+v", msg)
	}
}

func TestHandleInfoEvent_Deduplication(t *testing.T) {
	srv, _ := newTestServer(t)
	data := domain.InfoData{Artist: "Artist", Title: "Song"}
	srv.handleInfoEvent(data)
	first := string(srv.snapshot().infoJSON)
	srv.handleInfoEvent(data)
	second := string(srv.snapshot().infoJSON)
	if first != second {
		t.Fatal("info JSON changed for identical update")
	}
}

func TestHandleInfoEvent_AlbumArtHash(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleInfoEvent(domain.InfoData{Artist: "Artist", Title: "Title", ThumbnailContentType: "image/jpeg", ThumbnailData: []byte{0xFF, 0xD8}})
	state := srv.snapshot()
	var msg struct {
		Data wsproto.InfoPayload `json:"data"`
	}
	if err := json.Unmarshal(state.infoJSON, &msg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(msg.Data.AlbumArt, "/albumArt/") {
		t.Fatalf("albumArt = %q", msg.Data.AlbumArt)
	}
	if len(strings.TrimPrefix(msg.Data.AlbumArt, "/albumArt/")) != 64 {
		t.Fatalf("unexpected album art hash: %q", msg.Data.AlbumArt)
	}
}

func TestHandleInfoEvent_NoThumbnail(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleInfoEvent(domain.InfoData{Artist: "Artist", Title: "Title"})
	var msg struct {
		Data wsproto.InfoPayload `json:"data"`
	}
	if err := json.Unmarshal(srv.snapshot().infoJSON, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Data.AlbumArt != "" {
		t.Fatalf("albumArt = %q", msg.Data.AlbumArt)
	}
}

func TestHandleProgressEvent_MessageFormatting(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleProgressEvent(domain.ProgressData{Position: 60, Duration: 180, Status: 4, PlaybackRate: 1.0, LastUpdatedTime: 1700000000000})
	var msg struct {
		Type string                  `json:"type"`
		Data wsproto.ProgressPayload `json:"data"`
	}
	if err := json.Unmarshal(srv.snapshot().progressJSON, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != string(wsproto.MsgProgress) || msg.Data.Position != 60 || msg.Data.Duration != 180 || msg.Data.Status != 4 {
		t.Fatalf("unexpected payload: %+v", msg)
	}
}

func TestHandleProgressEvent_Deduplication(t *testing.T) {
	srv, _ := newTestServer(t)
	data := domain.ProgressData{Position: 10, Duration: 200, Status: 4}
	srv.handleProgressEvent(data)
	first := string(srv.snapshot().progressJSON)
	srv.handleProgressEvent(data)
	second := string(srv.snapshot().progressJSON)
	if first != second {
		t.Fatal("progress JSON changed for identical update")
	}
}

func TestHandleProgressEvent_IsShuffleActiveNil(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleProgressEvent(domain.ProgressData{IsShuffleActive: nil})
	var msg struct {
		Data wsproto.ProgressPayload `json:"data"`
	}
	if err := json.Unmarshal(srv.snapshot().progressJSON, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Data.IsShuffleActive != nil {
		t.Fatal("expected nil IsShuffleActive")
	}
}

func TestHandleProgressEvent_IsShuffleActiveTrue(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleProgressEvent(domain.ProgressData{IsShuffleActive: boolPtr(true)})
	var msg struct {
		Data wsproto.ProgressPayload `json:"data"`
	}
	if err := json.Unmarshal(srv.snapshot().progressJSON, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Data.IsShuffleActive == nil || !*msg.Data.IsShuffleActive {
		t.Fatal("expected true IsShuffleActive")
	}
}

func TestGetSessions_CopySemantics(t *testing.T) {
	srv, svc := newTestServer(t)
	svc.sessions = []smtc.SessionInfo{{AppID: "original.exe", Name: "Original"}}
	first := srv.GetSessions()
	first[0].Name = "Mutated"
	second := srv.GetSessions()
	if second[0].Name != "Original" {
		t.Fatalf("GetSessions copy semantics broken: %+v", second[0])
	}
}

func TestHandleAlbumArt_EmptyContentType(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.handleInfoEvent(domain.InfoData{Title: "Song", ThumbnailData: []byte{0x01, 0x02, 0x03}})
	state := srv.snapshot()
	req := httptest.NewRequest(http.MethodGet, "/albumArt/"+state.albumArtHash, nil)
	req.SetPathValue("hash", state.albumArtHash)
	w := httptest.NewRecorder()
	srv.handleAlbumArt(w, req)
	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestControlMethodsDelegate(t *testing.T) {
	srv, svc := newTestServer(t)
	svc.playErr = errors.New("boom")
	req := httptest.NewRequest(http.MethodPost, "/api/control/play", nil)
	req.SetPathValue("action", "play")
	req.RemoteAddr = "127.0.0.1:1000"
	w := httptest.NewRecorder()
	localhostOnly(srv.handleControl, false)(w, req)
	var body map[string]any
	_ = json.NewDecoder(w.Body).Decode(&body)
	if body["success"] != false {
		t.Fatalf("expected success false, got %+v", body)
	}
}

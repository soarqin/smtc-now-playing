package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smtc-now-playing/internal/smtc"
)

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

// TestNew_NoPanic verifies that constructing a WebServer does not panic and
// returns a non-nil value. No WinRT calls are made — smtc.New only allocates.
func TestNew_NoPanic(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)
	if srv == nil {
		t.Fatal("New returned nil")
	}
}

// TestHandleAlbumArt_NoData_Returns404 verifies that the /albumArt/ endpoint
// returns 404 when no album art has been received yet.
func TestHandleAlbumArt_NoData_Returns404(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodGet, "/albumArt/abc123", nil)
	w := httptest.NewRecorder()

	srv.handleAlbumArt(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("handleAlbumArt with no data: got HTTP %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestHandleAlbumArt_WithData_ReturnsContent verifies that the /albumArt/
// endpoint serves the stored album art with the correct Content-Type.
func TestHandleAlbumArt_WithData_ReturnsContent(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	// Inject album art data directly into the server state.
	srv.currentMutex.Lock()
	srv.albumArtData = []byte{0xFF, 0xD8, 0xFF} // minimal JPEG header
	srv.albumArtContentType = "image/jpeg"
	srv.currentMutex.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/albumArt/abc123", nil)
	w := httptest.NewRecorder()

	srv.handleAlbumArt(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleAlbumArt with data: got HTTP %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type: got %q, want \"image/jpeg\"", ct)
	}
}

// TestGetSessions_Empty verifies GetSessions returns nil when no sessions
// have been received.
func TestGetSessions_Empty(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)
	sessions := srv.GetSessions()
	if sessions != nil {
		t.Errorf("GetSessions on fresh server: got %v, want nil", sessions)
	}
}

// TestAddress verifies that Address returns the configured host:port string.
func TestAddress(t *testing.T) {
	srv := New("localhost", "9999", "default", "", false)
	addr := srv.Address()
	if addr != "localhost:9999" {
		t.Errorf("Address: got %q, want \"localhost:9999\"", addr)
	}
}

// TestHandleInfoUpdate_MessageFormatting verifies that handleInfoUpdate stores
// correctly formatted JSON with type "info" and the supplied artist/title.
func TestHandleInfoUpdate_MessageFormatting(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.handleInfoUpdate(smtc.InfoData{Artist: "TestArtist", Title: "TestTitle"})

	srv.currentMutex.Lock()
	info := srv.currentInfo
	srv.currentMutex.Unlock()

	if info == "" {
		t.Fatal("currentInfo is empty after handleInfoUpdate")
	}

	var msg struct {
		Type string `json:"type"`
		Data struct {
			Artist string `json:"artist"`
			Title  string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(info), &msg); err != nil {
		t.Fatalf("currentInfo is not valid JSON: %v", err)
	}
	if msg.Type != "info" {
		t.Errorf("type: got %q, want \"info\"", msg.Type)
	}
	if msg.Data.Artist != "TestArtist" {
		t.Errorf("artist: got %q, want \"TestArtist\"", msg.Data.Artist)
	}
	if msg.Data.Title != "TestTitle" {
		t.Errorf("title: got %q, want \"TestTitle\"", msg.Data.Title)
	}
}

// TestHandleInfoUpdate_Deduplication verifies that calling handleInfoUpdate
// twice with identical data does not change the stored JSON string.
func TestHandleInfoUpdate_Deduplication(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	data := smtc.InfoData{Artist: "Artist", Title: "Song"}
	srv.handleInfoUpdate(data)

	srv.currentMutex.Lock()
	first := srv.currentInfo
	srv.currentMutex.Unlock()

	if first == "" {
		t.Fatal("currentInfo empty after first handleInfoUpdate")
	}

	srv.handleInfoUpdate(data) // same data

	srv.currentMutex.Lock()
	second := srv.currentInfo
	srv.currentMutex.Unlock()

	if first != second {
		t.Errorf("deduplication failed: currentInfo changed on identical update")
	}
}

// TestHandleInfoUpdate_AlbumArtHash verifies that when ThumbnailData is present,
// the albumArt field contains a /albumArt/ prefix followed by a hex SHA256 hash.
func TestHandleInfoUpdate_AlbumArtHash(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.handleInfoUpdate(smtc.InfoData{
		Artist:               "Artist",
		Title:                "Title",
		ThumbnailData:        []byte{0xFF, 0xD8},
		ThumbnailContentType: "image/jpeg",
	})

	srv.currentMutex.Lock()
	info := srv.currentInfo
	srv.currentMutex.Unlock()

	var msg struct {
		Data struct {
			AlbumArt string `json:"albumArt"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(info), &msg); err != nil {
		t.Fatalf("currentInfo is not valid JSON: %v", err)
	}

	if !strings.HasPrefix(msg.Data.AlbumArt, "/albumArt/") {
		t.Errorf("albumArt: got %q, want prefix \"/albumArt/\"", msg.Data.AlbumArt)
	}
	hash := strings.TrimPrefix(msg.Data.AlbumArt, "/albumArt/")
	// SHA256 hex is 64 characters
	if len(hash) != 64 {
		t.Errorf("albumArt hash: got len %d, want 64 hex chars", len(hash))
	}
}

// TestHandleInfoUpdate_NoThumbnail verifies that when ThumbnailData is empty,
// the albumArt field in the JSON is an empty string.
func TestHandleInfoUpdate_NoThumbnail(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.handleInfoUpdate(smtc.InfoData{Artist: "Artist", Title: "Title"})

	srv.currentMutex.Lock()
	info := srv.currentInfo
	srv.currentMutex.Unlock()

	var msg struct {
		Data struct {
			AlbumArt string `json:"albumArt"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(info), &msg); err != nil {
		t.Fatalf("currentInfo is not valid JSON: %v", err)
	}
	if msg.Data.AlbumArt != "" {
		t.Errorf("albumArt: got %q, want empty string", msg.Data.AlbumArt)
	}
}

// TestHandleProgressUpdate_MessageFormatting verifies that handleProgressUpdate
// stores correctly formatted JSON with type "progress" and correct fields.
func TestHandleProgressUpdate_MessageFormatting(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.handleProgressUpdate(smtc.ProgressData{
		Position:        60,
		Duration:        180,
		Status:          4,
		PlaybackRate:    1.0,
		LastUpdatedTime: 1700000000000,
	})

	srv.currentMutex.Lock()
	progress := srv.currentProgress
	srv.currentMutex.Unlock()

	if progress == "" {
		t.Fatal("currentProgress is empty after handleProgressUpdate")
	}

	var msg struct {
		Type string `json:"type"`
		Data struct {
			Position        int     `json:"position"`
			Duration        int     `json:"duration"`
			Status          int     `json:"status"`
			PlaybackRate    float64 `json:"playbackRate"`
			LastUpdatedTime int64   `json:"lastUpdatedTime"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(progress), &msg); err != nil {
		t.Fatalf("currentProgress is not valid JSON: %v", err)
	}
	if msg.Type != "progress" {
		t.Errorf("type: got %q, want \"progress\"", msg.Type)
	}
	if msg.Data.Position != 60 {
		t.Errorf("position: got %d, want 60", msg.Data.Position)
	}
	if msg.Data.Duration != 180 {
		t.Errorf("duration: got %d, want 180", msg.Data.Duration)
	}
	if msg.Data.Status != 4 {
		t.Errorf("status: got %d, want 4", msg.Data.Status)
	}
	if msg.Data.PlaybackRate != 1.0 {
		t.Errorf("playbackRate: got %v, want 1.0", msg.Data.PlaybackRate)
	}
	if msg.Data.LastUpdatedTime != 1700000000000 {
		t.Errorf("lastUpdatedTime: got %d, want 1700000000000", msg.Data.LastUpdatedTime)
	}
}

// TestHandleProgressUpdate_Deduplication verifies that calling handleProgressUpdate
// twice with identical data does not change the stored JSON string.
func TestHandleProgressUpdate_Deduplication(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	data := smtc.ProgressData{Position: 10, Duration: 200, Status: 4}
	srv.handleProgressUpdate(data)

	srv.currentMutex.Lock()
	first := srv.currentProgress
	srv.currentMutex.Unlock()

	if first == "" {
		t.Fatal("currentProgress empty after first handleProgressUpdate")
	}

	srv.handleProgressUpdate(data) // same data

	srv.currentMutex.Lock()
	second := srv.currentProgress
	srv.currentMutex.Unlock()

	if first != second {
		t.Errorf("deduplication failed: currentProgress changed on identical update")
	}
}

// TestHandleProgressUpdate_IsShuffleActiveNil verifies that a nil IsShuffleActive
// is marshalled as JSON null in the stored progress message.
func TestHandleProgressUpdate_IsShuffleActiveNil(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.handleProgressUpdate(smtc.ProgressData{IsShuffleActive: nil})

	srv.currentMutex.Lock()
	progress := srv.currentProgress
	srv.currentMutex.Unlock()

	var msg struct {
		Data struct {
			IsShuffleActive *bool `json:"isShuffleActive"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(progress), &msg); err != nil {
		t.Fatalf("currentProgress is not valid JSON: %v", err)
	}
	if msg.Data.IsShuffleActive != nil {
		t.Errorf("isShuffleActive: got %v, want nil", msg.Data.IsShuffleActive)
	}
}

// TestHandleProgressUpdate_IsShuffleActiveTrue verifies that IsShuffleActive=true
// is marshalled correctly in the stored progress message.
func TestHandleProgressUpdate_IsShuffleActiveTrue(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.handleProgressUpdate(smtc.ProgressData{IsShuffleActive: boolPtr(true)})

	srv.currentMutex.Lock()
	progress := srv.currentProgress
	srv.currentMutex.Unlock()

	var msg struct {
		Data struct {
			IsShuffleActive *bool `json:"isShuffleActive"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(progress), &msg); err != nil {
		t.Fatalf("currentProgress is not valid JSON: %v", err)
	}
	if msg.Data.IsShuffleActive == nil || !*msg.Data.IsShuffleActive {
		t.Errorf("isShuffleActive: got %v, want true", msg.Data.IsShuffleActive)
	}
}

// TestGetSessions_CopySemantics verifies that GetSessions returns a defensive
// copy — modifying the returned slice does not affect subsequent calls.
func TestGetSessions_CopySemantics(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.sessionsMutex.Lock()
	srv.sessions = []smtc.SessionInfo{
		{AppID: "original.exe", Name: "Original"},
	}
	srv.sessionsMutex.Unlock()

	first := srv.GetSessions()
	if len(first) != 1 {
		t.Fatalf("GetSessions: got %d items, want 1", len(first))
	}

	// Mutate the returned slice.
	first[0].Name = "Mutated"

	second := srv.GetSessions()
	if len(second) != 1 {
		t.Fatalf("GetSessions (second): got %d items, want 1", len(second))
	}
	if second[0].Name != "Original" {
		t.Errorf("GetSessions copy semantics broken: got Name=%q, want \"Original\"", second[0].Name)
	}
}

// TestHandleAlbumArt_EmptyContentType verifies that when albumArtData is set
// but albumArtContentType is empty, the handler falls back to
// "application/octet-stream".
func TestHandleAlbumArt_EmptyContentType(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	srv.currentMutex.Lock()
	srv.albumArtData = []byte{0x01, 0x02, 0x03}
	srv.albumArtContentType = "" // intentionally empty
	srv.currentMutex.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/albumArt/abc", nil)
	w := httptest.NewRecorder()

	srv.handleAlbumArt(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("handleAlbumArt: got HTTP %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type: got %q, want \"application/octet-stream\"", ct)
	}
}

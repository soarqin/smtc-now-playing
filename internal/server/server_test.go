package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

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


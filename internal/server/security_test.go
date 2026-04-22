package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIsLocalhost covers the IP / zone / port variants isLocalhost must
// accept or reject. These used to be broken by the old string-compare
// implementation; net.ParseIP().IsLoopback() handles them uniformly.
func TestIsLocalhost(t *testing.T) {
	cases := []struct {
		remote string
		want   bool
	}{
		{"127.0.0.1:12345", true},
		{"127.0.0.2:12345", true},      // whole 127.0.0.0/8 is loopback
		{"[::1]:12345", true},
		{"[fe80::1%eth0]:12345", false}, // zone-qualified link-local is NOT loopback
		{"[::1%lo]:12345", true},        // zone-qualified loopback IS loopback
		{"192.168.1.100:12345", false},
		{"8.8.8.8:53", false},
		{"localhost:12345", true}, // last-resort literal match
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = c.remote
		if got := isLocalhost(req); got != c.want {
			t.Errorf("isLocalhost(%q) = %v, want %v", c.remote, got, c.want)
		}
	}
}

// TestSafeJoin_RejectsTraversal guards the directory-traversal fix in
// handleStatic / handleScript. Any rel path that escapes the base must be
// rejected — returning a resolved path here used to serve arbitrary files.
func TestSafeJoin_RejectsTraversal(t *testing.T) {
	badRels := []string{
		"../secret.json",
		"/../secret.json",
		"a/../../secret.json",
		"..\\windows\\system32\\config",
		"foo/..\\..\\secret.json",
		"/../../../etc/passwd",
	}
	for _, r := range badRels {
		if _, ok := safeJoin("themes/default", r); ok {
			t.Errorf("safeJoin(%q): expected rejection, got accept", r)
		}
	}
}

// TestSafeJoin_AcceptsInside confirms we haven't broken legitimate paths.
func TestSafeJoin_AcceptsInside(t *testing.T) {
	goodRels := []string{
		"index.html",
		"/index.html",
		"subdir/file.css",
		"assets/img/logo.png",
	}
	for _, r := range goodRels {
		if _, ok := safeJoin("themes/default", r); !ok {
			t.Errorf("safeJoin(%q): expected accept, got reject", r)
		}
	}
}

// TestHandleStatic_Traversal_403 verifies the HTTP handler rejects a
// traversal attempt with 403 rather than serving the escaped file.
func TestHandleStatic_Traversal_403(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodGet, "/../../../main.go", nil)
	// httptest's URL parsing cleans "..", so emulate a raw path by
	// overwriting URL.Path with a crafted traversal string.
	req.URL.Path = "/../../../main.go"
	w := httptest.NewRecorder()

	srv.handleStatic(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("handleStatic traversal: got HTTP %d, want %d", w.Code, http.StatusForbidden)
	}
	if strings.Contains(w.Body.String(), "package main") {
		t.Error("handleStatic leaked source file content via traversal")
	}
}

// TestHandleScript_Traversal_403 asserts the same guard for /script/*.
func TestHandleScript_Traversal_403(t *testing.T) {
	srv := New("localhost", "0", "default", "", false)

	req := httptest.NewRequest(http.MethodGet, "/script/../main.go", nil)
	req.URL.Path = "/script/../main.go"
	w := httptest.NewRecorder()

	srv.handleScript(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("handleScript traversal: got HTTP %d, want %d", w.Code, http.StatusForbidden)
	}
}

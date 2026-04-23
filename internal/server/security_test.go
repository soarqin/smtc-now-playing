package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsLocalhost(t *testing.T) {
	cases := []struct {
		remote string
		want   bool
	}{
		{"127.0.0.1:12345", true},
		{"127.0.0.2:12345", true},
		{"[::1]:12345", true},
		{"[fe80::1%eth0]:12345", false},
		{"[::1%lo]:12345", true},
		{"192.168.1.100:12345", false},
		{"8.8.8.8:53", false},
		{"localhost:12345", true},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = tc.remote
		if got := isLocalhost(req); got != tc.want {
			t.Fatalf("isLocalhost(%q) = %v, want %v", tc.remote, got, tc.want)
		}
	}
}

func TestSafeJoin_RejectsTraversal(t *testing.T) {
	for _, rel := range []string{"../secret.json", "/../secret.json", "a/../../secret.json", "..\\windows\\system32\\config", "foo/..\\..\\secret.json", "/../../../etc/passwd"} {
		if _, ok := safeJoin("themes/default", rel); ok {
			t.Fatalf("safeJoin accepted %q", rel)
		}
	}
}

func TestSafeJoin_AcceptsInside(t *testing.T) {
	for _, rel := range []string{"index.html", "/index.html", "subdir/file.css", "assets/img/logo.png"} {
		if _, ok := safeJoin("themes/default", rel); !ok {
			t.Fatalf("safeJoin rejected %q", rel)
		}
	}
}

func TestHandleTheme_Traversal_403(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/../../../main.go", nil)
	req.URL.Path = "/../../../main.go"
	w := httptest.NewRecorder()
	srv.handleTheme(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d want %d", w.Code, http.StatusForbidden)
	}
	if strings.Contains(w.Body.String(), "package main") {
		t.Fatal("leaked file content")
	}
}

func TestHandleScript_Traversal_403(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/script/../main.go", nil)
	req.URL.Path = "/script/../main.go"
	w := httptest.NewRecorder()
	srv.handleScript(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("got %d want %d", w.Code, http.StatusForbidden)
	}
}

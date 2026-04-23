package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsLocalhost(t *testing.T) {
	cases := []struct {
		remote string
		want   bool
	}{
		{remote: "127.0.0.1:12345", want: true},
		{remote: "127.0.0.2:12345", want: true},
		{remote: "[::1]:12345", want: true},
		{remote: "[fe80::1%eth0]:12345", want: false},
		{remote: "[::1%lo]:12345", want: true},
		{remote: "192.168.1.100:12345", want: false},
		{remote: "localhost:12345", want: true},
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
	bad := []string{
		"../secret.json",
		"/../secret.json",
		"a/../../secret.json",
		"..\\windows\\system32\\config",
		"foo/..\\..\\secret.json",
	}
	for _, rel := range bad {
		if _, ok := safeJoin("themes/default", rel); ok {
			t.Fatalf("safeJoin(%q) unexpectedly succeeded", rel)
		}
	}
}

func TestSafeJoin_AcceptsInside(t *testing.T) {
	good := []string{"index.html", "/index.html", "subdir/file.css"}
	for _, rel := range good {
		if _, ok := safeJoin("themes/default", rel); !ok {
			t.Fatalf("safeJoin(%q) unexpectedly failed", rel)
		}
	}
}

func TestHandleScript_TraversalRejected(t *testing.T) {
	srv, _, _ := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/script/../main.go", nil)
	req.SetPathValue("file", "../main.go")
	w := httptest.NewRecorder()
	srv.handleScript(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

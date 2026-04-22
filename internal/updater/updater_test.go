package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckForUpdate(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		serverVersion  string
		statusCode     int
		rawBody        string
		slowServer     bool // simulate timeout
		clientTimeout  time.Duration
		wantAvailable  bool
		wantErr        bool
	}{
		{
			name:           "newer version available",
			currentVersion: "1.2.0",
			serverVersion:  "v1.3.0",
			statusCode:     http.StatusOK,
			wantAvailable:  true,
		},
		{
			name:           "same version",
			currentVersion: "1.2.0",
			serverVersion:  "v1.2.0",
			statusCode:     http.StatusOK,
			wantAvailable:  false,
		},
		{
			name:           "older version",
			currentVersion: "1.2.0",
			serverVersion:  "v1.1.0",
			statusCode:     http.StatusOK,
			wantAvailable:  false,
		},
		{
			name:       "API error 500",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:           "network timeout",
			currentVersion: "1.2.0",
			slowServer:     true,
			clientTimeout:  50 * time.Millisecond,
			wantErr:        true,
		},
		{
			name:           "malformed JSON",
			currentVersion: "1.2.0",
			statusCode:     http.StatusOK,
			rawBody:        "this is not valid json {{{",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server

			switch {
			case tt.slowServer:
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(500 * time.Millisecond)
					w.WriteHeader(http.StatusOK)
				}))
			case tt.rawBody != "":
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
					_, _ = w.Write([]byte(tt.rawBody))
				}))
			default:
				release := struct {
					TagName string `json:"tag_name"`
					HTMLURL string `json:"html_url"`
					Body    string `json:"body"`
				}{
					TagName: tt.serverVersion,
					HTMLURL: "https://github.com/soarqin/smtc-now-playing/releases/tag/" + tt.serverVersion,
					Body:    "Release notes for " + tt.serverVersion,
				}
				srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
					if tt.statusCode == http.StatusOK {
						_ = json.NewEncoder(w).Encode(release)
					}
				}))
			}
			defer srv.Close()

			timeout := 5 * time.Second
			if tt.clientTimeout > 0 {
				timeout = tt.clientTimeout
			}
			client := &http.Client{Timeout: timeout}

			info, err := checkForUpdate(tt.currentVersion, srv.URL, client)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got nil (info=%v)", info)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info == nil {
				t.Fatal("expected non-nil UpdateInfo but got nil")
			}
			if info.Available != tt.wantAvailable {
				t.Errorf("Available = %v, want %v (latest=%q current=%q)",
					info.Available, tt.wantAvailable, info.Version, tt.currentVersion)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{
			name: "equal versions",
			a:    "1.2.3",
			b:    "1.2.3",
			want: false,
		},
		{
			name: "major newer",
			a:    "2.0.0",
			b:    "1.0.0",
			want: true,
		},
		{
			name: "major older",
			a:    "1.0.0",
			b:    "2.0.0",
			want: false,
		},
		{
			name: "minor newer",
			a:    "1.3.0",
			b:    "1.2.0",
			want: true,
		},
		{
			name: "minor older",
			a:    "1.2.0",
			b:    "1.3.0",
			want: false,
		},
		{
			name: "patch newer",
			a:    "1.2.4",
			b:    "1.2.3",
			want: true,
		},
		{
			name: "patch older",
			a:    "1.2.3",
			b:    "1.2.4",
			want: false,
		},
		{
			name: "large numbers",
			a:    "10.20.30",
			b:    "10.20.29",
			want: true,
		},
		{
			name: "zero versions",
			a:    "0.0.0",
			b:    "0.0.0",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCheckForUpdate_FieldValues(t *testing.T) {
	release := struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
	}{
		TagName: "v2.5.1",
		HTMLURL: "https://github.com/test/releases/tag/v2.5.1",
		Body:    "Some release notes",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	info, err := checkForUpdate("1.0.0", srv.URL, client)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil UpdateInfo but got nil")
	}

	if !info.Available {
		t.Errorf("Available = %v, want true", info.Available)
	}
	if info.Version != "v2.5.1" {
		t.Errorf("Version = %q, want %q", info.Version, "v2.5.1")
	}
	if info.URL != "https://github.com/test/releases/tag/v2.5.1" {
		t.Errorf("URL = %q, want %q", info.URL, "https://github.com/test/releases/tag/v2.5.1")
	}

	// json.Encoder adds trailing newline, so trim before comparing
	expectedNotes := "Some release notes"
	actualNotes := strings.TrimSpace(info.ReleaseNotes)
	if actualNotes != expectedNotes {
		t.Errorf("ReleaseNotes = %q, want %q", actualNotes, expectedNotes)
	}
}

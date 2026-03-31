package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

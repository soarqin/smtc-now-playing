package config

import (
	"path/filepath"
	"testing"
)

// TestAppDataConfigFile verifies that appDataConfigFile returns the correct path
// when APPDATA is set, and returns empty string when APPDATA is unset.
func TestAppDataConfigFile(t *testing.T) {
	tests := []struct {
		name    string
		appData string
		want    string // "" means expect empty string
	}{
		{
			name:    "APPDATA set",
			appData: "/some/path",
			want:    filepath.Join("/some/path", "soarqin", "smtc-now-playing", "config.json"),
		},
		{
			name:    "APPDATA empty",
			appData: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APPDATA", tt.appData)
			got := appDataConfigFile()
			if got != tt.want {
				t.Errorf("appDataConfigFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

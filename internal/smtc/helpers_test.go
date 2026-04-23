//go:build windows

package smtc

import (
	"testing"

	"smtc-now-playing/internal/domain"
)

// TestReadNullableBool_NilSafe verifies that readNullableBool safely handles nil IReference.
func TestReadNullableBool_NilSafe(t *testing.T) {
	val, ok := readNullableBool(nil)
	if val != false || ok != false {
		t.Errorf("readNullableBool(nil) = (%v, %v), want (false, false)", val, ok)
	}
}

// TestReadNullableFloat64_NilSafe verifies that readNullableFloat64 safely handles nil IReference.
func TestReadNullableFloat64_NilSafe(t *testing.T) {
	val, ok := readNullableFloat64(nil)
	if val != 0.0 || ok != false {
		t.Errorf("readNullableFloat64(nil) = (%v, %v), want (0.0, false)", val, ok)
	}
}

// TestReadNullableInt32_NilSafe verifies that readNullableInt32 safely handles nil IReference.
func TestReadNullableInt32_NilSafe(t *testing.T) {
	val, ok := readNullableInt32(nil)
	if val != 0 || ok != false {
		t.Errorf("readNullableInt32(nil) = (%v, %v), want (0, false)", val, ok)
	}
}

// TestFriendlyAppName verifies that friendlyAppName extracts a readable name
// from UWP and Win32 app identifiers, with title-casing applied.
func TestFriendlyAppName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "UWP full package family name",
			input: "Microsoft.ZuneMusic_8wekyb3d8bbwe!Microsoft.ZuneMusic",
			want:  "Microsoft.ZuneMusic",
		},
		{
			name:  "UWP simple app ID",
			input: "PackageFamily!App",
			want:  "App",
		},
		{
			name:  "Win32 exe lowercase",
			input: "Spotify.exe",
			want:  "Spotify",
		},
		{
			name:  "Win32 exe uppercase extension",
			input: "chrome.EXE",
			want:  "Chrome",
		},
		{
			name:  "no suffix",
			input: "vlc",
			want:  "Vlc",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "exclamation at end produces empty part then fallback",
			input: "Family!",
			want:  "Family!",
		},
		{
			name:  "single character",
			input: "a",
			want:  "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := friendlyAppName(tt.input)
			if got != tt.want {
				t.Errorf("friendlyAppName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSessionInfosToDomain(t *testing.T) {
	got := sessionInfosToDomain([]SessionInfo{{AppID: "app", Name: "Name", SourceAppID: "source"}})
	want := []domain.SessionInfo{{AppID: "app", Name: "Name", SourceAppID: "source"}}
	if len(got) != len(want) {
		t.Fatalf("len(sessionInfosToDomain()) = %d, want %d", len(got), len(want))
	}
	if got[0] != want[0] {
		t.Fatalf("sessionInfosToDomain()[0] = %+v, want %+v", got[0], want[0])
	}
}

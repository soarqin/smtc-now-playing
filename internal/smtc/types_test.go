package smtc

import (
	"testing"

	"smtc-now-playing/internal/domain"
)

// TestEscape verifies that escape() correctly escapes all special characters
// while passing through regular text and Unicode unchanged.
func TestEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty string", input: "", want: ""},
		{name: "no special chars", input: "Hello World", want: "Hello World"},
		{name: "newline", input: "\n", want: `\n`},
		{name: "carriage return", input: "\r", want: `\r`},
		{name: "tab", input: "\t", want: `\t`},
		{name: "backslash", input: "\\", want: `\\`},
		{name: "vertical tab", input: "\v", want: `\v`},
		{name: "backspace", input: "\b", want: `\b`},
		{name: "form feed", input: "\f", want: `\f`},
		{name: "alert/bell", input: "\a", want: `\a`},
		{name: "multiple specials", input: "line1\nline2\ttab", want: `line1\nline2\ttab`},
		{name: "unicode passthrough", input: "日本語アーティスト", want: "日本語アーティスト"},
		{name: "mixed unicode and special", input: "アーティスト\nタイトル", want: `アーティスト\nタイトル`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escape(tt.input)
			if got != tt.want {
				t.Errorf("escape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEventTypesImplementSealedInterface(t *testing.T) {
	var _ Event = InfoEvent{}
	var _ Event = ProgressEvent{}
	var _ Event = SessionsChangedEvent{}
	var _ Event = DeviceChangedEvent{}
}

func TestInfoDataToDomain(t *testing.T) {
	src := InfoData{
		Artist:               "Artist",
		Title:                "Title",
		ThumbnailContentType: "image/png",
		ThumbnailData:        []byte{1, 2, 3},
		AlbumTitle:           "Album",
		AlbumArtist:          "Album Artist",
		PlaybackType:         2,
		SourceApp:            "Spotify.exe",
	}

	got := infoDataToDomain(src)
	want := domain.InfoData{
		Artist:               "Artist",
		Title:                "Title",
		ThumbnailContentType: "image/png",
		ThumbnailData:        []byte{1, 2, 3},
		AlbumTitle:           "Album",
		AlbumArtist:          "Album Artist",
		PlaybackType:         2,
		SourceApp:            "Spotify.exe",
	}

	if !got.Equal(&want) {
		t.Fatalf("infoDataToDomain() = %+v, want %+v", got, want)
	}
	src.ThumbnailData[0] = 9
	if got.ThumbnailData[0] != 1 {
		t.Fatal("thumbnail data was not copied")
	}
}

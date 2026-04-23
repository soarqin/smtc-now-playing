package domain

import (
	"testing"
)

// TestEscape verifies that Escape() correctly escapes all special characters
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
			got := Escape(tt.input)
			if got != tt.want {
				t.Errorf("Escape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

package version

import "testing"

func TestVersionNonEmpty(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

// TestVersionDefault verifies that Version is "dev" when no ldflags are set.
func TestVersionDefault(t *testing.T) {
	if Version != "dev" {
		t.Errorf("Version should be 'dev' by default, got %q", Version)
	}
}

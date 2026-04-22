package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveConfigToFile_Atomic verifies that saveConfigToFile writes via a
// sibling temp file and leaves no ".tmp" fragments behind on success.
// This locks in the atomic-write fix that replaced the old
// os.Create-then-truncate approach.
func TestSaveConfigToFile_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{Port: 12345, Theme: "custom"}
	if err := saveConfigToFile(path, cfg); err != nil {
		t.Fatalf("saveConfigToFile: %v", err)
	}

	// Final file must exist and parse back to the same fields.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if got.Port != 12345 || got.Theme != "custom" {
		t.Errorf("roundtrip mismatch: got %+v, want port=12345 theme=custom", got)
	}

	// Directory must not contain leftover .tmp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("stale temp file left behind: %q", e.Name())
		}
	}
}

// TestSaveConfigToFile_OverwriteAtomic verifies saving over an existing
// file never leaves the target truncated mid-write. We can't easily
// crash mid-rename inside a test, so we check the weaker invariant:
// after a successful save the target always contains complete JSON.
func TestSaveConfigToFile_OverwriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Pre-populate with old content so we know the save replaces it.
	if err := os.WriteFile(path, []byte(`{"port":9999}`), 0600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	cfg := &Config{Port: 22222, Theme: "new"}
	if err := saveConfigToFile(path, cfg); err != nil {
		t.Fatalf("saveConfigToFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse saved config — atomic rename left a partial file: %v", err)
	}
	if got.Port != 22222 || got.Theme != "new" {
		t.Errorf("save did not overwrite: got %+v", got)
	}
}

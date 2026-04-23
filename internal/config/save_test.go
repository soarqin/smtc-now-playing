package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSave_Atomic verifies that Save writes via a sibling temp file and
// leaves no ".tmp" fragments behind on success.
func TestSave_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	cfg := &Config{
		Server:  ServerConfig{Port: 12345},
		UI:      UIConfig{Theme: "custom"},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
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
	if got.Server.Port != 12345 {
		t.Errorf("Server.Port: got %d, want 12345", got.Server.Port)
	}
	if got.UI.Theme != "custom" {
		t.Errorf("UI.Theme: got %q, want \"custom\"", got.UI.Theme)
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

// TestSave_OverwriteAtomic verifies saving over an existing file produces
// complete, parseable JSON with the new values.
func TestSave_OverwriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Pre-populate with old content so we know the save replaces it.
	if err := os.WriteFile(path, []byte(`{"server":{"port":9999},"ui":{"theme":"old"}}`), 0600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	cfg := &Config{
		Server:  ServerConfig{Port: 22222},
		UI:      UIConfig{Theme: "new"},
		Logging: LoggingConfig{Level: "info"},
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	var got Config
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse saved config — atomic rename left a partial file: %v", err)
	}
	if got.Server.Port != 22222 {
		t.Errorf("Server.Port: got %d, want 22222", got.Server.Port)
	}
	if got.UI.Theme != "new" {
		t.Errorf("UI.Theme: got %q, want \"new\"", got.UI.Theme)
	}
}

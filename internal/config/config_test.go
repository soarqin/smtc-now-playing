package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig_ValidPortableConfig verifies that loadConfigFromFile correctly
// parses a well-formed JSON file and overwrites the supplied Config fields.
func TestLoadConfig_ValidPortableConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{"port": 12345, "theme": "custom", "autostart": true, "debug": true}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg := &Config{Port: 11451, Theme: "default"}
	if err := loadConfigFromFile(path, cfg); err != nil {
		t.Fatalf("loadConfigFromFile returned unexpected error: %v", err)
	}

	if cfg.Port != 12345 {
		t.Errorf("Port: got %d, want 12345", cfg.Port)
	}
	if cfg.Theme != "custom" {
		t.Errorf("Theme: got %q, want \"custom\"", cfg.Theme)
	}
	if !cfg.AutoStart {
		t.Errorf("AutoStart: got false, want true")
	}
	if !cfg.Debug {
		t.Errorf("Debug: got false, want true")
	}
}

// TestLoadConfig_MissingFile_UsesDefaults verifies that when no config file
// exists the Config struct is left untouched (defaults are preserved).
func TestLoadConfig_MissingFile_UsesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	cfg := &Config{
		Port:  11451,
		Theme: "default",
	}

	// Attempting to load a non-existent file must return an error.
	err := loadConfigFromFile(path, cfg)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	// The config must not have been modified — defaults remain.
	if cfg.Port != 11451 {
		t.Errorf("Port: got %d, want default 11451", cfg.Port)
	}
	if cfg.Theme != "default" {
		t.Errorf("Theme: got %q, want default \"default\"", cfg.Theme)
	}
}

// TestLoadConfig_MalformedJSON_ReturnsError verifies that malformed JSON
// causes loadConfigFromFile to return a non-nil error.
func TestLoadConfig_MalformedJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("failed to write bad config file: %v", err)
	}

	cfg := &Config{}
	err := loadConfigFromFile(path, cfg)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

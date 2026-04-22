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

// TestConfigDefaults verifies that the package-level config variable is
// initialized with the correct default values by init().
func TestConfigDefaults(t *testing.T) {
	// config is set by init() — verify the known default values.
	if config.Port != 11451 {
		t.Errorf("Port: got %d, want 11451", config.Port)
	}
	if config.Theme != "default" {
		t.Errorf("Theme: got %q, want \"default\"", config.Theme)
	}
	if !config.PreviewAlwaysOnTop {
		t.Errorf("PreviewAlwaysOnTop: got false, want true")
	}
	// All other bools should be false.
	if config.AutoStart {
		t.Errorf("AutoStart: got true, want false")
	}
	if config.StartMinimized {
		t.Errorf("StartMinimized: got true, want false")
	}
	if config.ShowPreviewWindow {
		t.Errorf("ShowPreviewWindow: got true, want false")
	}
	if config.Debug {
		t.Errorf("Debug: got true, want false")
	}
	if config.HotReload {
		t.Errorf("HotReload: got true, want false")
	}
	if config.ControlAllowRemote {
		t.Errorf("ControlAllowRemote: got true, want false")
	}
}

// TestLoadConfig_PartialJSON verifies that loadConfigFromFile merges JSON
// fields into an existing Config, leaving unspecified fields unchanged.
func TestLoadConfig_PartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{"port": 9999}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg := &Config{Port: 11451, Theme: "existing-theme"}
	if err := loadConfigFromFile(path, cfg); err != nil {
		t.Fatalf("loadConfigFromFile returned unexpected error: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("Port: got %d, want 9999", cfg.Port)
	}
	if cfg.Theme != "existing-theme" {
		t.Errorf("Theme: got %q, want \"existing-theme\" (unchanged)", cfg.Theme)
	}
}

// TestLoadConfig_RoundTrip verifies that a Config with non-default values
// can be saved and loaded back with all fields intact.
func TestLoadConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := Config{
		Port:               9876,
		Theme:              "custom-theme",
		AutoStart:          true,
		StartMinimized:     true,
		ShowPreviewWindow:  true,
		PreviewAlwaysOnTop: false,
		SelectedDevice:     "Spotify.exe",
		Debug:              true,
		HotReload:          true,
		ControlAllowRemote: true,
	}

	if err := saveConfigToFile(path, &original); err != nil {
		t.Fatalf("saveConfigToFile: %v", err)
	}

	loaded := Config{}
	if err := loadConfigFromFile(path, &loaded); err != nil {
		t.Fatalf("loadConfigFromFile: %v", err)
	}

	if loaded.Port != original.Port {
		t.Errorf("Port: got %d, want %d", loaded.Port, original.Port)
	}
	if loaded.Theme != original.Theme {
		t.Errorf("Theme: got %q, want %q", loaded.Theme, original.Theme)
	}
	if loaded.AutoStart != original.AutoStart {
		t.Errorf("AutoStart: got %v, want %v", loaded.AutoStart, original.AutoStart)
	}
	if loaded.StartMinimized != original.StartMinimized {
		t.Errorf("StartMinimized: got %v, want %v", loaded.StartMinimized, original.StartMinimized)
	}
	if loaded.ShowPreviewWindow != original.ShowPreviewWindow {
		t.Errorf("ShowPreviewWindow: got %v, want %v", loaded.ShowPreviewWindow, original.ShowPreviewWindow)
	}
	if loaded.PreviewAlwaysOnTop != original.PreviewAlwaysOnTop {
		t.Errorf("PreviewAlwaysOnTop: got %v, want %v", loaded.PreviewAlwaysOnTop, original.PreviewAlwaysOnTop)
	}
	if loaded.SelectedDevice != original.SelectedDevice {
		t.Errorf("SelectedDevice: got %q, want %q", loaded.SelectedDevice, original.SelectedDevice)
	}
	if loaded.Debug != original.Debug {
		t.Errorf("Debug: got %v, want %v", loaded.Debug, original.Debug)
	}
	if loaded.HotReload != original.HotReload {
		t.Errorf("HotReload: got %v, want %v", loaded.HotReload, original.HotReload)
	}
	if loaded.ControlAllowRemote != original.ControlAllowRemote {
		t.Errorf("ControlAllowRemote: got %v, want %v", loaded.ControlAllowRemote, original.ControlAllowRemote)
	}
}

// TestLoadConfig_UnknownFields verifies that loadConfigFromFile ignores
// JSON fields that do not correspond to Config struct fields.
func TestLoadConfig_UnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{"port": 9999, "unknownField": "ignored", "anotherUnknown": 42}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg := &Config{}
	if err := loadConfigFromFile(path, cfg); err != nil {
		t.Fatalf("loadConfigFromFile returned unexpected error: %v", err)
	}

	if cfg.Port != 9999 {
		t.Errorf("Port: got %d, want 9999", cfg.Port)
	}
}

// TestLoadConfig_EmptyJSON verifies that loadConfigFromFile handles an empty
// JSON object without error, leaving all Config fields unchanged.
func TestLoadConfig_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg := &Config{Port: 12345, Theme: "myTheme"}
	if err := loadConfigFromFile(path, cfg); err != nil {
		t.Fatalf("loadConfigFromFile returned unexpected error: %v", err)
	}

	if cfg.Port != 12345 {
		t.Errorf("Port: got %d, want 12345 (unchanged)", cfg.Port)
	}
	if cfg.Theme != "myTheme" {
		t.Errorf("Theme: got %q, want \"myTheme\" (unchanged)", cfg.Theme)
	}
}

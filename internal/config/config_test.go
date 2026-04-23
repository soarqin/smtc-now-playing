package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultConfig verifies that DefaultConfig returns the correct defaults.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 11451 {
		t.Errorf("Server.Port: got %d, want 11451", cfg.Server.Port)
	}
	if cfg.UI.Theme != "default" {
		t.Errorf("UI.Theme: got %q, want \"default\"", cfg.UI.Theme)
	}
	if !cfg.UI.PreviewAlwaysOnTop {
		t.Error("UI.PreviewAlwaysOnTop: got false, want true")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level: got %q, want \"info\"", cfg.Logging.Level)
	}
	// All boolean flags must default to false.
	if cfg.Server.AllowRemote {
		t.Error("Server.AllowRemote: got true, want false")
	}
	if cfg.Server.HotReload {
		t.Error("Server.HotReload: got true, want false")
	}
	if cfg.UI.AutoStart {
		t.Error("UI.AutoStart: got true, want false")
	}
	if cfg.UI.StartMinimized {
		t.Error("UI.StartMinimized: got true, want false")
	}
	if cfg.UI.ShowPreviewWindow {
		t.Error("UI.ShowPreviewWindow: got true, want false")
	}
	if cfg.Logging.Debug {
		t.Error("Logging.Debug: got true, want false")
	}
}

// TestLoad_EmptyJSON verifies that Load with an empty JSON object {} returns
// DefaultConfig values unchanged.
func TestLoad_EmptyJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if cfg.Server.Port != 11451 {
		t.Errorf("Server.Port: got %d, want 11451 (default)", cfg.Server.Port)
	}
	if cfg.UI.Theme != "default" {
		t.Errorf("UI.Theme: got %q, want \"default\"", cfg.UI.Theme)
	}
}

// TestLoad_MissingFile_ReturnsDefaults verifies that Load returns DefaultConfig
// without error when the file does not exist.
func TestLoad_MissingFile_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load missing file: unexpected error: %v", err)
	}
	if cfg.Server.Port != 11451 {
		t.Errorf("Server.Port: got %d, want 11451", cfg.Server.Port)
	}
	if cfg.UI.Theme != "default" {
		t.Errorf("UI.Theme: got %q, want \"default\"", cfg.UI.Theme)
	}
}

// TestLoad_V2ValidJSON verifies Load parses a well-formed v2 JSON file.
func TestLoad_V2ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{
		"server": {"port": 12345, "hotReload": true},
		"ui": {"theme": "mini", "autoStart": true},
		"logging": {"level": "debug", "debug": true}
	}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Server.Port != 12345 {
		t.Errorf("Server.Port: got %d, want 12345", cfg.Server.Port)
	}
	if !cfg.Server.HotReload {
		t.Error("Server.HotReload: got false, want true")
	}
	if cfg.UI.Theme != "mini" {
		t.Errorf("UI.Theme: got %q, want \"mini\"", cfg.UI.Theme)
	}
	if !cfg.UI.AutoStart {
		t.Error("UI.AutoStart: got false, want true")
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level: got %q, want \"debug\"", cfg.Logging.Level)
	}
	if !cfg.Logging.Debug {
		t.Error("Logging.Debug: got false, want true")
	}
}

// TestLoad_V2PartialJSON verifies that fields absent from the JSON file retain
// their default values after loading.
func TestLoad_V2PartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{"server": {"port": 9999}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Server.Port: got %d, want 9999", cfg.Server.Port)
	}
	// Fields not in JSON must keep defaults.
	if cfg.UI.Theme != "default" {
		t.Errorf("UI.Theme: got %q, want \"default\" (default)", cfg.UI.Theme)
	}
	if !cfg.UI.PreviewAlwaysOnTop {
		t.Error("UI.PreviewAlwaysOnTop: got false, want true (default)")
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level: got %q, want \"info\" (default)", cfg.Logging.Level)
	}
}

// TestLoad_MalformedJSON verifies that malformed JSON causes Load to return
// a non-nil error.
func TestLoad_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("write bad config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

// TestLoad_UnknownFields verifies that unknown JSON keys are silently ignored.
func TestLoad_UnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	data := `{"server": {"port": 9999}, "unknownKey": "ignored", "anotherUnknown": 42}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("Server.Port: got %d, want 9999", cfg.Server.Port)
	}
}

// TestLoad_V2RoundTrip verifies that Save + Load preserves all fields.
func TestLoad_V2RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	original := &Config{
		Server: ServerConfig{
			Port:        9876,
			AllowRemote: true,
			HotReload:   true,
		},
		UI: UIConfig{
			Theme:              "custom-theme",
			AutoStart:          true,
			StartMinimized:     true,
			ShowPreviewWindow:  true,
			PreviewAlwaysOnTop: false,
		},
		SMTC: SMTCConfig{
			SelectedDevice: "Spotify.exe",
		},
		Logging: LoggingConfig{
			Level: "warn",
			Debug: true,
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Server.Port != original.Server.Port {
		t.Errorf("Server.Port: got %d, want %d", loaded.Server.Port, original.Server.Port)
	}
	if loaded.Server.AllowRemote != original.Server.AllowRemote {
		t.Errorf("Server.AllowRemote: got %v, want %v", loaded.Server.AllowRemote, original.Server.AllowRemote)
	}
	if loaded.Server.HotReload != original.Server.HotReload {
		t.Errorf("Server.HotReload: got %v, want %v", loaded.Server.HotReload, original.Server.HotReload)
	}
	if loaded.UI.Theme != original.UI.Theme {
		t.Errorf("UI.Theme: got %q, want %q", loaded.UI.Theme, original.UI.Theme)
	}
	if loaded.UI.AutoStart != original.UI.AutoStart {
		t.Errorf("UI.AutoStart: got %v, want %v", loaded.UI.AutoStart, original.UI.AutoStart)
	}
	if loaded.UI.StartMinimized != original.UI.StartMinimized {
		t.Errorf("UI.StartMinimized: got %v, want %v", loaded.UI.StartMinimized, original.UI.StartMinimized)
	}
	if loaded.UI.ShowPreviewWindow != original.UI.ShowPreviewWindow {
		t.Errorf("UI.ShowPreviewWindow: got %v, want %v", loaded.UI.ShowPreviewWindow, original.UI.ShowPreviewWindow)
	}
	if loaded.UI.PreviewAlwaysOnTop != original.UI.PreviewAlwaysOnTop {
		t.Errorf("UI.PreviewAlwaysOnTop: got %v, want %v", loaded.UI.PreviewAlwaysOnTop, original.UI.PreviewAlwaysOnTop)
	}
	if loaded.SMTC.SelectedDevice != original.SMTC.SelectedDevice {
		t.Errorf("SMTC.SelectedDevice: got %q, want %q", loaded.SMTC.SelectedDevice, original.SMTC.SelectedDevice)
	}
	if loaded.Logging.Level != original.Logging.Level {
		t.Errorf("Logging.Level: got %q, want %q", loaded.Logging.Level, original.Logging.Level)
	}
	if loaded.Logging.Debug != original.Logging.Debug {
		t.Errorf("Logging.Debug: got %v, want %v", loaded.Logging.Debug, original.Logging.Debug)
	}
}

// TestValidate_Valid confirms a well-formed Config passes validation.
func TestValidate_Valid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected nil error for valid config, got: %v", err)
	}
}

// TestValidate_PortZero verifies that port 0 fails validation.
func TestValidate_PortZero(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for port=0, got nil")
	}
}

// TestValidate_PortTooHigh verifies that port >65535 fails validation.
func TestValidate_PortTooHigh(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.Port = 70000
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for port=70000, got nil")
	}
}

// TestValidate_EmptyTheme verifies that an empty theme fails validation.
func TestValidate_EmptyTheme(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UI.Theme = ""
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty theme, got nil")
	}
}

// TestValidate_BadLevel verifies that an unrecognised log level fails validation.
func TestValidate_BadLevel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Logging.Level = "verbose"
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for level=\"verbose\", got nil")
	}
}

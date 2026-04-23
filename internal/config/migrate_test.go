package config

import (
	"os"
	"path/filepath"
	"testing"
)

// v1Fixture is a flat v1 JSON with all ten supported fields.
const v1Fixture = `{"port":12345,"theme":"mini","autostart":true,"startminimized":true,"showpreviewwindow":true,"previewalwaysontop":false,"selecteddevice":"Spotify.exe","debug":true,"hotReload":true,"controlAllowRemote":true}`

// TestMigrateV1_AllFields verifies that migrateV1 maps every v1 key to the
// correct nested field, overriding defaults where necessary.
func TestMigrateV1_AllFields(t *testing.T) {
	cfg, err := migrateV1([]byte(v1Fixture))
	if err != nil {
		t.Fatalf("migrateV1: unexpected error: %v", err)
	}

	if cfg.Server.Port != 12345 {
		t.Errorf("Server.Port: got %d, want 12345", cfg.Server.Port)
	}
	if cfg.UI.Theme != "mini" {
		t.Errorf("UI.Theme: got %q, want \"mini\"", cfg.UI.Theme)
	}
	if !cfg.UI.AutoStart {
		t.Error("UI.AutoStart: got false, want true")
	}
	if !cfg.UI.StartMinimized {
		t.Error("UI.StartMinimized: got false, want true")
	}
	if !cfg.UI.ShowPreviewWindow {
		t.Error("UI.ShowPreviewWindow: got false, want true")
	}
	// v1 fixture sets previewalwaysontop=false, overriding the default true.
	if cfg.UI.PreviewAlwaysOnTop {
		t.Error("UI.PreviewAlwaysOnTop: got true, want false")
	}
	if cfg.SMTC.SelectedDevice != "Spotify.exe" {
		t.Errorf("SMTC.SelectedDevice: got %q, want \"Spotify.exe\"", cfg.SMTC.SelectedDevice)
	}
	if !cfg.Logging.Debug {
		t.Error("Logging.Debug: got false, want true")
	}
	if !cfg.Server.HotReload {
		t.Error("Server.HotReload: got false, want true")
	}
	if !cfg.Server.AllowRemote {
		t.Error("Server.AllowRemote: got false, want true")
	}
}

// TestLoad_V1DetectedAndMigrated verifies that Load detects a v1 file by
// the presence of a top-level "port" key and migrates it correctly.
func TestLoad_V1DetectedAndMigrated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte(v1Fixture), 0644); err != nil {
		t.Fatalf("write v1 fixture: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load v1 file: unexpected error: %v", err)
	}

	if cfg.Server.Port != 12345 {
		t.Errorf("Server.Port: got %d, want 12345", cfg.Server.Port)
	}
	if cfg.UI.Theme != "mini" {
		t.Errorf("UI.Theme: got %q, want \"mini\"", cfg.UI.Theme)
	}
	if !cfg.UI.AutoStart {
		t.Error("UI.AutoStart: got false, want true")
	}
	if cfg.SMTC.SelectedDevice != "Spotify.exe" {
		t.Errorf("SMTC.SelectedDevice: got %q, want \"Spotify.exe\"", cfg.SMTC.SelectedDevice)
	}
	if !cfg.Server.AllowRemote {
		t.Error("Server.AllowRemote: got false, want true")
	}
}

// TestMigrateV1_DefaultsPreserved verifies that v1 fields absent from the
// JSON fall back to DefaultConfig values rather than zero values.
func TestMigrateV1_DefaultsPreserved(t *testing.T) {
	// Only "port" present — all other v1 fields absent.
	cfg, err := migrateV1([]byte(`{"port":9000}`))
	if err != nil {
		t.Fatalf("migrateV1: unexpected error: %v", err)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("Server.Port: got %d, want 9000", cfg.Server.Port)
	}
	// Theme and PreviewAlwaysOnTop must keep defaults.
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
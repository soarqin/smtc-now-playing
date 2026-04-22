package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	Port               int    `json:"port"`
	Theme              string `json:"theme"`
	AutoStart          bool   `json:"autostart"`
	StartMinimized     bool   `json:"startminimized"`
	ShowPreviewWindow  bool   `json:"showpreviewwindow"`
	PreviewAlwaysOnTop bool   `json:"previewalwaysontop"`
	SelectedDevice     string `json:"selecteddevice"`
	Debug              bool   `json:"debug"`
	HotReload          bool   `json:"hotReload"`
	// ControlAllowRemote allows media control endpoints to be accessed from
	// non-localhost addresses when true. Defaults to false (localhost-only).
	ControlAllowRemote bool `json:"controlAllowRemote"`
}

var (
	config *Config
	// mu serializes concurrent Save() calls from GUI / server threads.
	// Field-level access on the shared *Config is still the caller's
	// responsibility, but all disk I/O (Load/Save) holds this mutex so
	// no partial/interleaved writes ever reach the config file.
	mu sync.Mutex
)

func init() {
	config = &Config{
		Port:               11451,
		Theme:              "default",
		AutoStart:          false,
		StartMinimized:     false,
		ShowPreviewWindow:  false,
		PreviewAlwaysOnTop: true,
	}
}

func Get() *Config {
	return config
}

func Load() error {
	mu.Lock()
	defer mu.Unlock()

	dir := os.Args[0]
	dir = filepath.Dir(dir)
	portableConfigPath := filepath.Join(dir, "portable_config.json")
	if _, err := os.Stat(portableConfigPath); err == nil {
		err := loadConfigFromFile(portableConfigPath, config)
		if err == nil {
			slog.Info("config loaded", "port", config.Port, "theme", config.Theme)
		}
		return err
	}

	appDataConfigPath := appDataConfigFile()
	if appDataConfigPath == "" {
		slog.Info("config loaded (defaults, APPDATA unset)", "port", config.Port, "theme", config.Theme)
		return nil
	}
	if _, err := os.Stat(appDataConfigPath); err == nil {
		err := loadConfigFromFile(appDataConfigPath, config)
		if err == nil {
			slog.Info("config loaded", "port", config.Port, "theme", config.Theme)
		}
		return err
	}

	slog.Info("config loaded", "port", config.Port, "theme", config.Theme)
	return nil
}

func Save() error {
	mu.Lock()
	defer mu.Unlock()

	dir := os.Args[0]
	dir = filepath.Dir(dir)
	portableConfigPath := filepath.Join(dir, "portable_config.json")
	if _, err := os.Stat(portableConfigPath); err == nil {
		return saveConfigToFile(portableConfigPath, config)
	}

	appDataConfigPath := appDataConfigFile()
	if appDataConfigPath == "" {
		return fmt.Errorf("APPDATA not set; cannot save config")
	}
	appDataConfigDir := filepath.Dir(appDataConfigPath)
	// 0700: restrict to current user. Config may contain future user-specific settings.
	if err := os.MkdirAll(appDataConfigDir, 0700); err != nil {
		return fmt.Errorf("create config dir %q: %w", appDataConfigDir, err)
	}
	return saveConfigToFile(appDataConfigPath, config)
}

// appDataConfigFile returns the installed-mode config path, or "" if APPDATA
// is unset (in which case the caller should fall back to defaults / skip save).
func appDataConfigFile() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "soarqin", "smtc-now-playing", "config.json")
}

func loadConfigFromFile(path string, cfg *Config) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	err = decoder.Decode(cfg)
	if err != nil {
		return err
	}
	return nil
}

// saveConfigToFile writes cfg to path atomically: serialize to a sibling
// temp file, fsync it, then rename over the target. This guarantees the
// config file is never left partially-written even if the process is
// killed mid-save.
func saveConfigToFile(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tmpPath := tmp.Name()
	// Ensure the temp file is cleaned up on any failure path.
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config: %w", err)
	}
	// os.Rename is atomic on Windows when source and target are on the
	// same volume (which they are — both live in the APPDATA dir).
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp config to %q: %w", path, err)
	}
	cleanup = false
	return nil
}

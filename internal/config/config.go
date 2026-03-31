package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
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

var config *Config

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

	appDataConfigPath := filepath.Join(os.Getenv("APPDATA"), "soarqin", "smtc-now-playing", "config.json")
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
	dir := os.Args[0]
	dir = filepath.Dir(dir)
	portableConfigPath := filepath.Join(dir, "portable_config.json")
	if _, err := os.Stat(portableConfigPath); err == nil {
		return saveConfigToFile(portableConfigPath, config)
	}

	appDataConfigDir := filepath.Join(os.Getenv("APPDATA"), "soarqin", "smtc-now-playing")
	os.MkdirAll(appDataConfigDir, 0755)
	appDataConfigPath := filepath.Join(appDataConfigDir, "config.json")
	return saveConfigToFile(appDataConfigPath, config)
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

func saveConfigToFile(path string, cfg *Config) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	return encoder.Encode(cfg)
}

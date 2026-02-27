package config

import (
	"encoding/json"
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
		return loadConfigFromFile(portableConfigPath, config)
	}

	appDataConfigPath := filepath.Join(os.Getenv("APPDATA"), "soarqin", "smtc-now-playing", "config.json")
	if _, err := os.Stat(appDataConfigPath); err == nil {
		return loadConfigFromFile(appDataConfigPath, config)
	}

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

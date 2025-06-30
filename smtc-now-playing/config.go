package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is the configuration for the application
// It is loaded from:
//  - `portable_config.json` file, located in the same directory as the executable if exists, otherwise use next one
//  - `config.json` file, located in the %APPDATA%/soarqin/smtc-now-playing/ if exists, otherwise it is created with default values

type Config struct {
	Port      int    `json:"port"`
	Theme     string `json:"theme"`
	AutoStart bool   `json:"autostart"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:      11451,
		Theme:     "default",
		AutoStart: false,
	}

	dir := os.Args[0]
	dir = filepath.Dir(dir)
	portableConfigPath := filepath.Join(dir, "portable_config.json")
	if _, err := os.Stat(portableConfigPath); err == nil {
		err = loadConfigFromFile(portableConfigPath, cfg)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	appDataConfigPath := filepath.Join(os.Getenv("APPDATA"), "soarqin", "smtc-now-playing", "config.json")
	if _, err := os.Stat(appDataConfigPath); err == nil {
		err = loadConfigFromFile(appDataConfigPath, cfg)
		if err != nil {
			return nil, err
		}
		return cfg, nil
	}

	return cfg, nil
}

func SaveConfig(cfg *Config) error {
	dir := os.Args[0]
	dir = filepath.Dir(dir)
	portableConfigPath := filepath.Join(dir, "portable_config.json")
	if _, err := os.Stat(portableConfigPath); err == nil {
		return saveConfigToFile(portableConfigPath, cfg)
	}

	appDataConfigDir := filepath.Join(os.Getenv("APPDATA"), "soarqin", "smtc-now-playing")
	os.MkdirAll(appDataConfigDir, 0755)
	appDataConfigPath := filepath.Join(appDataConfigDir, "config.json")
	return saveConfigToFile(appDataConfigPath, cfg)
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

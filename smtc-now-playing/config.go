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

var config *Config

func init() {
	config = &Config{
		Port:      11451,
		Theme:     "default",
		AutoStart: false,
	}
}

func GetConfig() *Config {
	return config
}

func LoadConfig() error {
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

func SaveConfig() error {
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

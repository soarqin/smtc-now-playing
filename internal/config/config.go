package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port        int  `json:"port"`
	AllowRemote bool `json:"allowRemote"`
	HotReload   bool `json:"hotReload"`
}

// UIConfig holds user interface preferences.
type UIConfig struct {
	Theme              string `json:"theme"`
	AutoStart          bool   `json:"autoStart"`
	StartMinimized     bool   `json:"startMinimized"`
	ShowPreviewWindow  bool   `json:"showPreviewWindow"`
	PreviewAlwaysOnTop bool   `json:"previewAlwaysOnTop"`
}

// SMTCConfig holds System Media Transport Controls settings.
type SMTCConfig struct {
	SelectedDevice string `json:"selectedDevice"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level string `json:"level"`
	Debug bool   `json:"debug"`
}

// Config is the application configuration.
type Config struct {
	Server  ServerConfig  `json:"server"`
	UI      UIConfig      `json:"ui"`
	SMTC    SMTCConfig    `json:"smtc"`
	Logging LoggingConfig `json:"logging"`
}

// DefaultConfig returns a Config populated with application defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 11451,
		},
		UI: UIConfig{
			Theme:              "default",
			PreviewAlwaysOnTop: true,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// ResolvePath returns the config file path to use. It returns the portable
// config path (next to the executable) if portable_config.json exists there;
// otherwise returns the APPDATA path. Returns an error if APPDATA is unset
// and no portable file is found.
func ResolvePath() (string, error) {
	exe := os.Args[0]
	dir := filepath.Dir(exe)
	portablePath := filepath.Join(dir, "portable_config.json")
	if _, err := os.Stat(portablePath); err == nil {
		return portablePath, nil
	}
	appDataPath := appDataConfigFile()
	if appDataPath == "" {
		return "", nil
	}
	return appDataPath, nil
}

// Load reads config from path, migrating v1 flat JSON to the current nested
// format if needed. Missing fields are filled from DefaultConfig. If the file
// does not exist, Load returns DefaultConfig with a nil error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg, err := parseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config %q: %w", path, err)
	}
	slog.Info("config loaded", "port", cfg.Server.Port, "theme", cfg.UI.Theme)
	return cfg, nil
}

// Save validates cfg, then atomically writes it to path.
func (c *Config) Save(path string) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return saveConfigToFile(path, c)
}

// Validate returns an error if any config value is out of range.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("port %d out of range [1, 65535]", c.Server.Port)
	}
	if c.UI.Theme == "" {
		return errors.New("theme must not be empty")
	}
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("logging level %q must be one of: debug, info, warn, error", c.Logging.Level)
	}
	return nil
}

// parseConfig detects v1 vs v2 JSON format and returns a merged Config.
func parseConfig(data []byte) (*Config, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if _, isV1 := raw["port"]; isV1 {
		return migrateV1(data)
	}
	// v2: unmarshal into defaults so missing fields retain default values.
	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// migrateV1 reads a v1 flat config JSON and maps it into the v2 nested Config.
func migrateV1(rawJSON []byte) (*Config, error) {
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(rawJSON, &flat); err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	set := func(key string, dst any) {
		if v, ok := flat[key]; ok {
			_ = json.Unmarshal(v, dst)
		}
	}
	set("port", &cfg.Server.Port)
	set("controlAllowRemote", &cfg.Server.AllowRemote)
	set("hotReload", &cfg.Server.HotReload)
	set("theme", &cfg.UI.Theme)
	set("autostart", &cfg.UI.AutoStart)
	set("startminimized", &cfg.UI.StartMinimized)
	set("showpreviewwindow", &cfg.UI.ShowPreviewWindow)
	set("previewalwaysontop", &cfg.UI.PreviewAlwaysOnTop)
	set("selecteddevice", &cfg.SMTC.SelectedDevice)
	set("debug", &cfg.Logging.Debug)
	if cfg.Logging.Debug {
		cfg.Logging.Level = "debug"
	}
	slog.Info("config migrated from v1")
	return cfg, nil
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

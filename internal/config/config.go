// Package config owns SeatHawk's runtime configuration loading.
//
// Phase 1 intentionally keeps configuration small: one JSON file with stable
// local paths for the config file and SQLite database. Additional daemon
// settings should be added here when the rest of the app needs them.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the resolved runtime configuration for a SeatHawk process.
type Config struct {
	ConfigPath   string `json:"-"`
	DatabasePath string `json:"database_path"`
}

// Load reads configuration from path.
//
// If path is empty, Load uses SeatHawk's default per-user config location. If
// that file does not exist yet, Load creates a default config and returns it.
func Load(path string) (Config, error) {
	var err error
	if path == "" {
		path, err = defaultConfigPath()
		if err != nil {
			return Config{}, err
		}
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg, err := defaultConfig(path)
		if err != nil {
			return Config{}, err
		}
		if err := writeDefaultConfig(path, cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(contents, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	cfg.ConfigPath = path
	if cfg.DatabasePath == "" {
		cfg.DatabasePath, err = defaultDatabasePath()
		if err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

func defaultConfig(path string) (Config, error) {
	databasePath, err := defaultDatabasePath()
	if err != nil {
		return Config{}, err
	}
	return Config{
		ConfigPath:   path,
		DatabasePath: databasePath,
	}, nil
}

func defaultConfigPath() (string, error) {
	dataDir, err := defaultDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "config.json"), nil
}

func defaultDatabasePath() (string, error) {
	dataDir, err := defaultDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "seathawk.db"), nil
}

func defaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}

	return filepath.Join(home, ".config", "seathawk"), nil
}

func writeDefaultConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// ConfigPath is runtime metadata, not user configuration.
	contents, err := json.MarshalIndent(struct {
		DatabasePath string `json:"database_path"`
	}{
		DatabasePath: cfg.DatabasePath,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

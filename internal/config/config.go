package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// Config holds the global bcommit settings.
type Config struct {
	AutoCommit   bool   `json:"auto_commit"`
	Model        string `json:"model"`
	BranchPrefix string `json:"branch_prefix,omitempty"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		AutoCommit: false,
		Model:      "qwen2.5-coder:3b",
	}
}

// configDir returns the config directory path.
func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "bcommit")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "bcommit")
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(configDir(), "config.json")
}

// Load reads the config file, falling back to defaults for missing fields.
func Load() Config {
	cfg := Default()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}

	// Unmarshal on top of defaults — missing fields keep their default values
	json.Unmarshal(data, &cfg)
	return cfg
}

// Save writes the config to disk, creating the directory if needed.
func Save(cfg Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	data = append(data, '\n')
	return os.WriteFile(ConfigPath(), data, 0644)
}

// Set updates a single config field by key and saves to disk.
func Set(key, value string) error {
	cfg := Load()

	switch key {
	case "auto_commit":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid value for auto_commit: %q (use true/false)", value)
		}
		cfg.AutoCommit = b
	case "model":
		if value == "" {
			return fmt.Errorf("model cannot be empty")
		}
		cfg.Model = value
	case "branch_prefix":
		if value != "" {
			validPrefix := regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
			if !validPrefix.MatchString(value) {
				return fmt.Errorf("invalid branch_prefix: %q (only letters, numbers, dots, hyphens, underscores)", value)
			}
		}
		cfg.BranchPrefix = value
	default:
		return fmt.Errorf("unknown config key: %q\n\nAvailable keys: auto_commit, model, branch_prefix", key)
	}

	return Save(cfg)
}

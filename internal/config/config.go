// Package config loads auto-command configuration from a TOML file and the
// environment, applying built-in defaults. It never logs or echoes the API key.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DefaultModel is used when neither the config file nor the environment
// specifies a model.
const DefaultModel = "openai/gpt-4o-mini"

// DefaultMaxSuggestions is used when the config file does not specify a value.
const DefaultMaxSuggestions = 3

// Config holds the resolved settings for auto-command. The public surface is
// intentionally small.
type Config struct {
	// APIKey is the OpenRouter API key. It is never logged or printed.
	APIKey string `toml:"api_key"`
	// Model is the OpenRouter model identifier.
	Model string `toml:"model"`
	// MaxSuggestions caps how many command suggestions are requested.
	MaxSuggestions int `toml:"max_suggestions"`
}

// MissingError describes required configuration that could not be resolved. It
// deliberately names only which fields are missing, never any secret value.
type MissingError struct {
	Fields []string
}

func (e *MissingError) Error() string {
	return fmt.Sprintf("missing required configuration: %v", e.Fields)
}

// Path returns the path to the config file, resolved from XDG_CONFIG_HOME with
// a fallback to ~/.config. The returned path is
// <base>/auto-command/config.toml.
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "auto-command", "config.toml"), nil
}

// Load resolves configuration by reading the TOML file (if present), overlaying
// environment variables, and applying defaults. It returns a *MissingError when
// a required value (currently the API key) cannot be resolved.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	cfg := &Config{}

	// File values (lowest precedence above defaults). A missing file is fine.
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	// Environment overrides file values.
	if v := os.Getenv("AUTO_COMMAND_API_KEY"); v != "" {
		cfg.APIKey = v
	} else if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("AUTO_COMMAND_MODEL"); v != "" {
		cfg.Model = v
	}

	// Defaults fill anything still unset.
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.MaxSuggestions <= 0 {
		cfg.MaxSuggestions = DefaultMaxSuggestions
	}

	if cfg.APIKey == "" {
		return cfg, &MissingError{Fields: []string{"api_key"}}
	}

	return cfg, nil
}

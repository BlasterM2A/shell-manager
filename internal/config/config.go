// Package config loads what the user wants (intent). Facts the tool writes
// about the machine live in shellmanager's state, not here.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const envPrefix = "SHELL_MANAGER_"

var levels = []string{"debug", "info", "warn", "error"}

// Config is the user's configuration.
type Config struct {
	LogLevel string `koanf:"log_level"`
}

// DefaultPath is where the config file lives.
func DefaultPath(home string) string {
	return filepath.Join(home, ".config", "shell-manager", "config.toml")
}

// Load merges defaults, the file at path (optional), SHELL_MANAGER_* env vars
// and overrides, in that order.
func Load(path string, overrides map[string]any) (Config, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(map[string]any{"log_level": "info"}, "."), nil); err != nil {
		return Config{}, err
	}
	if _, err := os.Stat(path); err == nil {
		if err := k.Load(file.Provider(path), toml.Parser()); err != nil {
			return Config{}, fmt.Errorf("reading %s: %w", path, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Config{}, err
	}
	err := k.Load(env.Provider(".", env.Opt{
		Prefix: envPrefix,
		TransformFunc: func(key, value string) (string, any) {
			return strings.ToLower(strings.TrimPrefix(key, envPrefix)), value
		},
	}), nil)
	if err != nil {
		return Config{}, err
	}
	if len(overrides) > 0 {
		if err := k.Load(confmap.Provider(overrides, "."), nil); err != nil {
			return Config{}, err
		}
	}
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, err
	}
	if !slices.Contains(levels, cfg.LogLevel) {
		return Config{}, fmt.Errorf("invalid log_level %q (want one of %s)", cfg.LogLevel, strings.Join(levels, ", "))
	}
	return cfg, nil
}

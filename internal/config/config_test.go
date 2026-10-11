package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BlasterM2A/shell-manager/internal/config"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}

func TestDefaultsWhenFileIsMissing(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "missing.toml"), nil)
	require.NoError(t, err)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestPrecedence(t *testing.T) {
	path := writeConfig(t, `log_level = "warn"`)

	cfg, err := config.Load(path, nil)
	require.NoError(t, err)
	assert.Equal(t, "warn", cfg.LogLevel, "file overrides default")

	t.Setenv("SHELL_MANAGER_LOG_LEVEL", "error")
	cfg, err = config.Load(path, nil)
	require.NoError(t, err)
	assert.Equal(t, "error", cfg.LogLevel, "env overrides file")

	cfg, err = config.Load(path, map[string]any{"log_level": "debug"})
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.LogLevel, "overrides win")
}

func TestInvalidLevel(t *testing.T) {
	_, err := config.Load(writeConfig(t, `log_level = "loud"`), nil)
	assert.ErrorContains(t, err, `invalid log_level "loud"`)
}

func TestDefaultPath(t *testing.T) {
	assert.Equal(t, "/h/.shm/config.toml", config.DefaultPath("/h"))
}

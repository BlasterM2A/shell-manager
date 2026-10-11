package log_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BlasterM2A/shell-manager/internal/log"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

func TestFileGetsEverythingConsoleRespectsLevel(t *testing.T) {
	file := filepath.Join(t.TempDir(), "state", "shell-manager.log")
	var console bytes.Buffer
	logger, closeFn, err := log.New(log.Options{
		File: file, Level: slog.LevelInfo, Console: &console, Styles: styles.New(false),
	})
	require.NoError(t, err)

	logger.Debug("debug detail", "cmd", "mise --version")
	logger.Warn("careful")
	require.NoError(t, closeFn())

	data, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(data), "debug detail")
	assert.Contains(t, string(data), "careful")

	assert.NotContains(t, console.String(), "debug detail")
	assert.Contains(t, console.String(), styles.Glyph(styles.StatusWarn)+" careful")
}

func TestWithoutConsoleOrFile(t *testing.T) {
	logger, closeFn, err := log.New(log.Options{Level: slog.LevelInfo, Styles: styles.New(false)})
	require.NoError(t, err)
	logger.Info("nowhere")
	require.NoError(t, closeFn())
}

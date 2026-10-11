// Package log builds the slog logger: a plain-text debug log file plus an
// optional console handler styled with ui/styles, so console log lines look
// like the rest of the output.
package log

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	charmlog "charm.land/log/v2"

	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// Options configure New.
type Options struct {
	// File receives every record (debug and up). Empty disables it.
	File string
	// Level filters the console.
	Level slog.Level
	// Console receives styled records at Level and up. Nil disables it.
	Console io.Writer
	Styles  styles.Styles
}

// New returns the logger and a function that closes the log file.
func New(o Options) (*slog.Logger, func() error, error) {
	var handlers []slog.Handler
	closeFn := func() error { return nil }
	if o.File != "" {
		if err := os.MkdirAll(filepath.Dir(o.File), 0o755); err != nil {
			return nil, nil, err
		}
		f, err := os.OpenFile(o.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, err
		}
		handlers = append(handlers, slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}))
		closeFn = f.Close
	}
	if o.Console != nil {
		console := charmlog.NewWithOptions(o.Console, charmlog.Options{Level: charmlog.Level(o.Level)})
		console.SetStyles(consoleStyles(o.Styles))
		handlers = append(handlers, console)
	}
	return slog.New(slog.NewMultiHandler(handlers...)), closeFn, nil
}

// consoleStyles maps log levels to the shared status icons and colors.
func consoleStyles(s styles.Styles) *charmlog.Styles {
	cs := charmlog.DefaultStyles()
	cs.Levels[charmlog.DebugLevel] = s.Muted.SetString(styles.Glyph(styles.StatusInfo))
	cs.Levels[charmlog.InfoLevel] = s.Status(styles.StatusInfo).SetString(styles.Glyph(styles.StatusInfo))
	cs.Levels[charmlog.WarnLevel] = s.Status(styles.StatusWarn).SetString(styles.Glyph(styles.StatusWarn))
	cs.Levels[charmlog.ErrorLevel] = s.Status(styles.StatusFail).SetString(styles.Glyph(styles.StatusFail))
	cs.Levels[charmlog.FatalLevel] = s.Status(styles.StatusFail).SetString(styles.Glyph(styles.StatusFail))
	return cs
}

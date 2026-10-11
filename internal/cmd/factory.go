package cmd

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/BlasterM2A/shell-manager/internal/config"
	"github.com/BlasterM2A/shell-manager/internal/iostreams"
	applog "github.com/BlasterM2A/shell-manager/internal/log"
	"github.com/BlasterM2A/shell-manager/internal/selfupdate"
	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
	"github.com/BlasterM2A/shell-manager/internal/ui/common"
)

// Factory builds dependencies lazily, after flags are parsed (gh pattern).
type Factory struct {
	IOStreams *iostreams.IOStreams
	Home      string

	// Global flags; set by cobra before any getter below is called.
	Plain, Verbose, Quiet bool

	Config  func() (config.Config, error)
	Logger  func() (*slog.Logger, error)
	System  func() (*shellmanager.RealSystem, error)
	Engine  func() (*shellmanager.Engine, error)
	Updater func() *selfupdate.Updater

	// Executable is the real path of the running binary; Exec replaces the
	// process with another program (syscall.Exec in production).
	Executable func() string
	Exec       func(path string, args []string) error
}

// NewFactory returns the production factory.
func NewFactory(ios *iostreams.IOStreams, home string) *Factory {
	f := &Factory{IOStreams: ios, Home: home, Executable: executablePath}
	f.Exec = func(path string, args []string) error { return syscall.Exec(path, args, os.Environ()) }
	f.Config = sync.OnceValues(func() (config.Config, error) {
		return config.Load(config.DefaultPath(home), nil)
	})
	f.Logger = sync.OnceValues(func() (*slog.Logger, error) {
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}
		var level slog.Level
		if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
			return nil, err
		}
		if f.Verbose {
			level = slog.LevelDebug
		}
		if f.Quiet {
			level = slog.LevelError
		}
		opts := applog.Options{File: shellmanager.NewPaths(home).LogFile, Level: level, Styles: ios.Styles()}
		if !f.UseTUI() {
			opts.Console = ios.ErrOut
		}
		// The log file stays open for the life of the process.
		logger, _, err := applog.New(opts)
		return logger, err
	})
	f.System = sync.OnceValues(func() (*shellmanager.RealSystem, error) {
		logger, err := f.Logger()
		if err != nil {
			return nil, err
		}
		return shellmanager.NewRealSystem(home, logger), nil
	})
	f.Engine = sync.OnceValues(func() (*shellmanager.Engine, error) {
		sys, err := f.System()
		if err != nil {
			return nil, err
		}
		logger, err := f.Logger()
		if err != nil {
			return nil, err
		}
		client := retryablehttp.NewClient()
		client.RetryMax = 3
		client.Logger = logger
		e, err := shellmanager.NewEngine(sys, shellmanager.HTTPFetcher{
			Client: client.StandardClient(),
			Token:  os.Getenv("GITHUB_TOKEN"),
		})
		if err != nil {
			return nil, err
		}
		if os.Geteuid() == 0 {
			e.Sudo = nil
		}
		e.Executable = executablePath()
		return e, nil
	})
	f.Updater = sync.OnceValue(func() *selfupdate.Updater { return selfupdate.New(repoSlug, version) })
	return f
}

// executablePath is the real path of the running binary (symlinks such as
// the shm alias resolved), or "" when it cannot be determined.
func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real
	}
	return exe
}

// UseTUI reports whether to show the interactive UI.
func (f *Factory) UseTUI() bool { return !f.Plain && f.IOStreams.IsTTY() }

// Common is what UI components receive.
func (f *Factory) Common() common.Common { return common.New(f.IOStreams.Styles()) }

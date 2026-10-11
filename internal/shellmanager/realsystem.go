package shellmanager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// TerminalHandoff runs fn while the caller's UI has released the terminal.
type TerminalHandoff func(fn func() error) error

// RealSystem is the System backed by the real machine.
type RealSystem struct {
	home    string
	logger  *slog.Logger
	handoff TerminalHandoff
	// execFn replaces command execution in tests.
	execFn func(ctx context.Context, c Cmd) ([]byte, error)
}

var (
	_ System = (*RealSystem)(nil)
	_ System = (*DryRunSystem)(nil)
)

// NewRealSystem returns the real System for the user whose home is home.
func NewRealSystem(home string, logger *slog.Logger) *RealSystem {
	return &RealSystem{home: home, logger: logger}
}

// SetTerminalHandoff sets how interactive commands get the terminal; nil
// runs them directly.
func (s *RealSystem) SetTerminalHandoff(h TerminalHandoff) { s.handoff = h }

func (s *RealSystem) HomeDir() string                           { return s.home }
func (s *RealSystem) Getenv(key string) string                  { return os.Getenv(key) }
func (s *RealSystem) ReadFile(p string) ([]byte, error)         { return os.ReadFile(p) }
func (s *RealSystem) Rename(oldpath, newpath string) error      { return os.Rename(oldpath, newpath) }
func (s *RealSystem) Remove(p string) error                     { return os.Remove(p) }
func (s *RealSystem) RemoveAll(p string) error                  { return os.RemoveAll(p) }
func (s *RealSystem) MkdirAll(p string, perm fs.FileMode) error { return os.MkdirAll(p, perm) }
func (s *RealSystem) Stat(p string) (fs.FileInfo, error)        { return os.Stat(p) }
func (s *RealSystem) Glob(pattern string) ([]string, error)     { return filepath.Glob(pattern) }
func (s *RealSystem) Lstat(p string) (fs.FileInfo, error)       { return os.Lstat(p) }
func (s *RealSystem) Readlink(p string) (string, error)         { return os.Readlink(p) }
func (s *RealSystem) Symlink(target, link string) error         { return os.Symlink(target, link) }

func (s *RealSystem) WriteFile(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// searchPath is PATH as commands see it: user binaries and mise shims first,
// so tools installed during this run are found without a new shell.
func (s *RealSystem) searchPath() string {
	return strings.Join([]string{
		filepath.Join(s.home, ".local", "bin"),
		filepath.Join(s.miseDataDir(), "shims"),
		os.Getenv("PATH"),
	}, string(os.PathListSeparator))
}

// miseDataDir is where mise keeps its data: MISE_DATA_DIR, else
// $XDG_DATA_HOME/mise, else ~/.local/share/mise.
func (s *RealSystem) miseDataDir() string {
	if dir := os.Getenv("MISE_DATA_DIR"); dir != "" {
		return dir
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "mise")
	}
	return filepath.Join(s.home, ".local", "share", "mise")
}

func (s *RealSystem) LookPath(file string) (string, error) {
	for _, dir := range filepath.SplitList(s.searchPath()) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, file)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s: %w", file, exec.ErrNotFound)
}

func (s *RealSystem) Run(ctx context.Context, c Cmd) ([]byte, error) {
	if s.execFn != nil {
		return s.execFn(ctx, c)
	}
	name := c.Name
	if !strings.Contains(name, "/") {
		p, err := s.LookPath(name)
		if err != nil {
			return nil, err
		}
		name = p
	}
	cmd := exec.CommandContext(ctx, name, c.Args...)
	cmd.Env = append(os.Environ(), "PATH="+s.searchPath())
	if c.Interactive {
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		s.logger.Debug("exec (interactive)", "cmd", c.String())
		if s.handoff != nil {
			return nil, s.handoff(cmd.Run)
		}
		return nil, cmd.Run()
	}
	out, err := cmd.CombinedOutput()
	s.logger.Debug("exec", "cmd", c.String(), "output", string(out), "err", err)
	if err != nil {
		return out, fmt.Errorf("%s: %w%s", c, err, tail(out))
	}
	return out, nil
}

// tail returns the last lines of a command's output for error messages.
func tail(out []byte) string {
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return ""
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return ": " + strings.Join(lines, " | ")
}

func (s *RealSystem) Lock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

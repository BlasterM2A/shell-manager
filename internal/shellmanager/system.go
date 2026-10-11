// Package shellmanager is the domain: the tool catalog, how tools are
// installed and updated, the managed shell config, and doctor. It never
// prints; it reports through events and return values.
package shellmanager

import (
	"context"
	"errors"
	"io/fs"
	"strings"
)

// ErrLocked is returned when another shell-manager run holds the lock.
var ErrLocked = errors.New("another shell-manager run is in progress")

// Cmd describes a command to execute.
type Cmd struct {
	Name string
	Args []string
	// Interactive commands (sudo, chsh) get the user's terminal so they can
	// prompt for a password.
	Interactive bool
}

func (c Cmd) String() string {
	return strings.Join(append([]string{c.Name}, c.Args...), " ")
}

// System is every side effect shell-manager has on the machine. The domain
// never uses os, os/exec or the filesystem directly.
type System interface {
	HomeDir() string
	Getenv(key string) string
	ReadFile(path string) ([]byte, error)
	// WriteFile writes atomically (temp file + rename), creating parent dirs.
	WriteFile(path string, data []byte, perm fs.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(path string) error
	RemoveAll(path string) error
	MkdirAll(path string, perm fs.FileMode) error
	Stat(path string) (fs.FileInfo, error)
	Glob(pattern string) ([]string, error)
	// Lstat is Stat without following symlinks.
	Lstat(path string) (fs.FileInfo, error)
	Readlink(path string) (string, error)
	Symlink(target, link string) error
	// LookPath searches the same PATH that Run uses.
	LookPath(file string) (string, error)
	Run(ctx context.Context, c Cmd) ([]byte, error)
	// Lock takes an exclusive, non-blocking lock; ErrLocked if it is held.
	Lock(path string) (unlock func(), err error)
}

package shellmanager

import (
	"path/filepath"
	"strings"
)

// Paths are the locations shell-manager reads and writes.
type Paths struct {
	Home      string
	ShmDir    string // ~/.shm: shm's own config, state, log, lock, tmp
	StateFile string
	LockFile  string
	LogFile   string
	TmpDir    string
	Zshenv    string // ~/.zshenv: holds an shm block
	Zshrc     string // ~/.zshrc: holds an shm block
	ZshDir    string // ~/.zsh: the zsh config, shm's files and the user's
	AliasLink string // ~/.local/bin/shm → shell-manager
}

// NewPaths returns the paths for a home directory.
func NewPaths(home string) Paths {
	shm := filepath.Join(home, ".shm")
	return Paths{
		Home:      home,
		ShmDir:    shm,
		StateFile: filepath.Join(shm, "state.toml"),
		LockFile:  filepath.Join(shm, "lock"),
		LogFile:   filepath.Join(shm, "shm.log"),
		TmpDir:    filepath.Join(shm, "tmp"),
		Zshenv:    filepath.Join(home, ".zshenv"),
		Zshrc:     filepath.Join(home, ".zshrc"),
		ZshDir:    filepath.Join(home, ".zsh"),
		AliasLink: filepath.Join(home, ".local", "bin", AliasName),
	}
}

// Expand replaces a leading "~/" with the home directory.
func (p Paths) Expand(s string) string {
	if rest, ok := strings.CutPrefix(s, "~/"); ok {
		return filepath.Join(p.Home, rest)
	}
	return s
}

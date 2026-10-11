package shellmanager

import (
	"path/filepath"
	"strings"
)

// Paths are the locations shell-manager reads and writes.
type Paths struct {
	Home           string
	ConfigDir      string // ~/.config/shell-manager
	ZshD           string // ~/.config/shell-manager/zsh.d
	GeneratedZshrc string // ~/.config/shell-manager/zshrc
	Stub           string // ~/.zshrc
	UserZshDir     string // ~/.config/zsh
	ShmDir         string // ~/.shm: shm's own config, state, log, lock, tmp
	StateFile      string
	LockFile       string
	LogFile        string
	TmpDir         string
	AliasLink      string // ~/.local/bin/shm → shell-manager
}

// NewPaths returns the paths for a home directory.
func NewPaths(home string) Paths {
	config := filepath.Join(home, ".config", "shell-manager")
	shm := filepath.Join(home, ".shm")
	return Paths{
		Home:           home,
		ConfigDir:      config,
		ZshD:           filepath.Join(config, "zsh.d"),
		GeneratedZshrc: filepath.Join(config, "zshrc"),
		Stub:           filepath.Join(home, ".zshrc"),
		UserZshDir:     filepath.Join(home, ".config", "zsh"),
		ShmDir:         shm,
		StateFile:      filepath.Join(shm, "state.toml"),
		LockFile:       filepath.Join(shm, "lock"),
		LogFile:        filepath.Join(shm, "shm.log"),
		TmpDir:         filepath.Join(shm, "tmp"),
		AliasLink:      filepath.Join(home, ".local", "bin", AliasName),
	}
}

// Expand replaces a leading "~/" with the home directory.
func (p Paths) Expand(s string) string {
	if rest, ok := strings.CutPrefix(s, "~/"); ok {
		return filepath.Join(p.Home, rest)
	}
	return s
}

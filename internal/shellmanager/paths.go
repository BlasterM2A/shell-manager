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
	StateDir       string // ~/.local/state/shell-manager
	StateFile      string
	LockFile       string
	LogFile        string
	TmpDir         string
	AliasLink      string // ~/.local/bin/shm → shell-manager
}

// NewPaths returns the paths for a home directory.
func NewPaths(home string) Paths {
	config := filepath.Join(home, ".config", "shell-manager")
	state := filepath.Join(home, ".local", "state", "shell-manager")
	return Paths{
		Home:           home,
		ConfigDir:      config,
		ZshD:           filepath.Join(config, "zsh.d"),
		GeneratedZshrc: filepath.Join(config, "zshrc"),
		Stub:           filepath.Join(home, ".zshrc"),
		UserZshDir:     filepath.Join(home, ".config", "zsh"),
		StateDir:       state,
		StateFile:      filepath.Join(state, "state.toml"),
		LockFile:       filepath.Join(state, "lock"),
		LogFile:        filepath.Join(state, "shell-manager.log"),
		TmpDir:         filepath.Join(state, "tmp"),
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

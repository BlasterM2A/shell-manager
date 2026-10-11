package shellsetup

import (
	"path/filepath"
	"strings"
)

// Paths are the locations shell-setup reads and writes.
type Paths struct {
	Home           string
	ConfigDir      string // ~/.config/shell-setup
	ZshD           string // ~/.config/shell-setup/zsh.d
	GeneratedZshrc string // ~/.config/shell-setup/zshrc
	Stub           string // ~/.zshrc
	UserZshDir     string // ~/.config/zsh
	StateDir       string // ~/.local/state/shell-setup
	StateFile      string
	LockFile       string
	LogFile        string
	TmpDir         string
	AliasLink      string // ~/.local/bin/shs → shell-setup
}

// NewPaths returns the paths for a home directory.
func NewPaths(home string) Paths {
	config := filepath.Join(home, ".config", "shell-setup")
	state := filepath.Join(home, ".local", "state", "shell-setup")
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
		LogFile:        filepath.Join(state, "shell-setup.log"),
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

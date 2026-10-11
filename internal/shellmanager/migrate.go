package shellmanager

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

// oldName is the tool's name up to v0.1.x; setup and update migrate what it
// left on the machine.
const oldName = "shell-setup"

// oldInstall are the locations a shell-setup v0.1.x install used.
type oldInstall struct {
	ConfigDir string // ~/.config/shell-setup
	StateDir  string // ~/.local/state/shell-setup
	Binary    string // ~/.local/bin/shell-setup
	Alias     string // ~/.local/bin/shs
}

func oldInstallPaths(home string) oldInstall {
	return oldInstall{
		ConfigDir: filepath.Join(home, ".config", oldName),
		StateDir:  filepath.Join(home, ".local", "state", oldName),
		Binary:    filepath.Join(home, ".local", "bin", oldName),
		Alias:     filepath.Join(home, ".local", "bin", "shs"),
	}
}

// migrateFromShellSetup moves a shell-setup install to the new names. It runs
// before the lock is taken, since the lock lives in the new state directory.
// Entries are moved one by one and never overwrite existing ones (the logger
// has already created the new state directory and its log by now); whatever
// conflicts stays in the old directory, which doctor then reports.
func (e *Engine) migrateFromShellSetup() ([]FileReport, error) {
	old := oldInstallPaths(e.Paths.Home)
	var reports []FileReport

	moved, err := e.mergeDir(old.ConfigDir, e.Paths.ConfigDir)
	if err != nil {
		return reports, err
	}
	if len(moved) > 0 {
		reports = append(reports, FileReport{Path: e.Paths.ConfigDir, Result: ResultMigrated})
		// Until the shell phase rewrites ~/.zshrc, the old stub still sources
		// ~/.config/shell-setup/zshrc: keep that path working in case this run
		// stops early (apt failure, Ctrl-C). removeCompatLink drops it.
		if _, err := e.System.Lstat(old.ConfigDir); errors.Is(err, fs.ErrNotExist) {
			if err := e.System.Symlink(e.Paths.ConfigDir, old.ConfigDir); err != nil {
				return reports, err
			}
		}
	}

	// Files the new install.sh/run recreate are disposable in the old dir.
	moved, err = e.mergeDir(old.StateDir, e.Paths.StateDir, "bootstrap.sha256", "lock", "tmp")
	if err != nil {
		return reports, err
	}
	if len(moved) > 0 {
		reports = append(reports, FileReport{Path: e.Paths.StateDir, Result: ResultMigrated})
	}
	// Rewrite managed-file keys whenever the state file now in place may
	// still point into the old config directory.
	if err := e.rewriteStateKeys(old.ConfigDir); err != nil {
		return reports, err
	}

	for _, p := range []string{old.Binary, old.Alias} {
		info, err := e.System.Lstat(p)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return reports, err
		}
		if p == old.Alias && !e.isOldAliasLink(old.Alias, info) {
			continue // a real file, or a link to something else, belongs to the user
		}
		if err := e.System.Remove(p); err != nil {
			return reports, err
		}
		reports = append(reports, FileReport{Path: p, Result: ResultRemoved})
	}
	return reports, nil
}

// mergeDir moves every entry of from into to unless to already has an entry
// with that name; such a conflicting entry is deleted from from when its
// name is listed in disposable, and left in place otherwise. from is removed
// if it ended up empty; a symlink from (the compatibility link) is left
// alone. It returns the names it moved.
func (e *Engine) mergeDir(from, to string, disposable ...string) ([]string, error) {
	info, err := e.System.Lstat(from)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, nil
	}
	if err := e.System.MkdirAll(to, 0o755); err != nil {
		return nil, err
	}
	var entries []string
	for _, pattern := range []string{"*", ".*"} {
		matches, err := e.System.Glob(filepath.Join(from, pattern))
		if err != nil {
			return nil, err
		}
		entries = append(entries, matches...)
	}
	var moved []string
	for _, src := range entries {
		dst := filepath.Join(to, filepath.Base(src))
		if _, err := e.System.Lstat(dst); err == nil {
			if slices.Contains(disposable, filepath.Base(src)) {
				if err := e.System.RemoveAll(src); err != nil {
					return moved, err
				}
			}
			continue // never overwrite
		} else if !errors.Is(err, fs.ErrNotExist) {
			return moved, err
		}
		if err := e.System.Rename(src, dst); err != nil {
			return moved, err
		}
		moved = append(moved, filepath.Base(src))
	}
	_ = e.System.Remove(from) // only succeeds when empty; leftovers are reported by doctor
	return moved, nil
}

// rewriteStateKeys points managed-file entries recorded under the old config
// directory at the new one, so those files keep being recognized as
// unedited and are rewritten without backups.
func (e *Engine) rewriteStateKeys(oldConfigDir string) error {
	if _, err := e.System.Lstat(e.Paths.StateFile); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	st, err := LoadState(e.System, e.Paths.StateFile)
	if err != nil {
		return err
	}
	prefix := oldConfigDir + string(filepath.Separator)
	changed := false
	files := make(map[string]string, len(st.Files))
	for path, hash := range st.Files {
		if rest, ok := strings.CutPrefix(path, prefix); ok {
			path = filepath.Join(e.Paths.ConfigDir, rest)
			changed = true
		}
		files[path] = hash
	}
	if !changed {
		return nil
	}
	st.Files = files
	return st.Save(e.System, e.Paths.StateFile)
}

// oldInstallLeftovers lists shell-setup paths still present, for doctor.
func (e *Engine) oldInstallLeftovers() []string {
	old := oldInstallPaths(e.Paths.Home)
	var left []string
	for _, p := range []string{old.ConfigDir, old.StateDir, old.Binary} {
		if _, err := e.System.Lstat(p); err == nil {
			left = append(left, p)
		}
	}
	if info, err := e.System.Lstat(old.Alias); err == nil && e.isOldAliasLink(old.Alias, info) {
		left = append(left, old.Alias)
	}
	return left
}

// isOldAliasLink reports whether path is the shs link shell-setup created
// (pointing at a binary named shell-setup), not a link of the user's own.
func (e *Engine) isOldAliasLink(path string, info fs.FileInfo) bool {
	if info.Mode()&fs.ModeSymlink == 0 {
		return false
	}
	target, err := e.System.Readlink(path)
	return err == nil && filepath.Base(target) == oldName
}

// removeCompatLink drops the ~/.config/shell-setup link left by the
// migration once ~/.zshrc no longer sources the old path (a hand-edited stub
// keeps it, and doctor keeps reporting it).
func (e *Engine) removeCompatLink() error {
	link := oldInstallPaths(e.Paths.Home).ConfigDir
	info, err := e.System.Lstat(link)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return nil
	}
	if stub, err := e.System.ReadFile(e.Paths.Stub); err == nil && strings.Contains(string(stub), "/.config/"+oldName+"/") {
		return nil
	}
	return e.System.Remove(link)
}

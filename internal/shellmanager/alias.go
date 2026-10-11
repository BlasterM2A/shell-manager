package shellmanager

import (
	"errors"
	"io/fs"
	"path/filepath"
)

// AliasName is the short command linked to the shell-manager binary.
const AliasName = "shm"

// aliasTarget is what the alias link should point to: just the binary's name
// when both live in the same directory (survives moving that directory),
// its absolute path otherwise.
func (e *Engine) aliasTarget() string {
	if filepath.Dir(e.Executable) == filepath.Dir(e.Paths.AliasLink) {
		return filepath.Base(e.Executable)
	}
	return e.Executable
}

// ensureAlias points ~/.local/bin/shm at the running binary. A real file
// with that name belongs to the user and is left alone (ResultSkipped).
func (e *Engine) ensureAlias() (FileReport, error) {
	link := e.Paths.AliasLink
	rep := FileReport{Path: link, Result: ResultOK}
	target := e.aliasTarget()
	info, err := e.System.Lstat(link)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return rep, err
	case info.Mode()&fs.ModeSymlink == 0:
		rep.Result = ResultSkipped
		return rep, nil
	default:
		if current, err := e.System.Readlink(link); err == nil && current == target {
			return rep, nil
		}
		if err := e.System.Remove(link); err != nil {
			return rep, err
		}
	}
	if err := e.System.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return rep, err
	}
	return rep, e.System.Symlink(target, link)
}

// aliasCheck is doctor's view of the alias link.
func (e *Engine) aliasCheck() CheckResult {
	c := CheckResult{Name: AliasName, Status: CheckOK}
	link := e.Paths.AliasLink
	info, err := e.System.Lstat(link)
	switch {
	case err != nil:
		c.Status, c.Detail = CheckWarn, "missing (run shm setup)"
		return c
	case info.Mode()&fs.ModeSymlink == 0:
		c.Status, c.Detail = CheckWarn, link+" is not a link to shell-manager; remove it and run shm setup"
		return c
	}
	target, err := e.System.Readlink(link)
	if err != nil {
		c.Status, c.Detail = CheckWarn, err.Error()
		return c
	}
	c.Detail = "→ " + target
	if e.Executable != "" && target != e.aliasTarget() {
		c.Status, c.Detail = CheckWarn, "points to "+target+" (run shm setup)"
	}
	return c
}

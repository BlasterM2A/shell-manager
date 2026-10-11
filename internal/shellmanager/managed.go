package shellmanager

import (
	"bytes"
	"errors"
	"io/fs"
	"time"
)

// fileWriter writes files shell-manager owns. It never overwrites a manual
// edit unless forced, and backs up anything it replaces that it did not
// write itself.
type fileWriter struct {
	sys   System
	state *State
	now   func() time.Time
}

func (w fileWriter) write(path string, content []byte, force bool) (Result, error) {
	current, err := w.sys.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return w.put(path, content)
	}
	if err != nil {
		return ResultFailed, err
	}
	recorded, managed := w.state.Files[path]
	edited := managed && hashBytes(current) != recorded
	if edited && !force {
		return ResultModified, nil
	}
	if bytes.Equal(current, content) {
		w.state.Files[path] = hashBytes(content)
		return ResultOK, nil
	}
	if !managed || edited {
		if err := w.sys.Rename(path, w.backupPath(path)); err != nil {
			return ResultFailed, err
		}
	}
	return w.put(path, content)
}

// backupPath is where a file is saved before shell-manager replaces it.
func (w fileWriter) backupPath(path string) string {
	return path + ".bak." + w.now().Format("20060102150405")
}

func (w fileWriter) put(path string, content []byte) (Result, error) {
	if err := w.sys.WriteFile(path, content, 0o644); err != nil {
		return ResultFailed, err
	}
	w.state.Files[path] = hashBytes(content)
	return ResultOK, nil
}

// remove deletes a managed file that is no longer wanted, unless edited.
func (w fileWriter) remove(path string) (Result, error) {
	current, err := w.sys.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		delete(w.state.Files, path)
		return ResultOK, nil
	}
	if err != nil {
		return ResultFailed, err
	}
	if hashBytes(current) != w.state.Files[path] {
		return ResultModified, nil
	}
	if err := w.sys.Remove(path); err != nil {
		return ResultFailed, err
	}
	delete(w.state.Files, path)
	return ResultOK, nil
}

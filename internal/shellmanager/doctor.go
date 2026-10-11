package shellmanager

import (
	"context"
	"maps"
	"path/filepath"
	"slices"
)

// Doctor checks the machine without changing anything.
func (e *Engine) Doctor(ctx context.Context) (Report, error) {
	state, err := LoadState(e.System, e.Paths.StateFile)
	if err != nil {
		return Report{}, err
	}
	var rep Report
	env := Env{System: e.System, Fetcher: e.Fetcher, Paths: e.Paths, State: state}
	for _, t := range e.Catalog.Tools() {
		st := e.checkTool(ctx, env, t)
		c := CheckResult{Name: t.ID, Status: CheckOK, Detail: st.Version}
		if !st.Installed {
			c.Status, c.Detail = CheckFail, "not installed"
			if t.Optional {
				c.Status = CheckWarn
			}
		}
		rep.Checks = append(rep.Checks, c)
	}
	for _, path := range slices.Sorted(maps.Keys(state.Files)) {
		c := CheckResult{Name: path, Status: CheckOK}
		data, err := e.System.ReadFile(path)
		switch {
		case err != nil:
			c.Status, c.Detail = CheckWarn, "missing (run shm update)"
		case hashBytes(data) != state.Files[path]:
			c.Status, c.Detail = CheckWarn, "modified by hand (update --force overwrites it)"
		}
		rep.Checks = append(rep.Checks, c)
	}
	for _, path := range e.blockPaths(state) {
		rep.Checks = append(rep.Checks, e.blockCheck(path, state.Blocks[path]))
	}
	shell := e.loginShell(ctx)
	sc := CheckResult{Name: "default shell", Status: CheckOK, Detail: shell}
	if filepath.Base(shell) != "zsh" {
		sc.Status, sc.Detail = CheckWarn, shell+" (run shm setup)"
	}
	rep.Checks = append(rep.Checks, sc, e.aliasCheck())
	return rep, nil
}

// blockPaths lists the files whose shm block doctor checks: ~/.zshenv and
// ~/.zshrc always (state may be lost or never recorded them), then any other
// recorded path, without duplicates.
func (e *Engine) blockPaths(state *State) []string {
	paths := []string{e.Paths.Zshenv, e.Paths.Zshrc}
	for _, p := range slices.Sorted(maps.Keys(state.Blocks)) {
		if !slices.Contains(paths, p) {
			paths = append(paths, p)
		}
	}
	return paths
}

// blockCheck is doctor's view of the shm block in path. recorded is the hash
// shm stored for it, empty when there is no record: a block that is present
// then counts as ok (shm wrote it, or the next update adopts it).
func (e *Engine) blockCheck(path, recorded string) CheckResult {
	c := CheckResult{Name: path, Status: CheckOK}
	data, err := e.System.ReadFile(resolveLink(e.System, path))
	if err != nil {
		c.Status, c.Detail = CheckWarn, "missing (run shm update)"
		return c
	}
	start, end, found, err := findBlock(data)
	switch {
	case err != nil:
		c.Status, c.Detail = CheckWarn, ErrBrokenMarkers.Error()
	case !found:
		c.Status, c.Detail = CheckWarn, "shm block missing (run shm update)"
	case recorded != "" && hashBytes(data[start:end]) != recorded:
		c.Status, c.Detail = CheckWarn, "shm block modified by hand (update --force overwrites it)"
	}
	return c
}

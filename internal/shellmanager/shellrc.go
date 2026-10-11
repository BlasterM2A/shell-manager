package shellmanager

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

var (
	// zshDirs are created under ~/.zsh so the user sees where things go.
	zshDirs = []string{"env", "functions", "completions", "tools", "aliases", "local"}
	// shellProfile are shm's own files under ~/.zsh (from registry/files/zsh/).
	shellProfile = []string{"env.zsh", "init.zsh", "env/00-path.zsh", "tools/20-core.zsh", "aliases/00-base.zsh"}
	// fragmentDirs get one generated fragment per tool: env (every zsh) and
	// tools (interactive only).
	fragmentDirs = []string{"env", "tools"}
)

// shellWriter writes the managed zsh config: the shm blocks in ~/.zshenv
// and ~/.zshrc, shm's files under ~/.zsh and one fragment per tool.
type shellWriter struct {
	sys     System
	catalog *Catalog
	paths   Paths
	files   fileWriter
}

// apply writes the config for okTools (the tools that ended OK this run).
// The blocks go last: a block that can't be written (broken markers) fails
// on its own without stopping the rest.
func (w shellWriter) apply(okTools []Tool, force bool) ([]FileReport, error) {
	for _, d := range zshDirs {
		if err := w.sys.MkdirAll(filepath.Join(w.paths.ZshDir, d), 0o755); err != nil {
			return nil, err
		}
	}
	desired, err := w.desired(okTools)
	if err != nil {
		return nil, err
	}
	var reports []FileReport
	for _, path := range slices.Sorted(maps.Keys(desired)) {
		res, err := w.files.write(path, desired[path], force)
		if err != nil {
			return reports, fmt.Errorf("writing %s: %w", path, err)
		}
		reports = append(reports, FileReport{Path: path, Result: res})
	}
	orphans, err := w.orphans(desired, okTools)
	if err != nil {
		return reports, err
	}
	for _, path := range orphans {
		res, err := w.files.remove(path)
		if err != nil {
			return reports, fmt.Errorf("removing %s: %w", path, err)
		}
		reports = append(reports, FileReport{Path: path, Result: res})
	}
	var errs []error
	for _, b := range []struct{ path, src string }{
		{w.paths.Zshenv, "files/zsh/zshenv-block"},
		{w.paths.Zshrc, "files/zsh/zshrc-block"},
	} {
		block, err := w.catalog.File(b.src)
		if err != nil {
			return reports, err
		}
		res, err := w.files.writeBlock(b.path, block, force)
		reports = append(reports, FileReport{Path: b.path, Result: res})
		if err != nil {
			errs = append(errs, fmt.Errorf("writing the shm block: %w", err))
		}
	}
	return reports, errors.Join(errs...)
}

func (w shellWriter) desired(okTools []Tool) (map[string][]byte, error) {
	desired := map[string][]byte{}
	embed := func(dest, src string) error {
		data, err := w.catalog.File(src)
		if err != nil {
			return err
		}
		desired[dest] = data
		return nil
	}
	for _, name := range shellProfile {
		if err := embed(filepath.Join(w.paths.ZshDir, name), "files/zsh/"+name); err != nil {
			return nil, err
		}
	}
	for _, t := range okTools {
		if t.Shell != nil {
			name := fmt.Sprintf("%02d-%s.zsh", t.Shell.Priority, t.ID)
			if t.Shell.Env != "" {
				desired[filepath.Join(w.paths.ZshDir, "env", name)] = fragment(t.ID, t.Shell.Env)
			}
			if t.Shell.Snippet != "" {
				desired[filepath.Join(w.paths.ZshDir, "tools", name)] = fragment(t.ID, t.Shell.Snippet)
			}
		}
		for _, f := range t.Files {
			if err := embed(w.paths.Expand(f.Dest), f.Src); err != nil {
				return nil, err
			}
		}
	}
	return desired, nil
}

func fragment(id, body string) []byte {
	return fmt.Appendf(nil, "# managed by shm (tool: %s)\n%s\n", id, strings.TrimRight(body, "\n"))
}

// orphans are managed fragments in env/ and tools/ of tools no longer in
// the catalog. Fragments of catalog tools that failed this run are kept.
func (w shellWriter) orphans(desired map[string][]byte, okTools []Tool) ([]string, error) {
	ok := map[string]bool{}
	for _, t := range okTools {
		ok[t.ID] = true
	}
	var out []string
	for _, dir := range fragmentDirs {
		existing, err := w.sys.Glob(filepath.Join(w.paths.ZshDir, dir, "*.zsh"))
		if err != nil {
			return nil, err
		}
		for _, path := range existing {
			if _, wanted := desired[path]; wanted {
				continue
			}
			if _, managed := w.files.state.Files[path]; !managed {
				continue
			}
			if rel, err := filepath.Rel(w.paths.ZshDir, path); err == nil && slices.Contains(shellProfile, rel) {
				continue
			}
			_, id, _ := strings.Cut(strings.TrimSuffix(filepath.Base(path), ".zsh"), "-")
			// OK tools have exactly the fragments in desired, so any other
			// managed fragment of theirs is stale.
			if _, inCatalog := w.catalog.Get(id); inCatalog && !ok[id] {
				continue
			}
			out = append(out, path)
		}
	}
	return out, nil
}

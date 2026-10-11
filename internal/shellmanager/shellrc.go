package shellmanager

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
)

var (
	profileFragments = []string{"20-core.zsh", "70-aliases.zsh"}
	userZshDirs      = []string{"env.d", "functions.d", "aliases.d", "custom.d"}
)

// shellWriter writes the managed zsh config: the ~/.zshrc stub, the
// generated zshrc, profile fragments and one zsh.d fragment per tool.
type shellWriter struct {
	sys     System
	catalog *Catalog
	paths   Paths
	files   fileWriter
}

// apply writes the config for okTools (the tools that ended OK this run).
func (w shellWriter) apply(okTools []Tool, force bool) ([]FileReport, error) {
	for _, d := range userZshDirs {
		if err := w.sys.MkdirAll(filepath.Join(w.paths.UserZshDir, d), 0o755); err != nil {
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
	return reports, nil
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
	if err := embed(w.paths.Stub, "files/profile/zshrc-stub"); err != nil {
		return nil, err
	}
	if err := embed(w.paths.GeneratedZshrc, "files/profile/zshrc"); err != nil {
		return nil, err
	}
	for _, name := range profileFragments {
		if err := embed(filepath.Join(w.paths.ZshD, name), "files/profile/"+name); err != nil {
			return nil, err
		}
	}
	for _, t := range okTools {
		if t.Shell != nil {
			name := fmt.Sprintf("%02d-%s.zsh", t.Shell.Priority, t.ID)
			desired[filepath.Join(w.paths.ZshD, name)] = []byte(
				fmt.Sprintf("# managed by shell-manager (tool: %s)\n%s\n", t.ID, strings.TrimRight(t.Shell.Snippet, "\n")))
		}
		for _, f := range t.Files {
			if err := embed(w.paths.Expand(f.Dest), f.Src); err != nil {
				return nil, err
			}
		}
	}
	return desired, nil
}

// orphans are managed zsh.d fragments of tools no longer in the catalog.
// Fragments of catalog tools that failed this run are kept as they were.
func (w shellWriter) orphans(desired map[string][]byte, okTools []Tool) ([]string, error) {
	existing, err := w.sys.Glob(filepath.Join(w.paths.ZshD, "*.zsh"))
	if err != nil {
		return nil, err
	}
	ok := map[string]bool{}
	for _, t := range okTools {
		ok[t.ID] = true
	}
	var out []string
	for _, path := range existing {
		if _, wanted := desired[path]; wanted {
			continue
		}
		if _, managed := w.files.state.Files[path]; !managed {
			continue
		}
		name := filepath.Base(path)
		if slices.Contains(profileFragments, name) {
			continue
		}
		_, id, _ := strings.Cut(strings.TrimSuffix(name, ".zsh"), "-")
		// OK tools have exactly the fragments in desired, so any other
		// managed fragment of theirs is stale.
		if _, inCatalog := w.catalog.Get(id); inCatalog && !ok[id] {
			continue
		}
		out = append(out, path)
	}
	return out, nil
}

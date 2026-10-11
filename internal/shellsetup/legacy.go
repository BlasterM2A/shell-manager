package shellsetup

import (
	"context"
	"slices"
)

// legacyItem is a leftover of an older install still on the machine: a
// file (path set) or an apt package (apt set). name is how it is reported.
type legacyItem struct {
	name string
	path string
	apt  string
}

// legacyLeftovers lists what of t's legacy spec is still present.
func (e *Engine) legacyLeftovers(ctx context.Context, t Tool) []legacyItem {
	if t.Legacy == nil {
		return nil
	}
	var items []legacyItem
	for _, pattern := range t.Legacy.Paths {
		matches, err := e.System.Glob(e.Paths.Expand(pattern))
		if err != nil {
			continue // patterns are validated when the catalog loads
		}
		for _, m := range matches {
			items = append(items, legacyItem{name: m, path: m})
		}
	}
	missing := missingAptPackages(ctx, e.System, t.Legacy.Apt)
	for _, pkg := range t.Legacy.Apt {
		if !slices.Contains(missing, pkg) {
			items = append(items, legacyItem{name: "apt:" + pkg, apt: pkg})
		}
	}
	return items
}

// purgeLegacy removes t's leftovers. Call it only once t is installed by its
// own backend, so the user is never left without the tool. A failure is a
// warning: the leftover is harmless and doctor keeps reporting it.
func (e *Engine) purgeLegacy(ctx context.Context, t Tool) []FileReport {
	var reports []FileReport
	for _, it := range e.legacyLeftovers(ctx, t) {
		var err error
		if it.apt != "" {
			_, err = e.System.Run(ctx, e.privileged("apt-get", "remove", "-y", it.apt))
		} else {
			err = e.System.Remove(it.path)
		}
		res := ResultRemoved
		if err != nil {
			res = ResultWarned
		}
		reports = append(reports, FileReport{Path: it.name, Result: res})
	}
	return reports
}

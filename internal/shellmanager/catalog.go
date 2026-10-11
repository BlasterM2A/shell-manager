package shellmanager

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	pathpkg "path"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

//go:embed registry
var registryFS embed.FS

// A registry holds catalog.toml (the ids of the tools to install) and one
// module per tool in tools/<id>.toml. Modules left out of catalog.toml stay
// in the codebase but are not installed.
const (
	catalogFile = "catalog.toml"
	modulesDir  = "tools"
)

// Catalog is the set of included tools, sorted so dependencies come first.
type Catalog struct {
	tools []Tool
	byID  map[string]Tool
	files fs.FS
}

// DefaultCatalog is the catalog embedded in the binary.
func DefaultCatalog() (*Catalog, error) {
	sub, err := fs.Sub(registryFS, "registry")
	if err != nil {
		return nil, err
	}
	return LoadCatalog(sub)
}

// LoadCatalog loads the modules listed in fsys's catalog.toml. Files
// referenced by modules are read from the same fsys.
func LoadCatalog(fsys fs.FS) (*Catalog, error) {
	ids, err := readCatalogFile(fsys)
	if err != nil {
		return nil, err
	}
	c := &Catalog{byID: map[string]Tool{}, files: fsys}
	var tools []Tool
	for _, id := range ids {
		if _, dup := c.byID[id]; dup {
			return nil, fmt.Errorf("%s: %q is listed twice", catalogFile, id)
		}
		path := modulePath(id)
		if _, err := fs.Stat(fsys, path); err != nil {
			return nil, fmt.Errorf("%s: %q is listed in %s but %s does not exist", catalogFile, id, catalogFile, path)
		}
		t, err := loadTool(fsys, path)
		if err != nil {
			return nil, fmt.Errorf("registry/%s: %w", path, err)
		}
		c.byID[t.ID] = t
		tools = append(tools, t)
	}
	for _, t := range tools {
		for _, dep := range t.Depends {
			if _, ok := c.byID[dep]; !ok {
				return nil, fmt.Errorf("registry/%s: depends on %q, which is not in %s", modulePath(t.ID), dep, catalogFile)
			}
		}
	}
	if c.tools, err = topoSort(tools); err != nil {
		return nil, err
	}
	return c, nil
}

// ValidateModules checks that every module in tools/ is a valid manifest,
// whether or not catalog.toml includes it, so modules left out don't rot.
func ValidateModules(fsys fs.FS) error {
	paths, err := fs.Glob(fsys, modulesDir+"/*.toml")
	if err != nil {
		return err
	}
	var errs []error
	for _, path := range paths {
		if _, err := loadTool(fsys, path); err != nil {
			errs = append(errs, fmt.Errorf("registry/%s: %w", path, err))
		}
	}
	return errors.Join(errs...)
}

func readCatalogFile(fsys fs.FS) ([]string, error) {
	data, err := fs.ReadFile(fsys, catalogFile)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", catalogFile, err)
	}
	var index struct {
		Tools []string `toml:"tools"`
	}
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&index); err != nil {
		return nil, fmt.Errorf("reading %s: %w", catalogFile, err)
	}
	return index.Tools, nil
}

func modulePath(id string) string { return modulesDir + "/" + id + ".toml" }

func loadTool(fsys fs.FS, path string) (Tool, error) {
	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return Tool{}, err
	}
	var t Tool
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&t); err != nil {
		var strict *toml.StrictMissingError
		if errors.As(err, &strict) {
			return Tool{}, fmt.Errorf("unknown fields:\n%s", strict.String())
		}
		return Tool{}, err
	}
	if want := strings.TrimSuffix(pathpkg.Base(path), ".toml"); t.ID != want {
		return Tool{}, fmt.Errorf("id %q must match file name %q", t.ID, want)
	}
	if t.Kind != KindBuiltin && t.Kind != KindPlugin {
		return Tool{}, fmt.Errorf("kind must be %q or %q", KindBuiltin, KindPlugin)
	}
	if t.Install.Backend == "" {
		return Tool{}, errors.New("install.backend is required")
	}
	if (len(t.Check.Cmd) == 0) == (t.Check.Path == "") {
		return Tool{}, errors.New("check needs exactly one of cmd or path")
	}
	for _, argv := range t.PostInstall {
		if len(argv) == 0 {
			return Tool{}, errors.New("post_install contains an empty command")
		}
	}
	if t.Shell != nil && (t.Shell.Priority < 1 || t.Shell.Priority > 89 || t.Shell.Snippet == "") {
		return Tool{}, errors.New("shell needs a priority in 1..89 and a snippet")
	}
	for _, f := range t.Files {
		if _, err := fs.Stat(fsys, f.Src); err != nil {
			return Tool{}, fmt.Errorf("file %s: %w", f.Src, err)
		}
	}
	return t, nil
}

// topoSort puts dependencies first. Among tools that are ready, built-ins
// go first and then ids alphabetically, so the order is stable.
func topoSort(tools []Tool) ([]Tool, error) {
	done := map[string]bool{}
	remaining := slices.Clone(tools)
	out := make([]Tool, 0, len(tools))
	for len(remaining) > 0 {
		var ready []Tool
		for _, t := range remaining {
			if !slices.ContainsFunc(t.Depends, func(d string) bool { return !done[d] }) {
				ready = append(ready, t)
			}
		}
		if len(ready) == 0 {
			ids := make([]string, len(remaining))
			for i, t := range remaining {
				ids[i] = t.ID
			}
			slices.Sort(ids)
			return nil, fmt.Errorf("dependency cycle between: %s", strings.Join(ids, ", "))
		}
		next := slices.MinFunc(ready, func(a, b Tool) int {
			if (a.Kind == KindBuiltin) != (b.Kind == KindBuiltin) {
				if a.Kind == KindBuiltin {
					return -1
				}
				return 1
			}
			return strings.Compare(a.ID, b.ID)
		})
		out = append(out, next)
		done[next.ID] = true
		remaining = slices.DeleteFunc(remaining, func(t Tool) bool { return t.ID == next.ID })
	}
	return out, nil
}

// Tools returns the tools in install order.
func (c *Catalog) Tools() []Tool { return slices.Clone(c.tools) }

// Get returns a tool by id.
func (c *Catalog) Get(id string) (Tool, bool) {
	t, ok := c.byID[id]
	return t, ok
}

// File returns an embedded file referenced by a manifest or the profile.
func (c *Catalog) File(src string) ([]byte, error) { return fs.ReadFile(c.files, src) }

package shellmanager

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolIDs(tools []Tool) []string {
	ids := make([]string, len(tools))
	for i, t := range tools {
		ids[i] = t.ID
	}
	return ids
}

func manifest(body string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(body)} }

// module is a minimal valid manifest for id; extra goes before the tables.
func module(id, kind, extra string) string {
	return "id = \"" + id + "\"\nkind = \"" + kind + "\"\n" + extra +
		"\n[install]\nbackend = \"fake\"\n[check]\ncmd = [\"x\"]\n"
}

// testRegistry builds a registry: catalog.toml listing `included`, plus one
// tools/<id>.toml per entry in modules (included or not).
func testRegistry(included []string, modules map[string]string) fstest.MapFS {
	quoted := make([]string, len(included))
	for i, id := range included {
		quoted[i] = `"` + id + `"`
	}
	fsys := fstest.MapFS{"catalog.toml": manifest("tools = [" + strings.Join(quoted, ", ") + "]\n")}
	for id, body := range modules {
		fsys["tools/"+id+".toml"] = manifest(body)
	}
	return fsys
}

func TestCatalogLoadsOnlyIncludedModules(t *testing.T) {
	cat, err := LoadCatalog(testRegistry([]string{"a"}, map[string]string{
		"a":       module("a", "plugin", ""),
		"retired": module("retired", "plugin", ""),
	}))
	require.NoError(t, err)

	assert.Equal(t, []string{"a"}, toolIDs(cat.Tools()))
	_, ok := cat.Get("retired")
	assert.False(t, ok)
}

func TestCatalogOrder(t *testing.T) {
	// "a" depends on "z" (sorts after it), "m" is a built-in.
	cat, err := LoadCatalog(testRegistry([]string{"a", "b", "m", "z"}, map[string]string{
		"a": module("a", "plugin", `depends = ["z"]`),
		"b": module("b", "plugin", ""),
		"m": module("m", "builtin", ""),
		"z": module("z", "plugin", ""),
	}))
	require.NoError(t, err)
	assert.Equal(t, []string{"m", "b", "z", "a"}, toolIDs(cat.Tools()),
		"built-ins first, dependencies before dependents, otherwise by id")
}

func TestLoadCatalogErrors(t *testing.T) {
	cases := map[string]struct {
		fs   fstest.MapFS
		want string
	}{
		"missing catalog.toml": {
			fstest.MapFS{"tools/a.toml": manifest(module("a", "plugin", ""))},
			"catalog.toml",
		},
		"included module missing": {
			testRegistry([]string{"ghost"}, nil),
			`"ghost" is listed in catalog.toml but tools/ghost.toml does not exist`,
		},
		"duplicate entry": {
			testRegistry([]string{"a", "a"}, map[string]string{"a": module("a", "plugin", "")}),
			`"a" is listed twice`,
		},
		"dependency not included": {
			testRegistry([]string{"a"}, map[string]string{
				"a":    module("a", "plugin", `depends = ["mise"]`),
				"mise": module("mise", "builtin", ""),
			}),
			`depends on "mise", which is not in catalog.toml`,
		},
		"cycle": {
			testRegistry([]string{"a", "b"}, map[string]string{
				"a": module("a", "plugin", `depends = ["b"]`),
				"b": module("b", "plugin", `depends = ["a"]`),
			}),
			"dependency cycle between: a, b",
		},
		"id mismatch": {
			testRegistry([]string{"a"}, map[string]string{"a": module("b", "plugin", "")}),
			`id "b" must match file name "a"`,
		},
		"unknown field": {
			testRegistry([]string{"a"}, map[string]string{"a": module("a", "plugin", "bogus = 1")}),
			"bogus",
		},
		"missing file": {
			testRegistry([]string{"a"}, map[string]string{
				"a": module("a", "plugin", "") + "[[files]]\nsrc = \"files/nope\"\ndest = \"~/x\"\n",
			}),
			"files/nope",
		},
		"check needs one of cmd or path": {
			testRegistry([]string{"a"}, map[string]string{
				"a": "id = \"a\"\nkind = \"plugin\"\n[install]\nbackend = \"fake\"\n[check]\n",
			}),
			"check needs exactly one of cmd or path",
		},
		"empty post_install command": {
			testRegistry([]string{"a"}, map[string]string{"a": module("a", "plugin", "post_install = [[]]")}),
			"post_install",
		},
		"shell without env or snippet": {
			testRegistry([]string{"a"}, map[string]string{"a": module("a", "plugin", "") + "[shell]\npriority = 10\n"}),
			"shell needs a priority in 1..89 and an env or a snippet",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadCatalog(tc.fs)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestShellEnvOnlyIsValid(t *testing.T) {
	cat, err := LoadCatalog(testRegistry([]string{"a"}, map[string]string{
		"a": module("a", "plugin", "") + "[shell]\npriority = 10\nenv = 'export A=1'\n",
	}))
	require.NoError(t, err)
	a, _ := cat.Get("a")
	assert.Equal(t, "export A=1", a.Shell.Env)
}

func TestValidateModulesChecksModulesOutsideTheCatalog(t *testing.T) {
	fsys := testRegistry([]string{"a"}, map[string]string{
		"a":       module("a", "plugin", ""),
		"retired": module("retired", "plugin", "bogus = 1"),
	})
	_, err := LoadCatalog(fsys)
	require.NoError(t, err, "the catalog ignores modules it doesn't include")

	err = ValidateModules(fsys)
	assert.ErrorContains(t, err, "tools/retired.toml")
}

// The embedded registry is checked dynamically: whatever catalog.toml lists
// must load, and every module in tools/ must stay a valid manifest.

func embeddedRegistry(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(registryFS, "registry")
	require.NoError(t, err)
	return sub
}

func TestEmbeddedCatalogIsValid(t *testing.T) {
	cat, err := DefaultCatalog()
	require.NoError(t, err)
	require.NotEmpty(t, cat.Tools())
	for _, src := range []string{"files/profile/zshrc", "files/profile/zshrc-stub", "files/profile/20-core.zsh", "files/profile/70-aliases.zsh"} {
		_, err := cat.File(src)
		assert.NoError(t, err, src)
	}
}

func TestEmbeddedModulesAreValid(t *testing.T) {
	assert.NoError(t, ValidateModules(embeddedRegistry(t)))
}

package shellmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newShellWriter(t *testing.T) shellWriter {
	t.Helper()
	sys, _ := newTestSystem(t)
	cat, err := DefaultCatalog()
	require.NoError(t, err)
	st := &State{Files: map[string]string{}, Blocks: map[string]string{}, Versions: map[string]string{}}
	return shellWriter{
		sys:     sys,
		catalog: cat,
		paths:   NewPaths(sys.HomeDir()),
		files:   fileWriter{sys: sys, state: st, now: func() time.Time { return fixedNow }},
	}
}

func catalogTools(t *testing.T, w shellWriter, ids ...string) []Tool {
	t.Helper()
	var tools []Tool
	for _, id := range ids {
		tool, ok := w.catalog.Get(id)
		require.True(t, ok, id)
		tools = append(tools, tool)
	}
	return tools
}

func zshdNames(t *testing.T, w shellWriter) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(w.paths.ZshD, "*.zsh"))
	require.NoError(t, err)
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	return names
}

func TestApplyWritesProfileAndFragments(t *testing.T) {
	w := newShellWriter(t)

	reports, err := w.apply(catalogTools(t, w, "mise", "starship", "zoxide"), false)
	require.NoError(t, err)

	assert.Equal(t, []string{"10-mise.zsh", "20-core.zsh", "50-starship.zsh", "60-zoxide.zsh", "70-aliases.zsh"}, zshdNames(t, w))
	assert.Equal(t, "# managed by shell-manager (tool: zoxide)\neval \"$(zoxide init zsh)\"\nalias cd=\"z\"\n",
		read(t, filepath.Join(w.paths.ZshD, "60-zoxide.zsh")))
	assert.Contains(t, read(t, w.paths.Stub), `source "$HOME/.config/shell-manager/zshrc"`)
	assert.FileExists(t, filepath.Join(w.paths.Home, ".config", "starship.toml"))
	for _, d := range []string{"env.d", "functions.d", "aliases.d", "custom.d"} {
		assert.DirExists(t, filepath.Join(w.paths.UserZshDir, d))
	}
	assert.Len(t, reports, 8) // stub, zshrc, 5 fragments, starship.toml
	for _, r := range reports {
		assert.Equal(t, ResultOK, r.Result, r.Path)
	}
}

func TestApplyBacksUpUnmanagedZshrc(t *testing.T) {
	w := newShellWriter(t)
	require.NoError(t, os.WriteFile(w.paths.Stub, []byte("# my old zshrc\n"), 0o644))

	_, err := w.apply(nil, false)
	require.NoError(t, err)

	assert.Equal(t, "# my old zshrc\n", read(t, w.paths.Stub+backupSuffix))
	assert.Contains(t, read(t, w.paths.Stub), "managed by shell-manager")
}

func TestApplyTwiceIsIdempotent(t *testing.T) {
	w := newShellWriter(t)
	tools := catalogTools(t, w, "mise", "zoxide")
	_, err := w.apply(tools, false)
	require.NoError(t, err)

	reports, err := w.apply(tools, false)
	require.NoError(t, err)
	for _, r := range reports {
		assert.Equal(t, ResultOK, r.Result, r.Path)
	}
	backups, err := filepath.Glob(filepath.Join(w.paths.Home, "*.bak.*"))
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestApplyRemovesOnlyFragmentsOfRemovedTools(t *testing.T) {
	w := newShellWriter(t)
	gone := filepath.Join(w.paths.ZshD, "42-gone.zsh")       // managed, tool no longer in catalog
	failed := filepath.Join(w.paths.ZshD, "50-starship.zsh") // managed, tool failed this run
	mine := filepath.Join(w.paths.ZshD, "99-mine.zsh")       // not managed
	for _, p := range []string{gone, failed} {
		_, err := w.files.write(p, []byte("# old\n"), false)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(mine, []byte("# mine\n"), 0o644))

	reports, err := w.apply(nil, false)
	require.NoError(t, err)

	assert.NoFileExists(t, gone)
	assert.FileExists(t, failed)
	assert.FileExists(t, mine)
	assert.Contains(t, reports, FileReport{Path: gone, Result: ResultOK})
}

func TestApplyRemovesStaleFragmentOfOKTool(t *testing.T) {
	w := newShellWriter(t)
	stale := filepath.Join(w.paths.ZshD, "60-starship.zsh") // starship is priority 50 in the catalog
	_, err := w.files.write(stale, []byte("# old\n"), false)
	require.NoError(t, err)

	_, err = w.apply(catalogTools(t, w, "starship"), false)
	require.NoError(t, err)

	assert.NoFileExists(t, stale)
	assert.FileExists(t, filepath.Join(w.paths.ZshD, "50-starship.zsh"))
}

func TestShellConfigGolden(t *testing.T) {
	w := newShellWriter(t)
	_, err := w.apply(catalogTools(t, w, "mise", "antidote", "starship", "fzf", "zoxide"), false)
	require.NoError(t, err)

	var b strings.Builder
	files := append([]string{w.paths.Stub, w.paths.GeneratedZshrc}, globAll(t, w.paths.ZshD)...)
	for _, p := range files {
		b.WriteString("== " + strings.TrimPrefix(p, w.paths.Home) + " ==\n")
		b.WriteString(read(t, p))
	}
	golden.RequireEqual(t, []byte(b.String()))
}

func TestShellConfigIsValidZsh(t *testing.T) {
	path, err := lookSystemZsh()
	if err != nil {
		t.Skip("zsh not installed")
	}
	w := newShellWriter(t)
	_, err = w.apply(catalogTools(t, w, "mise", "antidote", "starship", "fzf", "zoxide"), false)
	require.NoError(t, err)
	for _, p := range append([]string{w.paths.Stub, w.paths.GeneratedZshrc}, globAll(t, w.paths.ZshD)...) {
		out, err := runZshSyntaxCheck(path, p)
		assert.NoError(t, err, "%s: %s", p, out)
	}
}

func globAll(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.zsh"))
	require.NoError(t, err)
	return paths
}

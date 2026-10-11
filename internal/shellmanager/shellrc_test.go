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

func names(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.zsh"))
	require.NoError(t, err)
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}

// zshFiles are every generated zsh file: the two dotfiles, then ~/.zsh/**.zsh.
func zshFiles(t *testing.T, w shellWriter) []string {
	t.Helper()
	files := []string{w.paths.Zshenv, w.paths.Zshrc}
	err := filepath.WalkDir(w.paths.ZshDir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".zsh") {
			files = append(files, p)
		}
		return err
	})
	require.NoError(t, err)
	return files
}

func TestApplyWritesZshLayout(t *testing.T) {
	w := newShellWriter(t)

	reports, err := w.apply(catalogTools(t, w, "mise", "starship", "zoxide"), false)
	require.NoError(t, err)

	zsh := w.paths.ZshDir
	assert.Equal(t, []string{"00-path.zsh", "10-mise.zsh"}, names(t, filepath.Join(zsh, "env")))
	assert.Equal(t, []string{"10-mise.zsh", "20-core.zsh", "50-starship.zsh", "60-zoxide.zsh"}, names(t, filepath.Join(zsh, "tools")))
	assert.Equal(t, []string{"00-base.zsh"}, names(t, filepath.Join(zsh, "aliases")))
	assert.Equal(t, "# managed by shm (tool: zoxide)\neval \"$(zoxide init zsh)\"\n",
		read(t, filepath.Join(zsh, "tools", "60-zoxide.zsh")))
	assert.FileExists(t, filepath.Join(zsh, "env.zsh"))
	assert.FileExists(t, filepath.Join(zsh, "init.zsh"))
	assert.Contains(t, read(t, w.paths.Zshrc), `source "$HOME/.zsh/init.zsh"`)
	assert.Contains(t, read(t, w.paths.Zshenv), "skip_global_compinit=1")
	assert.FileExists(t, filepath.Join(w.paths.Home, ".config", "starship.toml"))
	for _, d := range []string{"env", "functions", "completions", "tools", "aliases", "local"} {
		assert.DirExists(t, filepath.Join(zsh, d))
	}
	assert.NoDirExists(t, filepath.Join(w.paths.Home, ".config", "shell-manager"))
	assert.NoDirExists(t, filepath.Join(w.paths.Home, ".config", "zsh"))
	// 5 shm files + env/10-mise + 3 tool fragments + starship.toml + 2 blocks
	assert.Len(t, reports, 12)
	for _, r := range reports {
		assert.Equal(t, ResultOK, r.Result, r.Path)
	}
}

func TestApplyWritesAntidotePluginsAtItsDefaultPath(t *testing.T) {
	w := newShellWriter(t)
	_, err := w.apply(catalogTools(t, w, "antidote"), false)
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(w.paths.Home, ".zsh_plugins.txt"))
}

func TestApplyKeepsUserZshrcBelowTheBlock(t *testing.T) {
	w := newShellWriter(t)
	require.NoError(t, os.WriteFile(w.paths.Zshrc, []byte("export NVM_DIR=x\n"), 0o644))

	_, err := w.apply(nil, false)
	require.NoError(t, err)

	got := read(t, w.paths.Zshrc)
	assert.True(t, strings.HasPrefix(got, blockBegin+"\n"), got)
	assert.True(t, strings.HasSuffix(got, "\nexport NVM_DIR=x\n"), got)
	assert.Equal(t, "export NVM_DIR=x\n", read(t, w.paths.Zshrc+backupSuffix))
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
	backups, err := filepath.Glob(filepath.Join(w.paths.Home, ".*.bak.*"))
	require.NoError(t, err)
	assert.Empty(t, backups)
}

func TestApplyRemovesOnlyFragmentsOfRemovedTools(t *testing.T) {
	w := newShellWriter(t)
	tools := filepath.Join(w.paths.ZshDir, "tools")
	gone := filepath.Join(tools, "42-gone.zsh")                    // managed, tool no longer in catalog
	goneEnv := filepath.Join(w.paths.ZshDir, "env", "42-gone.zsh") // same, env part
	failed := filepath.Join(tools, "50-starship.zsh")              // managed, tool failed this run
	mine := filepath.Join(tools, "99-mine.zsh")                    // not managed
	for _, p := range []string{gone, goneEnv, failed} {
		_, err := w.files.write(p, []byte("# old\n"), false)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(mine, []byte("# mine\n"), 0o644))

	reports, err := w.apply(nil, false)
	require.NoError(t, err)

	assert.NoFileExists(t, gone)
	assert.NoFileExists(t, goneEnv)
	assert.FileExists(t, failed)
	assert.FileExists(t, mine)
	assert.Contains(t, reports, FileReport{Path: gone, Result: ResultOK})
}

func TestApplyRemovesStaleFragmentOfOKTool(t *testing.T) {
	w := newShellWriter(t)
	stale := filepath.Join(w.paths.ZshDir, "tools", "60-starship.zsh") // starship is priority 50 in the catalog
	_, err := w.files.write(stale, []byte("# old\n"), false)
	require.NoError(t, err)

	_, err = w.apply(catalogTools(t, w, "starship"), false)
	require.NoError(t, err)

	assert.NoFileExists(t, stale)
	assert.FileExists(t, filepath.Join(w.paths.ZshDir, "tools", "50-starship.zsh"))
}

func TestApplyBrokenZshrcMarkersStillWritesTheRest(t *testing.T) {
	w := newShellWriter(t)
	broken := blockBegin + "\n# half a block\n"
	require.NoError(t, os.WriteFile(w.paths.Zshrc, []byte(broken), 0o644))

	reports, err := w.apply(nil, false)

	assert.ErrorIs(t, err, ErrBrokenMarkers)
	assert.Equal(t, broken, read(t, w.paths.Zshrc))
	assert.FileExists(t, filepath.Join(w.paths.ZshDir, "init.zsh"))
	assert.Contains(t, read(t, w.paths.Zshenv), blockBegin)
	assert.Contains(t, reports, FileReport{Path: w.paths.Zshrc, Result: ResultFailed})
}

func TestShellConfigGolden(t *testing.T) {
	w := newShellWriter(t)
	_, err := w.apply(catalogTools(t, w, "mise", "antidote", "starship", "fzf", "zoxide"), false)
	require.NoError(t, err)

	var b strings.Builder
	for _, p := range zshFiles(t, w) {
		b.WriteString("== " + strings.TrimPrefix(p, w.paths.Home) + " ==\n")
		b.WriteString(read(t, p))
	}
	golden.RequireEqual(t, []byte(b.String()))
}

func TestShellConfigIsValidZsh(t *testing.T) {
	zsh, err := lookSystemZsh()
	if err != nil {
		t.Skip("zsh not installed")
	}
	w := newShellWriter(t)
	_, err = w.apply(catalogTools(t, w, "mise", "antidote", "starship", "fzf", "zoxide"), false)
	require.NoError(t, err)
	for _, p := range zshFiles(t, w) {
		out, err := runZshSyntaxCheck(zsh, p)
		assert.NoError(t, err, "%s: %s", p, out)
	}
}

// Agents and scripts run `zsh -c`, which reads only ~/.zshenv.
func TestNonInteractiveZshGetsThePathSilently(t *testing.T) {
	zsh, err := lookSystemZsh()
	if err != nil {
		t.Skip("zsh not installed")
	}
	w := newShellWriter(t)
	_, err = w.apply(catalogTools(t, w, "mise"), false)
	require.NoError(t, err)
	home := w.paths.Home
	shims := filepath.Join(home, ".local", "share", "mise", "shims")
	bin := filepath.Join(home, ".local", "bin")

	out, err := runZsh(zsh, home, "print -r -- $PATH")
	require.NoError(t, err, out)
	assert.True(t, strings.HasPrefix(out, shims+":"+bin+":"), "PATH=%q", out)

	require.NoError(t, os.RemoveAll(w.paths.ZshDir))
	out, err = runZsh(zsh, home, "print -r -- ok")
	require.NoError(t, err, out)
	assert.Equal(t, "ok\n", out, "no output besides the command's own")
}

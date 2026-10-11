package shellmanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const oldStub = "# ~/.zshrc — managed by shell-setup.\nsource \"$HOME/.config/shell-setup/zshrc\"\n"

// seedOldInstall lays out a shell-setup v0.1.x install under the engine's
// home: config dir with a generated zshrc and a fragment, state dir with
// state.toml (hashes for the stub and both files), its log, the old binary
// and the shs alias link.
func seedOldInstall(t *testing.T, e *Engine) oldInstall {
	t.Helper()
	old := oldInstallPaths(e.Paths.Home)
	write := func(path, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	zshrc := filepath.Join(old.ConfigDir, "zshrc")
	fragment := filepath.Join(old.ConfigDir, "zsh.d", "10-a.zsh")
	write(zshrc, "# old generated zshrc\n")
	write(fragment, "# old fragment\n")
	write(e.Paths.Stub, oldStub)
	write(filepath.Join(old.StateDir, "shell-setup.log"), "old log\n")
	st := &State{
		SchemaVersion: stateSchemaVersion,
		Files: map[string]string{
			e.Paths.Stub: hashBytes([]byte(oldStub)),
			zshrc:        hashBytes([]byte("# old generated zshrc\n")),
			fragment:     hashBytes([]byte("# old fragment\n")),
		},
		Versions: map[string]string{"nerdfont": "v3.5.1"},
	}
	require.NoError(t, st.Save(e.System, filepath.Join(old.StateDir, "state.toml")))
	writeExecutable(t, old.Binary)
	require.NoError(t, os.Symlink("shell-setup", old.Alias))
	return old
}

func TestMigrateMovesConfigAndStateAndRewritesStateKeys(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)

	reports, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.NoDirExists(t, old.ConfigDir)
	assert.NoDirExists(t, old.StateDir)
	assert.Equal(t, "# old fragment\n", read(t, filepath.Join(e.Paths.ConfigDir, "zsh.d", "10-a.zsh")))
	assert.FileExists(t, filepath.Join(e.Paths.StateDir, "shell-setup.log"))
	st, err := LoadState(e.System, e.Paths.StateFile)
	require.NoError(t, err)
	assert.Contains(t, st.Files, filepath.Join(e.Paths.ConfigDir, "zsh.d", "10-a.zsh"))
	assert.Contains(t, st.Files, e.Paths.GeneratedZshrc)
	assert.Contains(t, st.Files, e.Paths.Stub)
	assert.NotContains(t, st.Files, filepath.Join(old.ConfigDir, "zshrc"))
	assert.Equal(t, "v3.5.1", st.Versions["nerdfont"])
	assert.Contains(t, reports, FileReport{Path: e.Paths.ConfigDir, Result: ResultMigrated})
	assert.Contains(t, reports, FileReport{Path: e.Paths.StateDir, Result: ResultMigrated})
}

func TestMigrateRemovesOldBinaryAndAliasLink(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)

	reports, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.NoFileExists(t, old.Binary)
	_, err = os.Lstat(old.Alias)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Contains(t, reports, FileReport{Path: old.Binary, Result: ResultRemoved})
	assert.Contains(t, reports, FileReport{Path: old.Alias, Result: ResultRemoved})
}

func TestMigrateKeepsRealFileNamedShs(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := oldInstallPaths(e.Paths.Home)
	writeExecutable(t, old.Alias, "echo mine")

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.Contains(t, read(t, old.Alias), "echo mine")
}

func TestMigrateMergesIntoStateDirCreatedByTheLogger(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)
	// The logger opens the new log before migration runs.
	require.NoError(t, os.MkdirAll(e.Paths.StateDir, 0o755))
	require.NoError(t, os.WriteFile(e.Paths.LogFile, []byte("current run\n"), 0o644))

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.Equal(t, "current run\n", read(t, e.Paths.LogFile), "never overwritten")
	st, err := LoadState(e.System, e.Paths.StateFile)
	require.NoError(t, err)
	assert.Contains(t, st.Files, e.Paths.GeneratedZshrc)
	assert.NoDirExists(t, old.StateDir)
}

func TestMigrateLeavesConflictingEntriesInOldDir(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)
	require.NoError(t, os.MkdirAll(e.Paths.ConfigDir, 0o755))
	require.NoError(t, os.WriteFile(e.Paths.GeneratedZshrc, []byte("# new\n"), 0o644))

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.Equal(t, "# new\n", read(t, e.Paths.GeneratedZshrc), "never overwritten")
	assert.FileExists(t, filepath.Join(old.ConfigDir, "zshrc"), "conflict stays behind")
	assert.FileExists(t, filepath.Join(e.Paths.ConfigDir, "zsh.d", "10-a.zsh"), "the rest moves")
	assert.Contains(t, e.oldInstallLeftovers(), old.ConfigDir)
}

func TestMigrateIsIdempotent(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	seedOldInstall(t, e)
	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	reports, err := e.migrateFromShellSetup()
	require.NoError(t, err)
	assert.Empty(t, reports)
	// Only the compatibility link remains until setup rewrites ~/.zshrc.
	assert.Equal(t, []string{oldInstallPaths(e.Paths.Home).ConfigDir}, e.oldInstallLeftovers())
}

func TestSetupAfterMigrationRewritesStubWithoutBackup(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	seedOldInstall(t, e)

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Contains(t, read(t, e.Paths.Stub), `source "$HOME/.config/shell-manager/zshrc"`)
	backups, err := filepath.Glob(filepath.Join(e.Paths.Home, ".zshrc.bak.*"))
	require.NoError(t, err)
	assert.Empty(t, backups)
	res, ok := fileResult(rep, e.Paths.ConfigDir)
	assert.True(t, ok)
	assert.Equal(t, ResultMigrated, res)
}

func TestDoctorReportsShellSetupLeftovers(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := oldInstallPaths(e.Paths.Home)
	writeExecutable(t, old.Binary)

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	c := checksByName(rep)["shell-setup leftovers"]
	assert.Equal(t, CheckWarn, c.Status)
	assert.Contains(t, c.Detail, old.Binary)
	assert.Contains(t, c.Detail, "run shm setup")
}

func TestMigrateClearsOldStateDirWhenBootstrapStampExistsInBoth(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)
	require.NoError(t, os.WriteFile(filepath.Join(old.StateDir, "bootstrap.sha256"), []byte("old\n"), 0o644))
	// The new install.sh writes its stamp before setup runs.
	require.NoError(t, os.MkdirAll(e.Paths.StateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(e.Paths.StateDir, "bootstrap.sha256"), []byte("new\n"), 0o644))

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	assert.NoDirExists(t, old.StateDir)
	assert.Equal(t, "new\n", read(t, filepath.Join(e.Paths.StateDir, "bootstrap.sha256")))
	assert.NotContains(t, e.oldInstallLeftovers(), old.StateDir)
}

func TestOldStubKeepsWorkingIfSetupStopsAfterMigration(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, `system_packages = ["git"]`)})
	old := seedOldInstall(t, e)
	rec.setFail("sudo apt-get update -qq", errExit)

	_, _, err := runOp(t, setupOp(e))
	require.Error(t, err, "apt fails after the migration ran")

	assert.Equal(t, oldStub, read(t, e.Paths.Stub), "stub not rewritten yet")
	// The old stub sources ~/.config/shell-setup/zshrc: it must still resolve.
	assert.Equal(t, "# old generated zshrc\n", read(t, filepath.Join(old.ConfigDir, "zshrc")))
}

func TestSetupAfterMigrationRemovesCompatLink(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := seedOldInstall(t, e)

	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	_, err = os.Lstat(old.ConfigDir)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Empty(t, e.oldInstallLeftovers())
	backups, err := filepath.Glob(filepath.Join(e.Paths.ConfigDir, "*.bak.*"))
	require.NoError(t, err)
	assert.Empty(t, backups, "generated zshrc rewritten without backup")
}

func TestMigrateKeepsForeignShsLink(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	old := oldInstallPaths(e.Paths.Home)
	require.NoError(t, os.MkdirAll(filepath.Dir(old.Alias), 0o755))
	require.NoError(t, os.Symlink("/usr/bin/some-other-tool", old.Alias))

	_, err := e.migrateFromShellSetup()
	require.NoError(t, err)

	target, err := os.Readlink(old.Alias)
	require.NoError(t, err)
	assert.Equal(t, "/usr/bin/some-other-tool", target)
	assert.Empty(t, e.oldInstallLeftovers())
}

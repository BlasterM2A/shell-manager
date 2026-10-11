package shellmanager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const legacyBin = `
[legacy]
paths = ["~/.local/bin/a-old", "~/.local/share/fonts/A*.ttf"]
apt = ["a-apt"]
`

func TestSetupPurgesLegacyOfInstalledTool(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindPlugin, "") + legacyBin})
	oldBin := filepath.Join(e.Paths.Home, ".local", "bin", "a-old")
	writeExecutable(t, oldBin)
	fonts := filepath.Join(e.Paths.Home, ".local", "share", "fonts")
	require.NoError(t, os.MkdirAll(filepath.Join(fonts, "keep"), 0o755))
	for _, f := range []string{"A-Regular.ttf", "A-Bold.ttf", "B-Regular.ttf"} {
		require.NoError(t, os.WriteFile(filepath.Join(fonts, f), nil, 0o644))
	}
	rec.out["dpkg-query -W -f=${Status} a-apt"] = "install ok installed"

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.NoFileExists(t, oldBin)
	assert.NoFileExists(t, filepath.Join(fonts, "A-Regular.ttf"))
	assert.NoFileExists(t, filepath.Join(fonts, "A-Bold.ttf"))
	assert.FileExists(t, filepath.Join(fonts, "B-Regular.ttf"), "only matching files go")
	assert.Contains(t, rec.commands(), "sudo apt-get remove -y a-apt")
	res, ok := fileResult(rep, oldBin)
	assert.True(t, ok)
	assert.Equal(t, ResultRemoved, res)
	res, _ = fileResult(rep, "apt:a-apt")
	assert.Equal(t, ResultRemoved, res)
}

func TestSetupKeepsLegacyWhenToolFails(t *testing.T) {
	e, rec, backend := newTestEngine(t, map[string]string{"a": fakeTool("a", KindPlugin, "") + legacyBin})
	backend.failures["a"] = errors.New("boom")
	oldBin := filepath.Join(e.Paths.Home, ".local", "bin", "a-old")
	writeExecutable(t, oldBin)
	rec.out["dpkg-query -W -f=${Status} a-apt"] = "install ok installed"

	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.FileExists(t, oldBin, "the old copy is still the user's only one")
	assert.NotContains(t, rec.commands(), "sudo apt-get remove -y a-apt")
}

func TestSetupSkipsAbsentLegacy(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindPlugin, "") + legacyBin})

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.NotContains(t, rec.commands(), "sudo apt-get remove -y a-apt")
	_, reported := fileResult(rep, "apt:a-apt")
	assert.False(t, reported, "nothing to purge, nothing reported")
}

func TestLegacyPurgeFailureIsAWarning(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindPlugin, "") + legacyBin})
	rec.out["dpkg-query -W -f=${Status} a-apt"] = "install ok installed"
	rec.setFail("sudo apt-get remove -y a-apt", errExit)

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	res, _ := fileResult(rep, "apt:a-apt")
	assert.Equal(t, ResultWarned, res)
	assert.False(t, rep.Failed())
}

func TestDoctorReportsLegacyLeftovers(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindPlugin, "") + legacyBin})
	oldBin := filepath.Join(e.Paths.Home, ".local", "bin", "a-old")
	writeExecutable(t, oldBin)
	rec.out["dpkg-query -W -f=${Status} a-apt"] = "install ok installed"

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	checks := checksByName(rep)
	assert.Equal(t, CheckWarn, checks[oldBin].Status)
	assert.Contains(t, checks[oldBin].Detail, "shm update")
	assert.Equal(t, CheckWarn, checks["apt:a-apt"].Status)
}

func TestReportShellChanged(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)
	assert.False(t, rep.ShellChanged, "login shell already zsh")

	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/bin/bash"
	rep, _, err = runOp(t, setupOp(e))
	require.NoError(t, err)
	assert.True(t, rep.ShellChanged)
}

const legacyWithHook = `
[legacy]
paths = ["~/.local/share/fonts/A*.ttf"]
after_purge = [["fc-cache", "-f"]]
`

func TestAfterPurgeRunsOnlyWhenSomethingWasRemoved(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindPlugin, "") + legacyWithHook})

	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)
	assert.NotContains(t, rec.commands(), "fc-cache -f", "nothing removed, no hook")

	fonts := filepath.Join(e.Paths.Home, ".local", "share", "fonts")
	require.NoError(t, os.MkdirAll(fonts, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fonts, "A-Regular.ttf"), nil, 0o644))
	_, _, err = runOp(t, setupOp(e))
	require.NoError(t, err)
	assert.Contains(t, rec.commands(), "fc-cache -f")
}

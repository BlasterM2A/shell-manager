package shellmanager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checksByName(rep Report) map[string]CheckResult {
	m := map[string]CheckResult{}
	for _, c := range rep.Checks {
		m[c.Name] = c
	}
	return m
}

func TestDoctorReportsTools(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{
		"a": fakeTool("a", KindBuiltin, ""),
		"b": fakeTool("b", KindPlugin, "optional = true"),
		"c": fakeTool("c", KindPlugin, ""),
	})
	rec.setFail("a --version", nil)
	rec.out["a --version"] = "a 1.2.3\nextra"

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	checks := checksByName(rep)
	assert.Equal(t, CheckResult{Name: "a", Status: CheckOK, Detail: "a 1.2.3"}, checks["a"])
	assert.Equal(t, CheckWarn, checks["b"].Status)
	assert.Equal(t, CheckFail, checks["c"].Status)
	assert.Equal(t, CheckOK, checks["default shell"].Status)
	assert.True(t, rep.Failed())
}

func TestDoctorFlagsModifiedAndMissingManagedFiles(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)
	initZsh := filepath.Join(e.Paths.ZshDir, "init.zsh")
	envZsh := filepath.Join(e.Paths.ZshDir, "env.zsh")
	require.NoError(t, os.WriteFile(initZsh, []byte("hand edit"), 0o644))
	require.NoError(t, os.Remove(envZsh))

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	checks := checksByName(rep)
	assert.Equal(t, CheckWarn, checks[initZsh].Status)
	assert.Contains(t, checks[initZsh].Detail, "modified")
	assert.Equal(t, CheckWarn, checks[envZsh].Status)
	assert.Contains(t, checks[envZsh].Detail, "missing")
	assert.Equal(t, CheckOK, checks[filepath.Join(e.Paths.ZshDir, "tools", "20-core.zsh")].Status)
}

func TestDoctorWarnsAboutLoginShell(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/bin/bash"

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	assert.Equal(t, CheckWarn, checksByName(rep)["default shell"].Status)
}

func TestDoctorReportsMiseToolOutsideMiseAsNotInstalled(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": miseTool("a")})
	e.Backends["mise"] = miseBackend{}
	rec.setFail("a --version", nil) // apt-only copy
	rec.setFail("mise which a", errExit)

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	assert.Equal(t, CheckResult{Name: "a", Status: CheckFail, Detail: "not installed"}, checksByName(rep)["a"])
}

func TestDoctorDoesNotWrite(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	_, err := e.Doctor(context.Background())
	require.NoError(t, err)
	assert.NoFileExists(t, e.Paths.StateFile)
	assert.NoFileExists(t, e.Paths.LockFile)
}

func TestDoctorReportsShmBlocks(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	checks := checksByName(mustDoctor(t, e))
	assert.Equal(t, CheckOK, checks[e.Paths.Zshrc].Status)
	assert.Equal(t, CheckOK, checks[e.Paths.Zshenv].Status)

	require.NoError(t, os.WriteFile(e.Paths.Zshrc, []byte("export FOO=1\n"), 0o644)) // block deleted
	zshenv := read(t, e.Paths.Zshenv)
	require.NoError(t, os.WriteFile(e.Paths.Zshenv, []byte(strings.Replace(zshenv, "skip_global_compinit=1", "X=1", 1)), 0o644))
	checks = checksByName(mustDoctor(t, e))
	assert.Equal(t, CheckWarn, checks[e.Paths.Zshrc].Status)
	assert.Contains(t, checks[e.Paths.Zshrc].Detail, "missing")
	assert.Equal(t, CheckWarn, checks[e.Paths.Zshenv].Status)
	assert.Contains(t, checks[e.Paths.Zshenv].Detail, "modified")

	require.NoError(t, os.WriteFile(e.Paths.Zshrc, []byte(blockBegin+"\n"), 0o644))
	checks = checksByName(mustDoctor(t, e))
	assert.Equal(t, CheckWarn, checks[e.Paths.Zshrc].Status)
	assert.Contains(t, checks[e.Paths.Zshrc].Detail, "broken")
}

func mustDoctor(t *testing.T, e *Engine) Report {
	t.Helper()
	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)
	return rep
}

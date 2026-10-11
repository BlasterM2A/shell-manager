package shellsetup

import (
	"context"
	"os"
	"path/filepath"
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
	_, _, err := runOp(t, initOp(e))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(e.Paths.Stub, []byte("hand edit"), 0o644))
	require.NoError(t, os.Remove(e.Paths.GeneratedZshrc))

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)

	checks := checksByName(rep)
	assert.Equal(t, CheckWarn, checks[e.Paths.Stub].Status)
	assert.Contains(t, checks[e.Paths.Stub].Detail, "modified")
	assert.Equal(t, CheckWarn, checks[e.Paths.GeneratedZshrc].Status)
	assert.Contains(t, checks[e.Paths.GeneratedZshrc].Detail, "missing")
	assert.Equal(t, CheckOK, checks[filepath.Join(e.Paths.ZshD, "20-core.zsh")].Status)
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

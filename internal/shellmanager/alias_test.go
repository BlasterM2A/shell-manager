package shellmanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAliasEngine is a test engine whose binary lives in ~/.local/bin.
func newAliasEngine(t *testing.T) *Engine {
	t.Helper()
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	e.Executable = filepath.Join(e.Paths.Home, ".local", "bin", "shell-manager")
	writeExecutable(t, e.Executable)
	return e
}

func fileResult(rep Report, path string) (Result, bool) {
	for _, f := range rep.Files {
		if f.Path == path {
			return f.Result, true
		}
	}
	return "", false
}

func TestInitCreatesAliasLink(t *testing.T) {
	e := newAliasEngine(t)

	rep, _, err := runOp(t, initOp(e))
	require.NoError(t, err)

	target, err := os.Readlink(e.Paths.AliasLink)
	require.NoError(t, err)
	assert.Equal(t, "shell-manager", target, "relative link next to the binary")
	res, ok := fileResult(rep, e.Paths.AliasLink)
	assert.True(t, ok)
	assert.Equal(t, ResultOK, res)
}

func TestInitAliasIsIdempotent(t *testing.T) {
	e := newAliasEngine(t)
	_, _, err := runOp(t, initOp(e))
	require.NoError(t, err)

	_, _, err = runOp(t, initOp(e))
	require.NoError(t, err)
	target, err := os.Readlink(e.Paths.AliasLink)
	require.NoError(t, err)
	assert.Equal(t, "shell-manager", target)
}

func TestInitRepointsStaleAliasLink(t *testing.T) {
	e := newAliasEngine(t)
	require.NoError(t, os.Symlink("/usr/bin/somewhere-else", e.Paths.AliasLink))

	_, _, err := runOp(t, initOp(e))
	require.NoError(t, err)

	target, err := os.Readlink(e.Paths.AliasLink)
	require.NoError(t, err)
	assert.Equal(t, "shell-manager", target)
}

func TestInitKeepsRealFileNamedLikeAlias(t *testing.T) {
	e := newAliasEngine(t)
	require.NoError(t, os.WriteFile(e.Paths.AliasLink, []byte("#!/bin/sh\necho mine\n"), 0o755))

	rep, _, err := runOp(t, initOp(e))
	require.NoError(t, err)

	assert.Equal(t, "#!/bin/sh\necho mine\n", read(t, e.Paths.AliasLink))
	res, _ := fileResult(rep, e.Paths.AliasLink)
	assert.Equal(t, ResultSkipped, res)
}

func TestAliasLinkIsAbsoluteWhenBinaryLivesElsewhere(t *testing.T) {
	e := newAliasEngine(t)
	e.Executable = filepath.Join(t.TempDir(), "opt", "shell-manager")
	writeExecutable(t, e.Executable)

	_, _, err := runOp(t, initOp(e))
	require.NoError(t, err)

	target, err := os.Readlink(e.Paths.AliasLink)
	require.NoError(t, err)
	assert.Equal(t, e.Executable, target)
}

func TestInitWithoutExecutableSkipsAlias(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})

	_, _, err := runOp(t, initOp(e))
	require.NoError(t, err)
	_, err = os.Lstat(e.Paths.AliasLink)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestDoctorChecksAlias(t *testing.T) {
	e := newAliasEngine(t)

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)
	c := checksByName(rep)[AliasName]
	assert.Equal(t, CheckWarn, c.Status)
	assert.Contains(t, c.Detail, "missing")

	_, _, err = runOp(t, initOp(e))
	require.NoError(t, err)
	rep, err = e.Doctor(context.Background())
	require.NoError(t, err)
	assert.Equal(t, CheckOK, checksByName(rep)[AliasName].Status)

	require.NoError(t, os.Remove(e.Paths.AliasLink))
	require.NoError(t, os.WriteFile(e.Paths.AliasLink, []byte("x"), 0o755))
	rep, err = e.Doctor(context.Background())
	require.NoError(t, err)
	c = checksByName(rep)[AliasName]
	assert.Equal(t, CheckWarn, c.Status)
	assert.Contains(t, c.Detail, "not a link")
}

func TestDoctorFlagsAliasPointingElsewhere(t *testing.T) {
	e := newAliasEngine(t)
	require.NoError(t, os.Symlink("/usr/bin/somewhere-else", e.Paths.AliasLink))

	rep, err := e.Doctor(context.Background())
	require.NoError(t, err)
	c := checksByName(rep)[AliasName]
	assert.Equal(t, CheckWarn, c.Status)
	assert.Contains(t, c.Detail, "/usr/bin/somewhere-else")
}

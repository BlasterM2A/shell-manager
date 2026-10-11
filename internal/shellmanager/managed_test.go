package shellmanager

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var fixedNow = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

const backupSuffix = ".bak.20261010120000"

func newWriter(t *testing.T) (fileWriter, string) {
	sys, _ := newTestSystem(t)
	st := &State{Files: map[string]string{}, Versions: map[string]string{}}
	return fileWriter{sys: sys, state: st, now: func() time.Time { return fixedNow }},
		filepath.Join(sys.HomeDir(), ".zshrc")
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestWriteMissingFile(t *testing.T) {
	w, path := newWriter(t)
	res, err := w.write(path, []byte("new"), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, "new", read(t, path))
	assert.Equal(t, hashBytes([]byte("new")), w.state.Files[path])
}

func TestWriteFirstRunBacksUpUnmanagedFile(t *testing.T) {
	w, path := newWriter(t)
	require.NoError(t, os.WriteFile(path, []byte("mine"), 0o644))

	res, err := w.write(path, []byte("new"), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, "new", read(t, path))
	assert.Equal(t, "mine", read(t, path+backupSuffix))
}

func TestWriteFirstRunIdenticalContentNoBackup(t *testing.T) {
	w, path := newWriter(t)
	require.NoError(t, os.WriteFile(path, []byte("same"), 0o644))

	res, err := w.write(path, []byte("same"), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.NoFileExists(t, path+backupSuffix)
	assert.Equal(t, hashBytes([]byte("same")), w.state.Files[path])
}

func TestWriteUnmodifiedManagedFileUpdatesWithoutBackup(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.write(path, []byte("v1"), false)
	require.NoError(t, err)

	res, err := w.write(path, []byte("v2"), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, "v2", read(t, path))
	assert.NoFileExists(t, path+backupSuffix)
}

func TestWriteModifiedManagedFileIsLeftAlone(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.write(path, []byte("v1"), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("hand edit"), 0o644))

	res, err := w.write(path, []byte("v2"), false)
	require.NoError(t, err)
	assert.Equal(t, ResultModified, res)
	assert.Equal(t, "hand edit", read(t, path))
}

func TestWriteModifiedManagedFileWithForceBacksUp(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.write(path, []byte("v1"), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("hand edit"), 0o644))

	res, err := w.write(path, []byte("v2"), true)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, "v2", read(t, path))
	assert.Equal(t, "hand edit", read(t, path+backupSuffix))
}

func TestRemoveManagedFile(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.write(path, []byte("v1"), false)
	require.NoError(t, err)

	res, err := w.remove(path)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.NoFileExists(t, path)
	assert.NotContains(t, w.state.Files, path)
}

func TestRemoveKeepsHandEditedFile(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.write(path, []byte("v1"), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("hand edit"), 0o644))

	res, err := w.remove(path)
	require.NoError(t, err)
	assert.Equal(t, ResultModified, res)
	assert.FileExists(t, path)
}

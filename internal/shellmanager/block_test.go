package shellmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testBlock = "# >>> shm >>>\nsource x\n# <<< shm <<<\n"

func TestFindBlock(t *testing.T) {
	cases := map[string]struct {
		in         string
		start, end int // end < 0 means len(in)
		found      bool
		broken     bool
	}{
		"none":              {in: "a\nb\n"},
		"top":               {in: testBlock + "rest\n", start: 0, end: len(testBlock), found: true},
		"middle":            {in: "a\n" + testBlock + "b\n", start: 2, end: 2 + len(testBlock), found: true},
		"no final newline":  {in: "# >>> shm >>>\nx\n# <<< shm <<<", start: 0, end: -1, found: true},
		"crlf and spaces":   {in: "# >>> shm >>>  \r\nx\r\n# <<< shm <<<\r\n", start: 0, end: -1, found: true},
		"only begin":        {in: "# >>> shm >>>\nx\n", broken: true},
		"only end":          {in: "x\n# <<< shm <<<\n", broken: true},
		"end before begin":  {in: "# <<< shm <<<\n# >>> shm >>>\n", broken: true},
		"duplicated":        {in: testBlock + testBlock, broken: true},
		"indented is plain": {in: "  # >>> shm >>>\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			start, end, found, err := findBlock([]byte(tc.in))
			if tc.broken {
				assert.ErrorIs(t, err, ErrBrokenMarkers)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.found, found)
			if tc.end < 0 {
				tc.end = len(tc.in)
			}
			assert.Equal(t, tc.start, start)
			assert.Equal(t, tc.end, end)
		})
	}
}

func TestWriteBlockCreatesMissingFile(t *testing.T) {
	w, path := newWriter(t)
	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock, read(t, path))
	assert.Equal(t, hashBytes([]byte(testBlock)), w.state.Blocks[path])
	assert.NotContains(t, w.state.Files, path)
}

func TestWriteBlockInsertsAtTopAndKeepsContent(t *testing.T) {
	w, path := newWriter(t)
	require.NoError(t, os.WriteFile(path, []byte("export FOO=1"), 0o644)) // no final newline

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"\nexport FOO=1", read(t, path))
	assert.Equal(t, "export FOO=1", read(t, path+backupSuffix))
}

func TestWriteBlockEmptyFileGetsNoBackup(t *testing.T) {
	w, path := newWriter(t)
	require.NoError(t, os.WriteFile(path, nil, 0o644))

	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, testBlock, read(t, path))
	assert.NoFileExists(t, path+backupSuffix)
}

func TestWriteBlockUnchangedIsNoop(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(testBlock+"# mine\n"), 0o644)) // outside the block

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"# mine\n", read(t, path))
	assert.NoFileExists(t, path+backupSuffix)
}

func TestWriteBlockUpdatesTemplateChangeInPlace(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("# above\n"+testBlock+"# below\n"), 0o644))
	v2 := "# >>> shm >>>\nsource y\n# <<< shm <<<\n"

	res, err := w.writeBlock(path, []byte(v2), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, "# above\n"+v2+"# below\n", read(t, path))
	assert.NoFileExists(t, path+backupSuffix)
	assert.Equal(t, hashBytes([]byte(v2)), w.state.Blocks[path])
}

func TestWriteBlockEditedIsLeftAlone(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	edited := "# >>> shm >>>\nsource x\nexport MINE=1\n# <<< shm <<<\n"
	require.NoError(t, os.WriteFile(path, []byte(edited), 0o644))

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultModified, res)
	assert.Equal(t, edited, read(t, path))
}

func TestWriteBlockEditedWithForceBacksUp(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	edited := "# >>> shm >>>\nexport MINE=1\n# <<< shm <<<\n# tail\n"
	require.NoError(t, os.WriteFile(path, []byte(edited), 0o644))

	res, err := w.writeBlock(path, []byte(testBlock), true)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"# tail\n", read(t, path))
	assert.Equal(t, edited, read(t, path+backupSuffix))
}

func TestWriteBlockReinsertsDeletedBlock(t *testing.T) {
	w, path := newWriter(t)
	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte("export FOO=1\n"), 0o644)) // block deleted by hand

	res, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)
	assert.Equal(t, ResultOK, res)
	assert.Equal(t, testBlock+"\nexport FOO=1\n", read(t, path))
	assert.Equal(t, "export FOO=1\n", read(t, path+backupSuffix))
}

func TestWriteBlockBrokenMarkersLeaveFileAlone(t *testing.T) {
	w, path := newWriter(t)
	broken := "# >>> shm >>>\nsource x\n"
	require.NoError(t, os.WriteFile(path, []byte(broken), 0o644))

	res, err := w.writeBlock(path, []byte(testBlock), false)
	assert.ErrorIs(t, err, ErrBrokenMarkers)
	assert.ErrorContains(t, err, path)
	assert.Equal(t, ResultFailed, res)
	assert.Equal(t, broken, read(t, path))
}

func TestWriteBlockWritesThroughSymlink(t *testing.T) {
	w, path := newWriter(t)
	target := filepath.Join(filepath.Dir(path), "dotfiles", "zshrc")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("# from dotfiles\n"), 0o644))
	require.NoError(t, os.Symlink("dotfiles/zshrc", path)) // relative, like stow

	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)

	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the link stays a link")
	assert.Equal(t, testBlock+"\n# from dotfiles\n", read(t, target))
	assert.Equal(t, hashBytes([]byte(testBlock)), w.state.Blocks[path], "state is keyed by the link path")
}

func TestWriteBlockKeepsFilePermissions(t *testing.T) {
	w, path := newWriter(t)
	require.NoError(t, os.WriteFile(path, []byte("export TOKEN=secret\n"), 0o600))

	_, err := w.writeBlock(path, []byte(testBlock), false)
	require.NoError(t, err)

	for _, p := range []string{path, path + backupSuffix} {
		info, err := os.Stat(p)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), p)
	}
}

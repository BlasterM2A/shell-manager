package selfupdate

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckReleaseVersion(t *testing.T) {
	for _, v := range []string{"main", "abc1234", "", "dev", "none"} {
		assert.ErrorIs(t, checkReleaseVersion(v), ErrDevBuild, v)
	}
	for _, v := range []string{"v0.1.0", "0.1.0", "1.2.3-rc.1"} {
		assert.NoError(t, checkReleaseVersion(v), v)
	}
}

func TestNonReleaseVersionsDoNotPanic(t *testing.T) {
	for _, v := range []string{"main", "abc1234", ""} {
		u := New("BlasterM2A/shell-manager", v)
		_, _, err := u.Latest(context.Background())
		assert.ErrorIs(t, err, ErrDevBuild)
		_, _, err = u.Apply(context.Background())
		assert.ErrorIs(t, err, ErrDevBuild)
	}
}

func TestEnsureWritable(t *testing.T) {
	assert.NoError(t, ensureWritable(t.TempDir()))

	if os.Geteuid() == 0 {
		t.Skip("root can write to read-only directories")
	}
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	assert.Error(t, ensureWritable(dir))
}

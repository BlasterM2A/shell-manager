package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execRecorder replaces the real exec: it records what would have replaced
// the process.
type execRecorder struct {
	path string
	args []string
}

func (r *execRecorder) exec(path string, args []string) error {
	r.path, r.args = path, args
	return nil
}

func applyNewConfigFor(t *testing.T, exe string, plain, verbose, quiet bool) (*execRecorder, string) {
	t.Helper()
	f, out := newTestFactory(t)
	f.Plain, f.Verbose, f.Quiet = plain, verbose, quiet
	rec := &execRecorder{}
	f.Exec = rec.exec
	f.Executable = func() string { return exe }
	require.NoError(t, applyNewConfig(f))
	return rec, out.String()
}

func TestApplyNewConfigRunsTheNewBinaryConfigOnly(t *testing.T) {
	rec, _ := applyNewConfigFor(t, "/bin/shell-manager", false, false, false)
	assert.Equal(t, "/bin/shell-manager", rec.path)
	assert.Equal(t, []string{"shell-manager", "update", "--config-only"}, rec.args)
}

func TestApplyNewConfigForwardsGlobalFlags(t *testing.T) {
	rec, _ := applyNewConfigFor(t, "/bin/shell-manager", true, true, true)
	assert.Equal(t, []string{"shell-manager", "update", "--config-only", "--plain", "--verbose", "--quiet"}, rec.args)
}

func TestApplyNewConfigWithoutExecutableOnlyHints(t *testing.T) {
	rec, out := applyNewConfigFor(t, "", false, false, false)
	assert.Empty(t, rec.path)
	assert.Contains(t, out, "shm update --config-only")
}

func TestUpdateConfigOnlyFlag(t *testing.T) {
	f, _ := newTestFactory(t)
	var got *UpdateOptions
	cmd := newUpdateCmd(f, func(_ context.Context, o *UpdateOptions) error { got = o; return nil })
	cmd.SetArgs([]string{"--config-only"})

	require.NoError(t, cmd.Execute())
	assert.True(t, got.ConfigOnly)
}

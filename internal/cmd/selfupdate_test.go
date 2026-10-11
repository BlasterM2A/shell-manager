package cmd

import (
	"strings"
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

func offerFor(t *testing.T, tty bool, answer string, plain, verbose, quiet bool) (*execRecorder, string) {
	t.Helper()
	f, out := newTestFactory(t)
	f.IOStreams.In = strings.NewReader(answer)
	f.Plain, f.Verbose, f.Quiet = plain, verbose, quiet
	rec := &execRecorder{}
	f.Exec = rec.exec
	f.Executable = func() string { return "/bin/shell-manager" }
	require.NoError(t, offerUpdate(f, tty))
	return rec, out.String()
}

func TestOfferUpdateRunsUpdateOnYes(t *testing.T) {
	for _, answer := range []string{"\n", "y\n", "Y\n", "yes\n", ""} {
		rec, out := offerFor(t, true, answer, false, false, false)
		assert.Equal(t, "/bin/shell-manager", rec.path, "answer %q", answer)
		assert.Equal(t, []string{"shell-manager", "update"}, rec.args, "answer %q", answer)
		assert.Contains(t, out, "[Y/n]")
	}
}

func TestOfferUpdateSkipsOnNo(t *testing.T) {
	rec, out := offerFor(t, true, "n\n", false, false, false)
	assert.Empty(t, rec.path)
	assert.Contains(t, out, "shm update")
}

func TestOfferUpdateWithoutTerminalOnlyHints(t *testing.T) {
	rec, out := offerFor(t, false, "\n", false, false, false)
	assert.Empty(t, rec.path)
	assert.NotContains(t, out, "[Y/n]")
	assert.Contains(t, out, "Run `shm update`")
}

func TestOfferUpdateForwardsGlobalFlags(t *testing.T) {
	rec, _ := offerFor(t, true, "y\n", true, true, true)
	assert.Equal(t, []string{"shell-manager", "update", "--plain", "--verbose", "--quiet"}, rec.args)
}

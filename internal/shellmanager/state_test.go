package shellmanager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateRoundTrip(t *testing.T) {
	sys, _ := newTestSystem(t)
	path := filepath.Join(sys.HomeDir(), "state.toml")

	st, err := LoadState(sys, path)
	require.NoError(t, err)
	assert.Equal(t, stateSchemaVersion, st.SchemaVersion)
	st.Files["/h/.zshrc"] = "abc"
	st.Versions["nerdfont"] = "v3.4.0"
	require.NoError(t, st.Save(sys, path))

	again, err := LoadState(sys, path)
	require.NoError(t, err)
	assert.Equal(t, st, again)
}

func TestStateFromNewerVersionIsRejected(t *testing.T) {
	sys, _ := newTestSystem(t)
	path := filepath.Join(sys.HomeDir(), "state.toml")
	require.NoError(t, os.WriteFile(path, []byte("schema_version = 99\n"), 0o644))

	_, err := LoadState(sys, path)
	assert.ErrorContains(t, err, "newer shell-manager")
}

func TestReportFailed(t *testing.T) {
	assert.False(t, Report{Tools: []ToolReport{{Result: ResultOK}, {Result: ResultWarned}}}.Failed())
	assert.True(t, Report{Tools: []ToolReport{{Result: ResultFailed}}}.Failed())
	assert.True(t, Report{Checks: []CheckResult{{Status: CheckFail}}}.Failed())
	assert.False(t, Report{Checks: []CheckResult{{Status: CheckWarn}}}.Failed())
}

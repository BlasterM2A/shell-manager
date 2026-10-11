package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BlasterM2A/shell-manager/internal/iostreams"
)

func newTestFactory(t *testing.T) (*Factory, *bytes.Buffer) {
	t.Helper()
	ios, out, _ := iostreams.Test()
	return NewFactory(ios, t.TempDir()), out
}

func TestRootVersion(t *testing.T) {
	f, _ := newTestFactory(t)
	root := NewRootCmd(f)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})

	require.NoError(t, root.Execute())
	assert.Equal(t, "shell-manager dev (none)\n", out.String())
}

func TestRootRegistersCommands(t *testing.T) {
	f, _ := newTestFactory(t)
	var names []string
	for _, c := range NewRootCmd(f).Commands() {
		names = append(names, c.Name())
	}
	assert.Subset(t, names, []string{"setup", "update", "doctor", "self-update"})
	assert.NotContains(t, names, "init")
}

func TestGlobalFlags(t *testing.T) {
	f, _ := newTestFactory(t)
	root := NewRootCmd(f)
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--plain", "--verbose", "--version"})

	require.NoError(t, root.Execute())
	assert.True(t, f.Plain)
	assert.True(t, f.Verbose)
}

func TestExitErrorMessage(t *testing.T) {
	assert.Equal(t, "exit status 3", (&ExitError{Code: 3}).Error())
}

package shellmanager

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func realSystem(t *testing.T) *RealSystem {
	return NewRealSystem(t.TempDir(), slog.New(slog.DiscardHandler))
}

func TestWriteFileIsAtomicAndCreatesDirs(t *testing.T) {
	sys := realSystem(t)
	path := filepath.Join(sys.HomeDir(), "a", "b", "file.txt")

	require.NoError(t, sys.WriteFile(path, []byte("hello"), 0o600))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".*.tmp-*"))
	require.NoError(t, err)
	assert.Empty(t, leftovers)
}

func TestRunPrefersUserBin(t *testing.T) {
	sys := realSystem(t)
	writeExecutable(t, filepath.Join(sys.HomeDir(), ".local", "bin", "hello"), "echo from-home")

	out, err := sys.Run(context.Background(), Cmd{Name: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "from-home\n", string(out))
}

func TestRunErrorIncludesOutputTail(t *testing.T) {
	sys := realSystem(t)
	writeExecutable(t, filepath.Join(sys.HomeDir(), ".local", "bin", "boom"), "echo exploded", "exit 3")

	_, err := sys.Run(context.Background(), Cmd{Name: "boom"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exploded")
}

func TestRunMissingCommand(t *testing.T) {
	_, err := realSystem(t).Run(context.Background(), Cmd{Name: "definitely-not-a-command-xyz"})
	assert.Error(t, err)
}

func TestInteractiveRunGoesThroughHandoff(t *testing.T) {
	sys := realSystem(t)
	writeExecutable(t, filepath.Join(sys.HomeDir(), ".local", "bin", "ok"), "exit 0")
	called := false
	sys.SetTerminalHandoff(func(fn func() error) error { called = true; return fn() })

	_, err := sys.Run(context.Background(), Cmd{Name: "ok", Interactive: true})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestLockIsExclusive(t *testing.T) {
	sys := realSystem(t)
	path := filepath.Join(sys.HomeDir(), "state", "lock")

	unlock, err := sys.Lock(path)
	require.NoError(t, err)
	_, err = sys.Lock(path)
	assert.ErrorIs(t, err, ErrLocked)

	unlock()
	unlock2, err := sys.Lock(path)
	require.NoError(t, err)
	unlock2()
}

func TestDryRunRecordsWithoutTouching(t *testing.T) {
	base := realSystem(t)
	d := &DryRunSystem{Base: base}
	path := filepath.Join(base.HomeDir(), "x.txt")

	require.NoError(t, d.WriteFile(path, []byte("x"), 0o644))
	_, err := d.Run(context.Background(), Cmd{Name: "apt-get", Args: []string{"install", "-y", "zsh"}})
	require.NoError(t, err)

	assert.Equal(t, []string{"write " + path, "run apt-get install -y zsh"}, d.Ops)
	_, err = os.Stat(path)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestPaths(t *testing.T) {
	p := NewPaths("/h")
	assert.Equal(t, "/h/.zshrc", p.Stub)
	assert.Equal(t, "/h/.config/shell-manager/zsh.d", p.ZshD)
	assert.Equal(t, "/h/.local/state/shell-manager/state.toml", p.StateFile)
	assert.Equal(t, "/h/.antidote", p.Expand("~/.antidote"))
	assert.Equal(t, "/etc/x", p.Expand("/etc/x"))
}

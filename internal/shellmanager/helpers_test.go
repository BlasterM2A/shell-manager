package shellmanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var errExit = errors.New("exit status 1")

// recorder replaces command execution: it records every command and
// answers from out/fail, keyed by Cmd.String().
type recorder struct {
	mu   sync.Mutex
	cmds []string
	out  map[string]string
	fail map[string]error
	// after runs a hook once the keyed command has been recorded.
	after map[string]func()
}

func newRecorder() *recorder {
	return &recorder{out: map[string]string{}, fail: map[string]error{}, after: map[string]func(){}}
}

func (r *recorder) exec(_ context.Context, c Cmd) ([]byte, error) {
	r.mu.Lock()
	s := c.String()
	r.cmds = append(r.cmds, s)
	out, err, hook := []byte(r.out[s]), r.fail[s], r.after[s]
	r.mu.Unlock()
	if hook != nil {
		hook()
	}
	return out, err
}

// setFail makes cmd fail with err; a nil err makes it succeed again.
func (r *recorder) setFail(cmd string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		delete(r.fail, cmd)
		return
	}
	r.fail[cmd] = err
}

func (r *recorder) commands() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.cmds)
}

// newTestSystem is a RealSystem on a temporary HOME whose commands are
// recorded instead of executed.
func newTestSystem(t *testing.T) (*RealSystem, *recorder) {
	t.Helper()
	sys := NewRealSystem(t.TempDir(), slog.New(slog.DiscardHandler))
	rec := newRecorder()
	sys.execFn = rec.exec
	return sys, rec
}

func writeExecutable(t *testing.T, path string, body ...string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	content := "#!/bin/sh\n"
	for _, line := range body {
		content += line + "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0o755))
}

// fakeFetcher serves canned responses and records requested URLs.
type fakeFetcher struct {
	data      map[string][]byte
	requested []string
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	f.requested = append(f.requested, url)
	b, ok := f.data[url]
	if !ok {
		return nil, fmt.Errorf("GET %s: 404 Not Found", url)
	}
	return b, nil
}

func lookSystemZsh() (string, error) { return exec.LookPath("zsh") }

func runZshSyntaxCheck(zsh, file string) (string, error) {
	out, err := exec.Command(zsh, "-n", file).CombinedOutput()
	return string(out), err
}

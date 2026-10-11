package shellmanager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOSRelease = "ID=ubuntu\nID_LIKE=debian\nPRETTY_NAME=\"Ubuntu 24.04\"\n"

// fakeBackend records calls; a successful call makes the tool's check pass.
type fakeBackend struct {
	rec      *recorder
	calls    []string
	failures map[string]error
}

func (b *fakeBackend) Install(_ context.Context, _ Env, t Tool) error { return b.do("install", t) }
func (b *fakeBackend) Update(_ context.Context, _ Env, t Tool) error  { return b.do("update", t) }

func (b *fakeBackend) do(action string, t Tool) error {
	b.calls = append(b.calls, action+" "+t.ID)
	if err := b.failures[t.ID]; err != nil {
		return err
	}
	b.rec.setFail(strings.Join(t.Check.Cmd, " "), nil)
	return nil
}

func fakeTool(id string, kind Kind, extra string) string {
	return fmt.Sprintf("id = %q\nkind = %q\n%s\n[install]\nbackend = \"fake\"\n[check]\ncmd = [%q, \"--version\"]\n",
		id, kind, extra, id)
}

// newTestEngine builds an Engine over a temp HOME with the given manifests
// (all tools start missing), the real profile files, and fake commands.
func newTestEngine(t *testing.T, manifests map[string]string) (*Engine, *recorder, *fakeBackend) {
	t.Helper()
	sys, rec := newTestSystem(t)

	fsys := fstest.MapFS{}
	registry, err := fs.Sub(registryFS, "registry")
	require.NoError(t, err)
	err = fs.WalkDir(registry, "files/zsh", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(registry, p)
		fsys[p] = &fstest.MapFile{Data: data}
		return err
	})
	require.NoError(t, err)
	var included []string
	for id, m := range manifests {
		fsys["tools/"+id+".toml"] = &fstest.MapFile{Data: []byte(m)}
		included = append(included, `"`+id+`"`)
		rec.setFail(id+" --version", errExit)
	}
	fsys["catalog.toml"] = &fstest.MapFile{Data: []byte("tools = [" + strings.Join(included, ", ") + "]\n")}
	cat, err := LoadCatalog(fsys)
	require.NoError(t, err)

	osRelease := filepath.Join(t.TempDir(), "os-release")
	require.NoError(t, os.WriteFile(osRelease, []byte(testOSRelease), 0o644))

	t.Setenv("USER", "tester")
	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/usr/bin/zsh"
	writeExecutable(t, filepath.Join(sys.HomeDir(), ".local", "bin", "zsh"))

	backend := &fakeBackend{rec: rec, failures: map[string]error{}}
	paths := NewPaths(sys.HomeDir())
	e := &Engine{
		System:    sys,
		Fetcher:   &fakeFetcher{},
		Catalog:   cat,
		Backends:  map[string]Backend{"fake": backend, "apt": aptBackend{}},
		Paths:     paths,
		Sudo:      []string{"sudo"},
		OSRelease: osRelease,
		Now:       func() time.Time { return fixedNow },
	}
	return e, rec, backend
}

func runOp(t *testing.T, op func(chan<- Event) (Report, error)) (Report, []Event, error) {
	t.Helper()
	events := make(chan Event, 256)
	rep, err := op(events)
	var got []Event
	for ev := range events { // the engine closed it
		got = append(got, ev)
	}
	return rep, got, err
}

func setupOp(e *Engine) func(chan<- Event) (Report, error) {
	return func(ch chan<- Event) (Report, error) { return e.Setup(context.Background(), ch) }
}

func results(rep Report) map[string]Result {
	m := map[string]Result{}
	for _, tr := range rep.Tools {
		m[tr.ID] = tr.Result
	}
	return m
}

func TestSetupInstallsMissingToolsInDependencyOrder(t *testing.T) {
	e, _, backend := newTestEngine(t, map[string]string{
		"a": fakeTool("a", KindBuiltin, ""),
		"b": fakeTool("b", KindPlugin, `depends = ["a"]`),
	})

	rep, events, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Equal(t, []string{"install a", "install b"}, backend.calls)
	assert.Equal(t, map[string]Result{"a": ResultOK, "b": ResultOK}, results(rep))
	var started []string
	for _, ev := range events {
		if s, ok := ev.(ToolStarted); ok {
			started = append(started, s.ID)
		}
	}
	assert.Equal(t, []string{"a", "b"}, started)
	assert.Equal(t, PhaseStarted{Phase: PhasePreflight}, events[0])
	assert.FileExists(t, e.Paths.Zshrc)
	assert.FileExists(t, e.Paths.StateFile)
}

func TestSetupIsIdempotent(t *testing.T) {
	e, rec, backend := newTestEngine(t, map[string]string{
		"a": fakeTool("a", KindBuiltin, `system_packages = ["git"]`),
	})
	rec.out["dpkg-query -W -f=${Status} git"] = "install ok installed"
	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)
	before := len(rec.commands())

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Equal(t, []string{"install a"}, backend.calls, "nothing reinstalled")
	for _, c := range rec.commands()[before:] {
		assert.NotContains(t, c, "sudo", "no privileged command on re-run")
		assert.NotContains(t, c, "chsh")
	}
	for _, f := range rep.Files {
		assert.Equal(t, ResultOK, f.Result, f.Path)
	}
	backups, _ := filepath.Glob(filepath.Join(e.Paths.Home, "*.bak.*"))
	assert.Empty(t, backups)
}

func TestAptPreflightInstallsMissingPackagesWithSudo(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, `system_packages = ["git"]`)})

	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Subset(t, rec.commands(), []string{"sudo apt-get update -qq", "sudo apt-get install -y git"})
}

func TestAptPreflightWithoutSudoWhenRoot(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, `system_packages = ["git"]`)})
	e.Sudo = nil

	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Subset(t, rec.commands(), []string{"apt-get update -qq", "apt-get install -y git"})
	for _, c := range rec.commands() {
		assert.False(t, strings.HasPrefix(c, "sudo "), c)
	}
}

func TestAptFailureAbortsBeforeTools(t *testing.T) {
	e, rec, backend := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, `system_packages = ["git"]`)})
	rec.setFail("sudo apt-get update -qq", errExit)

	_, _, err := runOp(t, setupOp(e))
	assert.ErrorContains(t, err, "installing system packages")
	assert.Empty(t, backend.calls)
}

func TestUpdateUpdatesInstalledTools(t *testing.T) {
	e, rec, backend := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	rec.setFail("a --version", nil)

	_, _, err := runOp(t, func(ch chan<- Event) (Report, error) {
		return e.Update(context.Background(), UpdateOptions{}, ch)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"update a"}, backend.calls)
}

func TestRequiredFailureSkipsDependentsButNotOthers(t *testing.T) {
	e, _, backend := newTestEngine(t, map[string]string{
		"a": fakeTool("a", KindBuiltin, ""),
		"b": fakeTool("b", KindPlugin, `depends = ["a"]`),
		"c": fakeTool("c", KindPlugin, ""),
	})
	backend.failures["a"] = errors.New("boom")

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Equal(t, map[string]Result{"a": ResultFailed, "b": ResultSkipped, "c": ResultOK}, results(rep))
	for _, tr := range rep.Tools {
		if tr.ID == "b" {
			assert.ErrorIs(t, tr.Err, ErrDependencyFailed)
		}
	}
	assert.True(t, rep.Failed())
}

func TestOptionalFailureIsAWarning(t *testing.T) {
	e, _, backend := newTestEngine(t, map[string]string{"c": fakeTool("c", KindPlugin, "optional = true")})
	backend.failures["c"] = errors.New("network down")

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)
	assert.Equal(t, map[string]Result{"c": ResultWarned}, results(rep))
	assert.False(t, rep.Failed())
}

func TestPostInstallRunsAfterInstall(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{
		"a": fakeTool("a", KindBuiltin, `post_install = [["a", "settings", "set", "x", "true"]]`),
	})
	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)
	assert.Contains(t, rec.commands(), "a settings set x true")
}

func TestSetupFailsWhenLocked(t *testing.T) {
	e, _, backend := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	unlock, err := e.System.Lock(e.Paths.LockFile)
	require.NoError(t, err)
	defer unlock()

	_, _, err = runOp(t, setupOp(e))
	assert.ErrorIs(t, err, ErrLocked)
	assert.Empty(t, backend.calls)
	assert.NoFileExists(t, e.Paths.Zshrc)
}

func TestUnsupportedOS(t *testing.T) {
	e, _, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	require.NoError(t, os.WriteFile(e.OSRelease, []byte("ID=fedora\nPRETTY_NAME=\"Fedora 41\"\n"), 0o644))

	_, _, err := runOp(t, setupOp(e))
	assert.ErrorContains(t, err, "Debian/Ubuntu")
}

func TestChangesDefaultShellWhenNotZsh(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/bin/bash"

	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	zsh := filepath.Join(e.Paths.Home, ".local", "bin", "zsh")
	assert.Contains(t, rec.commands(), "chsh -s "+zsh)
}

func TestBrokenZshrcMarkersStillLinkAliasAndChangeShell(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/bin/bash"
	broken := blockBegin + "\n"
	require.NoError(t, os.WriteFile(e.Paths.Zshrc, []byte(broken), 0o644))

	rep, _, err := runOp(t, setupOp(e))

	assert.ErrorIs(t, err, ErrBrokenMarkers)
	zsh := filepath.Join(e.Paths.Home, ".local", "bin", "zsh")
	assert.Contains(t, rec.commands(), "chsh -s "+zsh)
	assert.True(t, rep.ShellChanged)
	assert.Equal(t, broken, read(t, e.Paths.Zshrc))
}

func TestChshFallsBackToSudo(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/bin/bash"
	zsh := filepath.Join(e.Paths.Home, ".local", "bin", "zsh")
	rec.setFail("chsh -s "+zsh, errExit)

	_, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)
	assert.Contains(t, rec.commands(), "sudo chsh -s "+zsh+" tester")
}

// cancelingBackend cancels the operation while installing a tool.
type cancelingBackend struct{ cancel context.CancelFunc }

func (b cancelingBackend) Install(context.Context, Env, Tool) error { b.cancel(); return nil }
func (b cancelingBackend) Update(context.Context, Env, Tool) error  { b.cancel(); return nil }

func TestCancelDuringLastToolSkipsShellPhase(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": fakeTool("a", KindBuiltin, "")})
	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/bin/bash"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.Backends["fake"] = cancelingBackend{cancel: cancel}

	_, events, err := runOp(t, func(ch chan<- Event) (Report, error) { return e.Setup(ctx, ch) })

	assert.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, events, Event(PhaseStarted{Phase: PhaseShell}))
	assert.NoFileExists(t, e.Paths.Zshrc)
	for _, c := range rec.commands() {
		assert.NotContains(t, c, "chsh")
	}
}

// miseTool is a manifest installed by the real mise backend.
func miseTool(id string) string {
	return fmt.Sprintf("id = %q\nkind = \"plugin\"\n[install]\nbackend = \"mise\"\npackage = %q\n[check]\ncmd = [%q, \"--version\"]\n",
		id, id, id)
}

func TestSetupInstallsMiseToolFoundOnlyOutsideMise(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": miseTool("a")})
	e.Backends["mise"] = miseBackend{}
	rec.setFail("a --version", nil) // e.g. an apt or ~/.local/bin copy
	rec.setFail("mise which a", errExit)
	rec.after["mise use -g a@latest"] = func() { rec.setFail("mise which a", nil) }

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Contains(t, rec.commands(), "mise use -g a@latest")
	assert.Equal(t, map[string]Result{"a": ResultOK}, results(rep))
}

func TestSetupSkipsMiseToolManagedByMise(t *testing.T) {
	e, rec, _ := newTestEngine(t, map[string]string{"a": miseTool("a")})
	e.Backends["mise"] = miseBackend{}
	rec.setFail("a --version", nil)

	rep, _, err := runOp(t, setupOp(e))
	require.NoError(t, err)

	assert.Contains(t, rec.commands(), "mise which a")
	assert.NotContains(t, rec.commands(), "mise use -g a@latest")
	assert.Equal(t, map[string]Result{"a": ResultOK}, results(rep))
}

func TestNewEngineLoadsEmbeddedCatalog(t *testing.T) {
	sys, _ := newTestSystem(t)
	e, err := NewEngine(sys, &fakeFetcher{})
	require.NoError(t, err, "the embedded catalog only uses known backends")
	assert.NotEmpty(t, e.Catalog.Tools())
}

func applyConfigOp(e *Engine) func(chan<- Event) (Report, error) {
	return func(ch chan<- Event) (Report, error) {
		return e.ApplyConfig(context.Background(), UpdateOptions{}, ch)
	}
}

// ApplyConfig rewrites the shell config for the tools that are installed,
// without installing, updating, apt-ing or touching the login shell.
func TestApplyConfigOnlyRewritesConfig(t *testing.T) {
	e, rec, backend := newTestEngine(t, map[string]string{
		"a": fakeTool("a", KindBuiltin, `system_packages = ["git"]`),
		"b": fakeTool("b", KindPlugin, "[shell]\npriority = 40\nsnippet = 'echo b'\n"),
	})
	rec.setFail("a --version", nil) // a is installed, b is not
	rec.out["getent passwd tester"] = "tester:x:1000:1000::/home/tester:/bin/bash"

	rep, _, err := runOp(t, applyConfigOp(e))
	require.NoError(t, err)

	assert.Empty(t, backend.calls, "no install or update")
	for _, c := range rec.commands() {
		assert.NotContains(t, c, "apt-get")
		assert.NotContains(t, c, "chsh")
	}
	assert.Equal(t, map[string]Result{"a": ResultOK, "b": ResultSkipped}, results(rep))
	assert.False(t, rep.Failed())
	assert.False(t, rep.ShellChanged)
	assert.Contains(t, read(t, e.Paths.Zshrc), blockBegin)
	assert.FileExists(t, filepath.Join(e.Paths.ZshDir, "init.zsh"))
	assert.NoFileExists(t, filepath.Join(e.Paths.ZshDir, "tools", "40-b.zsh"), "b is not installed")
}

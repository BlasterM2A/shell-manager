package shellmanager

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testEnv(t *testing.T, f *fakeFetcher) (Env, *recorder) {
	sys, rec := newTestSystem(t)
	st := &State{Files: map[string]string{}, Blocks: map[string]string{}, Versions: map[string]string{}}
	return Env{System: sys, Fetcher: f, Paths: NewPaths(sys.HomeDir()), State: st}, rec
}

func scriptTool(update ...string) Tool {
	return Tool{ID: "claude", Install: InstallSpec{Backend: "script", URL: "https://x/install.sh", UpdateCmd: update}}
}

func TestScriptInstallRunsDownloadedInstaller(t *testing.T) {
	f := &fakeFetcher{data: map[string][]byte{"https://x/install.sh": []byte("echo hi")}}
	env, rec := testEnv(t, f)
	script := filepath.Join(env.Paths.TmpDir, "claude-install.sh")

	require.NoError(t, scriptBackend{}.Install(context.Background(), env, scriptTool()))

	assert.Equal(t, []string{"bash " + script}, rec.commands())
	assert.NoFileExists(t, script, "installer is removed afterwards")
}

func TestScriptInterpreter(t *testing.T) {
	f := &fakeFetcher{data: map[string][]byte{"https://x/install.sh": []byte("echo hi")}}
	env, rec := testEnv(t, f)
	tool := scriptTool()
	tool.Install.Interpreter = "sh"

	require.NoError(t, scriptBackend{}.Install(context.Background(), env, tool))
	assert.Equal(t, []string{"sh " + filepath.Join(env.Paths.TmpDir, "claude-install.sh")}, rec.commands())
}

func TestScriptUpdateUsesOwnUpdater(t *testing.T) {
	env, rec := testEnv(t, &fakeFetcher{})
	require.NoError(t, scriptBackend{}.Update(context.Background(), env, scriptTool("claude", "update")))
	assert.Equal(t, []string{"claude update"}, rec.commands())
}

func TestScriptUpdateWithoutUpdaterReinstalls(t *testing.T) {
	f := &fakeFetcher{data: map[string][]byte{"https://x/install.sh": []byte("echo hi")}}
	env, rec := testEnv(t, f)
	require.NoError(t, scriptBackend{}.Update(context.Background(), env, scriptTool()))
	assert.Len(t, rec.commands(), 1)
	assert.Equal(t, []string{"https://x/install.sh"}, f.requested)
}

func TestScriptDownloadErrorRunsNothing(t *testing.T) {
	env, rec := testEnv(t, &fakeFetcher{})
	err := scriptBackend{}.Install(context.Background(), env, scriptTool())
	assert.ErrorContains(t, err, "404")
	assert.Empty(t, rec.commands())
}

func TestMiseBackend(t *testing.T) {
	env, rec := testEnv(t, &fakeFetcher{})
	tool := Tool{ID: "starship", Install: InstallSpec{Backend: "mise", Package: "starship"}}
	require.NoError(t, miseBackend{}.Install(context.Background(), env, tool))
	require.NoError(t, miseBackend{}.Update(context.Background(), env, tool))
	assert.Equal(t, []string{"mise use -g starship@latest", "mise upgrade starship"}, rec.commands())
}

func TestAptPackages(t *testing.T) {
	tools := []Tool{
		{ID: "zsh", Install: InstallSpec{Backend: "apt", Package: "zsh"}},
		{ID: "antidote", SystemPackages: []string{"git"}, Install: InstallSpec{Backend: "archive"}},
		{ID: "nerdfont", SystemPackages: []string{"fontconfig", "git"}, Install: InstallSpec{Backend: "font"}},
	}
	assert.Equal(t, []string{"fontconfig", "git", "zsh"}, aptPackages(tools))
}

func TestMissingAptPackages(t *testing.T) {
	env, rec := testEnv(t, &fakeFetcher{})
	rec.out["dpkg-query -W -f=${Status} zsh"] = "install ok installed"
	rec.setFail("dpkg-query -W -f=${Status} git", errExit)

	missing := missingAptPackages(context.Background(), env.System, []string{"git", "zsh"})
	assert.Equal(t, []string{"git"}, missing)
}

func TestDefaultBackendsCoverCatalog(t *testing.T) {
	cat, err := DefaultCatalog()
	require.NoError(t, err)
	backends := DefaultBackends()
	for _, tool := range cat.Tools() {
		assert.Contains(t, backends, tool.Install.Backend, tool.ID)
	}
}

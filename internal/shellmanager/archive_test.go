package shellmanager

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func zipFile(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

var antidoteTool = Tool{ID: "antidote", Install: InstallSpec{Backend: "archive", Repo: "mattmc3/antidote", Dest: "~/.antidote"}}

const (
	antidoteLatest  = "https://api.github.com/repos/mattmc3/antidote/releases/latest"
	antidoteTarball = "https://github.com/mattmc3/antidote/archive/refs/tags/v1.9.0.tar.gz"
)

func TestArchiveInstallExtractsAndRecordsTag(t *testing.T) {
	f := &fakeFetcher{data: map[string][]byte{
		antidoteLatest:  []byte(`{"tag_name":"v1.9.0"}`),
		antidoteTarball: tarGz(t, map[string]string{"antidote-1.9.0/antidote.zsh": "# antidote", "antidote-1.9.0/functions/x": "x"}),
	}}
	env, _ := testEnv(t, f)

	require.NoError(t, archiveBackend{}.Install(context.Background(), env, antidoteTool))

	data, err := os.ReadFile(filepath.Join(env.Paths.Home, ".antidote", "antidote.zsh"))
	require.NoError(t, err)
	assert.Equal(t, "# antidote", string(data))
	assert.FileExists(t, filepath.Join(env.Paths.Home, ".antidote", "functions", "x"))
	assert.Equal(t, "v1.9.0", env.State.Versions["antidote"])
}

func TestArchiveUpdateSkipsWhenCurrent(t *testing.T) {
	f := &fakeFetcher{data: map[string][]byte{antidoteLatest: []byte(`{"tag_name":"v1.9.0"}`)}}
	env, _ := testEnv(t, f)
	env.State.Versions["antidote"] = "v1.9.0"

	require.NoError(t, archiveBackend{}.Update(context.Background(), env, antidoteTool))
	assert.Equal(t, []string{antidoteLatest}, f.requested, "tarball not downloaded")
}

func TestArchiveRejectsPathTraversal(t *testing.T) {
	f := &fakeFetcher{data: map[string][]byte{
		antidoteLatest:  []byte(`{"tag_name":"v1.9.0"}`),
		antidoteTarball: tarGz(t, map[string]string{"top/../../evil": "x"}),
	}}
	env, _ := testEnv(t, f)
	err := archiveBackend{}.Install(context.Background(), env, antidoteTool)
	assert.ErrorContains(t, err, "unsafe path")
}

func TestFontInstallKeepsOnlyTTFAndRefreshesCache(t *testing.T) {
	tool := Tool{ID: "nerdfont", Install: InstallSpec{
		Backend: "font", Repo: "ryanoasis/nerd-fonts", Asset: "JetBrainsMono.zip",
		Dest: "~/.local/share/fonts/JetBrainsMonoNerdFont",
	}}
	f := &fakeFetcher{data: map[string][]byte{
		"https://api.github.com/repos/ryanoasis/nerd-fonts/releases/latest":                  []byte(`{"tag_name":"v3.4.0"}`),
		"https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/JetBrainsMono.zip": zipFile(t, map[string]string{"JetBrainsMonoNerdFont-Regular.ttf": "font", "README.md": "readme"}),
	}}
	env, rec := testEnv(t, f)
	dest := filepath.Join(env.Paths.Home, ".local", "share", "fonts", "JetBrainsMonoNerdFont")

	require.NoError(t, fontBackend{}.Install(context.Background(), env, tool))

	assert.FileExists(t, filepath.Join(dest, "JetBrainsMonoNerdFont-Regular.ttf"))
	assert.NoFileExists(t, filepath.Join(dest, "README.md"))
	assert.Equal(t, []string{"fc-cache -f " + dest}, rec.commands())
	assert.Equal(t, "v3.4.0", env.State.Versions["nerdfont"])
}

func TestFontInstallWithoutTTFKeepsExistingFonts(t *testing.T) {
	tool := Tool{ID: "nerdfont", Install: InstallSpec{
		Backend: "font", Repo: "ryanoasis/nerd-fonts", Asset: "JetBrainsMono.zip",
		Dest: "~/.local/share/fonts/JetBrainsMonoNerdFont",
	}}
	f := &fakeFetcher{data: map[string][]byte{
		"https://api.github.com/repos/ryanoasis/nerd-fonts/releases/latest":                  []byte(`{"tag_name":"v3.4.0"}`),
		"https://github.com/ryanoasis/nerd-fonts/releases/download/v3.4.0/JetBrainsMono.zip": zipFile(t, map[string]string{"README.md": "readme"}),
	}}
	env, rec := testEnv(t, f)
	dest := filepath.Join(env.Paths.Home, ".local", "share", "fonts", "JetBrainsMonoNerdFont")
	require.NoError(t, os.MkdirAll(dest, 0o755))
	old := filepath.Join(dest, "Old.ttf")
	require.NoError(t, os.WriteFile(old, []byte("old"), 0o644))

	err := fontBackend{}.Install(context.Background(), env, tool)

	assert.ErrorContains(t, err, "contains no .ttf")
	assert.FileExists(t, old)
	assert.NoDirExists(t, dest+".new")
	assert.Empty(t, rec.commands())
	assert.NotContains(t, env.State.Versions, "nerdfont")
}

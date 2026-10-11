package shellmanager

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// fontBackend installs the .ttf files of a GitHub release zip into
// Install.Dest and refreshes the font cache.
type fontBackend struct{}

func (fontBackend) Install(ctx context.Context, env Env, t Tool) error {
	return installFont(ctx, env, t, false)
}

func (fontBackend) Update(ctx context.Context, env Env, t Tool) error {
	return installFont(ctx, env, t, true)
}

func installFont(ctx context.Context, env Env, t Tool, skipIfCurrent bool) error {
	tag, err := latestTag(ctx, env.Fetcher, t.Install.Repo)
	if err != nil {
		return err
	}
	if skipIfCurrent && env.State.Versions[t.ID] == tag {
		return nil
	}
	data, err := env.Fetcher.Fetch(ctx, fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", t.Install.Repo, tag, t.Install.Asset))
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("reading %s: %w", t.Install.Asset, err)
	}
	dest := env.Paths.Expand(t.Install.Dest)
	staging := dest + ".new"
	if err := env.System.RemoveAll(staging); err != nil {
		return err
	}
	fonts, err := writeFonts(env.System, zr, staging)
	if err != nil || fonts == 0 {
		_ = env.System.RemoveAll(staging)
		if err != nil {
			return err
		}
		return fmt.Errorf("%s contains no .ttf fonts", t.Install.Asset)
	}
	if err := env.System.RemoveAll(dest); err != nil {
		return err
	}
	if err := env.System.Rename(staging, dest); err != nil {
		return err
	}
	if _, err := env.System.Run(ctx, Cmd{Name: "fc-cache", Args: []string{"-f", dest}}); err != nil {
		return err
	}
	env.State.Versions[t.ID] = tag
	return nil
}

// writeFonts copies the .ttf entries of zr into dir and returns their count.
func writeFonts(sys System, zr *zip.Reader, dir string) (int, error) {
	fonts := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(f.Name), ".ttf") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return fonts, err
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return fonts, err
		}
		if err := sys.WriteFile(filepath.Join(dir, filepath.Base(f.Name)), content, 0o644); err != nil {
			return fonts, err
		}
		fonts++
	}
	return fonts, nil
}

package shellmanager

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

// archiveBackend installs a GitHub repo's source tarball at its latest
// release tag into Install.Dest.
type archiveBackend struct{}

func (archiveBackend) Install(ctx context.Context, env Env, t Tool) error {
	return installArchive(ctx, env, t, false)
}

func (archiveBackend) Update(ctx context.Context, env Env, t Tool) error {
	return installArchive(ctx, env, t, true)
}

func installArchive(ctx context.Context, env Env, t Tool, skipIfCurrent bool) error {
	tag, err := latestTag(ctx, env.Fetcher, t.Install.Repo)
	if err != nil {
		return err
	}
	if skipIfCurrent && env.State.Versions[t.ID] == tag {
		return nil
	}
	data, err := env.Fetcher.Fetch(ctx, fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.tar.gz", t.Install.Repo, tag))
	if err != nil {
		return err
	}
	dest := env.Paths.Expand(t.Install.Dest)
	staging := dest + ".new"
	if err := env.System.RemoveAll(staging); err != nil {
		return err
	}
	if err := extractTarGz(env.System, data, staging); err != nil {
		_ = env.System.RemoveAll(staging)
		return fmt.Errorf("extracting %s: %w", t.ID, err)
	}
	if err := env.System.RemoveAll(dest); err != nil {
		return err
	}
	if err := env.System.Rename(staging, dest); err != nil {
		return err
	}
	env.State.Versions[t.ID] = tag
	return nil
}

// extractTarGz extracts directories and regular files into dest, dropping
// the top-level directory GitHub puts in source tarballs.
func extractTarGz(sys System, data []byte, dest string) error {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		_, rel, found := strings.Cut(hdr.Name, "/")
		if !found || rel == "" {
			continue
		}
		target, err := safeJoin(dest, rel)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := sys.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			content, err := io.ReadAll(tr)
			if err != nil {
				return err
			}
			if err := sys.WriteFile(target, content, fs.FileMode(hdr.Mode).Perm()); err != nil {
				return err
			}
		}
	}
}

// safeJoin joins rel under dest, rejecting paths that escape it.
func safeJoin(dest, rel string) (string, error) {
	p := filepath.Join(dest, rel)
	if !strings.HasPrefix(p, dest+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in archive: %s", rel)
	}
	return p, nil
}

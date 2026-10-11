// Package selfupdate replaces the running shell-manager binary with the
// latest GitHub release, verified against the release's checksums.txt.
package selfupdate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	gsu "github.com/creativeprojects/go-selfupdate"
)

// ErrDevBuild is returned for builds without a release version.
var ErrDevBuild = errors.New("development build: self-update only works for released versions")

// Updater checks and applies updates for one GitHub repo.
type Updater struct {
	slug    string
	current string
}

// New returns an Updater for repo slug (owner/name) at version current.
func New(slug, current string) *Updater {
	return &Updater{slug: slug, current: current}
}

// checkReleaseVersion rejects versions go-selfupdate cannot compare
// (its LessOrEqual panics on non-semver input).
func checkReleaseVersion(v string) error {
	if _, err := semver.NewVersion(v); err != nil {
		return fmt.Errorf("%w (version %q is not a release version)", ErrDevBuild, v)
	}
	return nil
}

// ensureWritable verifies files can be created in dir.
func ensureWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".shell-manager-write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func (u *Updater) detect(ctx context.Context) (*gsu.Updater, *gsu.Release, error) {
	if err := checkReleaseVersion(u.current); err != nil {
		return nil, nil, err
	}
	up, err := gsu.NewUpdater(gsu.Config{Validator: &gsu.ChecksumValidator{UniqueFilename: "checksums.txt"}})
	if err != nil {
		return nil, nil, err
	}
	rel, found, err := up.DetectLatest(ctx, gsu.ParseSlug(u.slug))
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, fmt.Errorf("no release of %s for this platform", u.slug)
	}
	return up, rel, nil
}

// Latest returns the latest released version and whether it is newer.
func (u *Updater) Latest(ctx context.Context) (string, bool, error) {
	_, rel, err := u.detect(ctx)
	if err != nil {
		return "", false, err
	}
	return rel.Version(), !rel.LessOrEqual(u.current), nil
}

// Apply installs the latest release over the running binary if newer.
func (u *Updater) Apply(ctx context.Context) (string, bool, error) {
	up, rel, err := u.detect(ctx)
	if err != nil {
		return "", false, err
	}
	if rel.LessOrEqual(u.current) {
		return u.current, false, nil
	}
	exe, err := gsu.ExecutablePath()
	if err != nil {
		return "", false, err
	}
	dir := filepath.Dir(exe)
	if err := ensureWritable(dir); err != nil {
		return "", false, fmt.Errorf("cannot update %s: %s is not writable (reinstall shell-manager to ~/.local/bin with the install script): %w", exe, dir, err)
	}
	if err := up.UpdateTo(ctx, rel, exe); err != nil {
		return "", false, fmt.Errorf("replacing %s: %w", exe, err)
	}
	return rel.Version(), true, nil
}

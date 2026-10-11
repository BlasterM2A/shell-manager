package shellmanager

import (
	"context"
	"io/fs"
)

// DryRunSystem reads through Base but only records mutations and commands,
// returning success for them. It backs tests and a future --dry-run.
type DryRunSystem struct {
	Base System
	Ops  []string
}

func (d *DryRunSystem) record(op string) { d.Ops = append(d.Ops, op) }

func (d *DryRunSystem) HomeDir() string                       { return d.Base.HomeDir() }
func (d *DryRunSystem) Getenv(key string) string              { return d.Base.Getenv(key) }
func (d *DryRunSystem) ReadFile(p string) ([]byte, error)     { return d.Base.ReadFile(p) }
func (d *DryRunSystem) Stat(p string) (fs.FileInfo, error)    { return d.Base.Stat(p) }
func (d *DryRunSystem) Glob(pattern string) ([]string, error) { return d.Base.Glob(pattern) }
func (d *DryRunSystem) LookPath(file string) (string, error)  { return d.Base.LookPath(file) }
func (d *DryRunSystem) Lstat(p string) (fs.FileInfo, error)   { return d.Base.Lstat(p) }
func (d *DryRunSystem) Readlink(p string) (string, error)     { return d.Base.Readlink(p) }

func (d *DryRunSystem) Symlink(target, link string) error {
	d.record("symlink " + link + " -> " + target)
	return nil
}

func (d *DryRunSystem) WriteFile(p string, _ []byte, _ fs.FileMode) error {
	d.record("write " + p)
	return nil
}

func (d *DryRunSystem) Rename(oldpath, newpath string) error {
	d.record("rename " + oldpath + " -> " + newpath)
	return nil
}

func (d *DryRunSystem) Remove(p string) error    { d.record("remove " + p); return nil }
func (d *DryRunSystem) RemoveAll(p string) error { d.record("remove-all " + p); return nil }

func (d *DryRunSystem) MkdirAll(p string, _ fs.FileMode) error {
	d.record("mkdir " + p)
	return nil
}

func (d *DryRunSystem) Run(_ context.Context, c Cmd) ([]byte, error) {
	d.record("run " + c.String())
	return nil, nil
}

func (d *DryRunSystem) Lock(p string) (func(), error) {
	d.record("lock " + p)
	return func() {}, nil
}

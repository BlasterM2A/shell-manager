package shellmanager

import (
	"context"
	"path/filepath"
)

// scriptBackend runs a vendor's official install script.
type scriptBackend struct{}

func (scriptBackend) Install(ctx context.Context, env Env, t Tool) error {
	script, err := env.Fetcher.Fetch(ctx, t.Install.URL)
	if err != nil {
		return err
	}
	path := filepath.Join(env.Paths.TmpDir, t.ID+"-install.sh")
	if err := env.System.WriteFile(path, script, 0o700); err != nil {
		return err
	}
	defer func() { _ = env.System.Remove(path) }()
	interpreter := t.Install.Interpreter
	if interpreter == "" {
		interpreter = "bash"
	}
	_, err = env.System.Run(ctx, Cmd{Name: interpreter, Args: []string{path}})
	return err
}

// Update uses the tool's own updater when it has one, else reinstalls.
func (b scriptBackend) Update(ctx context.Context, env Env, t Tool) error {
	if len(t.Install.UpdateCmd) == 0 {
		return b.Install(ctx, env, t)
	}
	_, err := env.System.Run(ctx, Cmd{Name: t.Install.UpdateCmd[0], Args: t.Install.UpdateCmd[1:]})
	return err
}

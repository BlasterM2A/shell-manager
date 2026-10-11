package shellmanager

import "context"

// miseBackend installs user tools as global mise tools.
type miseBackend struct{}

func (miseBackend) Install(ctx context.Context, env Env, t Tool) error {
	_, err := env.System.Run(ctx, Cmd{Name: "mise", Args: []string{"use", "-g", t.Install.Package + "@latest"}})
	return err
}

// Installed reports whether mise itself manages the tool.
func (miseBackend) Installed(ctx context.Context, env Env, t Tool) bool {
	_, err := env.System.Run(ctx, Cmd{Name: "mise", Args: []string{"which", t.Install.Package}})
	return err == nil
}

func (miseBackend) Update(ctx context.Context, env Env, t Tool) error {
	_, err := env.System.Run(ctx, Cmd{Name: "mise", Args: []string{"upgrade", t.Install.Package}})
	return err
}

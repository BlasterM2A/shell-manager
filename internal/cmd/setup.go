package cmd

import (
	"context"

	"github.com/spf13/cobra"
)

// SetupOptions are the inputs of setup.
type SetupOptions struct {
	Factory *Factory
}

func newSetupCmd(f *Factory, runF func(context.Context, *SetupOptions) error) *cobra.Command {
	opts := &SetupOptions{Factory: f}
	return &cobra.Command{
		Use:   "setup",
		Short: "Install what is missing, write the shell config and make zsh the login shell",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(cmd.Context(), opts)
			}
			return runSetup(cmd.Context(), opts)
		},
	}
}

func runSetup(ctx context.Context, opts *SetupOptions) error {
	e, err := opts.Factory.Engine()
	if err != nil {
		return err
	}
	return runApply(ctx, opts.Factory, e, "Setting up your shell", e.Setup)
}

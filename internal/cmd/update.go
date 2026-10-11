package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
)

// UpdateOptions are the inputs of update.
type UpdateOptions struct {
	Factory    *Factory
	Force      bool
	ConfigOnly bool
}

func newUpdateCmd(f *Factory, runF func(context.Context, *UpdateOptions) error) *cobra.Command {
	opts := &UpdateOptions{Factory: f}
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update every tool and the shell config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(cmd.Context(), opts)
			}
			return runUpdate(cmd.Context(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite managed files edited by hand (a backup is kept)")
	cmd.Flags().BoolVar(&opts.ConfigOnly, "config-only", false, "only rewrite the shell config; install and update no tools")
	return cmd
}

func runUpdate(ctx context.Context, opts *UpdateOptions) error {
	e, err := opts.Factory.Engine()
	if err != nil {
		return err
	}
	engineOp := e.Update
	title := "Updating your shell"
	if opts.ConfigOnly {
		engineOp, title = e.ApplyConfig, "Applying the new shell config"
	}
	op := func(ctx context.Context, events chan<- shellmanager.Event) (shellmanager.Report, error) {
		return engineOp(ctx, shellmanager.UpdateOptions{Force: opts.Force}, events)
	}
	return runApply(ctx, opts.Factory, e, title, op)
}

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/BlasterM2A/shell-manager/internal/ui/notice"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// SelfUpdateOptions are the inputs of self-update.
type SelfUpdateOptions struct {
	Factory *Factory
}

func newSelfUpdateCmd(f *Factory, runF func(context.Context, *SelfUpdateOptions) error) *cobra.Command {
	opts := &SelfUpdateOptions{Factory: f}
	return &cobra.Command{
		Use:   "self-update",
		Short: "Replace this binary with the latest release",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(cmd.Context(), opts)
			}
			return runSelfUpdate(cmd.Context(), opts)
		},
	}
}

func runSelfUpdate(ctx context.Context, opts *SelfUpdateOptions) error {
	f := opts.Factory
	c, out := f.Common(), f.IOStreams.Out
	v, updated, err := f.Updater().Apply(ctx)
	if err != nil {
		return err
	}
	if !updated {
		_, _ = fmt.Fprintln(out, notice.Render(c, notice.Props{Status: styles.StatusOK, Text: "Already up to date (" + v + ")"}))
		return nil
	}
	_, _ = fmt.Fprintln(out, notice.Render(c, notice.Props{Status: styles.StatusOK, Text: "Updated to " + v}))
	_, _ = fmt.Fprintln(out, notice.Render(c, notice.Props{Status: styles.StatusInfo, Text: "Run `shm update` to apply the new configuration."}))
	return nil
}

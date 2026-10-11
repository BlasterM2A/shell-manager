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
		Short: "Replace this binary with the latest release and apply its shell config",
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
	return applyNewConfig(f)
}

// applyNewConfig makes the new release's config templates take effect. They
// ship inside the binary, so this process (still the old binary) hands over
// to the new one with `update --config-only`: no sudo, no tool changes.
func applyNewConfig(f *Factory) error {
	exe := f.Executable()
	if exe == "" {
		hint := "Run `shm update --config-only` to apply the new configuration."
		_, _ = fmt.Fprintln(f.IOStreams.Out, notice.Render(f.Common(), notice.Props{Status: styles.StatusInfo, Text: hint}))
		return nil
	}
	args := []string{"shell-manager", "update", "--config-only"}
	for _, g := range []struct {
		on   bool
		flag string
	}{{f.Plain, "--plain"}, {f.Verbose, "--verbose"}, {f.Quiet, "--quiet"}} {
		if g.on {
			args = append(args, g.flag)
		}
	}
	return f.Exec(exe, args)
}

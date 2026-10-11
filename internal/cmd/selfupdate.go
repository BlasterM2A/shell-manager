package cmd

import (
	"bufio"
	"context"
	"fmt"
	"strings"

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
	return offerUpdate(f, f.IOStreams.IsTTY())
}

// offerUpdate asks whether to apply the new release's configuration now.
// The new binary carries the config templates, so `update` must run as the
// new binary: this process is still the old one, hence the exec. Without a
// terminal it only prints the hint.
func offerUpdate(f *Factory, tty bool) error {
	c, out := f.Common(), f.IOStreams.Out
	hint := notice.Render(c, notice.Props{Status: styles.StatusInfo, Text: "Run `shm update` to apply the new configuration."})
	if !tty {
		_, _ = fmt.Fprintln(out, hint)
		return nil
	}
	_, _ = fmt.Fprint(out, "Run 'shm update' now to apply the new configuration? [Y/n] ")
	answer, _ := bufio.NewReader(f.IOStreams.In).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "", "y", "yes":
		args := []string{"shell-manager", "update"}
		for _, g := range []struct {
			on   bool
			flag string
		}{{f.Plain, "--plain"}, {f.Verbose, "--verbose"}, {f.Quiet, "--quiet"}} {
			if g.on {
				args = append(args, g.flag)
			}
		}
		exe := f.Executable()
		if exe == "" {
			_, _ = fmt.Fprintln(out, hint)
			return nil
		}
		return f.Exec(exe, args)
	default:
		_, _ = fmt.Fprintln(out, hint)
		return nil
	}
}

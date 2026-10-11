// Package cmd wires the cobra commands: it binds UI components to the
// shellmanager domain and holds no business logic of its own.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/BlasterM2A/shell-manager/internal/iostreams"
	"github.com/BlasterM2A/shell-manager/internal/ui/notice"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// Set via -ldflags at release time (see .goreleaser.yml).
var (
	version  = "dev"
	commit   = "none"
	repoSlug = "BlasterM2A/shell-manager"
)

// ExitError ends the process with Code without printing anything else;
// commands return it after they have already reported the problem.
type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("exit status %d", e.Code) }

// NewRootCmd builds the command tree. Every command is listed here.
func NewRootCmd(f *Factory) *cobra.Command {
	root := &cobra.Command{
		Use:           "shell-manager",
		Short:         "Bootstrap and maintain your zsh environment",
		Version:       fmt.Sprintf("%s (%s)", version, commit),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("shell-manager {{.Version}}\n")
	flags := root.PersistentFlags()
	flags.BoolVar(&f.Plain, "plain", false, "plain text output, no interactive UI")
	flags.BoolVarP(&f.Verbose, "verbose", "v", false, "log debug details")
	flags.BoolVarP(&f.Quiet, "quiet", "q", false, "only log errors")
	root.AddCommand(
		newSetupCmd(f, nil),
		newUpdateCmd(f, nil),
		newDoctorCmd(f, nil),
		newSelfUpdateCmd(f, nil),
	)
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	ios := iostreams.System()
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintln(ios.ErrOut, "Error:", err)
		return 1
	}
	f := NewFactory(ios, home)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := NewRootCmd(f).ExecuteContext(ctx); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			return exitErr.Code
		}
		_, _ = fmt.Fprintln(ios.ErrOut, notice.Render(f.Common(), notice.Props{Status: styles.StatusFail, Text: err.Error()}))
		return 1
	}
	return 0
}

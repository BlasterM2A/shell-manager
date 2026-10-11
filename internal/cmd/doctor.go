package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/BlasterM2A/shell-manager/internal/selfupdate"
	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
	"github.com/BlasterM2A/shell-manager/internal/ui/statustable"
)

// DoctorOptions are the inputs of doctor.
type DoctorOptions struct {
	Factory *Factory
}

func newDoctorCmd(f *Factory, runF func(context.Context, *DoctorOptions) error) *cobra.Command {
	opts := &DoctorOptions{Factory: f}
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check tools, managed files and the default shell",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runF != nil {
				return runF(cmd.Context(), opts)
			}
			return runDoctor(cmd.Context(), opts)
		},
	}
}

func runDoctor(ctx context.Context, opts *DoctorOptions) error {
	f := opts.Factory
	e, err := f.Engine()
	if err != nil {
		return err
	}
	rep, err := e.Doctor(ctx)
	if err != nil {
		return err
	}
	rep.Checks = append(rep.Checks, versionCheck(ctx, f.Updater()))
	_, _ = fmt.Fprint(f.IOStreams.Out, statustable.Render(f.Common(), statustable.Props{
		Title: "shell-manager doctor", Rows: checkRows(f.Home, rep),
	}))
	if rep.Failed() {
		return &ExitError{Code: 1}
	}
	return nil
}

func versionCheck(ctx context.Context, u *selfupdate.Updater) shellmanager.CheckResult {
	c := shellmanager.CheckResult{Name: "shell-manager", Status: shellmanager.CheckOK, Detail: version}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	latest, newer, err := u.Latest(ctx)
	switch {
	case errors.Is(err, selfupdate.ErrDevBuild):
		c.Status, c.Detail = shellmanager.CheckSkip, "development build"
	case err != nil:
		c.Status, c.Detail = shellmanager.CheckSkip, "could not check for updates: "+err.Error()
	case newer:
		c.Status, c.Detail = shellmanager.CheckWarn, latest+" available, run shm self-update"
	}
	return c
}

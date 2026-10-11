package cmd

import (
	"context"
	"strings"

	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/notice"
	"github.com/BlasterM2A/shell-manager/internal/ui/statustable"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// runApply runs Init or Update with live progress, then shows doctor.
func runApply(ctx context.Context, f *Factory, e *shellmanager.Engine, title string, op operation) error {
	var doctor shellmanager.Report
	summary := func(rep shellmanager.Report, opErr error) string {
		c := f.Common()
		if opErr != nil {
			return notice.Render(c, notice.Props{Status: styles.StatusFail, Text: opErr.Error()}) + "\n"
		}
		var err error
		if doctor, err = e.Doctor(ctx); err != nil {
			return notice.Render(c, notice.Props{Status: styles.StatusFail, Text: err.Error()}) + "\n"
		}
		return renderSummary(c, f.Home, e.Paths.LogFile, rep, doctor)
	}
	res, err := runWithProgress(ctx, f, title, op, summary)
	if err != nil {
		return err
	}
	if res.OpErr != nil || res.Report.Failed() || doctor.Failed() {
		return &ExitError{Code: 1}
	}
	return nil
}

// nextStep tells the user how to start using the new shell config. A login
// shell changed with chsh is picked up by new terminal windows on most
// emulators; the current shell can only be replaced by the user.
func nextStep(rep shellmanager.Report) string {
	if rep.ShellChanged {
		return "Run `exec zsh` (or open a new terminal) to start using zsh. " +
			"If new terminals still open your old shell, log out and back in."
	}
	return "Run `exec zsh` to load the new configuration."
}

func renderSummary(c common.Common, home, logFile string, rep, doctor shellmanager.Report) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(statustable.Render(c, statustable.Props{Title: "Status", Rows: checkRows(home, doctor)}))
	b.WriteString("\n")
	if rep.Failed() || doctor.Failed() {
		b.WriteString(notice.Render(c, notice.Props{
			Status: styles.StatusFail,
			Text:   "Some required tools are missing. Details: " + shortPath(home, logFile),
		}))
	} else {
		b.WriteString(notice.Render(c, notice.Props{Status: styles.StatusOK, Text: "Done."}))
		b.WriteString("\n")
		b.WriteString(notice.Render(c, notice.Props{Status: styles.StatusInfo, Text: nextStep(rep)}))
		b.WriteString("\n")
		b.WriteString(notice.Render(c, notice.Props{
			Status: styles.StatusInfo,
			Text:   "Reopen your terminal (all windows), then select 'JetBrainsMono Nerd Font' in its font settings.",
		}))
	}
	b.WriteString("\n")
	return b.String()
}

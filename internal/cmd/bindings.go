package cmd

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/BlasterM2A/shell-setup/internal/shellsetup"
	"github.com/BlasterM2A/shell-setup/internal/ui/statustable"
	"github.com/BlasterM2A/shell-setup/internal/ui/steplist"
	"github.com/BlasterM2A/shell-setup/internal/ui/styles"
)

// eventMsg maps a domain event to the steplist message that shows it.
func eventMsg(home string, ev shellsetup.Event) (tea.Msg, bool) {
	switch ev := ev.(type) {
	case shellsetup.ToolStarted:
		return steplist.StepStarted{ID: "tool:" + ev.ID, Label: toolLabel(ev.ID, ev.Action)}, true
	case shellsetup.ToolFinished:
		detail := ev.Version
		if ev.Err != nil {
			detail = ev.Err.Error()
		}
		return steplist.StepFinished{
			ID: "tool:" + ev.ID, Label: toolLabel(ev.ID, ev.Action),
			Status: resultStatus(ev.Result), Detail: detail,
		}, true
	case shellsetup.FileFinished:
		detail := ""
		if ev.Result != shellsetup.ResultOK {
			detail = string(ev.Result)
		}
		return steplist.StepFinished{
			ID: "file:" + ev.Path, Label: shortPath(home, ev.Path),
			Status: resultStatus(ev.Result), Detail: detail,
		}, true
	}
	return nil, false
}

func toolLabel(id string, a shellsetup.Action) string {
	if a == shellsetup.ActionNone {
		return id
	}
	return id + " (" + string(a) + ")"
}

func resultStatus(r shellsetup.Result) styles.Status {
	switch r {
	case shellsetup.ResultOK, shellsetup.ResultRemoved:
		return styles.StatusOK
	case shellsetup.ResultSkipped:
		return styles.StatusSkip
	case shellsetup.ResultModified, shellsetup.ResultWarned:
		return styles.StatusWarn
	default:
		return styles.StatusFail
	}
}

func checkStatus(s shellsetup.CheckStatus) styles.Status {
	switch s {
	case shellsetup.CheckOK:
		return styles.StatusOK
	case shellsetup.CheckWarn:
		return styles.StatusWarn
	case shellsetup.CheckSkip:
		return styles.StatusSkip
	default:
		return styles.StatusFail
	}
}

// checkRows turns doctor checks into table rows.
func checkRows(home string, rep shellsetup.Report) []statustable.Row {
	rows := make([]statustable.Row, 0, len(rep.Checks))
	for _, c := range rep.Checks {
		rows = append(rows, statustable.Row{Status: checkStatus(c.Status), Name: shortPath(home, c.Name), Detail: c.Detail})
	}
	return rows
}

func phaseLabel(p shellsetup.Phase) string {
	switch p {
	case shellsetup.PhasePreflight:
		return "Checking system packages"
	case shellsetup.PhaseTools:
		return "Installing tools"
	default:
		return "Writing shell config"
	}
}

// shortPath shows paths under home as ~/...
func shortPath(home, p string) string {
	if rest, ok := strings.CutPrefix(p, home+"/"); ok {
		return "~/" + rest
	}
	return p
}

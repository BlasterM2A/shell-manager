package cmd

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
	"github.com/BlasterM2A/shell-manager/internal/ui/statustable"
	"github.com/BlasterM2A/shell-manager/internal/ui/steplist"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// eventMsg maps a domain event to the steplist message that shows it.
func eventMsg(home string, ev shellmanager.Event) (tea.Msg, bool) {
	switch ev := ev.(type) {
	case shellmanager.ToolStarted:
		return steplist.StepStarted{ID: "tool:" + ev.ID, Label: toolLabel(ev.ID, ev.Action)}, true
	case shellmanager.ToolFinished:
		detail := ev.Version
		if ev.Err != nil {
			detail = ev.Err.Error()
		}
		return steplist.StepFinished{
			ID: "tool:" + ev.ID, Label: toolLabel(ev.ID, ev.Action),
			Status: resultStatus(ev.Result), Detail: detail,
		}, true
	case shellmanager.FileFinished:
		detail := ""
		if ev.Result != shellmanager.ResultOK {
			detail = string(ev.Result)
		}
		return steplist.StepFinished{
			ID: "file:" + ev.Path, Label: shortPath(home, ev.Path),
			Status: resultStatus(ev.Result), Detail: detail,
		}, true
	}
	return nil, false
}

func toolLabel(id string, a shellmanager.Action) string {
	if a == shellmanager.ActionNone {
		return id
	}
	return id + " (" + string(a) + ")"
}

func resultStatus(r shellmanager.Result) styles.Status {
	switch r {
	case shellmanager.ResultOK, shellmanager.ResultRemoved, shellmanager.ResultMigrated:
		return styles.StatusOK
	case shellmanager.ResultSkipped:
		return styles.StatusSkip
	case shellmanager.ResultModified, shellmanager.ResultWarned:
		return styles.StatusWarn
	default:
		return styles.StatusFail
	}
}

func checkStatus(s shellmanager.CheckStatus) styles.Status {
	switch s {
	case shellmanager.CheckOK:
		return styles.StatusOK
	case shellmanager.CheckWarn:
		return styles.StatusWarn
	case shellmanager.CheckSkip:
		return styles.StatusSkip
	default:
		return styles.StatusFail
	}
}

// checkRows turns doctor checks into table rows.
func checkRows(home string, rep shellmanager.Report) []statustable.Row {
	rows := make([]statustable.Row, 0, len(rep.Checks))
	for _, c := range rep.Checks {
		rows = append(rows, statustable.Row{Status: checkStatus(c.Status), Name: shortPath(home, c.Name), Detail: c.Detail})
	}
	return rows
}

func phaseLabel(p shellmanager.Phase) string {
	switch p {
	case shellmanager.PhasePreflight:
		return "Checking system packages"
	case shellmanager.PhaseTools:
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

package cmd

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
	"github.com/BlasterM2A/shell-manager/internal/ui/statustable"
	"github.com/BlasterM2A/shell-manager/internal/ui/steplist"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

func TestEventMsg(t *testing.T) {
	cases := []struct {
		ev   shellmanager.Event
		want tea.Msg
		ok   bool
	}{
		{shellmanager.PhaseStarted{Phase: shellmanager.PhaseTools}, nil, false},
		{
			shellmanager.ToolStarted{ID: "starship", Action: shellmanager.ActionInstall},
			steplist.StepStarted{ID: "tool:starship", Label: "starship (install)"}, true,
		},
		{
			shellmanager.ToolFinished{ID: "starship", Action: shellmanager.ActionInstall, Result: shellmanager.ResultOK, Version: "1.0"},
			steplist.StepFinished{ID: "tool:starship", Label: "starship (install)", Status: styles.StatusOK, Detail: "1.0"}, true,
		},
		{
			shellmanager.ToolFinished{ID: "claude", Result: shellmanager.ResultWarned, Err: errors.New("claude: boom")},
			steplist.StepFinished{ID: "tool:claude", Label: "claude", Status: styles.StatusWarn, Detail: "claude: boom"}, true,
		},
		{
			shellmanager.FileFinished{Path: "/h/.zshrc", Result: shellmanager.ResultModified},
			steplist.StepFinished{ID: "file:/h/.zshrc", Label: "~/.zshrc", Status: styles.StatusWarn, Detail: "modified"}, true,
		},
	}
	for _, tc := range cases {
		got, ok := eventMsg("/h", tc.ev)
		assert.Equal(t, tc.ok, ok, "%#v", tc.ev)
		assert.Equal(t, tc.want, got, "%#v", tc.ev)
	}
}

func TestCheckRows(t *testing.T) {
	rep := shellmanager.Report{Checks: []shellmanager.CheckResult{
		{Name: "zsh", Status: shellmanager.CheckOK, Detail: "zsh 5.9"},
		{Name: "/h/.zshrc", Status: shellmanager.CheckWarn, Detail: "modified"},
		{Name: "antidote", Status: shellmanager.CheckFail, Detail: "not installed"},
		{Name: "version", Status: shellmanager.CheckSkip},
	}}
	assert.Equal(t, []statustable.Row{
		{Status: styles.StatusOK, Name: "zsh", Detail: "zsh 5.9"},
		{Status: styles.StatusWarn, Name: "~/.zshrc", Detail: "modified"},
		{Status: styles.StatusFail, Name: "antidote", Detail: "not installed"},
		{Status: styles.StatusSkip, Name: "version"},
	}, checkRows("/h", rep))
}

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

func TestSummaryAfterChangingTheLoginShell(t *testing.T) {
	c := common.New(styles.New(false))
	got := renderSummary(c, "/h", "/h/log", shellmanager.Report{ShellChanged: true}, shellmanager.Report{})
	assert.Contains(t, got, "Run `exec zsh` (or open a new terminal) to start using zsh.")
	assert.Contains(t, got, "If new terminals still open your old shell, log out and back in.")
	assert.Contains(t, got, "JetBrainsMono Nerd Font")
	assert.NotContains(t, got, "Log out and back in, then")
}

func TestSummaryWhenZshWasAlreadyTheShell(t *testing.T) {
	c := common.New(styles.New(false))
	got := renderSummary(c, "/h", "/h/log", shellmanager.Report{}, shellmanager.Report{})
	assert.Contains(t, got, "Run `exec zsh` to load the new configuration.")
	assert.NotContains(t, got, "log out")
}

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-setup/internal/shellsetup"
	"github.com/BlasterM2A/shell-setup/internal/ui/common"
	"github.com/BlasterM2A/shell-setup/internal/ui/styles"
)

func TestSummaryAfterChangingTheLoginShell(t *testing.T) {
	c := common.New(styles.New(false))
	got := renderSummary(c, "/h", "/h/log", shellsetup.Report{ShellChanged: true}, shellsetup.Report{})
	assert.Contains(t, got, "Run `exec zsh` (or open a new terminal) to start using zsh.")
	assert.Contains(t, got, "If new terminals still open your old shell, log out and back in.")
	assert.Contains(t, got, "JetBrainsMono Nerd Font")
	assert.NotContains(t, got, "Log out and back in, then")
}

func TestSummaryWhenZshWasAlreadyTheShell(t *testing.T) {
	c := common.New(styles.New(false))
	got := renderSummary(c, "/h", "/h/log", shellsetup.Report{}, shellsetup.Report{})
	assert.Contains(t, got, "Run `exec zsh` to load the new configuration.")
	assert.NotContains(t, got, "log out")
}

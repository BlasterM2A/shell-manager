package model_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/model"
	"github.com/BlasterM2A/shell-manager/internal/ui/steplist"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

func newProgress(onInterrupt func()) model.Progress {
	return model.NewProgress(common.New(styles.New(false)), "Setting up", onInterrupt)
}

func TestProgressForwardsStepMessages(t *testing.T) {
	var m tea.Model = newProgress(nil)
	m, _ = m.Update(steplist.StepFinished{ID: "mise", Label: "mise", Status: styles.StatusOK})
	assert.Equal(t, "Setting up\n  ✓ mise\n", m.(model.Progress).Render())
}

func TestDoneShowsSummaryAndQuits(t *testing.T) {
	var m tea.Model = newProgress(nil)
	m, cmd := m.Update(model.Done{Summary: "all good\n"})
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
	assert.Equal(t, "Setting up\n\nall good\n", m.(model.Progress).Render())
}

func TestCtrlCInterruptsOnce(t *testing.T) {
	calls := 0
	var m tea.Model = newProgress(func() { calls++ })
	key := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	m, _ = m.Update(key)
	m, _ = m.Update(key)
	assert.Equal(t, 1, calls)
	assert.Contains(t, m.(model.Progress).Render(), "Interrupted")
}

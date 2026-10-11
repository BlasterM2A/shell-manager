package steplist_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/steplist"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

func TestStepsKeepOrderAndUpdateInPlace(t *testing.T) {
	m := steplist.New(common.New(styles.New(false)), steplist.Props{Title: "Updating"})
	m = m.Update(steplist.StepStarted{ID: "mise", Label: "mise"})
	m = m.Update(steplist.StepStarted{ID: "starship", Label: "starship"})
	m = m.Update(steplist.StepFinished{ID: "mise", Status: styles.StatusOK, Detail: "2026.10.6"})

	want := "Updating\n" +
		"  ✓ mise      2026.10.6\n" +
		"  … starship\n"
	assert.Equal(t, want, m.View())
}

func TestFinishedWithoutStartIsAppended(t *testing.T) {
	m := steplist.New(common.New(styles.New(false)), steplist.Props{})
	m = m.Update(steplist.StepFinished{ID: "f1", Label: "~/.zshrc", Status: styles.StatusWarn, Detail: "modified"})
	assert.Equal(t, "  ! ~/.zshrc  modified\n", m.View())
}

func TestUnknownMessagesAreIgnored(t *testing.T) {
	m := steplist.New(common.New(styles.New(false)), steplist.Props{})
	m = m.Update("noise")
	assert.Equal(t, "", m.View())
}

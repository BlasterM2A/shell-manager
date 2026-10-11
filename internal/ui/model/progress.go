// Package model holds the top-level bubbletea models run by commands.
package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/notice"
	"github.com/BlasterM2A/shell-manager/internal/ui/steplist"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// Done ends the progress view; Summary is shown below the steps.
type Done struct {
	Summary string
}

// Progress shows a step list while an operation runs, then its summary.
type Progress struct {
	c           common.Common
	steps       steplist.Model
	summary     string
	interrupted bool
	onInterrupt func()
}

// NewProgress returns the view. onInterrupt is called once on ctrl+c; the
// view keeps running until Done arrives so the last step can finish.
func NewProgress(c common.Common, title string, onInterrupt func()) Progress {
	return Progress{
		c:           c,
		steps:       steplist.New(c, steplist.Props{Title: title}),
		onInterrupt: onInterrupt,
	}
}

func (m Progress) Init() tea.Cmd { return nil }

func (m Progress) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case Done:
		m.summary = msg.Summary
		return m, tea.Quit
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" && !m.interrupted {
			m.interrupted = true
			if m.onInterrupt != nil {
				m.onInterrupt()
			}
		}
		return m, nil
	}
	m.steps = m.steps.Update(msg)
	return m, nil
}

// Render returns the view's text (View wraps it for bubbletea).
func (m Progress) Render() string {
	s := m.steps.View()
	if m.interrupted && m.summary == "" {
		s += "\n" + notice.Render(m.c, notice.Props{
			Status: styles.StatusWarn,
			Text:   "Interrupted, finishing the current step…",
		}) + "\n"
	}
	if m.summary != "" {
		s += "\n" + m.summary
	}
	return s
}

func (m Progress) View() tea.View { return tea.NewView(m.Render()) }

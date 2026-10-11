// Package steplist renders the live progress of a sequence of steps.
package steplist

import (
	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/statustable"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// StepStarted marks a step as running, adding it if new.
type StepStarted struct {
	ID    string
	Label string
}

// StepFinished sets a step's final status, adding it if new.
type StepFinished struct {
	ID     string
	Label  string
	Status styles.Status
	Detail string
}

// Props are the inputs of the list.
type Props struct {
	Title string
}

type step struct {
	label  string
	detail string
	status styles.Status
}

// Model is the step list. It has a single owner: copies share state.
type Model struct {
	c     common.Common
	props Props
	order []string
	steps map[string]step
}

// New returns an empty list.
func New(c common.Common, p Props) Model {
	return Model{c: c, props: p, steps: map[string]step{}}
}

// Update applies StepStarted/StepFinished; other messages are ignored.
func (m Model) Update(msg any) Model {
	switch msg := msg.(type) {
	case StepStarted:
		m.upsert(msg.ID, msg.Label, func(s *step) { s.status = styles.StatusRunning })
	case StepFinished:
		m.upsert(msg.ID, msg.Label, func(s *step) {
			s.status = msg.Status
			s.detail = msg.Detail
		})
	}
	return m
}

func (m *Model) upsert(id, label string, apply func(*step)) {
	s, ok := m.steps[id]
	if !ok {
		m.order = append(m.order, id)
		s.label = id
	}
	if label != "" {
		s.label = label
	}
	apply(&s)
	m.steps[id] = s
}

// View renders the steps in the order they first appeared.
func (m Model) View() string {
	rows := make([]statustable.Row, 0, len(m.order))
	for _, id := range m.order {
		s := m.steps[id]
		rows = append(rows, statustable.Row{Status: s.status, Name: s.label, Detail: s.detail})
	}
	if len(rows) == 0 && m.props.Title == "" {
		return ""
	}
	return statustable.Render(m.c, statustable.Props{Title: m.props.Title, Rows: rows})
}

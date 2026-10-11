package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
)

func fakeOp(events ...shellmanager.Event) operation {
	return func(_ context.Context, ch chan<- shellmanager.Event) (shellmanager.Report, error) {
		defer close(ch)
		for _, ev := range events {
			ch <- ev
		}
		return shellmanager.Report{Tools: []shellmanager.ToolReport{{ID: "starship", Result: shellmanager.ResultOK}}}, nil
	}
}

func staticSummary(shellmanager.Report, error) string { return "SUMMARY\n" }

func TestRunPlainWhenNotTTY(t *testing.T) {
	f, out := newTestFactory(t)
	op := fakeOp(
		shellmanager.PhaseStarted{Phase: shellmanager.PhaseTools},
		shellmanager.ToolFinished{ID: "starship", Result: shellmanager.ResultOK, Version: "1.0"},
		shellmanager.FileFinished{Path: filepath.Join(f.Home, ".zshrc"), Result: shellmanager.ResultModified},
	)

	res, err := runWithProgress(context.Background(), f, "Setting up", op, staticSummary)
	require.NoError(t, err)
	require.NoError(t, res.OpErr)
	assert.Len(t, res.Report.Tools, 1)
	assert.Equal(t, "Setting up\n"+
		"• Installing tools\n"+
		"  ✓ starship  1.0\n"+
		"  ! ~/.zshrc  modified\n"+
		"SUMMARY\n", out.String())
}

func TestRunTUIReturnsReport(t *testing.T) {
	f, _ := newTestFactory(t)
	var screen bytes.Buffer
	op := fakeOp(shellmanager.ToolFinished{ID: "starship", Result: shellmanager.ResultOK})

	res, err := runTUI(context.Background(), f, "Setting up", op, staticSummary,
		tea.WithInput(nil), tea.WithOutput(&screen), tea.WithWindowSize(80, 24))
	require.NoError(t, err)
	assert.Len(t, res.Report.Tools, 1)
	assert.Contains(t, screen.String(), "SUMMARY")
}

func TestRunTUIWaitsForOpWhenProgramFails(t *testing.T) {
	f, _ := newTestFactory(t)
	killed, kill := context.WithCancel(context.Background())
	kill() // p.Run returns an error right away
	var finished atomic.Bool
	op := func(ctx context.Context, ch chan<- shellmanager.Event) (shellmanager.Report, error) {
		defer close(ch)
		<-ctx.Done() // runTUI must cancel the op...
		time.Sleep(50 * time.Millisecond)
		finished.Store(true) // ...and wait for it to finish
		return shellmanager.Report{}, ctx.Err()
	}

	_, err := runTUI(context.Background(), f, "Setting up", op, staticSummary,
		tea.WithContext(killed), tea.WithInput(nil), tea.WithOutput(&bytes.Buffer{}), tea.WithWindowSize(80, 24))
	require.Error(t, err)
	assert.True(t, finished.Load(), "runTUI returned before the operation finished")
}

package cmd

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/BlasterM2A/shell-manager/internal/shellmanager"
	"github.com/BlasterM2A/shell-manager/internal/ui/model"
	"github.com/BlasterM2A/shell-manager/internal/ui/notice"
	"github.com/BlasterM2A/shell-manager/internal/ui/statustable"
	"github.com/BlasterM2A/shell-manager/internal/ui/steplist"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// operation is an Engine call that publishes events and closes the channel.
type operation func(ctx context.Context, events chan<- shellmanager.Event) (shellmanager.Report, error)

// progressResult is what the operation returned.
type progressResult struct {
	Report shellmanager.Report
	OpErr  error
}

// runWithProgress runs op and shows its progress: the TUI on a terminal,
// plain lines otherwise. summary renders the final block once op is done.
// The returned error is a UI failure; op's own error is in OpErr.
func runWithProgress(ctx context.Context, f *Factory, title string, op operation,
	summary func(shellmanager.Report, error) string) (progressResult, error) {
	if f.UseTUI() {
		return runTUI(ctx, f, title, op, summary)
	}
	return runPlain(ctx, f, title, op, summary), nil
}

func runPlain(ctx context.Context, f *Factory, title string, op operation,
	summary func(shellmanager.Report, error) string) progressResult {
	c, out := f.Common(), f.IOStreams.Out
	_, _ = fmt.Fprintln(out, c.Styles.Title.Render(title))
	events := make(chan shellmanager.Event)
	done := make(chan progressResult, 1)
	go func() {
		rep, err := op(ctx, events)
		done <- progressResult{Report: rep, OpErr: err}
	}()
	for ev := range events {
		if p, ok := ev.(shellmanager.PhaseStarted); ok {
			_, _ = fmt.Fprintln(out, notice.Render(c, notice.Props{Status: styles.StatusInfo, Text: phaseLabel(p.Phase)}))
			continue
		}
		msg, ok := eventMsg(f.Home, ev)
		if fin, isFinished := msg.(steplist.StepFinished); ok && isFinished {
			_, _ = fmt.Fprint(out, statustable.Render(c, statustable.Props{Rows: []statustable.Row{
				{Status: fin.Status, Name: fin.Label, Detail: fin.Detail},
			}}))
		}
	}
	res := <-done
	_, _ = fmt.Fprint(out, summary(res.Report, res.OpErr))
	return res
}

func runTUI(ctx context.Context, f *Factory, title string, op operation,
	summary func(shellmanager.Report, error) string, opts ...tea.ProgramOption) (progressResult, error) {
	sys, err := f.System()
	if err != nil {
		return progressResult{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Execute already turns SIGINT/SIGTERM into ctx cancellation.
	opts = append([]tea.ProgramOption{
		tea.WithInput(f.IOStreams.In), tea.WithOutput(f.IOStreams.Out), tea.WithoutSignalHandler(),
	}, opts...)
	p := tea.NewProgram(model.NewProgress(f.Common(), title, cancel), opts...)

	// sudo/chsh password prompts need the real terminal: suspend the TUI.
	sys.SetTerminalHandoff(func(run func() error) error {
		if err := p.ReleaseTerminal(); err != nil {
			return err
		}
		defer func() { _ = p.RestoreTerminal() }()
		return run()
	})
	defer sys.SetTerminalHandoff(nil)

	done := make(chan progressResult, 1)
	go func() {
		// Send blocks until the event loop runs, so the terminal is set up
		// before op can hand it off (or returns once the program is gone).
		p.Send(tuiStarted{})
		events := make(chan shellmanager.Event)
		forwarded := make(chan struct{})
		go func() {
			for ev := range events {
				if msg, ok := eventMsg(f.Home, ev); ok {
					p.Send(msg)
				}
			}
			close(forwarded)
		}()
		rep, err := op(ctx, events)
		<-forwarded
		p.Send(model.Done{Summary: summary(rep, err)})
		done <- progressResult{Report: rep, OpErr: err}
	}()
	if _, err := p.Run(); err != nil {
		// Let op stop at a step boundary and save its state before exiting.
		cancel()
		<-done
		return progressResult{}, err
	}
	return <-done, nil
}

// tuiStarted is the handshake message; the Progress model ignores it.
type tuiStarted struct{}

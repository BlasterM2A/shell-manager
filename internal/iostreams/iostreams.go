// Package iostreams wraps stdin/stdout/stderr with TTY and color detection.
// Output written outside the TUI uses Styles() so it matches the TUI.
package iostreams

import (
	"bytes"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// IOStreams are the process streams.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	tty   bool
	color bool
}

// System returns the real process streams.
func System() *IOStreams {
	tty := term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
	return &IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
		tty:    tty,
		color:  colorEnabled(tty, os.LookupEnv),
	}
}

// Test returns non-TTY streams backed by buffers.
func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer) {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	return &IOStreams{In: &bytes.Buffer{}, Out: out, ErrOut: errOut}, out, errOut
}

func colorEnabled(tty bool, lookup func(string) (string, bool)) bool {
	if !tty {
		return false
	}
	if _, ok := lookup("NO_COLOR"); ok {
		return false
	}
	termName, _ := lookup("TERM")
	return termName != "dumb"
}

// IsTTY reports whether both stdin and stdout are terminals.
func (s *IOStreams) IsTTY() bool { return s.tty }

// ColorEnabled reports whether output may use colors.
func (s *IOStreams) ColorEnabled() bool { return s.color }

// Styles returns the shared styles, plain when color is disabled.
func (s *IOStreams) Styles() styles.Styles { return styles.New(s.color) }

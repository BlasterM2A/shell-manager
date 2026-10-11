// Package styles is the single source of truth for colors, icons and text
// styles. TUI components, plain output and console logs all render with it;
// no other package defines colors.
package styles

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Status is the semantic state shared by every component and log line.
type Status int

const (
	StatusInfo Status = iota
	StatusOK
	StatusWarn
	StatusFail
	StatusSkip
	StatusRunning
)

var glyphs = map[Status]string{
	StatusInfo:    "•",
	StatusOK:      "✓",
	StatusWarn:    "!",
	StatusFail:    "✗",
	StatusSkip:    "-",
	StatusRunning: "…",
}

// Glyph returns the unstyled icon for a status.
func Glyph(s Status) string { return glyphs[s] }

// Palette holds the semantic colors of the theme.
type Palette struct {
	Text, Muted, Accent, Success, Warning, Error color.Color
}

// DefaultPalette is the shell-manager theme.
func DefaultPalette() Palette {
	return Palette{
		Text:    lipgloss.Color("#C0CAF5"),
		Muted:   lipgloss.Color("#737AA2"),
		Accent:  lipgloss.Color("#7AA2F7"),
		Success: lipgloss.Color("#9ECE6A"),
		Warning: lipgloss.Color("#E0AF68"),
		Error:   lipgloss.Color("#F7768E"),
	}
}

// Styles are the lipgloss styles every component renders with.
type Styles struct {
	Title  lipgloss.Style
	Text   lipgloss.Style
	Muted  lipgloss.Style
	status map[Status]lipgloss.Style
}

// New builds the styles. With color=false every style renders plain text,
// which is what non-terminal output and NO_COLOR use.
func New(color bool) Styles {
	s := Styles{
		Title:  lipgloss.NewStyle(),
		Text:   lipgloss.NewStyle(),
		Muted:  lipgloss.NewStyle(),
		status: make(map[Status]lipgloss.Style, len(glyphs)),
	}
	for st := range glyphs {
		s.status[st] = lipgloss.NewStyle()
	}
	if !color {
		return s
	}
	p := DefaultPalette()
	s.Title = s.Title.Bold(true).Foreground(p.Accent)
	s.Text = s.Text.Foreground(p.Text)
	s.Muted = s.Muted.Foreground(p.Muted)
	s.status[StatusInfo] = s.status[StatusInfo].Foreground(p.Accent)
	s.status[StatusOK] = s.status[StatusOK].Foreground(p.Success)
	s.status[StatusWarn] = s.status[StatusWarn].Foreground(p.Warning)
	s.status[StatusFail] = s.status[StatusFail].Foreground(p.Error)
	s.status[StatusSkip] = s.status[StatusSkip].Foreground(p.Muted)
	s.status[StatusRunning] = s.status[StatusRunning].Foreground(p.Accent)
	return s
}

// Status returns the style for a status.
func (s Styles) Status(st Status) lipgloss.Style { return s.status[st] }

// Icon renders the status glyph with its style.
func (s Styles) Icon(st Status) string { return s.status[st].Render(Glyph(st)) }

// Package statustable renders rows of "icon name detail" with aligned names.
package statustable

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// Row is one line of the table.
type Row struct {
	Status styles.Status
	Name   string
	Detail string
}

// Props are the inputs of the table.
type Props struct {
	Title string
	Rows  []Row
}

// Render returns the table; every row ends with a newline.
func Render(c common.Common, p Props) string {
	var b strings.Builder
	if p.Title != "" {
		b.WriteString(c.Styles.Title.Render(p.Title))
		b.WriteByte('\n')
	}
	width := 0
	for _, r := range p.Rows {
		width = max(width, lipgloss.Width(r.Name))
	}
	for _, r := range p.Rows {
		b.WriteString("  ")
		b.WriteString(c.Styles.Icon(r.Status))
		b.WriteByte(' ')
		b.WriteString(c.Styles.Text.Render(r.Name))
		if r.Detail != "" {
			b.WriteString(strings.Repeat(" ", width-lipgloss.Width(r.Name)+2))
			b.WriteString(c.Styles.Muted.Render(r.Detail))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

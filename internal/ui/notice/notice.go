// Package notice renders a single status message line.
package notice

import (
	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

// Props are the inputs of a notice.
type Props struct {
	Status styles.Status
	Text   string
}

// Render returns the notice as one line, without a trailing newline.
func Render(c common.Common, p Props) string {
	return c.Styles.Icon(p.Status) + " " + c.Styles.Status(p.Status).Render(p.Text)
}

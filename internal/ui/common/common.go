// Package common carries what every UI component receives.
package common

import "github.com/BlasterM2A/shell-manager/internal/ui/styles"

// Common is passed to every component.
type Common struct {
	Styles styles.Styles
}

// New returns the Common for a set of styles.
func New(s styles.Styles) Common { return Common{Styles: s} }

package styles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

var all = []styles.Status{
	styles.StatusInfo, styles.StatusOK, styles.StatusWarn,
	styles.StatusFail, styles.StatusSkip, styles.StatusRunning,
}

func TestEveryStatusHasAGlyph(t *testing.T) {
	for _, st := range all {
		assert.NotEmpty(t, styles.Glyph(st), "status %d", st)
	}
}

func TestPlainStylesRenderNoEscapes(t *testing.T) {
	s := styles.New(false)
	for _, st := range all {
		assert.Equal(t, styles.Glyph(st), s.Icon(st))
	}
	assert.Equal(t, "title", s.Title.Render("title"))
	assert.Equal(t, "muted", s.Muted.Render("muted"))
}

func TestColorStylesEmitANSI(t *testing.T) {
	got := styles.New(true).Icon(styles.StatusOK)
	assert.Contains(t, got, "✓")
	assert.Contains(t, got, "\x1b[")
}

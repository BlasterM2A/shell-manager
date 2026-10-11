package statustable_test

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-manager/internal/ui/common"
	"github.com/BlasterM2A/shell-manager/internal/ui/statustable"
	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

var props = statustable.Props{
	Title: "Doctor",
	Rows: []statustable.Row{
		{Status: styles.StatusOK, Name: "starship", Detail: "1.20.0"},
		{Status: styles.StatusFail, Name: "zoxide", Detail: "not installed"},
		{Status: styles.StatusSkip, Name: "claude"},
	},
}

func TestRenderPlain(t *testing.T) {
	got := statustable.Render(common.New(styles.New(false)), props)
	want := "Doctor\n" +
		"  ✓ starship  1.20.0\n" +
		"  ✗ zoxide    not installed\n" +
		"  - claude\n"
	assert.Equal(t, want, got)
}

func TestRenderWithoutTitle(t *testing.T) {
	got := statustable.Render(common.New(styles.New(false)), statustable.Props{
		Rows: []statustable.Row{{Status: styles.StatusOK, Name: "mise", Detail: "2026.10.6"}},
	})
	assert.Equal(t, "  ✓ mise  2026.10.6\n", got)
}

func TestRenderColor(t *testing.T) {
	golden.RequireEqual(t, []byte(statustable.Render(common.New(styles.New(true)), props)))
}

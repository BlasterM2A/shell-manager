package iostreams

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-manager/internal/ui/styles"
)

func env(vars map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
}

func TestColorRequiresTTY(t *testing.T) {
	assert.False(t, colorEnabled(false, env(nil)))
	assert.True(t, colorEnabled(true, env(map[string]string{"TERM": "xterm-256color"})))
}

func TestNoColorAndDumbTermDisableColor(t *testing.T) {
	assert.False(t, colorEnabled(true, env(map[string]string{"NO_COLOR": ""})))
	assert.False(t, colorEnabled(true, env(map[string]string{"TERM": "dumb"})))
}

func TestTestStreamsArePlain(t *testing.T) {
	ios, out, _ := Test()
	assert.False(t, ios.IsTTY())
	assert.Equal(t, styles.Glyph(styles.StatusOK), ios.Styles().Icon(styles.StatusOK))
	_, _ = ios.Out.Write([]byte("hi"))
	assert.Equal(t, "hi", out.String())
}

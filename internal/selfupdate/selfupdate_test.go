package selfupdate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BlasterM2A/shell-manager/internal/selfupdate"
)

func TestDevBuildCannotSelfUpdate(t *testing.T) {
	u := selfupdate.New("BlasterM2A/shell-manager", "dev")

	_, _, err := u.Latest(context.Background())
	assert.ErrorIs(t, err, selfupdate.ErrDevBuild)

	_, _, err = u.Apply(context.Background())
	assert.ErrorIs(t, err, selfupdate.ErrDevBuild)
}

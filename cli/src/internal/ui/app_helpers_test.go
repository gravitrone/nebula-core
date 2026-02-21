package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCenterBlockPadsShortLines handles test center block pads short lines.
func TestCenterBlockPadsShortLines(t *testing.T) {
	in := "hi\nworld"
	out := centerBlock(in, 10)

	lines := strings.Split(out, "\n")
	assert.Equal(t, 2, len(lines))
	assert.True(t, strings.HasPrefix(lines[0], " "), "expected first line padded")
	assert.True(t, strings.HasPrefix(lines[1], " "), "expected second line padded")
	assert.Contains(t, lines[0], "hi")
	assert.Contains(t, lines[1], "world")
}

// TestCenterBlockLeavesWideLinesUnchanged handles test center block leaves wide lines unchanged.
func TestCenterBlockLeavesWideLinesUnchanged(t *testing.T) {
	in := "0123456789"
	assert.Equal(t, in, centerBlock(in, 5))
	assert.Equal(t, in, centerBlock(in, 0))
}

// TestAppRenderTipsDoesNotPanic handles test app render tips does not panic.
func TestAppRenderTipsDoesNotPanic(t *testing.T) {
	app := App{width: 80}
	assert.NotPanics(t, func() { _ = app.renderTips() })
}

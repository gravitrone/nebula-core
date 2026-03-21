package components

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestStatusBarHandlesNonPositiveAvailableWidth(t *testing.T) {
	orig := statusBarBorder
	statusBarBorder = lipgloss.NewStyle().Padding(0, 2)
	t.Cleanup(func() { statusBarBorder = orig })

	out := StatusBar([]string{Hint("q", "Quit")}, 2)
	assert.NotEmpty(t, out)
}

func TestWrapSegmentsWidthNonPositiveReturnsSingleJoinedRow(t *testing.T) {
	segments := []string{"a", "b", "c"}
	rows := wrapSegments(segments, 0)

	assert.Len(t, rows, 1)
	assert.Contains(t, rows[0], "a")
	assert.Contains(t, rows[0], "b")
	assert.Contains(t, rows[0], "c")
}

func TestClampStatusSegmentsReturnsInputForGuardBranches(t *testing.T) {
	assert.Nil(t, clampStatusSegments(nil, 10))
	assert.Equal(t, []string{"a"}, clampStatusSegments([]string{"a"}, 0))
}

func TestClampStatusSegmentsTinyWidthDegenerateFallback(t *testing.T) {
	segments := []string{"very-long-segment"}
	out := clampStatusSegments(segments, 1)

	assert.Len(t, out, 1)
	assert.LessOrEqual(t, lipgloss.Width(out[0]), 1)
}

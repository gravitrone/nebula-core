package components

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSI handles strip ansi.
func stripANSI(input string) string {
	return ansiPattern.ReplaceAllString(input, "")
}

// TestHintIncludesKeyAndDesc handles test hint includes key and desc.
func TestHintIncludesKeyAndDesc(t *testing.T) {
	out := Hint("↑/↓", "Scroll")
	assert.True(t, strings.Contains(out, "Scroll"))
	assert.True(t, strings.Contains(out, "↑/↓"))
}

// TestStatusBarRendersHints handles test status bar renders hints.
func TestStatusBarRendersHints(t *testing.T) {
	out := StatusBar([]string{Hint("q", "Quit")}, 0)
	assert.True(t, strings.Contains(out, "Quit"))
	assert.True(t, strings.Contains(out, "q"))
}

// TestWrapSegmentsWrapsWhenNarrow handles test wrap segments wraps when narrow.
func TestWrapSegmentsWrapsWhenNarrow(t *testing.T) {
	segments := []string{"123456", "abcdef", "ghijkl"}
	rows := wrapSegments(segments, 10)
	assert.Len(t, rows, 3)
	for _, row := range rows {
		assert.LessOrEqual(t, lipgloss.Width(row), 10)
	}
}

// TestStatusBarKeepsSingleRowAndAddsOverflowHint handles test status bar keeps single row and adds overflow hint.
func TestStatusBarKeepsSingleRowAndAddsOverflowHint(t *testing.T) {
	hints := []string{
		Hint("1-9/0", "Tabs"),
		Hint("/", "Command"),
		Hint("?", "Help"),
		Hint("q", "Quit"),
		Hint("ctrl+u/d", "View"),
		Hint("↑/↓", "Scroll"),
		Hint("space", "Select"),
		Hint("b", "Select All"),
		Hint("A", "Approve All"),
		Hint("a", "Approve"),
		Hint("r", "Reject"),
		Hint("enter", "Details"),
		Hint("f", "Filter"),
	}

	out := StatusBar(hints, 140)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 3, "status bar should stay a single boxed row")
	assert.True(t, strings.Contains(out, "More") || strings.Contains(out, "..."))
	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(stripANSI(line)), 140)
	}
}

// TestStatusBarClampNeverWrapsToSecondContentRow handles test status bar clamp never wraps to second content row.
func TestStatusBarClampNeverWrapsToSecondContentRow(t *testing.T) {
	hints := []string{
		Hint("1-9/0", "Tabs"),
		Hint("/", "Command"),
		Hint("?", "Help"),
		Hint("q", "Quit"),
		Hint("ctrl+u/d", "View"),
		Hint("↑/↓", "Scroll"),
		Hint("space", "Select"),
		Hint("b", "Select All"),
		Hint("A", "Approve All"),
		Hint("a", "Approve"),
		Hint("r", "Reject"),
		Hint("enter", "Details"),
		Hint("f", "Filter"),
	}

	out := StatusBar(hints, 120)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 3, "status bar should always stay one bordered content row")
	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(stripANSI(line)), 120)
	}
}

// TestStatusBarCentersHintsWhenWidthProvided handles test status bar centers hints when width provided.
func TestStatusBarCentersHintsWhenWidthProvided(t *testing.T) {
	out := StatusBar([]string{Hint("q", "Quit")}, 100)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 3)

	for _, line := range lines {
		plain := stripANSI(line)
		if strings.TrimSpace(plain) == "" {
			continue
		}
		assert.True(t, strings.HasPrefix(plain, " "), "line should be centered with left padding")
	}
}

func TestStatusSegmentsWidthBranches(t *testing.T) {
	assert.Equal(t, 0, statusSegmentsWidth(nil))
	assert.Equal(t, 0, statusSegmentsWidth([]string{}))
	assert.Equal(
		t,
		lipgloss.Width("abc"),
		statusSegmentsWidth([]string{"abc"}),
	)
	assert.Equal(
		t,
		lipgloss.Width(lipgloss.JoinHorizontal(lipgloss.Top, "abc", "def")),
		statusSegmentsWidth([]string{"abc", "def"}),
	)
}

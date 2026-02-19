package components

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(input string) string {
	return ansiPattern.ReplaceAllString(input, "")
}

func TestHintIncludesKeyAndDesc(t *testing.T) {
	out := Hint("↑/↓", "Scroll")
	assert.True(t, strings.Contains(out, "Scroll"))
	assert.True(t, strings.Contains(out, "↑/↓"))
}

func TestStatusBarRendersHints(t *testing.T) {
	out := StatusBar([]string{Hint("q", "Quit")}, 0)
	assert.True(t, strings.Contains(out, "Quit"))
	assert.True(t, strings.Contains(out, "q"))
}

func TestWrapSegmentsWrapsWhenNarrow(t *testing.T) {
	segments := []string{"123456", "abcdef", "ghijkl"}
	rows := wrapSegments(segments, 10)
	assert.Len(t, rows, 3)
	for _, row := range rows {
		assert.LessOrEqual(t, lipgloss.Width(row), 10)
	}
}

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
}

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

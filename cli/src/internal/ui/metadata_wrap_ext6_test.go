package ui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataGridRowsWrappedFallbackAndBalancing(t *testing.T) {
	rows := metadataGridRowsWrapped("", "", "line one\nline two", 10, 10, 10)
	require.Len(t, rows, 2)
	assert.Equal(t, "-", rows[0][0])
	assert.Equal(t, "-", rows[0][1])
	assert.Equal(t, "line one", rows[0][2])
	assert.Equal(t, "", rows[1][0])
	assert.Equal(t, "", rows[1][1])
	assert.Equal(t, "line two", rows[1][2])
}

func TestMetadataValueWrappedLinesSanitizeAndKeepBlankRows(t *testing.T) {
	lines := metadataValueWrappedLines(" \x1b[31mabc\x1b[0m \n\nsecond ", 10)
	require.Len(t, lines, 3)
	assert.Equal(t, "abc", components.SanitizeText(lines[0]))
	assert.Equal(t, "", lines[1])
	assert.Equal(t, "second", components.SanitizeText(lines[2]))
}

func TestMetadataColumnWidthsOverflowRebalanceBranch(t *testing.T) {
	group, field, value := metadataColumnWidths(48)
	assert.Equal(t, 10, group)
	assert.Equal(t, 14, field)
	assert.Equal(t, 22, value)
	assert.Equal(t, 46, group+field+value) // content(48) - separators(2)
}

func TestWrapMetadataDisplayLinesWidthZeroPassthrough(t *testing.T) {
	input := []string{"  a", "b"}
	assert.Equal(t, input, wrapMetadataDisplayLines(input, 0))
}

func TestWrapMetadataDisplayLineBulletAndShortPaths(t *testing.T) {
	short := wrapMetadataDisplayLine("ok", 10)
	assert.Equal(t, []string{"ok"}, short)

	bullet := wrapMetadataDisplayLine("  - alpha beta gamma delta", 12)
	require.Greater(t, len(bullet), 1)
	assert.Contains(t, bullet[0], "- ")
	for _, line := range bullet {
		assert.LessOrEqual(t, lipgloss.Width(components.SanitizeText(line)), 12)
	}
}

func TestWrapMetadataWordsWidthAndLongWordBranches(t *testing.T) {
	assert.Equal(t, []string{"alpha"}, wrapMetadataWords(" alpha ", 20))
	assert.Equal(t, []string{"alpha"}, wrapMetadataWords("alpha", 0))
	assert.Nil(t, wrapMetadataWords("   ", 5))

	long := wrapMetadataWords("supercalifragilisticexpialidocious", 6)
	require.Len(t, long, 1)
	assert.LessOrEqual(t, lipgloss.Width(components.SanitizeText(long[0])), 6)

	parts := wrapMetadataWords("one two three four", 7)
	assert.GreaterOrEqual(t, len(parts), 2)
}

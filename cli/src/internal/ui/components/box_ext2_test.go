package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestBoxWidthAndSafeBoxWidthEdgeCases(t *testing.T) {
	assert.Equal(t, 0, boxWidth(0))
	assert.Equal(t, 40, boxWidth(20))
	assert.Equal(t, 20, safeBoxWidth(20))
	assert.Equal(t, 0, safeBoxWidth(0))
}

func TestRenderBoxZeroWidthRendersRawStyle(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)
	assert.Equal(t, style.Render("hello"), renderBox(style, 0, "hello"))
}

func TestBoxContentWidthHandlesTightAndZeroWidths(t *testing.T) {
	assert.Equal(t, 0, BoxContentWidth(0))
	assert.Equal(t, 0, BoxContentWidth(1))
}

func TestClampTextWidthReturnsOriginalWhenWidthNonPositive(t *testing.T) {
	input := "line one\nline two"
	assert.Equal(t, input, ClampTextWidth(input, 0))
}

func TestPadRightReturnsInputWhenAlreadyWideEnough(t *testing.T) {
	assert.Equal(t, "abcd", padRight("abcd", 2))
	assert.Equal(t, "abcd", padRight("abcd", 4))
}

func TestTableSupportsUntitledValueColorRows(t *testing.T) {
	out := Table("", []TableRow{
		{Label: "status", Value: "ok", ValueColor: "#3f866b"},
	}, 80)
	clean := SanitizeText(out)
	assert.Contains(t, clean, "status")
	assert.Contains(t, clean, "ok")
}

func TestWrapTableValueEdgeBranches(t *testing.T) {
	assert.Equal(t, []string{"alpha"}, wrapTableValue("alpha", 0))
	assert.Equal(t, []string{""}, wrapTableValue("   ", 10))
	assert.Equal(t, []string{"a", "", "b"}, wrapTableValue("a\n\nb", 10))
}

func TestCenterLineReturnsOriginalWhenNoPaddingPossible(t *testing.T) {
	assert.Equal(t, "hello", CenterLine("hello", 0))
	assert.Equal(t, "hello", CenterLine("hello", 3))
}

func TestDiffTableAndWrapDiffLineEdgeBranches(t *testing.T) {
	assert.Equal(t, "", DiffTable("Diff", nil, 80))
	assert.Equal(t, []string{"None"}, wrapDiffLine("   ", 10))
	assert.Equal(t, []string{"alpha"}, wrapDiffLine("alpha", 0))
}

func TestMetadataTableReturnsEmptyForEmptyInput(t *testing.T) {
	assert.Equal(t, "", MetadataTable(nil, 80))
	assert.Equal(t, "", MetadataTable(map[string]any{}, 80))
}

func TestEmptyStateBoxSkipsWhitespaceActions(t *testing.T) {
	out := EmptyStateBox("State", "nothing here", []string{"  ", "\t", "run /help"}, 80)
	clean := SanitizeText(out)
	assert.Contains(t, clean, "Try:")
	assert.Equal(t, 1, strings.Count(clean, "run /help"))
}

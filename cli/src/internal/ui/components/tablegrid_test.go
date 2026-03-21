package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

// TestFitGridColumnsPrefersShrinkingWideColumns handles test fit grid columns prefers shrinking wide columns.
func TestFitGridColumnsPrefersShrinkingWideColumns(t *testing.T) {
	columns := []TableColumn{
		{Header: "Rel", Width: 12, Align: lipgloss.Left},
		{Header: "Edge", Width: 42, Align: lipgloss.Left},
		{Header: "Status", Width: 9, Align: lipgloss.Left},
		{Header: "At", Width: 11, Align: lipgloss.Left},
	}

	// Force deficit so at least one column must shrink.
	fitted := fitGridColumns(columns, "|", 56)

	assert.Equal(t, 12, fitted[0].Width, "short system columns should remain stable")
	assert.Less(t, fitted[1].Width, 42, "wide edge column should absorb shrink first")
	assert.Equal(t, 9, fitted[2].Width, "status column should remain readable")
	assert.Equal(t, 11, fitted[3].Width, "time column should remain readable")
}

// TestFitGridColumnsPinsWideTimestampColumnsBeforeTitle handles test fit grid columns pins wide timestamp columns before title.
func TestFitGridColumnsPinsWideTimestampColumnsBeforeTitle(t *testing.T) {
	columns := []TableColumn{
		{Header: "Title", Width: 48, Align: lipgloss.Left},
		{Header: "Type", Width: 10, Align: lipgloss.Left},
		{Header: "Status", Width: 10, Align: lipgloss.Left},
		{Header: "At", Width: 15, Align: lipgloss.Left},
	}

	// Force a deficit where either title or timestamp must shrink.
	fitted := fitGridColumns(columns, "|", 64)

	assert.Less(t, fitted[0].Width, 48, "title should absorb shrink before timestamp")
	assert.Equal(t, 15, fitted[3].Width, "timestamp column should remain fully readable")
}

// TestShrinkColumnsStopsAtMinimums handles test shrink columns stops at minimums.
func TestShrinkColumnsStopsAtMinimums(t *testing.T) {
	columns := []TableColumn{
		{Header: "A", Width: 4},
		{Header: "B", Width: 4},
	}
	remaining := shrinkColumns(columns, []int{4, 4}, 10)
	assert.Equal(t, 10, remaining)
	assert.Equal(t, 4, columns[0].Width)
	assert.Equal(t, 4, columns[1].Width)
}

// TestTableGridWithActiveRowClampsWidthAndRendersRows handles test table grid with active row clamps width and renders rows.
func TestTableGridWithActiveRowClampsWidthAndRendersRows(t *testing.T) {
	columns := []TableColumn{
		{Header: "Name", Width: 16, Align: lipgloss.Left},
		{Header: "Notes", Width: 28, Align: lipgloss.Left},
		{Header: "State", Width: 10, Align: lipgloss.Right},
	}
	rows := [][]string{
		{"alpha", strings.Repeat("very-long-", 8), "[X] ready"},
		{"beta", "short", "open"},
	}
	table := TableGridWithActiveRow(columns, rows, 64, 0)
	lines := strings.Split(table, "\n")
	assert.GreaterOrEqual(t, len(lines), 3)
	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(line), 64)
	}
}

// TestRenderGridCellAlignModes handles test render grid cell align modes.
func TestRenderGridCellAlignModes(t *testing.T) {
	left := renderGridCell("x", 6, lipgloss.Left)
	right := renderGridCell("x", 6, lipgloss.Right)
	center := renderGridCell("x", 6, lipgloss.Center)

	assert.Equal(t, 6, lipgloss.Width(left))
	assert.Equal(t, 6, lipgloss.Width(right))
	assert.Equal(t, 6, lipgloss.Width(center))
	assert.True(t, strings.HasSuffix(left, " "))
	assert.True(t, strings.HasPrefix(right, " "))
	assert.True(t, strings.HasPrefix(center, " "))
}

// TestHighlightSelectionMarkersStylesKnownTokens handles test highlight selection markers styles known tokens.
func TestHighlightSelectionMarkersStylesKnownTokens(t *testing.T) {
	out := highlightSelectionMarkers(" [X] row [x] ")
	clean := SanitizeText(out)
	assert.Contains(t, clean, "[X]")
	assert.Contains(t, clean, "[x]")
}

func TestStyleDiffCellByHeaderMatrix(t *testing.T) {
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("before", "x", "plain")))
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("after", "x", "plain")))
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("change", "added", "plain")))
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("change", "removed", "plain")))
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("change", "updated", "plain")))
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("change", "same", "plain")))
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("change", "unknown", "plain")))
	assert.Equal(t, "plain", SanitizeText(styleDiffCellByHeader("field", "x", "plain")))
}

// TestTableGridWrapperRendersSameContract handles test table grid wrapper renders same contract.
func TestTableGridWrapperRendersSameContract(t *testing.T) {
	columns := []TableColumn{
		{Header: "Name", Width: 12, Align: lipgloss.Left},
		{Header: "Status", Width: 10, Align: lipgloss.Left},
	}
	rows := [][]string{{"alpha", "active"}}
	table := TableGrid(columns, rows, 40)
	assert.NotEmpty(t, table)
	for _, line := range strings.Split(table, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 40)
	}
}

// TestTableGridWithActiveRowCanDisableHighlighting handles test table grid with active row can disable highlighting.
func TestTableGridWithActiveRowCanDisableHighlighting(t *testing.T) {
	columns := []TableColumn{
		{Header: "Name", Width: 12, Align: lipgloss.Left},
		{Header: "Status", Width: 10, Align: lipgloss.Left},
	}
	rows := [][]string{{"alpha", "active"}, {"beta", "idle"}}

	withoutActive := TableGridWithActiveRow(columns, rows, 40, -1)

	SetTableGridActiveRowsEnabled(false)
	defer SetTableGridActiveRowsEnabled(true)

	withSuppressedActive := TableGridWithActiveRow(columns, rows, 40, 0)
	assert.Equal(t, withoutActive, withSuppressedActive)
}

// TestTableGridWithActiveRowGuardBranches handles width/column guards.
func TestTableGridWithActiveRowGuardBranches(t *testing.T) {
	columns := []TableColumn{{Header: "Name", Width: 8, Align: lipgloss.Left}}
	rows := [][]string{{"alpha"}}

	assert.Equal(t, "", TableGridWithActiveRow(columns, rows, 0, 0))
	assert.Equal(t, "", TableGridWithActiveRow(columns, rows, -4, 0))
	assert.Equal(t, "     ", TableGridWithActiveRow(nil, rows, 5, 0))
}

func TestTableGridWithActiveRowRendersHeaderAndRuleWithNoRows(t *testing.T) {
	columns := []TableColumn{
		{Header: "Name", Width: 12, Align: lipgloss.Left},
		{Header: "Status", Width: 10, Align: lipgloss.Left},
	}

	table := TableGridWithActiveRow(columns, nil, 40, 0)
	lines := strings.Split(table, "\n")
	assert.Len(t, lines, 2)
	for _, line := range lines {
		assert.LessOrEqual(t, lipgloss.Width(line), 40)
	}
}

// TestRenderGridRuleAndCellFallbackBranches covers remaining fallback paths.
func TestRenderGridRuleAndCellFallbackBranches(t *testing.T) {
	cols := []TableColumn{
		{Header: "A", Width: 0, Align: lipgloss.Left},
		{Header: "B", Width: 2, Align: lipgloss.Left},
	}

	rule := renderGridRule(cols, "+", "", 16)
	assert.NotEmpty(t, rule)
	for _, line := range strings.Split(rule, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 16)
	}

	// width<=0 branch in renderGridCell
	assert.Equal(t, "", renderGridCell("abc", 0, lipgloss.Left))

	// no marker branch in highlighter
	assert.Equal(t, "plain text", highlightSelectionMarkers("plain text"))
}

// TestFitGridColumnsExpandAndTinyWidthBranches covers remaining width-fit branches.
func TestFitGridColumnsExpandAndTinyWidthBranches(t *testing.T) {
	cols := []TableColumn{
		{Header: "Name", Width: 8, Align: lipgloss.Left},
		{Header: "Type", Width: 5, Align: lipgloss.Left},
	}

	// sep="" exercises sep width fallback and positive delta expansion.
	fitted := fitGridColumns(cols, "", 40)
	assert.Greater(t, fitted[0].Width+fitted[1].Width, cols[0].Width+cols[1].Width)
	assert.GreaterOrEqual(t, fitted[0].Width, fitted[1].Width)

	// Tiny width forces contentWidth<len(columns) fallback branch.
	tiny := fitGridColumns(cols, "|", 1)
	assert.GreaterOrEqual(t, tiny[0].Width, 1)
	assert.GreaterOrEqual(t, tiny[1].Width, 1)
}

func TestFitGridColumnsDeficitWithEmptyHeaderMinFloor(t *testing.T) {
	cols := []TableColumn{
		{Header: "", Width: 3, Align: lipgloss.Left},
		{Header: "Type", Width: 3, Align: lipgloss.Left},
	}

	// Tight width forces deficit path and exercises headerMin<2 floor branch.
	fitted := fitGridColumns(cols, "|", 4)
	assert.Len(t, fitted, 2)
	assert.GreaterOrEqual(t, fitted[0].Width, 2)
	assert.GreaterOrEqual(t, fitted[1].Width, 2)
}

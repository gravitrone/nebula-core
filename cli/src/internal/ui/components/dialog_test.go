package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

// TestConfirmDialogIncludesTitleMessageAndHints handles test confirm dialog includes title message and hints.
func TestConfirmDialogIncludesTitleMessageAndHints(t *testing.T) {
	out := ConfirmDialog("Confirm", "Are you sure?")
	clean := SanitizeText(out)

	assert.Contains(t, clean, "Confirm")
	assert.Contains(t, clean, "Are you sure?")
	assert.Contains(t, clean, "enter: confirm | esc: cancel")
	assert.Contains(t, clean, "alias")
}

// TestInputDialogIncludesTitleInputAndHints handles test input dialog includes title input and hints.
func TestInputDialogIncludesTitleInputAndHints(t *testing.T) {
	out := InputDialog("Filter", "hello")
	clean := SanitizeText(out)

	assert.Contains(t, clean, "Filter")
	assert.Contains(t, clean, "> hello")
	assert.Contains(t, clean, "enter: submit | esc: cancel")
}

// TestConfirmPreviewDialogIncludesSummaryAndChanges handles test confirm preview dialog includes summary and changes.
func TestConfirmPreviewDialogIncludesSummaryAndChanges(t *testing.T) {
	out := ConfirmPreviewDialog(
		"Archive Entity",
		[]TableRow{{Label: "Entity", Value: "Alpha"}},
		[]DiffRow{{Label: "status", From: "active", To: "archived"}},
		80,
	)
	clean := SanitizeText(out)

	assert.NotContains(t, clean, "Archive Entity")
	assert.Contains(t, clean, "Summary")
	assert.Contains(t, clean, "Entity")
	assert.Contains(t, clean, "Alpha")
	assert.Contains(t, clean, "Changes")
	assert.Contains(t, clean, "status")
	assert.Contains(t, clean, "- active")
	assert.Contains(t, clean, "+ archived")
	assert.Equal(t, 1, strings.Count(clean, "╭"))
	assert.Equal(t, 1, strings.Count(clean, "╮"))
}

func TestRenderSummaryRowsBranchMatrix(t *testing.T) {
	assert.Equal(t, "", renderSummaryRows(nil, 80))

	rows := []TableRow{
		{Label: strings.Repeat("label-", 8), Value: strings.Repeat("value-", 8)},
		{Label: "short", Value: "ok"},
	}

	wide := renderSummaryRows(rows, 80)
	wideClean := SanitizeText(wide)
	assert.Contains(t, wideClean, "label-label")
	assert.Contains(t, wideClean, "short")

	tight := renderSummaryRows(rows, 10)
	tightClean := SanitizeText(tight)
	assert.Contains(t, tightClean, "labe")
	assert.Contains(t, tightClean, "ok")
	assert.NotContains(t, tightClean, "label-label-label-label-label-label-label-label-")
}

func TestConfirmPreviewDialogEmptySectionsMatrix(t *testing.T) {
	out := ConfirmPreviewDialog("Confirm", nil, nil, 72)
	clean := SanitizeText(out)
	assert.NotContains(t, clean, "Confirm")
	assert.NotContains(t, clean, "Summary")
	assert.NotContains(t, clean, "Changes")
}

func TestRenderDiffRowsBranchMatrix(t *testing.T) {
	assert.Equal(t, "", renderDiffRows(nil, 80))

	rows := []DiffRow{
		{Label: "\x1b[31mstatus\x1b[0m", From: "active\nold", To: "archived\nnew"},
		{Label: "owner", From: "  ", To: "--"},
	}
	out := renderDiffRows(rows, 0) // contentWidth fallback branch
	clean := SanitizeText(out)

	assert.Contains(t, clean, "status")
	assert.Contains(t, clean, "owner")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Before")
	assert.Contains(t, clean, "After")
	assert.Contains(t, clean, "- active")
	assert.Contains(t, clean, "+ archived")
	assert.Contains(t, clean, "None")
	assert.GreaterOrEqual(t, strings.Count(clean, "owner"), 1)
}

func TestRenderDiffValuePlaceholderAndMultilineBranches(t *testing.T) {
	style := lipgloss.NewStyle()

	none := SanitizeText(renderDiffValue(style, "  - ", "<nil>", 12))
	assert.Contains(t, none, "- None")

	none = SanitizeText(renderDiffValue(style, "  + ", "-", 12))
	assert.Contains(t, none, "+ None")

	multi := SanitizeText(renderDiffValue(style, "  + ", "line1\nline2", 12))
	assert.Contains(t, multi, "+ line1")
	assert.Contains(t, multi, "\n    line2")
}

func TestRenderSummaryRowsWidthFallbackAndTinyLabelBranch(t *testing.T) {
	rows := []TableRow{
		{Label: "id", Value: "alpha"},
		{Label: "ok", Value: "beta"},
	}

	// width=0 forces BoxContentWidth fallback path and keeps labels <4 chars.
	out := renderSummaryRows(rows, 0)
	clean := SanitizeText(out)

	assert.Contains(t, clean, "id")
	assert.Contains(t, clean, "alpha")
	assert.Contains(t, clean, "ok")
	assert.Contains(t, clean, "beta")
}

func TestRenderDiffRowsMinimumValueWidthBranch(t *testing.T) {
	rows := []DiffRow{
		{
			Label: "status",
			From:  "active with a long suffix",
			To:    "archived with a long suffix",
		},
	}

	// width=10 => content width is positive but tiny, so valueWidth clamps to 8.
	out := renderDiffRows(rows, 10)
	clean := SanitizeText(out)

	assert.Contains(t, clean, "status")
	assert.Contains(t, clean, "- active")
	assert.Contains(t, clean, "+ archive")
}

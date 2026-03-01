package components

import (
	"strings"
	"testing"

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

	assert.Contains(t, clean, "Archive Entity")
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

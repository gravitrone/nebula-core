package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataEditorHandleKeySpaceClampsNegativeCursorToFirstRow(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: "alxx"},
		},
		selected: map[int]bool{},
	}
	// Force cursor below valid range so syncList (called inside HandleKey) clamps it to 0.
	ed.list.SetRows(nil)

	done := ed.HandleKey(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.False(t, done)
	assert.True(t, ed.selected[0])
}

func TestMetadataEditorRenderTableModeInitializesListAndFiltersInvalidSelection(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: "alxx"},
		},
		selected: map[int]bool{
			0: false,
			9: true,
		},
		Scopes: []string{"public"},
		notice: "heads up",
	}

	out := components.SanitizeText(ed.renderTableMode(80))
	assert.Contains(t, out, "Rows 1-1 of 1")
	assert.Contains(t, out, "heads up")
	assert.NotContains(t, out, "selected 1")
}

func TestMetadataEditorRenderTableModeSkipsStaleVisibleRows(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: "alxx"},
		},
		selected: map[int]bool{},
	}

	out := components.SanitizeText(ed.renderTableMode(80))
	assert.Contains(t, out, "Rows 1-1 of 1")
	assert.Contains(t, out, "owner")
}

func TestMetadataEditorRenderTableModeClampsNegativeStartIndex(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: "alxx"},
		},
		selected: map[int]bool{},
	}

	out := components.SanitizeText(ed.renderTableMode(80))
	assert.Contains(t, out, "Rows 1-")
}

func TestMetadataEditorRenderInspectModeClampsOffsets(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "profile.note", value: "line one"},
		},
		inspectMode:   true,
		inspectRowIdx: 0,
		inspectOffset: -10,
	}

	out := components.SanitizeText(ed.renderInspectMode(80))
	assert.Contains(t, out, "Lines 1-")

	ed.inspectOffset = 999
	out = components.SanitizeText(ed.renderInspectMode(80))
	assert.Contains(t, out, "Lines")
}

func TestMetadataEditorToggleSelectionAndSelectAllGuardBranches(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: "alxx"},
		},
	}

	ed.toggleSelection(0)
	require.NotNil(t, ed.selected)
	assert.True(t, ed.selected[0])

	empty := MetadataEditor{}
	empty.toggleSelectAll()
	assert.Nil(t, empty.selected)
}

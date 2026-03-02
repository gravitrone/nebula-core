package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataEditorHandleKeySpaceClampsNegativeCursorToFirstRow(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: "alxx"},
		},
		list:     components.NewList(metadataPanelPageSize(false)),
		selected: map[int]bool{},
	}
	syncMetadataList(ed.list, ed.toDisplayRows(), metadataPanelPageSize(false))
	ed.list.Cursor = -1

	done := ed.HandleKey(tea.KeyMsg{Type: tea.KeySpace})
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
		list: &components.List{
			Items:    []string{"owner", "stale"},
			PageSize: metadataPanelPageSize(false),
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
		list: &components.List{
			Offset:   -1,
			PageSize: metadataPanelPageSize(false),
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

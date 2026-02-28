package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitiesDetailMetadataPanelHidesSelectionColumnUntilAnyRowSelected handles test entities detail metadata panel hides selection column until any row selected.
func TestEntitiesDetailMetadataPanelHidesSelectionColumnUntilAnyRowSelected(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.view = entitiesViewDetail
	model.metaExpanded = true
	model.detail = &api.Entity{
		ID:     "ent-1",
		Name:   "Alpha",
		Type:   "person",
		Status: "active",
		Metadata: api.JSONMap{
			"note":  "hello",
			"owner": "alxx",
			"context_segments": []any{
				map[string]any{"text": "first"},
			},
		},
	}
	model.syncDetailMetadataRows()

	out := model.renderDetail()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Metadata")
	assert.NotContains(t, clean, "Sel")
	assert.Contains(t, clean, "Group")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, strings.ToLower(clean), "segment 1")
	assert.NotContains(t, clean, "[ ]")
	assert.NotContains(t, clean, ">[")
	assert.Contains(t, clean, "enter inspect")
	assert.Contains(t, clean, "copy row")
	assert.Contains(t, clean, "mode row")
}

// TestEntitiesDetailMetadataPanelHidesSelectorsWhenNotInSelectMode handles test entities detail metadata panel hides selectors when not in select mode.
func TestEntitiesDetailMetadataPanelHidesSelectorsWhenNotInSelectMode(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.view = entitiesViewDetail
	model.metaExpanded = true
	model.detail = &api.Entity{
		ID:     "ent-1",
		Name:   "Alpha",
		Type:   "person",
		Status: "active",
		Metadata: api.JSONMap{
			"note": "hello",
		},
	}
	model.syncDetailMetadataRows()
	model.metaSelected[0] = true
	model.metaSelectMode = false

	out := model.renderDetail()
	clean := components.SanitizeText(out)

	assert.NotContains(t, clean, "Sel")
	assert.NotContains(t, clean, "[X]")
}

// TestEntitiesDetailMetadataPanelSelectModeHintsFollowSelection handles select-mode hint copy target text.
func TestEntitiesDetailMetadataPanelSelectModeHintsFollowSelection(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.view = entitiesViewDetail
	model.metaExpanded = true
	model.detail = &api.Entity{
		ID:     "ent-1",
		Name:   "Alpha",
		Type:   "person",
		Status: "active",
		Metadata: api.JSONMap{
			"note":  "hello",
			"owner": "alxx",
		},
	}
	model.syncDetailMetadataRows()
	model.metaSelectMode = true
	model.metaSelected[0] = true

	out := model.renderDetail()
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "Sel")
	assert.Contains(t, clean, "[X]")
	assert.Contains(t, clean, "copy selected")
	assert.Contains(t, clean, "mode select")
}

// TestEntitiesDetailMetadataCopyCurrentRow handles test entities detail metadata copy current row.
func TestEntitiesDetailMetadataCopyCurrentRow(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.view = entitiesViewDetail
	model.metaExpanded = true
	model.detail = &api.Entity{
		ID:       "ent-1",
		Name:     "Alpha",
		Metadata: api.JSONMap{"note": "hello"},
	}
	model.syncDetailMetadataRows()

	prevCopy := copyEntityMetadataClipboard
	defer func() { copyEntityMetadataClipboard = prevCopy }()
	copied := ""
	copyEntityMetadataClipboard = func(text string) error {
		copied = text
		return nil
	}

	next, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	require.True(t, next.metaInspect)

	next, cmd = next.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	copiedMsg, ok := msg.(entityMetadataCopiedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, copiedMsg.count)
	assert.Equal(t, "hello", strings.TrimSpace(copied))
	assert.Equal(t, entitiesViewDetail, next.view)
}

// TestEntitiesDetailMetadataInspectEscReturnsToTable handles test entities detail metadata inspect esc returns to table.
func TestEntitiesDetailMetadataInspectEscReturnsToTable(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.view = entitiesViewDetail
	model.metaExpanded = true
	model.detail = &api.Entity{
		ID:       "ent-1",
		Name:     "Alpha",
		Metadata: api.JSONMap{"note": "hello"},
	}
	model.syncDetailMetadataRows()

	next, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	require.True(t, next.metaInspect)

	next, cmd = next.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	require.False(t, next.metaInspect)
}

// TestEntitiesDetailMetadataMultiSelectCopy handles test entities detail metadata multi select copy.
func TestEntitiesDetailMetadataMultiSelectCopy(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.view = entitiesViewDetail
	model.metaExpanded = true
	model.detail = &api.Entity{
		ID:   "ent-1",
		Name: "Alpha",
		Metadata: api.JSONMap{
			"note":  "hello",
			"owner": "alxx",
		},
	}
	model.syncDetailMetadataRows()

	prevCopy := copyEntityMetadataClipboard
	defer func() { copyEntityMetadataClipboard = prevCopy }()
	copied := ""
	copyEntityMetadataClipboard = func(text string) error {
		copied = text
		return nil
	}

	var cmd tea.Cmd
	model, _ = model.handleDetailKeys(tea.KeyMsg{Type: tea.KeySpace})
	model, _ = model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.handleDetailKeys(tea.KeyMsg{Type: tea.KeySpace})
	model, cmd = model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	require.NotNil(t, cmd)
	msg := cmd()
	copiedMsg, ok := msg.(entityMetadataCopiedMsg)
	require.True(t, ok)
	assert.Equal(t, 2, copiedMsg.count)

	lines := strings.Split(strings.TrimSpace(copied), "\n")
	require.Len(t, lines, 2)
	assert.NotEmpty(t, strings.TrimSpace(lines[0]))
	assert.NotEmpty(t, strings.TrimSpace(lines[1]))
}

// TestEntitiesDetailMetadataInspectScrollClamp ensures inspect scrolling stays
// bounded and renders scroll affordances for long values.
func TestEntitiesDetailMetadataInspectScrollClamp(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.height = 24
	model.view = entitiesViewDetail
	model.metaExpanded = true
	model.detail = &api.Entity{
		ID:   "ent-1",
		Name: "Alpha",
		Metadata: api.JSONMap{
			"note": strings.Join([]string{
				"line 01", "line 02", "line 03", "line 04", "line 05",
				"line 06", "line 07", "line 08", "line 09", "line 10",
				"line 11", "line 12", "line 13", "line 14", "line 15",
			}, "\n"),
		},
	}
	model.syncDetailMetadataRows()
	model.openMetaInspect(0)

	model.moveMetaInspect(999)
	lines := model.metaInspectLines()
	page := model.metaInspectPageSize()
	maxOffset := len(lines) - page
	if maxOffset < 0 {
		maxOffset = 0
	}
	assert.Equal(t, maxOffset, model.metaInspectO)

	rendered := components.SanitizeText(model.renderMetaInspect())
	assert.Contains(t, rendered, "Metadata Value")
	assert.Contains(t, rendered, "Lines")
	assert.Contains(t, rendered, "... ↑ more")
}

// TestEntitiesDetailMetadataCopyCurrentRowWithoutList ensures copy command is
// skipped when metadata list state is unavailable.
func TestEntitiesDetailMetadataCopyCurrentRowWithoutList(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.metaRows = []metadataDisplayRow{{field: "note", value: "hello"}}
	model.metaList = nil
	assert.Nil(t, model.copyCurrentMetadataRow())
}

// TestEntitiesDetailMetadataCopyRowsNormalizesEmptyValue ensures empty metadata
// values copy as explicit None.
func TestEntitiesDetailMetadataCopyRowsNormalizesEmptyValue(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.metaRows = []metadataDisplayRow{{field: "empty", value: "   "}}

	prevCopy := copyEntityMetadataClipboard
	defer func() { copyEntityMetadataClipboard = prevCopy }()
	copied := ""
	copyEntityMetadataClipboard = func(text string) error {
		copied = text
		return nil
	}

	cmd := model.copyMetadataRows([]int{0})
	require.NotNil(t, cmd)
	msg := cmd()
	copiedMsg, ok := msg.(entityMetadataCopiedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, copiedMsg.count)
	assert.Equal(t, "None", copied)
}

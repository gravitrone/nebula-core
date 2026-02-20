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

func TestEntitiesDetailMetadataPanelShowsSelectionColumns(t *testing.T) {
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
	assert.Contains(t, clean, "Sel")
	assert.Contains(t, clean, "Group")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, strings.ToLower(clean), "segment 1")
	assert.NotContains(t, clean, ">[")
	assert.Contains(t, clean, "enter inspect")
}

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

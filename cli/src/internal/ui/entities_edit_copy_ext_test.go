package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesHandleEditKeysBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)

	t.Run("editSaving short-circuits", func(t *testing.T) {
		model.editSaving = true
		model.editFocus = editFieldStatus
		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		require.Nil(t, cmd)
		assert.Equal(t, editFieldStatus, updated.editFocus)
	})

	t.Run("scope selector branches", func(t *testing.T) {
		model.editSaving = false
		model.editFocus = editFieldScopes
		model.editScopeSelecting = true
		model.scopeOptions = []string{"public", "private"}
		model.editScopes = []string{"public"}
		model.editScopeIdx = 0

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		require.Nil(t, cmd)
		assert.Equal(t, 1, updated.editScopeIdx)

		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		assert.Equal(t, []string{"public", "private"}, updated.editScopes)
		assert.True(t, updated.editScopesDirty)

		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, 0, updated.editScopeIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.editScopeSelecting)

		updated.editScopeSelecting = true
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.False(t, updated.editScopeSelecting)
	})

	t.Run("navigation, status, tags branches", func(t *testing.T) {
		model.editFocus = editFieldTags
		model.editTagBuf = "ab"

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		require.Nil(t, cmd)
		assert.Equal(t, "a", updated.editTagBuf)

		updated.editTagBuf = ""
		updated.editTags = []string{"alpha", "beta"}
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, []string{"alpha"}, updated.editTags)

		updated.editFocus = editFieldScopes
		updated.editScopes = []string{"public"}
		updated.editScopesDirty = false
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Empty(t, updated.editScopes)
		assert.True(t, updated.editScopesDirty)

		updated.editFocus = editFieldTags
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		assert.Equal(t, "x", updated.editTagBuf)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Equal(t, []string{"alpha", "x"}, updated.editTags)
		assert.Equal(t, "", updated.editTagBuf)

		updated.editFocus = editFieldScopes
		updated.editScopeSelecting = false
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.True(t, updated.editScopeSelecting)

		updated.editFocus = editFieldStatus
		startStatus := updated.editStatusIdx
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Equal(t, (startStatus+1)%len(entityStatusOptions), updated.editStatusIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, startStatus, updated.editStatusIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		assert.Equal(t, (startStatus+1)%len(entityStatusOptions), updated.editStatusIdx)

		updated.editFocus = editFieldStatus
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, editFieldScopes, updated.editFocus)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, editFieldStatus, updated.editFocus)

		updated.editScopeSelecting = true
		updated.view = entitiesViewEdit
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Equal(t, entitiesViewDetail, updated.view)
		assert.False(t, updated.editScopeSelecting)
	})
}

func TestEntitiesCompactJSONAndRelationshipDirectionBranches(t *testing.T) {
	assert.Equal(t, "", compactJSON(map[string]any{}))
	assert.Equal(t, "", compactJSON(nil))
	assert.Equal(t, `{"a":1}`, compactJSON(map[string]any{"a": 1}))

	model := NewEntitiesModel(nil)
	rel := api.Relationship{
		SourceID:   "ent-1",
		SourceName: "Alpha",
		TargetID:   "ent-2",
		TargetName: "Beta",
	}

	direction, other := model.relationshipDirection(rel)
	assert.Equal(t, "", direction)
	assert.Equal(t, "Beta", other)

	model.detail = &api.Entity{ID: "ent-1"}
	direction, other = model.relationshipDirection(rel)
	assert.Equal(t, "outgoing", direction)
	assert.Equal(t, "Beta", other)

	model.detail = &api.Entity{ID: "ent-3"}
	direction, other = model.relationshipDirection(rel)
	assert.Equal(t, "incoming", direction)
	assert.Equal(t, "Alpha", other)
}

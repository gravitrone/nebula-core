package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesHandleAddKeysBranchMatrix(t *testing.T) {
	t.Run("saving and saved short-circuits", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.addSaving = true
		next, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		assert.Nil(t, cmd)
		assert.True(t, next.addSaving)

		model = NewEntitiesModel(nil)
		model.addSaved = true
		model.addFields[addFieldName].value = "keep"
		next, cmd = model.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Nil(t, cmd)
		assert.False(t, next.addSaved)
		assert.Equal(t, "", next.addFields[addFieldName].value)
	})

	t.Run("mode focus delegates to mode handler", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.view = entitiesViewAdd
		model.modeFocus = true
		next, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewList, next.view)
		assert.False(t, next.modeFocus)
	})

	t.Run("status and scope selectors cycle and toggle", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.addFocus = addFieldStatus
		model.addStatusIdx = 0

		next, _ := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, len(entityStatusOptions)-1, next.addStatusIdx)

		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Equal(t, 0, next.addStatusIdx)

		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.Equal(t, 1, next.addStatusIdx)

		next.addFocus = addFieldScopes
		next.scopeOptions = []string{"public", "private"}
		next.addScopeSelecting = true
		next.addScopeIdx = 0

		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, 1, next.addScopeIdx)

		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Equal(t, 0, next.addScopeIdx)

		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.Equal(t, []string{"public"}, next.addScopes)

		next.scopeOptions = nil
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, 0, next.addScopeIdx)

		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, next.addScopeSelecting)
	})

	t.Run("navigation, save, delete and text input branches", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.scopeOptions = []string{"public"}

		// Up from first field enters mode focus.
		model.addFocus = 0
		next, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Nil(t, cmd)
		assert.True(t, next.modeFocus)

		// Up from non-first field moves focus to the previous field.
		next.modeFocus = false
		next.addFocus = addFieldType
		next, cmd = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Nil(t, cmd)
		assert.Equal(t, addFieldName, next.addFocus)

		// Ctrl+S runs save validation path.
		next.modeFocus = false
		next.addFocus = addFieldName
		next, cmd = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
		assert.Nil(t, cmd)
		assert.Equal(t, "Name is required", next.errText)

		// Tag input branches.
		next.addFocus = addFieldTags
		next.addTagBuf = "ab"
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, "a", next.addTagBuf)
		next.addTagBuf = ""
		next.addTags = []string{"alpha", "beta"}
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, []string{"alpha"}, next.addTags)
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
		assert.Equal(t, "z", next.addTagBuf)
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Equal(t, []string{"alpha", "z"}, next.addTags)
		assert.Equal(t, "", next.addTagBuf)

		// Scope delete and scope-select activation.
		next.addFocus = addFieldScopes
		next.addScopes = []string{"public"}
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Empty(t, next.addScopes)
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.True(t, next.addScopeSelecting)

		// Default field input/delete branches.
		next.addFocus = addFieldType
		next.addFields[addFieldType].value = "pers"
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, "per", next.addFields[addFieldType].value)
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		assert.Equal(t, "pers", next.addFields[addFieldType].value)

		// Esc resets the whole form.
		next.addFields[addFieldName].value = "Alpha"
		next, _ = next.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Equal(t, "", next.addFields[addFieldName].value)
		assert.Equal(t, 0, next.addFocus)
	})
}

func TestEntitiesHandleDetailKeysContextPromptsAndShortcuts(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewDetail
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	model.width = 90

	// link prompt
	linked, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.Nil(t, cmd)
	assert.True(t, linked.contextLinking)
	linked, _ = linked.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	assert.Equal(t, "c", linked.contextLinkBuf)
	linked, _ = linked.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, linked.contextLinking)
	assert.Equal(t, "", linked.contextLinkBuf)

	// create prompt
	created, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.True(t, created.contextCreating)
	created, _ = created.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	assert.Equal(t, "N", created.contextCreateBuf)
	created, _ = created.handleDetailKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "", created.contextCreateBuf)
	created, _ = created.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, created.contextCreating)
	assert.Equal(t, "", created.contextCreateBuf)

	// shortcuts
	shortcuts, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewEdit, shortcuts.view)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewRelationships, shortcuts.view)
	assert.True(t, shortcuts.relLoading)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewHistory, shortcuts.view)
	assert.True(t, shortcuts.historyLoading)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewConfirm, shortcuts.view)
	assert.Equal(t, "entity-archive", shortcuts.confirmKind)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewList, shortcuts.view)
	assert.Nil(t, shortcuts.detail)
}

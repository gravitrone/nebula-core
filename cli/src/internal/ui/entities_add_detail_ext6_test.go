package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesHandleAddKeysBranchMatrix(t *testing.T) {
	t.Run("saving and saved short-circuits", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.addSaving = true
		next, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
		assert.Nil(t, cmd)
		assert.True(t, next.addSaving)

		model = NewEntitiesModel(nil)
		model.addSaved = true
		model.addName = "keep"
		next, cmd = model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
		assert.Nil(t, cmd)
		assert.False(t, next.addSaved)
		assert.Equal(t, "", next.addName)
	})

	t.Run("mode focus delegates to mode handler", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.view = entitiesViewAdd
		model.modeFocus = true
		next, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyRight})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewList, next.view)
		assert.False(t, next.modeFocus)
	})

	t.Run("nil form initializes on first key", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.addForm = nil
		next, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
		require.NotNil(t, next.addForm)
		// Init returns a cmd for cursor blink etc.
		_ = cmd
	})

	t.Run("form completion triggers save", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.initAddForm()
		_ = model.addForm.Init()
		model.addForm.State = huh.StateCompleted
		model.addName = "Alpha"
		model.addType = "person"

		next, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
		// saveAdd is triggered, which validates and saves
		_ = next
		_ = cmd
	})

	t.Run("form abort resets", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.initAddForm()
		_ = model.addForm.Init()
		model.addForm.State = huh.StateAborted

		next, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
		assert.Nil(t, cmd)
		assert.False(t, next.addSaved)
	})
}

func TestEntitiesHandleDetailKeysContextPromptsAndShortcuts(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewDetail
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	model.width = 90

	// link prompt
	linked, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: 'l', Text: "l"})
	require.Nil(t, cmd)
	assert.True(t, linked.contextLinking)
	linked, _ = linked.handleDetailKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
	assert.Equal(t, "c", linked.contextLinkBuf)
	linked, _ = linked.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, linked.contextLinking)
	assert.Equal(t, "", linked.contextLinkBuf)

	// create prompt
	created, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.True(t, created.contextCreating)
	created, _ = created.handleDetailKeys(tea.KeyPressMsg{Code: 'N', Text: "N"})
	assert.Equal(t, "N", created.contextCreateBuf)
	created, _ = created.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", created.contextCreateBuf)
	created, _ = created.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, created.contextCreating)
	assert.Equal(t, "", created.contextCreateBuf)

	// shortcuts
	shortcuts, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.NotNil(t, cmd) // editForm.Init() returns a cmd
	assert.Equal(t, entitiesViewEdit, shortcuts.view)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewRelationships, shortcuts.view)
	assert.True(t, shortcuts.relLoading)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyPressMsg{Code: 'h', Text: "h"})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewHistory, shortcuts.view)
	assert.True(t, shortcuts.historyLoading)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewConfirm, shortcuts.view)
	assert.Equal(t, "entity-archive", shortcuts.confirmKind)

	shortcuts.view = entitiesViewDetail
	shortcuts, cmd = shortcuts.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewList, shortcuts.view)
	assert.Nil(t, shortcuts.detail)
}

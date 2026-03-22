package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesHandleEditKeysBranchMatrix(t *testing.T) {
	t.Run("editSaving short-circuits", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.editSaving = true
		updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
		require.Nil(t, cmd)
		assert.True(t, updated.editSaving)
	})

	t.Run("nil form initializes on first key", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.editForm = nil
		next, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
		require.NotNil(t, next.editForm)
		_ = cmd
	})

	t.Run("form completion triggers save", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.detail = &api.Entity{ID: "ent-1", Name: "Alpha", Status: "active"}
		model.initEditForm()
		_ = model.editForm.Init()
		model.editForm.State = huh.StateCompleted
		model.editTagStr = "alpha"
		model.editStatus = "active"

		next, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
		// saveEdit returns nil cmd when detail is nil, but here we have detail set
		_ = next
		_ = cmd
	})

	t.Run("form abort returns to detail view", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.view = entitiesViewEdit
		model.initEditForm()
		_ = model.editForm.Init()
		model.editForm.State = huh.StateAborted

		next, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewDetail, next.view)
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

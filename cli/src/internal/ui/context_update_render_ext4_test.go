package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextUpdateMessageMatrix(t *testing.T) {
	model := NewContextModel(nil)

	model.saving = true
	updated, cmd := model.Update(contextSavedMsg{})
	require.Nil(t, cmd)
	assert.False(t, updated.saving)
	assert.True(t, updated.saved)

	model = NewContextModel(nil)
	model.saving = true
	model.editSaving = true
	updated, cmd = model.Update(errMsg{err: errors.New("ctx boom")})
	require.Nil(t, cmd)
	assert.False(t, updated.saving)
	assert.False(t, updated.editSaving)
	assert.Equal(t, "ctx boom", updated.errText)

	model = NewContextModel(nil)
	model.linkLoading = true
	updated, cmd = model.Update(contextLinkResultsMsg{
		items: []api.Entity{
			{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active"},
			{ID: "ent-2", Name: "Beta", Type: "tool", Status: "inactive"},
		},
	})
	require.Nil(t, cmd)
	assert.False(t, updated.linkLoading)
	require.Len(t, updated.linkResults, 2)
	assert.Equal(t, 2, len(updated.linkTable.Rows()))

	model = NewContextModel(nil)
	model.loadingList = true
	updated, cmd = model.Update(contextListLoadedMsg{
		items: []api.Context{
			{ID: "ctx-1", Title: "Alpha", SourceType: "note", Status: "active"},
			{ID: "ctx-2", Title: "Beta", SourceType: "url", Status: "inactive"},
		},
	})
	require.Nil(t, cmd)
	assert.False(t, updated.loadingList)
	require.Len(t, updated.allItems, 2)
	require.Len(t, updated.items, 2)

	model = NewContextModel(nil)
	model.scopeNames = nil // hit nil-map init branch
	updated, cmd = model.Update(contextScopesLoadedMsg{
		names: map[string]string{
			"scope-1": "public",
			"scope-2": "private",
		},
	})
	require.Nil(t, cmd)
	assert.Equal(t, "public", updated.scopeNames["scope-1"])
	assert.Equal(t, "private", updated.scopeNames["scope-2"])
	assert.Contains(t, updated.scopeOptions, "public")
	assert.Contains(t, updated.scopeOptions, "private")

	model = NewContextModel(nil)
	updated, cmd = model.Update(contextDetailLoadedMsg{
		item:          api.Context{ID: "ctx-1", Title: "Alpha"},
		relationships: []api.Relationship{{ID: "rel-1"}},
	})
	require.Nil(t, cmd)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "ctx-1", updated.detail.ID)
	require.Len(t, updated.detailRelationships, 1)

	model = NewContextModel(nil)
	model.editSaving = true
	model.view = contextViewEdit
	updated, cmd = model.Update(contextUpdatedMsg{item: api.Context{ID: "ctx-9", Title: "Updated"}})
	require.Nil(t, cmd)
	assert.False(t, updated.editSaving)
	assert.Equal(t, contextViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "ctx-9", updated.detail.ID)
}

func TestContextViewEarlyReturnBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 90

	model.saving = true
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Saving...")

	model.saving = false
	model.saved = true
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Context saved!")

	model.saved = false
	model.linkSearching = true
	model.linkQuery = "alpha"
	model.linkResults = nil
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "No matches")

	model.linkSearching = false
	model.filtering = true
	model.view = contextViewList
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Filter Context")
}

func TestContextRenderAddBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 92

	// With nil addForm, renders "Initializing..." placeholder.
	model.addForm = nil
	out := components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Initializing")

	// With linked entities, entity names are shown (requires addForm to be non-nil).
	model.initAddForm()
	model.linkEntities = []api.Entity{{ID: "ent-1", Name: "Entity One"}}
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Entity One")

	// With errText, error message is rendered.
	model.errText = "add render error"
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "add render error")
}

func TestContextRenderEditBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 92

	// editSaving shows saving indicator.
	model.editSaving = true
	out := components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Saving...")

	// nil editForm without editSaving shows initializing.
	model.editSaving = false
	model.editForm = nil
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Initializing")

	// errText is rendered when editForm is initialized.
	model.detail = &api.Context{ID: "ctx-1", Title: "Alpha", SourceType: "note", Status: "active"}
	model.startEdit()
	model.errText = "edit render error"
	out = components.SanitizeText(model.renderEdit())
	assert.True(t, strings.Contains(out, "edit render error"))
}

func TestContextUpdateKeyBranchMatrixAdditional(t *testing.T) {
	// Mode toggle from add goes to list and emits a load command.
	model := NewContextModel(nil)
	model.view = contextViewAdd
	var cmd tea.Cmd
	model, cmd = model.toggleMode()
	assert.NotNil(t, cmd)
	assert.Equal(t, contextViewList, model.view)
	assert.True(t, model.loadingList)

	// Empty title in save() emits errText.
	model2 := NewContextModel(nil)
	model2.addTitle = ""
	model2, _ = model2.save()
	assert.Equal(t, "Title is required", model2.errText)

	// Keys in add view with addForm nil init the form and return non-nil cmd.
	model3 := NewContextModel(nil)
	model3.view = contextViewAdd
	model3.addForm = nil
	_, cmd = model3.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.NotNil(t, cmd)
}

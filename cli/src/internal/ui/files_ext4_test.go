package ui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesRenderModeLineAddAndFocusStates(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd

	line := components.SanitizeText(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")

	model.modeFocus = true
	line = components.SanitizeText(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")

	model.view = filesViewList
	line = components.SanitizeText(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")
}

func TestFilesHandleModeKeysCoversUpDownSpaceEnterAndBack(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewList
	model.modeFocus = true

	updated, cmd := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, filesViewList, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, filesViewAdd, updated.view)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, filesViewList, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
}

func TestFilesViewCoversMetadataAndFilterBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.width = 80
	model.view = filesViewList

	model.addMeta.Active = true
	assert.Contains(t, components.SanitizeText(model.View()), "No metadata rows.")
	model.addMeta.Active = false

	model.editMeta.Active = true
	assert.Contains(t, components.SanitizeText(model.View()), "No metadata rows.")
	model.editMeta.Active = false

	model.filtering = true
	model.searchInput.SetValue("mime:text")
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Filter Files")
	assert.Contains(t, out, "mime:text")
}

func TestFilesUpdateMessageBranchesAndRelationshipMismatchNoop(t *testing.T) {
	model := NewFilesModel(nil)
	model.detail = &api.File{ID: "file-1"}
	model.detailRels = []api.Relationship{{ID: "rel-old"}}
	model.addSaving = true
	model.editSaving = true
	model.loading = true

	updated, cmd := model.Update(fileRelationshipsLoadedMsg{
		id:            "file-other",
		relationships: []api.Relationship{{ID: "rel-new"}},
	})
	require.Nil(t, cmd)
	assert.Equal(t, []api.Relationship{{ID: "rel-old"}}, updated.detailRels)

	updated, cmd = updated.Update(fileRelationshipsLoadedMsg{
		id:            "file-1",
		relationships: []api.Relationship{{ID: "rel-new"}},
	})
	require.Nil(t, cmd)
	assert.Equal(t, []api.Relationship{{ID: "rel-new"}}, updated.detailRels)

	updated, cmd = updated.Update(filesScopesLoadedMsg{options: []string{"public", "private"}})
	require.Nil(t, cmd)
	assert.Equal(t, []string{"public", "private"}, updated.scopeOptions)
	assert.Equal(t, []string{"public", "private"}, updated.addMeta.scopeOptions)
	assert.Equal(t, []string{"public", "private"}, updated.editMeta.scopeOptions)

	updated, cmd = updated.Update(errMsg{err: errors.New("boom")})
	require.Nil(t, cmd)
	assert.False(t, updated.loading)
	assert.False(t, updated.addSaving)
	assert.False(t, updated.editSaving)
	assert.Equal(t, "boom", updated.errText)
}

func TestFilesUpdateCreatedAndUpdatedMessagesTriggerReload(t *testing.T) {
	model := NewFilesModel(nil)
	model.addSaving = true

	updated, cmd := model.Update(fileCreatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, updated.addSaving)
	assert.True(t, updated.addSaved)
	assert.True(t, updated.loading)

	updated.editSaving = true
	updated.view = filesViewDetail
	updated.detail = &api.File{ID: "file-1", CreatedAt: time.Now()}
	updated, cmd = updated.Update(fileUpdatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, updated.editSaving)
	assert.Nil(t, updated.detail)
	assert.Equal(t, filesViewList, updated.view)
	assert.True(t, updated.loading)
}

func TestFilesUpdateRoutesKeysToActiveMetadataEditors(t *testing.T) {
	model := NewFilesModel(nil)
	model.addMeta.Active = true

	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.True(t, updated.addMeta.entryMode)

	model = NewFilesModel(nil)
	model.editMeta.Active = true
	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.True(t, updated.editMeta.entryMode)
}

func TestFilesHandleFilterInputCoversSpaceBackspaceEnterAndBack(t *testing.T) {
	model := NewFilesModel(nil)
	model.filtering = true
	model.all = []api.File{
		{ID: "file-1", Filename: "workout.txt", Status: "active"},
		{ID: "file-2", Filename: "study.txt", Status: "active"},
	}
	model.applyFileSearch()

	updated, cmd := model.handleFilterInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchInput.Value())

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: 'w', Text: "w"})
	assert.Equal(t, "w", updated.searchInput.Value())

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.searchInput.Value())

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, updated.filtering)

	updated.filtering = true
	updated.searchInput.SetValue("x")
	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.searchInput.Value())
	assert.Equal(t, "", updated.searchSuggest)
}

func TestFilesHandleListKeysSearchModeAndEnterMatrix(t *testing.T) {
	model := NewFilesModel(nil)
	model.all = []api.File{
		{ID: "file-1", Filename: "workout.txt", Status: "active", CreatedAt: time.Now()},
		{ID: "file-2", Filename: "study.txt", Status: "active", CreatedAt: time.Now()},
	}
	model.applyFileSearch()

	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated.modeFocus = false
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.list.Selected())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, updated.list.Selected())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: 'w', Text: "w"})
	assert.Equal(t, "w", updated.searchInput.Value())
	assert.Equal(t, "workout.txt", updated.searchSuggest)

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, "workout.txt", updated.searchInput.Value())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "workout.tx", updated.searchInput.Value())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	assert.Equal(t, "", updated.searchInput.Value())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: 'f', Text: "f"})
	assert.True(t, updated.filtering)

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.searchInput.Value())

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, filesViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "file-1", updated.detail.ID)
}

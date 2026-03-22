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

func TestLogsRenderModeLineAddAndFocusStates(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewAdd

	line := components.SanitizeText(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")

	model.modeFocus = true
	line = components.SanitizeText(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")

	model.view = logsViewList
	line = components.SanitizeText(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")
}

func TestLogsHandleModeKeysCoversUpDownSpaceEnterAndBack(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewList
	model.modeFocus = true

	updated, cmd := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, logsViewList, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewAdd, updated.view)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
}

func TestLogsViewCoversEditorAndFilterBranches(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 80
	model.view = logsViewList

	model.addValue.Active = true
	assert.Contains(t, components.SanitizeText(model.View()), "No metadata rows.")
	model.addValue.Active = false

	model.addMeta.Active = true
	assert.Contains(t, components.SanitizeText(model.View()), "No metadata rows.")
	model.addMeta.Active = false

	model.editValue.Active = true
	assert.Contains(t, components.SanitizeText(model.View()), "No metadata rows.")
	model.editValue.Active = false

	model.editMeta.Active = true
	assert.Contains(t, components.SanitizeText(model.View()), "No metadata rows.")
	model.editMeta.Active = false

	model.filtering = true
	model.searchInput.SetValue("type:workout")
	assert.Contains(t, components.SanitizeText(model.View()), "Filter Logs")
	assert.Contains(t, components.SanitizeText(model.View()), "type:workout")

	model.filtering = false
	model.view = logsViewAdd
	addView := components.SanitizeText(model.View())
	assert.Contains(t, addView, "Type:")
	assert.Contains(t, addView, "Timestamp:")

	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "workout", Status: "active", Timestamp: time.Now().UTC()}
	model.startEdit()
	editView := components.SanitizeText(model.View())
	assert.Contains(t, editView, "Status:")
	assert.Contains(t, editView, "Metadata:")
}

func TestLogsUpdateMessageBranchesAndNoopRelationshipMismatch(t *testing.T) {
	model := NewLogsModel(nil)
	model.detail = &api.Log{ID: "log-1"}
	model.detailRels = []api.Relationship{{ID: "rel-old"}}
	model.addSaving = true
	model.editSaving = true
	model.loading = true

	updated, cmd := model.Update(logRelationshipsLoadedMsg{
		id:            "log-other",
		relationships: []api.Relationship{{ID: "rel-new"}},
	})
	require.Nil(t, cmd)
	assert.Equal(t, []api.Relationship{{ID: "rel-old"}}, updated.detailRels)

	updated, cmd = updated.Update(logRelationshipsLoadedMsg{
		id:            "log-1",
		relationships: []api.Relationship{{ID: "rel-new"}},
	})
	require.Nil(t, cmd)
	assert.Equal(t, []api.Relationship{{ID: "rel-new"}}, updated.detailRels)

	updated, cmd = updated.Update(logsScopesLoadedMsg{options: []string{"public", "private"}})
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

func TestLogsUpdateCreatedAndUpdatedMessagesTriggerReload(t *testing.T) {
	model := NewLogsModel(nil)
	model.addSaving = true

	updated, cmd := model.Update(logCreatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, updated.addSaving)
	assert.True(t, updated.addSaved)
	assert.True(t, updated.loading)

	updated.editSaving = true
	updated.view = logsViewDetail
	updated.detail = &api.Log{ID: "log-1", Timestamp: time.Now()}
	updated, cmd = updated.Update(logUpdatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, updated.editSaving)
	assert.Nil(t, updated.detail)
	assert.Equal(t, logsViewList, updated.view)
	assert.True(t, updated.loading)
}

func TestLogsUpdateRoutesKeysToActiveMetadataEditors(t *testing.T) {
	model := NewLogsModel(nil)

	model.addValue.Active = true
	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.True(t, updated.addValue.entryMode)

	model = NewLogsModel(nil)
	model.addMeta.Active = true
	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.True(t, updated.addMeta.entryMode)

	model = NewLogsModel(nil)
	model.editValue.Active = true
	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.True(t, updated.editValue.entryMode)

	model = NewLogsModel(nil)
	model.editMeta.Active = true
	updated, cmd = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.True(t, updated.editMeta.entryMode)
}

func TestLogsHandleFilterInputCoversSpaceBackspaceEnterAndBack(t *testing.T) {
	model := NewLogsModel(nil)
	model.filtering = true
	model.allItems = []api.Log{
		{ID: "log-1", LogType: "workout", Status: "active"},
		{ID: "log-2", LogType: "study", Status: "active"},
	}
	model.applyLogSearch()

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

func TestLogsCommitAddTagAndRenderAddTagsMatrix(t *testing.T) {
	model := NewLogsModel(nil)

	model.addTagInput.SetValue("   ")
	model.commitAddTag()
	assert.Empty(t, model.addTags)
	assert.Equal(t, "", model.addTagInput.Value())

	model.addTagInput.SetValue("#")
	model.commitAddTag()
	assert.Empty(t, model.addTags)
	assert.Equal(t, "", model.addTagInput.Value())

	model.addTagInput.SetValue("alpha")
	model.commitAddTag()
	assert.Equal(t, []string{"alpha"}, model.addTags)
	assert.Equal(t, "", model.addTagInput.Value())

	model.addTagInput.SetValue("alpha")
	model.commitAddTag()
	assert.Equal(t, []string{"alpha"}, model.addTags)
	assert.Equal(t, "", model.addTagInput.Value())

	model.addTagInput.SetValue("beta tag")
	model.commitAddTag()
	assert.Equal(t, []string{"alpha", "beta-tag"}, model.addTags)

	model.addTags = nil
	model.addTagInput.SetValue("")
	assert.Equal(t, "-", model.renderAddTags(false))

	model.addTags = []string{"alpha"}
	out := stripANSI(model.renderAddTags(false))
	assert.Contains(t, out, "[alpha]")
	assert.NotContains(t, out, "█")

	model.addTagInput.SetValue("beta")
	out = stripANSI(model.renderAddTags(false))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "beta")
	assert.NotContains(t, out, "█")

	out = stripANSI(model.renderAddTags(true))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "beta")
	assert.Contains(t, out, "█")
}

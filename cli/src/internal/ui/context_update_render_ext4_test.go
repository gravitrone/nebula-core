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
	model.scopeOptions = []string{"public", "private"}
	model.fields[fieldTitle].value = "Title A"
	model.fields[fieldURL].value = "https://a"
	model.fields[fieldNotes].value = "notes"
	model.tags = []string{"alpha"}
	model.tagBuf = "beta"
	model.scopes = []string{"public"}
	model.linkEntities = []api.Entity{{ID: "ent-1", Name: "Entity One"}}

	model.focus = fieldType
	model.typeSelecting = true
	out := components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "[note]")

	model.typeSelecting = false
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Type:")
	assert.Contains(t, out, "note")

	model.focus = fieldTags
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")

	model.focus = fieldScopes
	model.scopeSelecting = true
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "public")

	model.scopeSelecting = false
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Scopes:")

	model.focus = fieldEntities
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Entity One")

	model.focus = fieldTitle
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Title A")
	assert.Contains(t, out, "█")

	model.focus = fieldTitle
	model.fields[fieldURL].value = ""
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "URL:")
	assert.Contains(t, out, "-")

	model.errText = "add render error"
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "add render error")
}

func TestContextRenderEditBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 92
	model.scopeOptions = []string{"public", "private"}
	model.contextEditFields[contextEditFieldTitle].value = "Title E"
	model.contextEditFields[contextEditFieldURL].value = "https://e"
	model.contextEditFields[contextEditFieldNotes].value = "notes edit"
	model.editTags = []string{"alpha"}
	model.editTagBuf = "beta"
	model.editScopes = []string{"public"}

	model.editFocus = contextEditFieldType
	model.editTypeSelecting = true
	out := components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "[note]")

	model.editTypeSelecting = false
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Type:")

	model.editFocus = contextEditFieldStatus
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Status:")

	model.editFocus = contextEditFieldTags
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")

	model.editFocus = contextEditFieldScopes
	model.editScopeSelecting = true
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "public")

	model.editScopeSelecting = false
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Scopes:")

	model.editFocus = contextEditFieldNotes
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "notes edit")

	model.editFocus = contextEditFieldTitle
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Title E")
	assert.Contains(t, out, "█")

	model.contextEditFields[contextEditFieldURL].value = ""
	model.editFocus = contextEditFieldTitle
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "URL:")
	assert.Contains(t, out, "-")

	model.errText = "edit render error"
	model.editSaving = true
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "edit render error")
	assert.True(t, strings.Contains(out, "Saving..."))
}

func TestContextUpdateKeyBranchMatrixAdditional(t *testing.T) {
	model := NewContextModel(nil)

	// Type selector branches.
	model.focus = fieldType
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.True(t, updated.typeSelecting)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.typeIdx)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.typeIdx)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.False(t, updated.typeSelecting)

	// Scope selector branches.
	updated.focus = fieldScopes
	updated.scopeOptions = []string{"public", "private"}
	updated.scopeSelecting = true
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.scopeIdx)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, []string{"private"}, updated.scopes)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.scopeIdx)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.scopeSelecting)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.True(t, updated.scopeSelecting)
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.scopeSelecting)

	// Tags commit + typing branches.
	updated.focus = fieldTags
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	require.Nil(t, cmd)
	assert.Equal(t, "A", updated.tagBuf)
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: ',', Text: ","})
	require.Nil(t, cmd)
	assert.Equal(t, []string{"a"}, updated.tags)
	assert.Equal(t, "", updated.tagBuf)

	// Backspace branches for tags/scopes/entities/default fields.
	updated.tags = []string{"a", "b"}
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, []string{"a"}, updated.tags)

	updated.focus = fieldScopes
	updated.scopes = []string{"public"}
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Empty(t, updated.scopes)

	updated.focus = fieldEntities
	updated.linkEntities = []api.Entity{{ID: "ent-1"}}
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Empty(t, updated.linkEntities)

	updated.focus = fieldTitle
	updated.fields[fieldTitle].value = "abc"
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "ab", updated.fields[fieldTitle].value)

	// Entities enter opens search mode.
	updated.focus = fieldEntities
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.True(t, updated.linkSearching)

	// Entities typing starts link search and emits command.
	updated.focus = fieldEntities
	updated.linkSearching = false
	updated.linkQuery = ""
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.NotNil(t, cmd)
	assert.True(t, updated.linkSearching)
	assert.Equal(t, "x", updated.linkQuery)

	// Global navigation + reset + save branches.
	updated.linkSearching = false
	updated.focus = fieldTitle
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, fieldURL, updated.focus)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, fieldTitle, updated.focus)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, "ab", updated.fields[fieldTitle].value)
	assert.False(t, updated.modeFocus)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.fields[fieldTitle].value)
	assert.False(t, updated.modeFocus)

	updated.fields[fieldTitle].value = ""
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "Title is required", updated.errText)
}

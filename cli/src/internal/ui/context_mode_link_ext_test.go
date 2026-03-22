package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	huh "charm.land/huh/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHandleModeKeysBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.modeFocus = true
	model.view = contextViewAdd

	updated, cmd := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated.view = contextViewAdd
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.NotNil(t, cmd)
	assert.Equal(t, contextViewList, updated.view)
	assert.True(t, updated.loadingList)

	updated.modeFocus = true
	updated.view = contextViewDetail
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, contextViewList, updated.view)
}

func TestContextHandleFilterInputBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.filtering = true
	model.allItems = []api.Context{
		{ID: "ctx-1", Name: "Alpha", SourceType: "note", Status: "active"},
		{ID: "ctx-2", Name: "Beta", SourceType: "video", Status: "inactive"},
	}
	model.applyContextFilter()
	assert.Len(t, model.items, 2)

	updated, cmd := model.handleFilterInput(tea.KeyPressMsg{Code: ' ', Text: " "})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated.filterBuf = "beta"
	updated.applyContextFilter()
	assert.Len(t, updated.items, 1)
	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated.filtering = true
	updated.filterBuf = "active"
	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "active", updated.filterBuf)
}

func TestContextHandleDetailKeysBranchMatrix(t *testing.T) {
	now := time.Now().UTC()
	content := "notes"
	url := "https://example.com"
	model := NewContextModel(nil)
	model.scopeNames = map[string]string{"scope-1": "public"}
	model.view = contextViewDetail
	model.detail = &api.Context{
		ID:              "ctx-1",
		Name:            "Alpha",
		URL:             &url,
		Content:         &content,
		SourceType:      "note",
		Status:          "active",
		Tags:            []string{"demo"},
		PrivacyScopeIDs: []string{"scope-1"},
		CreatedAt:       now,
	}

	updated, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated, _ = updated.handleDetailKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
	assert.True(t, updated.contentExpanded)
	updated, _ = updated.handleDetailKeys(tea.KeyPressMsg{Code: 'v', Text: "v"})
	assert.True(t, updated.sourcePathExpanded)

	// Enter edit: populates edit form fields.
	updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, contextViewEdit, updated.view)
	assert.Equal(t, "Alpha", updated.editTitle)

	updated.view = contextViewDetail
	updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, contextViewList, updated.view)
	assert.Nil(t, updated.detail)
	assert.False(t, updated.contentExpanded)
	assert.False(t, updated.sourcePathExpanded)
}

func TestContextHandleEditKeysSavingAndModeFocusBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.view = contextViewEdit
	model.editSaving = true

	// editSaving suppresses key handling.
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.editSaving)

	// modeFocus delegates to handleModeKeys (down clears modeFocus).
	model.editSaving = false
	model.modeFocus = true
	updated, cmd = model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
}

func TestContextHandleEditKeysFormInitWhenNil(t *testing.T) {
	model := NewContextModel(nil)
	model.view = contextViewEdit
	model.editForm = nil
	model.editSaving = false
	model.modeFocus = false

	// Passing a key when editForm is nil should init it and return cmd.
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.NotNil(t, cmd)
	assert.NotNil(t, updated.editForm)
}

func TestContextHandleEditKeysAbortGoesBackToDetail(t *testing.T) {
	model := NewContextModel(nil)
	model.view = contextViewEdit
	model.detail = &api.Context{ID: "ctx-1", Title: "Alpha", SourceType: "note", Status: "active"}
	model.initEditForm()
	require.NotNil(t, model.editForm)
	_ = model.editForm.Init()

	// Set StateAborted directly to simulate the huh form abort signal.
	model.editForm.State = huh.StateAborted

	// handleEditKeys detects StateAborted -> go back to detail, nil out editForm.
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	_ = cmd
	assert.Equal(t, contextViewDetail, updated.view)
	assert.Nil(t, updated.editForm)
}

func TestContextRenderTagsAndEditTagsViaParseNormalize(t *testing.T) {
	// Tags in the new API are stored as a comma-separated string in addTagStr/editTagStr.
	// Parse + normalize + dedup is done in save/saveEdit.
	addTagStr := "Alpha, beta_tag, #Gamma"
	tags := parseCommaSeparated(addTagStr)
	for i, t := range tags {
		tags[i] = normalizeTag(t)
	}
	tags = dedup(tags)
	assert.Equal(t, []string{"alpha", "beta-tag", "gamma"}, tags)

	// Scope normalization works the same way.
	editScopeStr := "public, Team Scope, #private"
	scopes := parseCommaSeparated(editScopeStr)
	for i, s := range scopes {
		scopes[i] = normalizeScope(s)
	}
	scopes = normalizeScopeList(scopes)
	assert.Contains(t, scopes, "public")
	assert.Contains(t, scopes, "team-scope")
	assert.Contains(t, scopes, "private")
}

func TestContextRenderLinkedEntitiesFallbackAndFocusBranches(t *testing.T) {
	model := NewContextModel(nil)

	// No entities: returns empty string.
	assert.Equal(t, "", model.renderLinkedEntities())

	// With entities: renders name pills.
	model.linkEntities = []api.Entity{
		{ID: "entity-long-id-1", Name: ""},
		{ID: "entity-2", Name: "Alpha"},
	}
	out := components.SanitizeText(model.renderLinkedEntities())
	assert.Contains(t, out, "entity-l")
	assert.Contains(t, out, "Alpha")
}

func TestContextHandleLinkSearchBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.startLinkSearch()
	assert.True(t, model.linkSearching)
	assert.Equal(t, "", model.linkQuery)

	model.linkResults = []api.Entity{{ID: "ent-1", Name: "Alpha"}}
	model.linkTable.SetRows([]table.Row{{"Alpha"}})
	model.linkTable.SetCursor(0)
	model, cmd := model.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, model.linkSearching)
	assert.Len(t, model.linkEntities, 1)
	assert.Empty(t, model.linkResults)

	model.linkSearching = true
	model.linkQuery = "ab"
	model, cmd = model.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.NotNil(t, cmd)
	assert.Equal(t, "a", model.linkQuery)

	model.linkResults = []api.Entity{{ID: "ent-2", Name: "Beta"}}
	model.linkTable.SetRows([]table.Row{{"Beta"}})
	model, cmd = model.handleLinkSearch(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.NotNil(t, cmd)
	assert.Equal(t, "ax", model.linkQuery)
	assert.Empty(t, model.linkResults)

	model.linkQuery = "query"
	model.linkResults = []api.Entity{{ID: "ent-3"}}
	model.linkTable.SetRows([]table.Row{{"ent-3"}})
	model, cmd = model.handleLinkSearch(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "", model.linkQuery)
	assert.Empty(t, model.linkResults)

	model.linkSearching = true
	model.linkQuery = "z"
	model.linkResults = []api.Entity{{ID: "ent-4"}}
	model.linkTable.SetRows([]table.Row{{"ent-4"}})
	model, cmd = model.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, model.linkSearching)
	assert.Equal(t, "", model.linkQuery)
	assert.Empty(t, model.linkResults)
}

func TestContextRenderLinkSearchAndPreviewBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 96
	model.linkLoading = true
	view := components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "Searching")

	model.linkLoading = false
	model.linkQuery = ""
	view = components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "Type to search")

	model.linkQuery = "alpha"
	model.linkResults = nil
	view = components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "No matches")

	model.linkResults = []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active", Tags: []string{"demo"}},
		{ID: "ent-2", Name: "Beta", Type: "project", Status: "inactive"},
	}
	model.linkTable.SetRows([]table.Row{{"Alpha"}, {"Beta"}})
	view = components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "2 results")
	assert.Contains(t, view, "Name")
	assert.Contains(t, view, "Status")
}

func TestContextRenderLinkEntityPreviewFallbacks(t *testing.T) {
	model := NewContextModel(nil)
	assert.Equal(t, "", model.renderLinkEntityPreview(api.Entity{}, 0))

	preview := components.SanitizeText(model.renderLinkEntityPreview(api.Entity{}, 40))
	assert.Contains(t, preview, "Selected")
	assert.Contains(t, preview, "entity")
	assert.Contains(t, preview, "Type")
	assert.Contains(t, preview, "Status")
}

func TestContextRenderLinkSearchWideLayoutAndSelectionFallbackBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 220 // trigger side-by-side layout branch
	model.linkQuery = "alpha"
	model.linkResults = []api.Entity{
		{ID: "ent-1", Name: "", Type: "", Status: ""},
	}
	model.linkTable.SetRows([]table.Row{{"ent-1"}})
	model.linkTable.SetCursor(0)

	view := components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "1 results")
	assert.Contains(t, view, "Name")
	assert.Contains(t, view, "Type")
	assert.Contains(t, view, "Status")
	assert.Contains(t, view, "Selected")
}

func TestContextHandleLinkSearchEmptyTableAndOutOfRangeSelectionBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.startLinkSearch()

	updated, cmd := model.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.linkSearching)
	updated, cmd = updated.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.linkSearching)
	updated, cmd = updated.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.linkQuery)
	updated, cmd = updated.handleLinkSearch(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.linkQuery)

	model = NewContextModel(nil)
	model.startLinkSearch()
	model.linkQuery = "alpha"
	model.linkResults = []api.Entity{{ID: "ent-1", Name: "Alpha"}}
	model.linkTable.SetRows([]table.Row{{"Alpha"}, {"Ghost"}})
	model.linkTable.SetCursor(1)

	updated, cmd = model.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.linkSearching)
	assert.Empty(t, updated.linkEntities)
	assert.Empty(t, updated.linkResults)
	assert.Equal(t, "", updated.linkQuery)
}

func TestContextRenderLinkSearchSideBySidePreviewAndNarrowTableBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 220
	model.linkQuery = "alpha"
	model.linkResults = []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active"},
	}
	model.linkTable.SetRows([]table.Row{{"Alpha"}})
	model.linkTable.SetCursor(0)

	view := components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "Selected")
	assert.Contains(t, view, "Type")
	assert.Contains(t, view, "Status")

	model.width = 20
	view = components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "1 results")
	assert.Contains(t, view, "Name")
	assert.Contains(t, view, "Type")
	assert.Contains(t, view, "Status")
}

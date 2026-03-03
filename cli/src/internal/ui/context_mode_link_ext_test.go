package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHandleModeKeysBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.modeFocus = true
	model.view = contextViewAdd
	model.focus = fieldNotes

	updated, cmd := model.handleModeKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, 0, updated.focus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated.view = contextViewEdit
	updated.editFocus = contextEditFieldMeta
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, 0, updated.editFocus)

	updated.modeFocus = true
	updated.view = contextViewAdd
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyLeft})
	require.NotNil(t, cmd)
	assert.Equal(t, contextViewList, updated.view)
	assert.True(t, updated.loadingList)

	updated.modeFocus = true
	updated.view = contextViewDetail
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyRight})
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

	updated, cmd := model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated.filterBuf = "beta"
	updated.applyContextFilter()
	assert.Len(t, updated.items, 1)
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.items, 2)

	updated.filtering = true
	updated.filterBuf = "active"
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEnter})
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
		Metadata:        api.JSONMap{"topic": "build"},
	}

	updated, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated, cmd = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	require.Nil(t, cmd)
	assert.True(t, updated.metaExpanded)
	updated, _ = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	assert.True(t, updated.contentExpanded)
	updated, _ = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	assert.True(t, updated.sourcePathExpanded)

	updated, cmd = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	require.Nil(t, cmd)
	assert.Equal(t, contextViewEdit, updated.view)
	assert.Equal(t, contextEditFieldTitle, updated.editFocus)
	assert.Equal(t, "Alpha", updated.contextEditFields[contextEditFieldTitle].value)

	updated.view = contextViewDetail
	updated, cmd = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, contextViewList, updated.view)
	assert.Nil(t, updated.detail)
	assert.False(t, updated.metaExpanded)
	assert.False(t, updated.contentExpanded)
	assert.False(t, updated.sourcePathExpanded)
}

func TestContextHandleEditKeysBranchMatrix(t *testing.T) {
	t.Run("early returns and mode focus delegation", func(t *testing.T) {
		model := NewContextModel(nil)
		model.view = contextViewEdit
		model.editSaving = true
		model.editFocus = contextEditFieldStatus

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		require.Nil(t, cmd)
		assert.Equal(t, contextEditFieldStatus, updated.editFocus)

		model.editSaving = false
		model.modeFocus = true
		updated, cmd = model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		require.Nil(t, cmd)
		assert.False(t, updated.modeFocus)
		assert.Equal(t, contextEditFieldTitle, updated.editFocus)
	})

	t.Run("type, scope, and status selector paths", func(t *testing.T) {
		model := NewContextModel(nil)
		model.view = contextViewEdit
		model.detail = &api.Context{ID: "ctx-1"}
		model.scopeOptions = []string{"public", "private"}
		model.editFocus = contextEditFieldType

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		require.Nil(t, cmd)
		assert.True(t, updated.editTypeSelecting)

		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Equal(t, 1, updated.editTypeIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, 0, updated.editTypeIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.editTypeSelecting)

		updated.editFocus = contextEditFieldScopes
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.True(t, updated.editScopeSelecting)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Equal(t, 1, updated.scopeIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.Equal(t, []string{"private"}, updated.editScopes)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, 0, updated.scopeIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.Equal(t, []string{"private", "public"}, updated.editScopes)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.editScopeSelecting)

		updated.editFocus = contextEditFieldStatus
		startStatus := updated.editStatusIdx
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Equal(t, (startStatus+1)%len(contextStatusOptions), updated.editStatusIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, startStatus, updated.editStatusIdx)
	})

	t.Run("backspace, default input, and save branches", func(t *testing.T) {
		model := NewContextModel(nil)
		model.view = contextViewEdit
		model.detail = &api.Context{ID: "ctx-1"}
		model.editFocus = contextEditFieldTags
		model.editTagBuf = "ab"

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		require.Nil(t, cmd)
		assert.Equal(t, "a", updated.editTagBuf)

		updated.editTagBuf = ""
		updated.editTags = []string{"alpha", "beta"}
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, []string{"alpha"}, updated.editTags)

		updated.editFocus = contextEditFieldScopes
		updated.editScopes = []string{"public"}
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Empty(t, updated.editScopes)

		updated.editFocus = contextEditFieldTitle
		updated.contextEditFields[contextEditFieldTitle].value = "Alpha"
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, "Alph", updated.contextEditFields[contextEditFieldTitle].value)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.Equal(t, "Alpha ", updated.contextEditFields[contextEditFieldTitle].value)

		updated.editFocus = contextEditFieldMeta
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, updated.editMeta.Active)

		updated.editMeta.Buffer = "bad metadata line"
		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
		require.Nil(t, cmd)
		assert.False(t, updated.editSaving)

		updated.editMeta.Buffer = "profile: value"
		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
		require.NotNil(t, cmd)
		assert.True(t, updated.editSaving)

		updated.editSaving = false
		updated.editScopeSelecting = true
		updated.editFocus = contextEditFieldScopes
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Equal(t, contextViewEdit, updated.view)
		assert.False(t, updated.editScopeSelecting)

		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Equal(t, contextViewDetail, updated.view)
		assert.False(t, updated.editScopeSelecting)
	})
}

func TestContextRenderTagsAndEditTagsBranches(t *testing.T) {
	model := NewContextModel(nil)
	assert.Equal(t, "-", model.renderTags(false))
	assert.Equal(t, "-", model.renderEditTags(false))

	model.tags = []string{"alpha"}
	model.tagBuf = "beta"
	out := components.SanitizeText(model.renderTags(false))
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")

	out = components.SanitizeText(model.renderTags(true))
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")
	assert.Contains(t, out, "█")

	model.editTags = []string{"x"}
	model.editTagBuf = "y"
	out = components.SanitizeText(model.renderEditTags(false))
	assert.Contains(t, out, "x")
	assert.Contains(t, out, "y")

	out = components.SanitizeText(model.renderEditTags(true))
	assert.Contains(t, out, "x")
	assert.Contains(t, out, "y")
	assert.Contains(t, out, "█")
}

func TestContextRenderLinkedEntitiesFallbackAndFocusBranches(t *testing.T) {
	model := NewContextModel(nil)
	assert.Equal(t, "-", model.renderLinkedEntities(false))
	assert.Equal(t, "", model.renderLinkedEntities(true))

	model.linkEntities = []api.Entity{
		{ID: "entity-long-id-1", Name: ""},
		{ID: "entity-2", Name: "Alpha"},
	}
	out := components.SanitizeText(model.renderLinkedEntities(false))
	assert.Contains(t, out, "[entity-l]")
	assert.Contains(t, out, "[Alpha]")
}

func TestContextHandleLinkSearchBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.startLinkSearch()
	assert.True(t, model.linkSearching)
	assert.Equal(t, "", model.linkQuery)

	model.linkResults = []api.Entity{{ID: "ent-1", Name: "Alpha"}}
	model.linkList.SetItems([]string{"Alpha"})
	model, cmd := model.handleLinkSearch(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, model.linkSearching)
	assert.Len(t, model.linkEntities, 1)
	assert.Empty(t, model.linkResults)

	model.linkSearching = true
	model.linkQuery = "ab"
	model, cmd = model.handleLinkSearch(tea.KeyMsg{Type: tea.KeyBackspace})
	require.NotNil(t, cmd)
	assert.Equal(t, "a", model.linkQuery)

	model.linkResults = []api.Entity{{ID: "ent-2", Name: "Beta"}}
	model.linkList.SetItems([]string{"Beta"})
	model, cmd = model.handleLinkSearch(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	require.NotNil(t, cmd)
	assert.Equal(t, "ax", model.linkQuery)
	assert.Empty(t, model.linkResults)

	model.linkQuery = "query"
	model.linkResults = []api.Entity{{ID: "ent-3"}}
	model.linkList.SetItems([]string{"ent-3"})
	model, cmd = model.handleLinkSearch(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, "", model.linkQuery)
	assert.Empty(t, model.linkResults)

	model.linkSearching = true
	model.linkQuery = "z"
	model.linkResults = []api.Entity{{ID: "ent-4"}}
	model.linkList.SetItems([]string{"ent-4"})
	model, cmd = model.handleLinkSearch(tea.KeyMsg{Type: tea.KeyEsc})
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
		{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active", Tags: []string{"demo"}, Metadata: api.JSONMap{"role": "builder"}},
		{ID: "ent-2", Name: "Beta", Type: "project", Status: "inactive"},
	}
	model.linkList.SetItems([]string{"Alpha", "Beta"})
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
	// Intentionally provide more list rows than results and point cursor out of range
	// to exercise absIdx guard and previewItem=nil branch.
	model.linkList.SetItems([]string{"phantom-a", "phantom-b"})
	model.linkList.Cursor = 9

	view := components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "Link Entity")
	assert.Contains(t, view, "1 results")
	assert.Contains(t, view, "Name")
	assert.Contains(t, view, "Type")
	assert.Contains(t, view, "Status")
	// No selected preview section because selection is out of range.
	assert.NotContains(t, view, "Selected")
}

func TestContextHandleEditKeysNavigationBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.view = contextViewEdit
	model.editFocus = contextEditFieldType
	model.editTypeSelecting = true
	model.editScopeSelecting = true

	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, contextEditFieldStatus, updated.editFocus)
	assert.False(t, updated.editTypeSelecting)
	assert.False(t, updated.editScopeSelecting)

	updated.editFocus = contextEditFieldTitle
	updated.editTypeSelecting = true
	updated.editScopeSelecting = true
	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)
	assert.Equal(t, contextEditFieldTitle, updated.editFocus)
	assert.False(t, updated.editTypeSelecting)
	assert.False(t, updated.editScopeSelecting)

	updated.modeFocus = false
	updated.editFocus = contextEditFieldNotes
	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, contextEditFieldScopes, updated.editFocus)
}

func TestContextHandleLinkSearchNilListAndOutOfRangeSelectionBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.startLinkSearch()
	model.linkList = nil

	updated, cmd := model.handleLinkSearch(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.linkSearching)
	updated, cmd = updated.handleLinkSearch(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.linkSearching)
	updated, cmd = updated.handleLinkSearch(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.linkQuery)
	updated, cmd = updated.handleLinkSearch(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.linkQuery)

	model = NewContextModel(nil)
	model.startLinkSearch()
	model.linkQuery = "alpha"
	model.linkResults = []api.Entity{{ID: "ent-1", Name: "Alpha"}}
	model.linkList.SetItems([]string{"Alpha", "Ghost"})
	model.linkList.Cursor = 1

	updated, cmd = model.handleLinkSearch(tea.KeyMsg{Type: tea.KeyEnter})
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
		{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active", Metadata: api.JSONMap{"role": "builder"}},
	}
	model.linkList.SetItems([]string{"Alpha"})
	model.linkList.Cursor = 0

	view := components.SanitizeText(model.renderLinkSearch())
	assert.Contains(t, view, "Link Entity")
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

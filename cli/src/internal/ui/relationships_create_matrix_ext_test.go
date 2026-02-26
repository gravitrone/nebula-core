package ui

import (
	"encoding/json"
	"net/http"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationshipsHandleCreateKeysSourceSearchClearAndExit(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch
	model.createQuery = "alpha"
	model.createResults = []relationshipCreateCandidate{{ID: "ent-1"}}
	model.createList.SetItems([]string{"ent-1"})
	model.createLoading = true

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)
	assert.Equal(t, "", updated.createQuery)
	assert.Empty(t, updated.createResults)
	assert.Empty(t, updated.createList.Items)
	assert.False(t, updated.createLoading)

	updated, cmd = updated.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewList, updated.view)
}

func TestRelationshipsHandleCreateKeysSourceSearchBackspaceUsesCache(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch
	model.createQuery = "be"
	model.entityCache = []api.Entity{{ID: "ent-1", Name: "beta", Type: "person", Status: "active"}}
	model.contextCache = []api.Context{{ID: "ctx-1", Title: "runbook", SourceType: "note", Status: "active"}}
	model.jobCache = []api.Job{{ID: "job-1", Title: "beta task", Status: "active"}}

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "b", updated.createQuery)
	assert.False(t, updated.createLoading)
	require.NotEmpty(t, updated.createResults)
	require.NotEmpty(t, updated.createList.Items)
}

func TestRelationshipsHandleCreateKeysSourceSearchEnterAdvancesToTarget(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch
	model.createResults = []relationshipCreateCandidate{{ID: "ent-1", NodeType: "entity", Name: "alpha"}}
	model.createList.SetItems([]string{"alpha"})

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	require.NotNil(t, updated.createSource)
	assert.Equal(t, "ent-1", updated.createSource.ID)
	assert.Equal(t, "", updated.createQuery)
	assert.Empty(t, updated.createResults)
}

func TestRelationshipsHandleCreateKeysSourceSelectBackAndEnter(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSelect
	model.createResults = []relationshipCreateCandidate{{ID: "ent-2", NodeType: "entity", Name: "beta"}}
	model.createList.SetItems([]string{"beta"})

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)

	updated.view = relsViewCreateSourceSelect
	updated, cmd = updated.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	require.NotNil(t, updated.createSource)
	assert.Equal(t, "ent-2", updated.createSource.ID)
}

func TestRelationshipsHandleCreateKeysTargetSearchClearAndBack(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateTargetSearch
	model.createQuery = "gamma"
	model.createResults = []relationshipCreateCandidate{{ID: "ent-3"}}
	model.createList.SetItems([]string{"ent-3"})
	model.createLoading = true

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	assert.Equal(t, "", updated.createQuery)
	assert.Empty(t, updated.createResults)
	assert.Empty(t, updated.createList.Items)
	assert.False(t, updated.createLoading)

	updated, cmd = updated.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)
}

func TestRelationshipsHandleCreateKeysTargetSelectBackAndEnter(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateTargetSelect
	model.typeOptions = []string{"depends-on"}
	model.createResults = []relationshipCreateCandidate{{ID: "ent-4", NodeType: "entity", Name: "delta"}}
	model.createList.SetItems([]string{"delta"})

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)

	updated.view = relsViewCreateTargetSelect
	updated, cmd = updated.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateType, updated.view)
	require.NotNil(t, updated.createTarget)
	assert.Equal(t, "ent-4", updated.createTarget.ID)
	assert.Equal(t, []string{"depends-on"}, updated.createTypeResults)
}

func TestRelationshipsHandleCreateKeysTypeNavigationAndShortcuts(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateType
	model.typeOptions = []string{"depends-on", "works-with"}
	model.resetTypeSuggestions()

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.createTypeNav)

	updated, cmd = updated.handleCreateKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.createTypeNav)

	updated.createType = "dep"
	updated, cmd = updated.handleCreateKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createType)
	assert.False(t, updated.createTypeNav)

	updated, cmd = updated.handleCreateKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
}

func TestRelationshipsHandleCreateKeysTypeEnterRequiresState(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateType
	model.createType = ""
	model.createTypeResults = []string{"depends-on"}
	model.createTypeList.SetItems(model.createTypeResults)
	model.createTypeNav = true

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateType, updated.view)
}

func TestRelationshipsHandleCreateKeysTypeEnterUsesSuggestion(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateType
	model.createTypeResults = []string{"depends-on"}
	model.createTypeList.SetItems(model.createTypeResults)
	model.createTypeNav = true
	model.createSource = &relationshipCreateCandidate{ID: "ent-1", NodeType: "entity"}
	model.createTarget = &relationshipCreateCandidate{ID: "ent-2", NodeType: "entity"}

	updated, cmd := model.handleCreateKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, relsViewList, updated.view)
	assert.True(t, updated.loading)
}

func TestRelationshipsRenderCreateSearchStateMessages(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 90

	model.view = relsViewCreateSourceSearch
	model.createLoading = true
	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "Searching...")

	model.createLoading = false
	model.createQuery = ""
	out = components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "Type to search.")

	model.createQuery = "x"
	model.createResults = nil
	model.createList.SetItems(nil)
	out = components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "No matches.")
}

func TestRelationshipsRenderCreateSearchTableFallbacks(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 92
	model.createQuery = "a"
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "", Kind: "", Status: ""},
		{ID: "ctx-1", NodeType: "context", Name: "alpha note", Kind: "context/note", Status: "active"},
	}
	model.createList.SetItems([]string{"ent-1", "ctx-1"})
	model.createList.Cursor = -1 // hide side preview, keep assertions focused on table fallbacks

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "2 results")
	assert.Contains(t, out, "node")
	assert.Contains(t, out, "entity")
	assert.Contains(t, out, "active")
}

func TestRelationshipsRenderCreateTypeStateMessages(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 92

	model.createType = ""
	model.typeOptions = nil
	model.createTypeResults = nil
	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "Type a relationship type.")

	model.createType = "dep"
	model.createTypeResults = nil
	out = components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "No suggestions.")

	model.createTypeResults = []string{"depends-on", "related-to"}
	model.createTypeList.SetItems(model.createTypeResults)
	out = components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "2 suggestions")
	assert.Contains(t, out, "depends-on")
}

func TestRelationshipsApplyListFilterAndSelectionHelpers(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.names["ent-1"] = "alpha"
	model.names["ent-2"] = "beta"
	model.allItems = []api.Relationship{
		{ID: "rel-1", Type: "depends-on", Status: "active", SourceID: "ent-1", TargetID: "ent-2"},
		{ID: "rel-2", Type: "blocks", Status: "inactive", SourceID: "ent-2", TargetID: "ent-1"},
	}
	model.list.SetItems([]string{"x", "y"})

	model.filterBuf = "depends"
	model.applyListFilter()
	require.Len(t, model.items, 1)
	assert.Equal(t, "rel-1", model.items[0].ID)

	model.filterBuf = ""
	model.applyListFilter()
	require.Len(t, model.items, 2)

	model.list.Cursor = 99
	assert.Nil(t, model.selectedRelationship())
	model.list.Cursor = 0
	require.NotNil(t, model.selectedRelationship())
	assert.Equal(t, "rel-1", model.selectedRelationship().ID)
}

func TestRelationshipsDisplayNodeFallbackMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.names["ent-1"] = "cached alpha"

	assert.Equal(t, "explicit name", model.displayNode("ent-1", "entity", "explicit name"))
	assert.Equal(t, "cached alpha", model.displayNode("ent-1", "entity", ""))
	assert.Equal(t, "unknown entity", model.displayNode("ent-2", "entity", ""))
	assert.Equal(t, "unknown context", model.displayNode("ctx-1", "context", ""))
	assert.Equal(t, "unknown job", model.displayNode("job-1", "job", ""))
	assert.Equal(t, "unknown", model.displayNode("x-1", "custom", ""))
}

func TestRelationshipsLoadCachesNilAndErrorPaths(t *testing.T) {
	nilClientModel := NewRelationshipsModel(nil)
	assert.Nil(t, nilClientModel.loadContextCache())
	assert.Nil(t, nilClientModel.loadJobCache())

	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	model := NewRelationshipsModel(client)

	ctxCmd := model.loadContextCache()
	require.NotNil(t, ctxCmd)
	_, ok := ctxCmd().(errMsg)
	assert.True(t, ok)

	jobCmd := model.loadJobCache()
	require.NotNil(t, jobCmd)
	_, ok = jobCmd().(errMsg)
	assert.True(t, ok)
}

func TestRelationshipsSearchCreateNodesErrorBranches(t *testing.T) {
	t.Run("entity query fails", func(t *testing.T) {
		_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		model := NewRelationshipsModel(client)
		cmd := model.searchCreateNodes("alpha")
		require.NotNil(t, cmd)
		_, ok := cmd().(errMsg)
		assert.True(t, ok)
	})

	t.Run("context query fails", func(t *testing.T) {
		_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/entities":
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
		model := NewRelationshipsModel(client)
		cmd := model.searchCreateNodes("alpha")
		require.NotNil(t, cmd)
		_, ok := cmd().(errMsg)
		assert.True(t, ok)
	})

	t.Run("jobs query fails", func(t *testing.T) {
		_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/entities", "/api/context":
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
		model := NewRelationshipsModel(client)
		cmd := model.searchCreateNodes("alpha")
		require.NotNil(t, cmd)
		_, ok := cmd().(errMsg)
		assert.True(t, ok)
	})
}

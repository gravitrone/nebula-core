package ui

import (
	"encoding/json"
	"net/http"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationshipsHandleCreateKeysSourceSearchClearAndExit(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch
	model.createQueryInput.SetValue("alpha")
	model.createResults = []relationshipCreateCandidate{{ID: "ent-1"}}
	model.createTable.SetRows([]table.Row{{"ent-1"}})
	model.createLoading = true

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)
	assert.Equal(t, "", updated.createQueryInput.Value())
	assert.Empty(t, updated.createResults)
	assert.Empty(t, updated.createTable.Rows())
	assert.False(t, updated.createLoading)

	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewList, updated.view)
}

func TestRelationshipsHandleCreateKeysSourceSearchBackspaceUsesCache(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch
	model.createQueryInput.SetValue("be")
	model.entityCache = []api.Entity{{ID: "ent-1", Name: "beta", Type: "person", Status: "active"}}
	model.contextCache = []api.Context{{ID: "ctx-1", Title: "runbook", SourceType: "note", Status: "active"}}
	model.jobCache = []api.Job{{ID: "job-1", Title: "beta task", Status: "active"}}

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "b", updated.createQueryInput.Value())
	assert.False(t, updated.createLoading)
	require.NotEmpty(t, updated.createResults)
	require.NotEmpty(t, updated.createTable.Rows())
}

func TestRelationshipsHandleCreateKeysSourceSearchEnterAdvancesToTarget(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch
	model.createResults = []relationshipCreateCandidate{{ID: "ent-1", NodeType: "entity", Name: "alpha"}}
	model.createTable.SetRows([]table.Row{{"alpha"}})

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	require.NotNil(t, updated.createSource)
	assert.Equal(t, "ent-1", updated.createSource.ID)
	assert.Equal(t, "", updated.createQueryInput.Value())
	assert.Empty(t, updated.createResults)
}

func TestRelationshipsHandleCreateKeysSourceSelectBackAndEnter(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSelect
	model.createResults = []relationshipCreateCandidate{{ID: "ent-2", NodeType: "entity", Name: "beta"}}
	model.createTable.SetRows([]table.Row{{"beta"}})

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)

	updated.view = relsViewCreateSourceSelect
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	require.NotNil(t, updated.createSource)
	assert.Equal(t, "ent-2", updated.createSource.ID)
}

func TestRelationshipsHandleCreateKeysSourceSearchAdditionalBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch
	model.createTable.SetRows([]table.Row{{"alpha"}, {"beta"}})

	// Query-empty ctrl-u exits to list.
	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewList, updated.view)

	// Query-present ctrl-u clears search state.
	updated.view = relsViewCreateSourceSearch
	updated.createQueryInput.SetValue("abc")
	updated.createResults = []relationshipCreateCandidate{{ID: "ent-1"}}
	updated.createLoading = true
	updated.createTable.SetRows([]table.Row{{"ent-1"}})
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)
	assert.Equal(t, "", updated.createQueryInput.Value())
	assert.Empty(t, updated.createResults)
	assert.Empty(t, updated.createTable.Rows())
	assert.False(t, updated.createLoading)

	// Navigation branches.
	updated.createTable.SetRows([]table.Row{{"row-a"}, {"row-b"}})
	updated.createTable.SetCursor(0)
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.createTable.Cursor())
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.createTable.Cursor())

	// Enter with empty results does nothing (cursor -1).
	updated.createTable.SetRows(nil)
	updated.createResults = nil
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Nil(t, updated.createSource)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)

	// Backspace with empty query should no-op.
	updated.createQueryInput.SetValue("")
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createQueryInput.Value())
}

func TestRelationshipsHandleCreateKeysSelectViewNavigationBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "alpha"},
		{ID: "ent-2", NodeType: "entity", Name: "beta"},
	}
	model.createTable.SetRows([]table.Row{{"alpha"}, {"beta"}})

	model.view = relsViewCreateSourceSelect
	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.createTable.Cursor())
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.createTable.Cursor())

	// Enter with empty results does nothing (cursor -1).
	updated.createResults = nil
	updated.createTable.SetRows(nil)
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Nil(t, updated.createSource)
	assert.Equal(t, relsViewCreateSourceSelect, updated.view)

	model.view = relsViewCreateTargetSelect
	model.createTypeResults = []string{"depends-on"}
	model.typeOptions = []string{"depends-on"}
	model.createTable.SetRows([]table.Row{{"alpha"}, {"beta"}})
	model.createTable.SetCursor(0)
	updated, cmd = model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.createTable.Cursor())
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.createTable.Cursor())

	// Enter with empty results does nothing.
	updated.createResults = nil
	updated.createTable.SetRows(nil)
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Nil(t, updated.createTarget)
	assert.Equal(t, relsViewCreateTargetSelect, updated.view)
}

func TestRelationshipsHandleCreateKeysTargetSearchClearAndBack(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateTargetSearch
	model.createQueryInput.SetValue("gamma")
	model.createResults = []relationshipCreateCandidate{{ID: "ent-3"}}
	model.createTable.SetRows([]table.Row{{"ent-3"}})
	model.createLoading = true

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	assert.Equal(t, "", updated.createQueryInput.Value())
	assert.Empty(t, updated.createResults)
	assert.Empty(t, updated.createTable.Rows())
	assert.False(t, updated.createLoading)

	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)
}

func TestRelationshipsHandleCreateKeysTargetSearchAdditionalBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateTargetSearch
	model.createTable.SetRows([]table.Row{{"target-a"}, {"target-b"}})
	model.createSource = &relationshipCreateCandidate{ID: "ent-1", NodeType: "entity", Name: "alpha"}

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)

	updated.view = relsViewCreateTargetSearch
	updated.createQueryInput.SetValue("abc")
	updated.createResults = []relationshipCreateCandidate{{ID: "ent-2"}}
	updated.createLoading = true
	updated.createTable.SetRows([]table.Row{{"ent-2"}})
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	assert.Equal(t, "", updated.createQueryInput.Value())
	assert.Empty(t, updated.createResults)
	assert.Empty(t, updated.createTable.Rows())
	assert.False(t, updated.createLoading)

	updated.createTable.SetRows([]table.Row{{"target-a"}, {"target-b"}})
	updated.createTable.SetCursor(0)
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.createTable.Cursor())
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.createTable.Cursor())

	// Enter with empty results does nothing.
	updated.createResults = nil
	updated.createTable.SetRows(nil)
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Nil(t, updated.createTarget)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
}

func TestRelationshipsHandleCreateKeysTargetSelectBackAndEnter(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateTargetSelect
	model.typeOptions = []string{"depends-on"}
	model.createResults = []relationshipCreateCandidate{{ID: "ent-4", NodeType: "entity", Name: "delta"}}
	model.createTable.SetRows([]table.Row{{"delta"}})

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)

	updated.view = relsViewCreateTargetSelect
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
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

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.createTypeNav)

	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.createTypeNav)

	updated.createTypeInput.SetValue("dep")
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createTypeInput.Value())
	assert.False(t, updated.createTypeNav)

	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
}

func TestRelationshipsHandleCreateKeysTypeInputBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateType
	model.typeOptions = []string{"depends-on", "related-to"}
	model.resetTypeSuggestions()

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createTypeInput.Value())

	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createTypeInput.Value())

	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.Nil(t, cmd)
	assert.Equal(t, "d", updated.createTypeInput.Value())
	assert.False(t, updated.createTypeNav)
	require.NotEmpty(t, updated.createTypeResults)

	updated.createSource = &relationshipCreateCandidate{ID: "ent-1", NodeType: "entity"}
	updated.createTarget = &relationshipCreateCandidate{ID: "ent-2", NodeType: "entity"}
	updated.createTypeNav = false
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, relsViewList, updated.view)
	assert.True(t, updated.loading)
}

func TestRelationshipsHandleCreateKeysTypeEnterRequiresState(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateType
	model.createTypeInput.SetValue("")
	model.createTypeResults = []string{"depends-on"}
	model.createTypeTable.SetRows([]table.Row{{"depends-on"}})
	model.createTypeNav = true

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateType, updated.view)
}

func TestRelationshipsHandleCreateKeysTypeEnterUsesSuggestion(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateType
	model.createTypeResults = []string{"depends-on"}
	model.createTypeTable.SetRows([]table.Row{{"depends-on"}})
	model.createTypeNav = true
	model.createSource = &relationshipCreateCandidate{ID: "ent-1", NodeType: "entity"}
	model.createTarget = &relationshipCreateCandidate{ID: "ent-2", NodeType: "entity"}

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	model.createQueryInput.SetValue("")
	out = components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "Type to search.")

	model.createQueryInput.SetValue("x")
	model.createResults = nil
	model.createTable.SetRows(nil)
	out = components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "No matches.")
}

func TestRelationshipsRenderCreateSearchTableFallbacks(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 92
	model.createQueryInput.SetValue("a")
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "", Kind: "", Status: ""},
		{ID: "ctx-1", NodeType: "context", Name: "alpha note", Kind: "context/note", Status: "active"},
	}
	model.createTable.SetRows([]table.Row{{"ent-1"}, {"ctx-1"}})
	model.createTable.SetCursor(-1) // hide side preview, keep assertions focused on table fallbacks

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "2 results")
	assert.Contains(t, out, "node")
	assert.Contains(t, out, "entity")
	assert.Contains(t, out, "active")
}

func TestRelationshipsRenderCreateTypeStateMessages(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 92

	model.createTypeInput.SetValue("")
	model.typeOptions = nil
	model.createTypeResults = nil
	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "Type a relationship type.")

	model.createTypeInput.SetValue("dep")
	model.createTypeResults = nil
	out = components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "No suggestions.")

	model.createTypeResults = []string{"depends-on", "related-to"}
	model.createTypeTable.SetRows([]table.Row{{"depends-on"}})
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
	model.dataTable.SetRows([]table.Row{{"x"}, {"y"}})

	model.filterInput.SetValue("depends")
	model.applyListFilter()
	require.Len(t, model.items, 1)
	assert.Equal(t, "rel-1", model.items[0].ID)

	model.filterInput.SetValue("")
	model.applyListFilter()
	require.Len(t, model.items, 2)

	model.items = nil
	model.dataTable.SetRows(nil)
	assert.Nil(t, model.selectedRelationship())
	model.items = []api.Relationship{
		{ID: "rel-1", Type: "depends-on", Status: "active", SourceID: "ent-1", TargetID: "ent-2"},
		{ID: "rel-2", Type: "blocks", Status: "inactive", SourceID: "ent-2", TargetID: "ent-1"},
	}
	model.dataTable.SetRows([]table.Row{{"rel-1"}, {"rel-2"}})
	model.dataTable.SetCursor(0)
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

func TestRelationshipsLoadCachesSuccessPaths(t *testing.T) {
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/context":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ctx-1", "title": "runbook", "source_type": "note", "status": "active"},
				},
			}))
		case "/api/jobs":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "job-1", "title": "ship", "status": "planning", "priority": "high"},
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	model := NewRelationshipsModel(client)

	ctxCmd := model.loadContextCache()
	require.NotNil(t, ctxCmd)
	ctxMsg := ctxCmd()
	ctxLoaded, ok := ctxMsg.(relTabContextCacheLoadedMsg)
	require.True(t, ok)
	require.Len(t, ctxLoaded.items, 1)
	assert.Equal(t, "ctx-1", ctxLoaded.items[0].ID)
	model, _ = model.Update(ctxMsg)
	require.Len(t, model.contextCache, 1)

	jobCmd := model.loadJobCache()
	require.NotNil(t, jobCmd)
	jobMsg := jobCmd()
	jobLoaded, ok := jobMsg.(relTabJobCacheLoadedMsg)
	require.True(t, ok)
	require.Len(t, jobLoaded.items, 1)
	assert.Equal(t, "job-1", jobLoaded.items[0].ID)
	model, _ = model.Update(jobMsg)
	require.Len(t, model.jobCache, 1)
}

func TestUniqueRelationshipTypesAndCandidateCombinerBranches(t *testing.T) {
	types := uniqueRelationshipTypes([]api.Relationship{
		{Type: " Uses "},
		{Type: "uses"},
		{Type: "BLOCKS"},
		{Type: "  "},
		{Type: ""},
	})
	assert.Equal(t, []string{"uses", "blocks"}, types)

	priority := " high "
	candidates := combineCreateCandidates(
		[]api.Entity{
			{ID: "ent-1", Name: " ", Type: " ", Status: " ", Tags: []string{"t1"}},
			{ID: "ent-2", Name: "alpha", Type: "person", Status: "active"},
		},
		[]api.Context{
			{ID: "ctx-1", Title: " ", Name: "fallback", SourceType: " ", Status: " ", Tags: []string{"ct"}},
			{ID: "ctx-2", Title: "", Name: "", SourceType: "doc", Status: "archived"},
		},
		[]api.Job{
			{ID: "job-1", Title: " ", Priority: &priority, Status: " "},
			{ID: "job-2", Title: "deliver", Status: "running"},
		},
	)
	require.Len(t, candidates, 6)
	assert.Equal(t, relationshipCreateCandidate{
		ID:       "ent-1",
		NodeType: "entity",
		Name:     "entity",
		Kind:     "entity/entity",
		Status:   "-",
		Tags:     []string{"t1"},
	}, candidates[0])
	assert.Equal(t, "entity/person", candidates[1].Kind)
	assert.Equal(t, "fallback", candidates[2].Name)
	assert.Equal(t, "context/note", candidates[2].Kind)
	assert.Equal(t, "context", candidates[3].Name)
	assert.Equal(t, "context/doc", candidates[3].Kind)
	assert.Equal(t, relationshipCreateCandidate{
		ID:       "job-1",
		NodeType: "job",
		Name:     "job",
		Kind:     "job/high",
		Status:   "-",
	}, candidates[4])
	assert.Equal(t, "job", candidates[5].Kind)
}

func TestFilterCreateCandidatesByQueryCopyAndNoMatch(t *testing.T) {
	base := []relationshipCreateCandidate{
		{Name: "Alpha", Kind: "entity/person", Status: "active", NodeType: "entity", Tags: []string{"blue"}},
	}
	copyFiltered := filterCreateCandidatesByQuery(base, "")
	require.Len(t, copyFiltered, 1)
	copyFiltered[0].Name = "changed"
	assert.Equal(t, "Alpha", base[0].Name)

	tagMatch := filterCreateCandidatesByQuery(base, "blue")
	require.Len(t, tagMatch, 1)
	assert.Equal(t, "Alpha", tagMatch[0].Name)

	none := filterCreateCandidatesByQuery(base, "missing")
	assert.Empty(t, none)
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

func TestRelationshipsUpdateCreateSearchBranchMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)

	model.createQueryInput.SetValue("   ")
	model.createResults = []relationshipCreateCandidate{{ID: "ent-1"}}
	model.createTable.SetRows([]table.Row{{"ent-1"}})
	model.createLoading = true
	cmd := model.updateCreateSearch()
	assert.Nil(t, cmd)
	assert.False(t, model.createLoading)
	assert.Nil(t, model.createResults)
	assert.Empty(t, model.createTable.Rows())

	model.entityCache = []api.Entity{{ID: "ent-1", Name: "alpha", Type: "person", Status: "active"}}
	model.contextCache = []api.Context{{ID: "ctx-1", Title: "alpha context", SourceType: "note", Status: "active"}}
	model.jobCache = []api.Job{{ID: "job-1", Title: "alpha job", Status: "active"}}
	model.createQueryInput.SetValue("alpha")
	cmd = model.updateCreateSearch()
	assert.Nil(t, cmd)
	assert.False(t, model.createLoading)
	require.NotEmpty(t, model.createResults)
	require.NotEmpty(t, model.createTable.Rows())

	model = NewRelationshipsModel(nil)
	model.createQueryInput.SetValue("alpha")
	cmd = model.updateCreateSearch()
	require.NotNil(t, cmd)
	assert.True(t, model.createLoading)
}

func TestRelationshipsUpdateCreateSearchRemoteCmdResult(t *testing.T) {
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "alpha", "type": "person", "status": "active"},
				},
			}))
			return
		case "/api/context":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		case "/api/jobs":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewRelationshipsModel(client)
	model.createQueryInput.SetValue("alpha")
	cmd := model.updateCreateSearch()
	require.NotNil(t, cmd)
	msg := cmd()
	results, ok := msg.(relTabResultsMsg)
	require.True(t, ok)
	assert.Equal(t, "alpha", results.query)
	require.Len(t, results.items, 1)
	assert.Equal(t, "ent-1", results.items[0].ID)
}

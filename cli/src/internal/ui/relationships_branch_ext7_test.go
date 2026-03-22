package ui

import (
	"net/http"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationshipsUpdateAdditionalBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.names = nil

	updated, cmd := model.Update(relTabNamesLoadedMsg{
		names: map[string]string{"ent-1": "Alpha"},
	})
	require.Nil(t, cmd)
	require.NotNil(t, updated.names)
	assert.Equal(t, "Alpha", updated.names["ent-1"])

	updated.createQueryInput.SetValue("fresh")
	updated.createLoading = true
	updated.createResults = []relationshipCreateCandidate{{ID: "old"}}
	updated, cmd = updated.Update(relTabResultsMsg{
		query: "stale",
		items: []relationshipCreateCandidate{{ID: "new"}},
	})
	require.Nil(t, cmd)
	assert.True(t, updated.createLoading)
	require.Len(t, updated.createResults, 1)
	assert.Equal(t, "old", updated.createResults[0].ID)

	updated.editMeta.Active = true
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.True(t, updated.editMeta.Active)

	updated.editMeta.Active = false
	updated.view = relsViewEdit
	updated.detail = &api.Relationship{ID: "rel-1", Status: "active", Properties: api.JSONMap{}}
	updated.editFocus = relsEditFieldStatus
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewDetail, updated.view)
}

func TestRelationshipsViewAndListFilterRouteBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 84
	model.view = relsViewEdit
	model.detail = &api.Relationship{ID: "rel-1", Status: "active", Properties: api.JSONMap{}}

	model.view = relsViewList
	model.filtering = true
	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.Equal(t, "x", updated.filterInput.Value())
}

func TestRelationshipsRenderListTinyWidthAndPreviewGuardBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 24
	model.items = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "entity",
			SourceID:   "ent-1",
			TargetType: "entity",
			TargetID:   "ent-2",
			Type:       "depends-on",
		},
	}
	model.list.SetItems([]string{"rel-1"})

	assert.Equal(t, "", model.renderRelationshipPreview(model.items[0], 0))
}

func TestRelationshipsRenderEditFallbackAndSaveErrorBranch(t *testing.T) {
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewRelationshipsModel(client)
	model.width = 92
	model.detail = &api.Relationship{ID: "rel-1", Status: "active"}
	model.editFocus = relsEditFieldProperties
	model.editMeta.Buffer = ""
	model.editSaving = true

	rendered := components.SanitizeText(model.renderEdit())
	assert.Contains(t, rendered, "Saving...")
	assert.Contains(t, rendered, "-")

	model.editSaving = false
	model.editMeta.Buffer = "note: ok"
	updated, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)

	msg := cmd()
	_, ok := msg.(errMsg)
	assert.True(t, ok)
}

func TestRelationshipsRenderEditNormalizesEmptyPreviewToDash(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 92
	model.detail = &api.Relationship{ID: "rel-1", Status: "active"}
	model.editFocus = relsEditFieldProperties
	model.editMeta.Buffer = "\x00"

	preview := renderMetadataEditorPreview(model.editMeta.Buffer, model.editMeta.Scopes, model.width, 6)
	assert.Equal(t, "", strings.TrimSpace(components.SanitizeText(preview)))

	rendered := components.SanitizeText(model.renderEdit())
	assert.Contains(t, rendered, "Properties:")
}

func TestRelationshipsCreateKeyAdditionalBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewCreateSourceSearch

	updated, cmd := model.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createQueryInput.Value())

	updated.view = relsViewCreateTargetSearch
	updated.createQueryInput.SetValue("ab")
	updated.createResults = []relationshipCreateCandidate{{ID: "ent-1"}}
	updated.createList.SetItems([]string{"ent-1"})
	updated.createLoading = true

	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)
	assert.Equal(t, "", updated.createQueryInput.Value())
	assert.Empty(t, updated.createResults)
	assert.Empty(t, updated.createList.Items)
	assert.False(t, updated.createLoading)

	updated.entityCache = []api.Entity{{ID: "ent-1", Name: "alpha", Type: "person", Status: "active"}}
	updated.createQueryInput.SetValue("al")
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.createQueryInput.Value())
	assert.NotEmpty(t, updated.createResults)

	updated.createQueryInput.SetValue("")
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createQueryInput.Value())

	updated.view = relsViewCreateType
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateTargetSearch, updated.view)

	updated.view = relsViewCreateType
	updated.typeOptions = []string{"depends-on"}
	updated.createTypeInput.SetValue("de")
	updated.createTypeNav = true
	updated.resetTypeSuggestions()
	updated, cmd = updated.handleCreateKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "d", updated.createTypeInput.Value())
	assert.False(t, updated.createTypeNav)
}

func TestRelationshipsCreatePreviewAndNameLoadBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	nodeOut := components.SanitizeText(model.renderCreateNodePreview(relationshipCreateCandidate{
		Name:     "Alpha",
		NodeType: "entity",
		Kind:     "person",
		Status:   "active",
	}, 48))
	assert.NotContains(t, nodeOut, "Meta")

	model.createSource = &relationshipCreateCandidate{Name: "Source Name"}
	model.createTarget = &relationshipCreateCandidate{Name: "Target Name"}
	typeOut := components.SanitizeText(model.renderCreateTypePreview("depends-on", 48))
	assert.Contains(t, typeOut, "Source Name")
	assert.Contains(t, typeOut, "Target Name")

	var entityCalls int
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities/ent-1" {
			entityCalls++
		}
		w.WriteHeader(http.StatusNotFound)
	})
	model.client = client

	load := model.loadRelationshipNames([]api.Relationship{
		{SourceID: "ent-1", SourceType: "entity", SourceName: "Alpha", TargetID: "ctx-1", TargetType: "context"},
		{SourceID: "ent-1", SourceType: "entity"},
	})
	require.NotNil(t, load)
	msg := load()
	namesMsg, ok := msg.(relTabNamesLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "Alpha", namesMsg.names["ent-1"])
	assert.Equal(t, 0, entityCalls)
}

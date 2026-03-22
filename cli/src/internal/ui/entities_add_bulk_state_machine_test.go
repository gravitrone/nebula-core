package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitiesAddFlowSavesEntityViaHuhForm verifies the add flow with huh form integration.
func TestEntitiesAddFlowSavesEntityViaHuhForm(t *testing.T) {
	var captured api.CreateEntityInput
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" && r.Method == http.MethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-1",
					"name": captured.Name,
					"tags": captured.Tags,
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.width = 60

	// Directly set form values and call saveAdd (huh form handles input internally).
	model.addName = "Alpha"
	model.addType = "person"
	model.addStatus = "active"
	model.addTagStr = "alpha"
	model.addScopeStr = "private"

	next, cmd := model.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, next.addSaving)

	msg := cmd()
	next, _ = next.Update(msg)

	assert.True(t, next.addSaved)
	assert.Equal(t, "Alpha", captured.Name)
	assert.Equal(t, "person", captured.Type)
	assert.Equal(t, []string{"alpha"}, captured.Tags)
	assert.Equal(t, []string{"private"}, captured.Scopes)
}

// TestEntitiesAddFlowDedupsTags verifies tag deduplication in saveAdd.
func TestEntitiesAddFlowDedupsTags(t *testing.T) {
	var captured api.CreateEntityInput
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" && r.Method == http.MethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-1",
					"name": captured.Name,
					"tags": captured.Tags,
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.addName = "Alpha"
	model.addType = "person"
	model.addTagStr = "alpha, alpha, beta"

	_, cmd := model.saveAdd()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(entityCreatedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"alpha", "beta"}, captured.Tags)
}

// TestEntitiesBulkUpdateTagsCallsEndpoint handles test entities bulk update tags calls endpoint.
func TestEntitiesBulkUpdateTagsCallsEndpoint(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path == "/api/entities/bulk/tags" && r.Method == http.MethodPost {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotPayload))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"updated": 1, "entity_ids": []string{"ent-1"}},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.width = 60
	model, _ = model.Update(entitiesLoadedMsg{items: []api.Entity{
		{ID: "ent-1", Name: "One", Type: "person", Tags: []string{}},
	}})

	// Select first item.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, 1, model.bulkCount())

	// Open bulk tags prompt.
	model, _ = model.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	assert.NotEmpty(t, model.bulkPrompt)

	// Enter spec and submit.
	for _, r := range "add:alpha" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "/api/entities/bulk/tags", gotPath)
	assert.Equal(t, "add", gotPayload["op"])
	assert.Contains(t, gotPayload["tags"], "alpha")
}

// TestEntitiesBulkUpdateScopesErrorShowsMessage handles test entities bulk update scopes error shows message.
func TestEntitiesBulkUpdateScopesErrorShowsMessage(t *testing.T) {
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities/bulk/scopes" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"BAD_REQUEST","message":"nope"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.width = 60
	model, _ = model.Update(entitiesLoadedMsg{items: []api.Entity{
		{ID: "ent-1", Name: "One", Type: "person", Tags: []string{}},
	}})

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, 1, model.bulkCount())

	model, _ = model.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	require.NotEmpty(t, model.bulkPrompt)

	for _, r := range "add:public" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.NotEmpty(t, model.errText)
	assert.True(t, strings.Contains(model.errText, "BAD_REQUEST"))
}

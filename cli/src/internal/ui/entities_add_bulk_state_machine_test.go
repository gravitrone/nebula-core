package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitiesAddFlowSavesEntityAndDedupsTags handles test entities add flow saves entity and dedups tags.
func TestEntitiesAddFlowSavesEntityAndDedupsTags(t *testing.T) {
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

	// Move focus to mode line and toggle to Add view.
	var cmd tea.Cmd
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, model.modeFocus)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, entitiesViewAdd, model.view)

	// Populate scope options via the same message path the model uses.
	model, _ = model.Update(
		entityScopesLoadedMsg{names: map[string]string{"s1": "public", "s2": "private"}},
	)

	// Name field (focus 0).
	for _, r := range "Alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Type field (focus 1).
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	for _, r := range "person" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Tags field (focus 3).
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown}) // status
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown}) // tags
	for _, r := range "alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit

	// Re-add same tag, should dedup.
	for _, r := range "alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit
	assert.Equal(t, []string{"alpha"}, model.addTags)

	// Scopes field (focus 4): enter selecting mode and select first option.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace}) // enter selecting
	assert.True(t, model.addScopeSelecting)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace}) // toggle selected
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter}) // exit selecting
	assert.False(t, model.addScopeSelecting)
	assert.Contains(t, model.addScopes, "private")

	// Save.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.True(t, model.addSaved)
	assert.Equal(t, "Alpha", captured.Name)
	assert.Equal(t, "person", captured.Type)
	assert.Equal(t, []string{"alpha"}, captured.Tags)
	assert.Equal(t, []string{"private"}, captured.Scopes)
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
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, 1, model.bulkCount())

	// Open bulk tags prompt.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.NotEmpty(t, model.bulkPrompt)

	// Enter spec and submit.
	for _, r := range "add:alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
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

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, 1, model.bulkCount())

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	require.NotEmpty(t, model.bulkPrompt)

	for _, r := range "add:public" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.NotEmpty(t, model.errText)
	assert.True(t, strings.Contains(model.errText, "BAD_REQUEST"))
}

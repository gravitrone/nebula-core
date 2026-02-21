package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// searchTestClient handles search test client.
func searchTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestSearchModelQueryCallsEndpoints handles test search model query calls endpoints.
func TestSearchModelQueryCallsEndpoints(t *testing.T) {
	var entityQuery, contextQuery, jobQuery string
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			entityQuery = r.URL.Query().Get("search_text")
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "alxx", "type": "person"},
				},
			})
			require.NoError(t, err)
		case "/api/context":
			contextQuery = r.URL.Query().Get("search_text")
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "kn-1", "name": "Nebula Notes", "source_type": "note"},
				},
			})
			require.NoError(t, err)
		case "/api/jobs":
			jobQuery = r.URL.Query().Get("search_text")
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "job-1", "title": "Alpha Job", "status": "active"},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "a", entityQuery)
	assert.Equal(t, "a", contextQuery)
	assert.Equal(t, "a", jobQuery)
	assert.Len(t, model.items, 3)
}

// TestSearchModelSelectionEmitsMsg handles test search model selection emits msg.
func TestSearchModelSelectionEmitsMsg(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "alpha", "type": "tool"},
				},
			})
			require.NoError(t, err)
		case "/api/context":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		case "/api/jobs":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msg := cmd()
	model, _ = model.Update(msg)

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	selection := cmd().(searchSelectionMsg)

	assert.Equal(t, "entity", selection.kind)
	require.NotNil(t, selection.entity)
	assert.Equal(t, "ent-1", selection.entity.ID)
}

// TestSearchModelSemanticModeCallsSemanticEndpoint handles test search model semantic mode calls semantic endpoint.
func TestSearchModelSemanticModeCallsSemanticEndpoint(t *testing.T) {
	var semanticQuery string
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/semantic":
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			semanticQuery = payload["query"].(string)
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"kind":     "entity",
						"id":       "ent-1",
						"title":    "Agent Memory Mesh",
						"subtitle": "project",
						"snippet":  "project · memory",
						"score":    0.93,
					},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, searchModeSemantic, model.mode)

	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "m", semanticQuery)
	require.Len(t, model.items, 1)
	assert.Equal(t, "entity", model.items[0].kind)
}

// TestSearchModelSemanticSelectionFetchesDetail handles test search model semantic selection fetches detail.
func TestSearchModelSemanticSelectionFetchesDetail(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/semantic":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"kind":     "entity",
						"id":       "ent-1",
						"title":    "Agent Memory Mesh",
						"subtitle": "project",
						"snippet":  "project · memory",
						"score":    0.93,
					},
				},
			})
			require.NoError(t, err)
		case "/api/entities/ent-1":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-1",
					"name": "Agent Memory Mesh",
					"type": "project",
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	msg := cmd()
	model, _ = model.Update(msg)

	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	selection := cmd().(searchSelectionMsg)
	require.NotNil(t, selection.entity)
	assert.Equal(t, "ent-1", selection.entity.ID)
}

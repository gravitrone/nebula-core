package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSemanticSearch handles test semantic search.
func TestSemanticSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/search/semantic", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "agent memory", payload["query"])
		assert.Equal(t, float64(20), payload["limit"])
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"kind":     "entity",
					"id":       "ent-1",
					"title":    "Agent Memory Mesh",
					"subtitle": "project",
					"snippet":  "project · memory",
					"score":    0.92,
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key")
	items, err := client.SemanticSearch("agent memory", []string{"entity"}, 20)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "ent-1", items[0].ID)
	assert.Equal(t, "entity", items[0].Kind)
}

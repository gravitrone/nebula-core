package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateEntity handles test update entity.
func TestUpdateEntity(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Contains(t, r.URL.Path, "/api/entities/")

		var body UpdateEntityInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))

		_, err := w.Write(jsonResponse(map[string]any{
			"id":   "ent-1",
			"name": body.Name,
			"tags": body.Tags,
		}))
		require.NoError(t, err)
	})

	entity, err := client.UpdateEntity("ent-1", UpdateEntityInput{
		Name: stringPtr("updated name"),
		Tags: stringSlicePtr([]string{"new-tag"}),
	})
	require.NoError(t, err)
	assert.Equal(t, "ent-1", entity.ID)
	assert.Equal(t, "updated name", entity.Name)
}

// TestSearchEntities handles test search entities.
func TestSearchEntities(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/entities/search", r.URL.Path)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.NotNil(t, body["metadata_query"])

		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "1", "name": "match1", "tags": []string{}},
			{"id": "2", "name": "match2", "tags": []string{}},
		}))
		require.NoError(t, err)
	})

	results, err := client.SearchEntities(map[string]any{"role": "professor"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

// TestSearchEntitiesEmpty handles test search entities empty.
func TestSearchEntitiesEmpty(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(jsonResponse([]map[string]any{}))
		require.NoError(t, err)
	})

	results, err := client.SearchEntities(map[string]any{})
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

// TestGetEntityNotFound handles test get entity not found.
func TestGetEntityNotFound(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "NOT_FOUND",
				"message": "entity not found",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.GetEntity("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_FOUND")
}

// TestQueryEntitiesWithMultipleParams handles test query entities with multiple params.
func TestQueryEntitiesWithMultipleParams(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "active", r.URL.Query().Get("status"))
		assert.Equal(t, "person", r.URL.Query().Get("type"))
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "1", "name": "test", "tags": []string{}},
		}))
		require.NoError(t, err)
	})

	entities, err := client.QueryEntities(QueryParams{
		"status": "active",
		"type":   "person",
	})
	require.NoError(t, err)
	assert.Len(t, entities, 1)
}

// TestCreateEntityMissingFields handles test create entity missing fields.
func TestCreateEntityMissingFields(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "VALIDATION_ERROR",
				"message": "missing required field: name",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.CreateEntity(CreateEntityInput{
		Scopes: []string{"public"},
		Type:   "person",
		Status: "active",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VALIDATION_ERROR")
}

// TestGetEntityHistory handles test get entity history.
func TestGetEntityHistory(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/entities/ent-1/history")
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		assert.Equal(t, "0", r.URL.Query().Get("offset"))

		now := time.Now().UTC().Format(time.RFC3339)
			_, err := w.Write(jsonResponse([]map[string]any{
				{
					"id":             "audit-1",
					"table_name":     "entities",
				"record_id":      "ent-1",
				"action":         "update",
				"changed_fields": []string{"tags"},
					"changed_at":     now,
				},
			}))
			require.NoError(t, err)
		})

	rows, err := client.GetEntityHistory("ent-1", 50, 0)
	require.NoError(t, err)
	if assert.Len(t, rows, 1) {
		assert.Equal(t, "audit-1", rows[0].ID)
		assert.Equal(t, "update", rows[0].Action)
	}
}

// TestRevertEntity handles test revert entity.
func TestRevertEntity(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/api/entities/ent-1/revert")

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "audit-1", body["audit_id"])

		_, err := w.Write(jsonResponse(map[string]any{
			"id":   "ent-1",
			"name": "Restored",
			"tags": []string{},
		}))
		require.NoError(t, err)
	})

	entity, err := client.RevertEntity("ent-1", "audit-1")
	require.NoError(t, err)
	assert.Equal(t, "ent-1", entity.ID)
	assert.Equal(t, "Restored", entity.Name)
}

// stringPtr handles string ptr.
func stringPtr(s string) *string {
	return &s
}

// stringSlicePtr handles string slice ptr.
func stringSlicePtr(v []string) *[]string {
	return &v
}

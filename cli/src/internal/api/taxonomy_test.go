package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListTaxonomy handles test list taxonomy.
func TestListTaxonomy(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/taxonomy/scopes", r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("include_inactive"))
		assert.Equal(t, "sdk", r.URL.Query().Get("search"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		assert.Equal(t, "10", r.URL.Query().Get("offset"))

		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "scope-1", "name": "public", "is_active": true, "is_builtin": true},
		}))
		require.NoError(t, err)
	})

	items, err := client.ListTaxonomy("scopes", true, "sdk", 50, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "scope-1", items[0].ID)
	assert.Equal(t, "public", items[0].Name)
}

// TestCreateTaxonomy handles test create taxonomy.
func TestCreateTaxonomy(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/taxonomy/entity-types", r.URL.Path)

		var body CreateTaxonomyInput
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "document", body.Name)

		_, err = w.Write(jsonResponse(map[string]any{
			"id":         "et-1",
			"name":       "document",
			"is_active":  true,
			"is_builtin": false,
		}))
		require.NoError(t, err)
	})

	row, err := client.CreateTaxonomy("entity-types", CreateTaxonomyInput{Name: "document"})
	require.NoError(t, err)
	assert.Equal(t, "et-1", row.ID)
}

// TestUpdateTaxonomy handles test update taxonomy.
func TestUpdateTaxonomy(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/taxonomy/relationship-types/rt-1", r.URL.Path)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":           "rt-1",
			"name":         "depends-on",
			"is_active":    true,
			"is_builtin":   false,
			"is_symmetric": false,
		}))
		require.NoError(t, err)
	})

	name := "depends-on"
	row, err := client.UpdateTaxonomy("relationship-types", "rt-1", UpdateTaxonomyInput{
		Name: &name,
	})
	require.NoError(t, err)
	assert.Equal(t, "rt-1", row.ID)
	assert.Equal(t, "depends-on", row.Name)
}

// TestArchiveAndActivateTaxonomy handles test archive and activate taxonomy.
func TestArchiveAndActivateTaxonomy(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		switch r.URL.Path {
		case "/api/taxonomy/log-types/lt-1/archive":
			_, err := w.Write(jsonResponse(map[string]any{
				"id":         "lt-1",
				"name":       "metric",
				"is_active":  false,
				"is_builtin": false,
			}))
			require.NoError(t, err)
		case "/api/taxonomy/log-types/lt-1/activate":
			_, err := w.Write(jsonResponse(map[string]any{
				"id":         "lt-1",
				"name":       "metric",
				"is_active":  true,
				"is_builtin": false,
			}))
			require.NoError(t, err)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	})

	archived, err := client.ArchiveTaxonomy("log-types", "lt-1")
	require.NoError(t, err)
	assert.False(t, archived.IsActive)

	active, err := client.ActivateTaxonomy("log-types", "lt-1")
	require.NoError(t, err)
	assert.True(t, active.IsActive)
}

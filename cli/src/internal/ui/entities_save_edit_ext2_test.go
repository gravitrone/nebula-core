package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesSaveEditGuardAndScopeErrorBranches(t *testing.T) {
	t.Run("nil detail returns nil cmd", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		updated, cmd := model.saveEdit()
		assert.Nil(t, cmd)
		assert.Nil(t, updated.detail)
	})

	t.Run("bulk scope update error branch", func(t *testing.T) {
		var updateInput api.UpdateEntityInput
		var bulkInput api.BulkUpdateEntityScopesInput

		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/api/entities/") && r.Method == http.MethodPatch:
				require.NoError(t, json.NewDecoder(r.Body).Decode(&updateInput))
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"id":         "ent-1",
						"name":       "Alpha",
						"type":       "person",
						"status":     "active",
						"tags":       []string{"alpha", "beta-tag"},
						"created_at": time.Now().UTC(),
						"updated_at": time.Now().UTC(),
					},
				}))
				return
			case r.URL.Path == "/api/entities/bulk/scopes" && r.Method == http.MethodPost:
				require.NoError(t, json.NewDecoder(r.Body).Decode(&bulkInput))
				http.Error(w, `{"error":{"code":"BULK_SCOPES_FAILED","message":"bulk failed"}}`, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.detail = &api.Entity{ID: "ent-1", Status: "active"}
		model.editStatus = "active"
		model.editTagStr = "alpha, Beta Tag"
		model.editScopeStr = "Public Scope"
		// Scopes differ from original (empty), so editScopesDirty will be set.

		updated, cmd := model.saveEdit()
		require.NotNil(t, cmd)
		assert.True(t, updated.editSaving)

		msg := cmd()
		errOut, ok := msg.(errMsg)
		require.True(t, ok)
		assert.ErrorContains(t, errOut.err, "BULK_SCOPES_FAILED")

		require.NotNil(t, updateInput.Tags)
		assert.Equal(t, []string{"alpha", "beta-tag"}, *updateInput.Tags)

		assert.Equal(t, []string{"ent-1"}, bulkInput.EntityIDs)
		assert.Equal(t, []string{"public-scope"}, bulkInput.Scopes)
		assert.Equal(t, "set", bulkInput.Op)
	})

	t.Run("update entity error branch", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/entities/") && r.Method == http.MethodPatch {
				http.Error(w, `{"error":{"code":"UPDATE_ENTITY_FAILED","message":"update failed"}}`, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.detail = &api.Entity{ID: "ent-1", Status: "active"}
		model.editStatus = "active"
		model.editTagStr = "alpha"
		model.editScopeStr = ""

		updated, cmd := model.saveEdit()
		require.NotNil(t, cmd)
		assert.True(t, updated.editSaving)

		msg := cmd()
		errOut, ok := msg.(errMsg)
		require.True(t, ok)
		assert.ErrorContains(t, errOut.err, "UPDATE_ENTITY_FAILED")
	})

	t.Run("get entity error after bulk update", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasPrefix(r.URL.Path, "/api/entities/") && r.Method == http.MethodPatch:
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"id":         "ent-1",
						"name":       "Alpha",
						"type":       "person",
						"status":     "active",
						"tags":       []string{"alpha"},
						"created_at": time.Now().UTC(),
						"updated_at": time.Now().UTC(),
					},
				}))
				return
			case r.URL.Path == "/api/entities/bulk/scopes" && r.Method == http.MethodPost:
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"updated_count": 1, "entity_ids": []string{"ent-1"}},
				}))
				return
			case r.URL.Path == "/api/entities/ent-1" && r.Method == http.MethodGet:
				http.Error(w, `{"error":{"code":"GET_ENTITY_FAILED","message":"get failed"}}`, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.detail = &api.Entity{ID: "ent-1", Status: "active"}
		model.editStatus = "active"
		model.editTagStr = "alpha"
		model.editScopeStr = "public"
		// Scopes differ from original (empty), so editScopesDirty will be set.

		updated, cmd := model.saveEdit()
		require.NotNil(t, cmd)
		assert.True(t, updated.editSaving)

		msg := cmd()
		errOut, ok := msg.(errMsg)
		require.True(t, ok)
		assert.ErrorContains(t, errOut.err, "GET_ENTITY_FAILED")
	})
}

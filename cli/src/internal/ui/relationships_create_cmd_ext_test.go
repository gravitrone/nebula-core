package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesCreateRelationshipErrorBranch(t *testing.T) {
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/relationships" && r.Method == http.MethodPost {
			http.Error(w, `{"error":{"code":"REL_CREATE_FAILED","message":"create failed"}}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	cmd := model.createRelationship(
		api.Entity{ID: "ent-1"},
		api.Entity{ID: "ent-2"},
		"depends-on",
	)
	require.NotNil(t, cmd)
	msg := cmd()

	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "REL_CREATE_FAILED")
}

func TestRelationshipsCreateRelationshipCommandMatrix(t *testing.T) {
	t.Run("success path emits saved msg", func(t *testing.T) {
		var captured api.CreateRelationshipInput
		_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/relationships" && r.Method == http.MethodPost {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"id": "rel-1"},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewRelationshipsModel(client)
		source := relationshipCreateCandidate{ID: "ctx-1", NodeType: "context"}
		target := relationshipCreateCandidate{ID: "job-1", NodeType: "job"}

		cmd := model.createRelationship(source, target, "references")
		require.NotNil(t, cmd)
		msg := cmd()
		_, ok := msg.(relTabSavedMsg)
		require.True(t, ok)

		assert.Equal(t, "context", captured.SourceType)
		assert.Equal(t, "ctx-1", captured.SourceID)
		assert.Equal(t, "job", captured.TargetType)
		assert.Equal(t, "job-1", captured.TargetID)
		assert.Equal(t, "references", captured.Type)
	})

	t.Run("error path emits err msg", func(t *testing.T) {
		_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/relationships" && r.Method == http.MethodPost {
				http.Error(w, `{"error":{"code":"REL_POST_FAILED","message":"post failed"}}`, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewRelationshipsModel(client)
		source := relationshipCreateCandidate{ID: "ent-1", NodeType: "entity"}
		target := relationshipCreateCandidate{ID: "ent-2", NodeType: "entity"}

		cmd := model.createRelationship(source, target, "depends-on")
		require.NotNil(t, cmd)
		msg := cmd()
		errOut, ok := msg.(errMsg)
		require.True(t, ok)
		assert.ErrorContains(t, errOut.err, "REL_POST_FAILED")
	})
}

func TestRelationshipsSelectedRelationshipEmptyAndWhitespaceFilterBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)

	assert.Nil(t, model.selectedRelationship())

	model.allItems = []api.Relationship{{ID: "rel-1", Type: "depends-on"}}
	model.filterInput.SetValue("   ")
	model.applyListFilter()
	require.Len(t, model.items, 1)
	assert.Equal(t, "rel-1", model.items[0].ID)

	model.filterInput.SetValue(strings.Repeat(" ", 4))
	model.applyListFilter()
	require.Len(t, model.items, 1)
}

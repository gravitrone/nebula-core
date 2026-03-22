package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextUpdateLinkSearchClearsAndStartsLoading(t *testing.T) {
	model := NewContextModel(nil)
	model.linkQuery = "   "
	model.linkLoading = true
	model.linkResults = []api.Entity{{ID: "ent-1"}}
	model.linkTable.SetRows([]table.Row{{"existing"}})

	cmd := model.updateLinkSearch()
	assert.Nil(t, cmd)
	assert.False(t, model.linkLoading)
	assert.Nil(t, model.linkResults)
	assert.Empty(t, model.linkTable.Rows())

	model.linkQuery = "alpha"
	cmd = model.updateLinkSearch()
	require.NotNil(t, cmd)
	assert.True(t, model.linkLoading)
}

func TestContextSearchLinkEntitiesSuccessAndError(t *testing.T) {
	now := time.Now().UTC()
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			if r.URL.Query().Get("search_text") == "broken" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id":         "ent-1",
					"name":       "Alpha",
					"type":       "project",
					"status":     "active",
					"tags":       []string{},
					"created_at": now,
					"updated_at": now,
				}},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	model := NewContextModel(client)
	msg := model.searchLinkEntities("Alpha")().(contextLinkResultsMsg) //nolint:forcetypeassert
	require.Len(t, msg.items, 1)
	assert.Equal(t, "ent-1", msg.items[0].ID)

	msgAny := model.searchLinkEntities("broken")()
	_, ok := msgAny.(errMsg)
	assert.True(t, ok)
}

func TestContextAddLinkedEntitySkipsDuplicates(t *testing.T) {
	model := NewContextModel(nil)
	model.linkEntities = []api.Entity{{ID: "ent-1", Name: "Alpha"}}

	model.addLinkedEntity(api.Entity{ID: "ent-1", Name: "Alpha Again"})
	require.Len(t, model.linkEntities, 1)

	model.addLinkedEntity(api.Entity{ID: "ent-2", Name: "Beta"})
	require.Len(t, model.linkEntities, 2)
	assert.Equal(t, "ent-2", model.linkEntities[1].ID)
}

package ui

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesRenderAddShowsFormView(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 90
	model.initAddForm()
	_ = model.addForm.Init()

	out := components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Type")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "Scopes")
}

func TestEntitiesRenderAddNilFormShowsInitializing(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 90
	model.addForm = nil

	out := components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Initializing")
}

func TestEntitiesSaveAddValidationAndDefaults(t *testing.T) {
	t.Run("name and type validation errors", func(t *testing.T) {
		model := NewEntitiesModel(nil)

		next, cmd := model.saveAdd()
		assert.Nil(t, cmd)
		assert.Equal(t, "Name is required", next.errText)

		model.addName = "Alpha"
		next, cmd = model.saveAdd()
		assert.Nil(t, cmd)
		assert.Equal(t, "Type is required", next.errText)
	})

	t.Run("successful save defaults scopes and parses comma tags", func(t *testing.T) {
		var captured api.CreateEntityInput
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/entities" && r.Method == http.MethodPost {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"id":     "ent-1",
						"name":   captured.Name,
						"type":   captured.Type,
						"status": captured.Status,
						"tags":   captured.Tags,
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.addName = "Alpha"
		model.addType = "person"
		model.addStatus = "inactive"
		model.addTagStr = "alpha"
		model.addScopeStr = ""

		next, cmd := model.saveAdd()
		require.NotNil(t, cmd)
		assert.True(t, next.addSaving)

		msg := cmd()
		created, ok := msg.(entityCreatedMsg)
		require.True(t, ok)
		assert.Equal(t, "ent-1", created.entity.ID)
		assert.Equal(t, []string{"private"}, captured.Scopes)
		assert.Equal(t, "inactive", captured.Status)
		assert.Equal(t, []string{"alpha"}, captured.Tags)
	})

	t.Run("api create error returns errMsg", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/entities" && r.Method == http.MethodPost {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "INTERNAL_ERROR",
						"message": "db down",
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.addName = "Alpha"
		model.addType = "person"

		next, cmd := model.saveAdd()
		require.NotNil(t, cmd)
		assert.True(t, next.addSaving)

		msg := cmd()
		_, ok := msg.(errMsg)
		assert.True(t, ok)
	})
}

func TestParseCommaSeparatedAndDedup(t *testing.T) {
	assert.Equal(t, []string{"alpha", "beta"}, parseCommaSeparated("alpha, beta"))
	assert.Nil(t, parseCommaSeparated(""))
	assert.Nil(t, parseCommaSeparated("  ,  ,  "))
	assert.Equal(t, []string{"a"}, parseCommaSeparated("a"))

	assert.Equal(t, []string{"a", "b"}, dedup([]string{"a", "b", "a"}))
	assert.Nil(t, dedup(nil))
	assert.Nil(t, dedup([]string{"", ""}))
}

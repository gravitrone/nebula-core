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

func TestEntitiesRenderAddBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 90
	model.scopeOptions = []string{"public", "private"}
	model.addStatusIdx = 1
	model.addTags = []string{"alpha"}
	model.addScopes = []string{"public"}
	model.addMeta.Buffer = `{"profile":{"name":"Alpha"}}`
	model.addFields[addFieldName].value = "Alpha"
	model.addFields[addFieldType].value = "person"

	// Focus each branch at least once so renderAdd executes all switch paths.
	model.addFocus = addFieldStatus
	out := components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Status:")
	assert.Contains(t, out, "inactive")

	model.addFocus = addFieldTags
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Tags:")
	assert.Contains(t, out, "alpha")

	model.addFocus = addFieldScopes
	model.addScopeSelecting = true
	model.addScopeIdx = 0
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Scopes:")
	assert.Contains(t, out, "public")
	model.addScopeSelecting = false
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Scopes:")
	assert.Contains(t, out, "public")

	model.addFocus = addFieldMetadata
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Metadata:")
	assert.Contains(t, out, "profile")

	model.addFocus = addFieldName
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Name:")
	assert.Contains(t, out, "Alpha")

	// Unfocused default field with empty value should show "-".
	model.addFocus = addFieldType
	model.addFields[addFieldName].value = ""
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Name:")
	assert.Contains(t, out, "-")

	// Empty metadata preview fallback branch.
	model.addMeta.Buffer = ""
	model.addMeta.Scopes = nil
	model.addFocus = addFieldMetadata
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Metadata:")
	assert.Contains(t, out, "-")

	model.errText = "boom"
	out = components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Error")
	assert.Contains(t, out, "boom")
}

func TestEntitiesSaveAddValidationAndDefaults(t *testing.T) {
	t.Run("name type and metadata validation errors", func(t *testing.T) {
		model := NewEntitiesModel(nil)

		next, cmd := model.saveAdd()
		assert.Nil(t, cmd)
		assert.Equal(t, "Name is required", next.errText)

		model.addFields[addFieldName].value = "Alpha"
		next, cmd = model.saveAdd()
		assert.Nil(t, cmd)
		assert.Equal(t, "Type is required", next.errText)

		model.addFields[addFieldType].value = "person"
		model.addMeta.Buffer = "{"
		next, cmd = model.saveAdd()
		assert.Nil(t, cmd)
		assert.NotEmpty(t, next.errText)
	})

	t.Run("successful save defaults scopes and commits tag buffer", func(t *testing.T) {
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
		model.addFields[addFieldName].value = "Alpha"
		model.addFields[addFieldType].value = "person"
		model.addStatusIdx = 1
		model.addTagBuf = "alpha"
		model.addScopes = nil
		model.addMeta.Buffer = ""

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
		model.addFields[addFieldName].value = "Alpha"
		model.addFields[addFieldType].value = "person"

		next, cmd := model.saveAdd()
		require.NotNil(t, cmd)
		assert.True(t, next.addSaving)

		msg := cmd()
		_, ok := msg.(errMsg)
		assert.True(t, ok)
	})
}

func TestEntitiesRenderAddTagsMultipleUnfocusedSpacing(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.addTags = []string{"alpha", "beta"}

	rendered := components.SanitizeText(model.renderAddTags(false))
	assert.Contains(t, rendered, "alpha")
	assert.Contains(t, rendered, "beta")
}

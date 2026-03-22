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

func TestContextSaveValidationBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.fields[fieldTitle].value = "   "
	updated, cmd := model.save()
	assert.Nil(t, cmd)
	assert.Contains(t, updated.errText, "Title is required")

	model = NewContextModel(nil)
	model.fields[fieldTitle].value = "Alpha"
	updated, cmd = model.save()
	assert.NotNil(t, cmd)
	assert.Equal(t, "", updated.errText)
	assert.True(t, updated.saving)
}

func TestContextSaveCreateAndLinkErrorBranches(t *testing.T) {
	t.Run("create context error", func(t *testing.T) {
		_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/context" && r.Method == http.MethodPost {
				http.Error(w, `{"error":{"code":"CREATE_CONTEXT_FAILED","message":"create failed"}}`, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewContextModel(client)
		model.fields[fieldTitle].value = "Alpha"
		model.fields[fieldNotes].value = "Notes"

		updated, cmd := model.save()
		require.NotNil(t, cmd)
		assert.True(t, updated.saving)

		msg := cmd()
		errOut, ok := msg.(errMsg)
		require.True(t, ok)
		assert.ErrorContains(t, errOut.err, "CREATE_CONTEXT_FAILED")
	})

	t.Run("link context error with default private scope", func(t *testing.T) {
		var created api.CreateContextInput
		var linkedID string
		var linkedType string
		_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/api/context" && r.Method == http.MethodPost:
				require.NoError(t, json.NewDecoder(r.Body).Decode(&created))
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"id": "ctx-1", "title": "Alpha", "source_type": "note"},
				}))
				return
			case strings.HasPrefix(r.URL.Path, "/api/context/") && strings.HasSuffix(r.URL.Path, "/link"):
				var body map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				linkedID = body["owner_id"]
				linkedType = body["owner_type"]
				http.Error(w, `{"error":{"code":"LINK_CONTEXT_FAILED","message":"link failed"}}`, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewContextModel(client)
		model.fields[fieldTitle].value = "Alpha"
		model.fields[fieldNotes].value = "Notes"
		model.tagInput.SetValue("Core Tag")
		model.linkEntities = []api.Entity{{ID: "ent-1", Name: "Entity One"}}

		updated, cmd := model.save()
		require.NotNil(t, cmd)
		assert.True(t, updated.saving)

		msg := cmd()
		errOut, ok := msg.(errMsg)
		require.True(t, ok)
		assert.ErrorContains(t, errOut.err, "LINK_CONTEXT_FAILED")

		assert.Equal(t, []string{"private"}, created.Scopes)
		assert.Equal(t, []string{"core-tag"}, created.Tags)
		assert.Equal(t, "ent-1", linkedID)
		assert.Equal(t, "entity", linkedType)
	})
}

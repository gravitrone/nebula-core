package ui

import (
	"encoding/json"
	"net/http"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolsRenderPreviewAddAndEditEdgeBranches(t *testing.T) {
	t.Run("preview returns empty on non-positive width", func(t *testing.T) {
		model := NewProtocolsModel(nil)
		out := model.renderProtocolPreview(api.Protocol{Name: "alpha", Title: "Alpha"}, 0)
		assert.Equal(t, "", out)
	})

	t.Run("add and edit up keys wrap to final field", func(t *testing.T) {
		model := NewProtocolsModel(nil)
		model.addFocus = 0

		next, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Nil(t, cmd)
		assert.Equal(t, protoFieldCount-1, next.addFocus)

		next.editFocus = 0
		next, cmd = next.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Nil(t, cmd)
		assert.Equal(t, protoEditFieldCount-1, next.editFocus)
	})

	t.Run("render add shows addErr line", func(t *testing.T) {
		model := NewProtocolsModel(nil)
		model.width = 92
		model.addErr = "boom"

		out := components.SanitizeText(model.renderAdd())
		assert.Contains(t, out, "boom")
	})

	t.Run("startEdit is no-op when detail is nil", func(t *testing.T) {
		model := NewProtocolsModel(nil)
		model.editFields[protoEditFieldTitle].value = "keep"

		model.startEdit()
		assert.Equal(t, "keep", model.editFields[protoEditFieldTitle].value)
	})
}

func TestProtocolsSaveErrorBranches(t *testing.T) {
	t.Run("saveAdd returns errMsg when create fails", func(t *testing.T) {
		_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/protocols" && r.Method == http.MethodPost {
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

		model := NewProtocolsModel(client)
		model.addFields[protoFieldName].value = "alpha"
		model.addFields[protoFieldTitle].value = "Alpha"
		model.addFields[protoFieldContent].value = "rules"

		next, cmd := model.saveAdd()
		require.NotNil(t, cmd)
		assert.True(t, next.addSaving)

		msg := cmd()
		_, ok := msg.(errMsg)
		assert.True(t, ok)
	})

	t.Run("saveEdit returns errMsg when update fails", func(t *testing.T) {
		_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/protocols/alpha" && r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "INTERNAL_ERROR",
						"message": "write failed",
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewProtocolsModel(client)
		model.detail = &api.Protocol{
			ID:     "proto-1",
			Name:   "alpha",
			Title:  "Alpha",
			Status: "active",
		}
		model.editFields[protoEditFieldTitle].value = "Alpha"
		model.editFields[protoEditFieldContent].value = "rules"
		model.editStatusIdx = 0

		next, cmd := model.saveEdit()
		require.NotNil(t, cmd)
		assert.True(t, next.editSaving)

		msg := cmd()
		_, ok := msg.(errMsg)
		assert.True(t, ok)
	})
}

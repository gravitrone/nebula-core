package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitiesHandleEditScopesHelpers handles test entities handle edit scopes helpers.
func TestEntitiesHandleEditScopesHelpers(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.scopeOptions = []string{"public", "private"}
	model.editFocus = editFieldScopes
	model.editScopeSelecting = true
	model.editScopeIdx = 0
	model.editScopes = []string{"public"}

	updated, _ := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, updated.editScopeIdx)

	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.True(t, updated.editScopesDirty)
	assert.Contains(t, updated.editScopes, "private")

	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, updated.editScopeSelecting)

}

// TestEntitiesRelationshipRenderAndEditHelpers handles test entities relationship render and edit helpers.
func TestEntitiesRelationshipRenderAndEditHelpers(t *testing.T) {
	now := time.Now()
	model := NewEntitiesModel(nil)
	model.width = 96
	model.detail = &api.Entity{
		ID:     "ent-1",
		Name:   "Alpha",
		Type:   "person",
		Status: "active",
	}
	model.rels = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "entity",
			SourceID:   "ent-1",
			SourceName: "Alpha",
			TargetType: "entity",
			TargetID:   "ent-2",
			TargetName: "Beta",
			Type:       "related-to",
			Status:     "active",
			CreatedAt:  now,
		},
	}
	model.relList.SetItems([]string{"related-to"})

	out := components.SanitizeText(model.renderRelationships())
	assert.Contains(t, out, "Direction")
	assert.Contains(t, out, "Beta")

	model.view = entitiesViewRelateSelect
	model.relateResults = []api.Entity{
		{
			ID:     "ent-2",
			Name:   "Beta",
			Type:   "person",
			Status: "active",
			Tags:   []string{"core"},
		},
	}
	model.relateList.SetItems([]string{"Beta"})
	out = components.SanitizeText(model.renderRelate())
	assert.Contains(t, out, "Beta")

	preview := components.SanitizeText(model.renderRelateEntityPreview(model.relateResults[0], 48))
	assert.Contains(t, preview, "Selected")
	assert.Contains(t, preview, "Type")
	assert.Contains(t, preview, "Status")

	model.startRelEdit()
	model.view = entitiesViewRelEdit
	edit := components.SanitizeText(model.renderRelEdit())
	assert.Contains(t, edit, "Properties")
}

// TestEntitiesSaveRelEditSendsPatchPayload handles test entities save rel edit sends patch payload.
func TestEntitiesSaveRelEditSendsPatchPayload(t *testing.T) {
	now := time.Now()
	var captured api.UpdateRelationshipInput
	var capturedID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodPatch {
			capturedID = strings.TrimPrefix(r.URL.Path, "/api/relationships/")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                capturedID,
				"source_type":       "entity",
				"source_id":         "ent-1",
				"target_type":       "entity",
				"target_id":         "ent-2",
				"relationship_type": "related-to",
				"status":            "active",
				"properties":        map[string]any{"note": "ok"},
				"created_at":        now,
			}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	model := NewEntitiesModel(api.NewClient(srv.URL, "test-key"))
	model.relEditID = "rel-1"
	model.relEditStatusIdx = 0
	model.relEditBuf = `{"note":"ok"}`

	updated, cmd := model.saveRelEdit()
	require.NotNil(t, cmd)
	msg := cmd()
	relMsg, ok := msg.(relationshipUpdatedMsg)
	require.True(t, ok)
	assert.Equal(t, "rel-1", relMsg.rel.ID)
	assert.Equal(t, entitiesViewRelationships, updated.view)

	assert.Equal(t, "rel-1", capturedID)
	require.NotNil(t, captured.Status)
	assert.Equal(t, "active", *captured.Status)
	assert.Equal(t, "ok", captured.Properties["note"])
}

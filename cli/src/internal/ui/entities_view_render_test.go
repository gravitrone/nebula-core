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

func TestEntitiesViewListRendersModeLineCountAndBulkSelection(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 80

	items := []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "person"},
		{ID: "ent-2", Name: "Beta", Type: "tool"},
	}
	model, _ = model.Update(entitiesLoadedMsg{items: items})
	model.bulkSelected["ent-2"] = true

	out := model.View()
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "Entities")
	assert.Contains(t, clean, "2 total")
	assert.Contains(t, clean, "Alpha")
	assert.Contains(t, clean, "Beta")
	assert.Contains(t, clean, "selected: 1")
	assert.Contains(t, clean, "Add")
	assert.Contains(t, clean, "Library")
	assert.Contains(t, clean, "[X]")
}

func TestEntitiesViewAddSavedResetsOnEsc(t *testing.T) {
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" && r.Method == http.MethodPost {
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-1",
					"name": "Alpha",
					"type": "person",
					"tags": []string{},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.width = 80

	// Enter add view.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, entitiesViewAdd, model.view)

	// Name.
	for _, r := range []rune("Alpha") {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Type.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	for _, r := range []rune("person") {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.True(t, model.addSaved)
	assert.Contains(t, components.SanitizeText(model.View()), "Entity saved!")

	// Esc should clear addSaved and reset fields.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, model.addSaved)
	assert.Empty(t, model.addFields[addFieldName].value)
	assert.Empty(t, model.addFields[addFieldType].value)
}

func TestEntitiesSearchInputEnterTriggersQueryAndResetsBuffer(t *testing.T) {
	var searchText string
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			searchText = r.URL.Query().Get("search_text")
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.width = 80
	model.view = entitiesViewSearch

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "a", model.searchBuf)

	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "a", searchText)
	assert.Equal(t, entitiesViewList, model.view)
	assert.Empty(t, model.searchBuf)
}

func TestEntitiesDetailViewRendersMetadataWhenExpanded(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 80
	model.view = entitiesViewDetail
	model.detail = &api.Entity{
		ID:              "ent-1",
		Name:            "Alpha",
		Type:            "person",
		Status:          "active",
		Tags:            []string{"t1"},
		PrivacyScopeIDs: []string{"s1"},
		Metadata:        api.JSONMap{"note": "hello"},
	}
	model.metaExpanded = true

	out := model.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Alpha")
	assert.Contains(t, clean, "Metadata")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, clean, "note")
	assert.Contains(t, clean, "hello")
}

func TestEntitiesSelectedPreviewFormatsScopesAndMetadataSnippet(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.scopeNames = map[string]string{
		"scope-public": "public",
		"scope-admin":  "admin",
	}

	items := []api.Entity{
		{
			ID:              "ent-1",
			Name:            "Alpha",
			Type:            "person",
			Status:          "active",
			PrivacyScopeIDs: []string{"scope-public", "scope-admin"},
			Metadata: api.JSONMap{
				"context_segments": []any{
					map[string]any{"text": "private note", "scopes": []any{"public"}},
				},
			},
		},
	}
	model, _ = model.Update(entitiesLoadedMsg{items: items})

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "Preview")
	assert.NotContains(t, out, "map[")
}

func TestEntitiesFormMetadataUsesStructuredPreviewTable(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.addFocus = addFieldMetadata
	model.addMeta.Buffer = "profile | timezone | Europe/Warsaw"

	addView := components.SanitizeText(model.renderAdd())
	assert.Contains(t, addView, "Group")
	assert.Contains(t, addView, "Field")
	assert.Contains(t, addView, "Value")
	assert.Contains(t, addView, "profile")
	assert.Contains(t, addView, "timezone")
	assert.Contains(t, addView, "Europe/Warsaw")

	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	model.editFocus = editFieldMetadata
	model.editMeta.Buffer = "ops | board | nebula-core"

	editView := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editView, "Group")
	assert.Contains(t, editView, "Field")
	assert.Contains(t, editView, "Value")
	assert.Contains(t, editView, "ops")
	assert.Contains(t, editView, "board")
	assert.Contains(t, editView, "nebula-core")
}

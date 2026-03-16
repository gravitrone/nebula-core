package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitiesViewListRendersModeLineCountAndBulkSelection handles test entities view list renders mode line count and bulk selection.
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

	assert.Contains(t, clean, "2 total")
	assert.Contains(t, clean, "Alpha")
	assert.Contains(t, clean, "Beta")
	assert.Contains(t, clean, "selected: 1")
	assert.Contains(t, clean, "Add")
	assert.Contains(t, clean, "Library")
	assert.Contains(t, clean, "[X]")
}

// TestEntitiesViewAddSavedResetsOnEsc handles test entities view add saved resets on esc.
func TestEntitiesViewAddSavedResetsOnEsc(t *testing.T) {
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" && r.Method == http.MethodPost {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-1",
					"name": "Alpha",
					"type": "person",
					"tags": []string{},
				},
			}))
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
	for _, r := range "Alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Type.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	for _, r := range "person" {
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

// TestEntitiesSearchInputEnterTriggersQueryAndResetsBuffer handles test entities search input enter triggers query and resets buffer.
func TestEntitiesSearchInputEnterTriggersQueryAndResetsBuffer(t *testing.T) {
	var searchText string
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			searchText = r.URL.Query().Get("search_text")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{},
			}))
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

// TestEntitiesDetailViewRendersContextSummary handles test entities detail view renders context summary.
func TestEntitiesDetailViewRendersContextSummary(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 80
	model.view = entitiesViewDetail
	model.detail = &api.Entity{
		ID:     "ent-1",
		Name:   "Alpha",
		Type:   "person",
		Status: "active",
	}
	model.contextLoading = false
	model.detailContext = []api.Context{
		{ID: "ctx-1", Title: "Onboarding Notes", SourceType: "note", Status: "active"},
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Onboarding Notes")
	assert.Contains(t, out, "note")
	assert.Contains(t, out, "active")
}

// TestEntitiesSelectedPreviewIncludesScopesAndContextCount handles test entities selected preview includes scopes and context count.
func TestEntitiesSelectedPreviewIncludesScopesAndContextCount(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.scopeNames = map[string]string{
		"scope-public": "public",
		"scope-admin":  "admin",
	}

	item := api.Entity{
		ID:              "ent-1",
		Name:            "Alpha",
		Type:            "person",
		Status:          "active",
		PrivacyScopeIDs: []string{"scope-public", "scope-admin"},
	}
	model.detail = &api.Entity{ID: "ent-1"}
	model.detailRels = []api.Relationship{{ID: "rel-1"}}
	model.detailContext = []api.Context{{ID: "ctx-1"}}

	preview := components.SanitizeText(model.renderEntityPreview(item, 48))
	assert.Contains(t, preview, "Scopes")
	assert.Contains(t, preview, "public")
	assert.Contains(t, preview, "Context")
	assert.Contains(t, preview, "1")
}

func TestEntitiesRenderEntityPreviewEdgeBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 80

	assert.Equal(t, "", model.renderEntityPreview(api.Entity{}, 0))

	entity := api.Entity{
		ID:        "ent-1",
		Name:      "Alpha",
		Type:      "",
		Status:    "active",
		CreatedAt: time.Date(2026, 3, 3, 10, 0, 0, 0, time.UTC),
	}
	model.detail = &api.Entity{ID: "ent-1"}
	model.detailRels = []api.Relationship{{ID: "rel-1"}, {ID: "rel-2"}}

	preview := components.SanitizeText(model.renderEntityPreview(entity, 42))
	assert.Contains(t, preview, "Type")
	assert.Contains(t, preview, "?")
	assert.Contains(t, preview, "Links")
	assert.Contains(t, preview, "2")
}

func TestEntitiesRenderListClampsColumnsOnNarrowWidth(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 22

	model, _ = model.Update(entitiesLoadedMsg{items: []api.Entity{
		{
			ID:     "ent-1",
			Name:   "Alpha with a long display name",
			Type:   "person",
			Status: "active",
		},
	}})

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Status")
}

func TestEntitiesRenderDetailFallsBackToListWhenDetailMissing(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 80

	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "No entities found.")
}

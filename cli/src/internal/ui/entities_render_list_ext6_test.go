package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestEntitiesRenderListLoadingAndEmptyBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 120
	model.loading = true

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Loading entities")

	model.loading = false
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "No entities found")
	assert.Contains(t, out, "Type to live-search")
}

func TestEntitiesRenderListCountLineSelectionAndPreviewBranches(t *testing.T) {
	now := time.Now().UTC()
	model := NewEntitiesModel(nil)
	model.width = 180
	model.scopeNames = map[string]string{"scope-1": "public"}
	model.items = []api.Entity{
		{
			ID:              "ent-1",
			Name:            "Alpha",
			Type:            "person",
			Status:          "active",
			UpdatedAt:       now,
			Tags:            []string{"demo"},
			PrivacyScopeIDs: []string{"scope-1"},
		},
		{
			ID:        "ent-2",
			Name:      "",
			Type:      "",
			Status:    "",
			CreatedAt: now,
		},
	}
	model.list.SetItems([]string{"row-1", "row-2", "phantom-row"})
	model.list.Cursor = 0
	model.searchBuf = "alpha"
	model.searchSuggest = "alphabet"
	model.filterTypes = map[string]bool{"person": true}
	model.bulkSelected = map[string]bool{"ent-1": true}

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "2 total · selected: 1 · search: alpha · next: alphabet · filters active")
	assert.Contains(t, out, "[X] Alpha")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Scopes: [public]")

	// Keep query/suggest equal (case-insensitive) and force previewItem=nil branch.
	model.searchSuggest = "ALPHA"
	model.list.Cursor = 99
	model.modeFocus = true
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "2 total · selected: 1 · search: alpha · filters active")
	assert.NotContains(t, out, "next: ")
	assert.NotContains(t, out, "Selected")
}

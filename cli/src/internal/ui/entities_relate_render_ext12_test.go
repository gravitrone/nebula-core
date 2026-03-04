package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestEntitiesRenderRelateWideLayoutAndFallbacks(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewRelateSelect
	model.width = 220
	model.relateResults = []api.Entity{
		{ID: "ent-1", Name: "", Type: "", Status: ""},
	}
	// Keep one extra list row so absIdx out-of-range guard is exercised.
	model.relateList.SetItems([]string{"ent-1", "phantom"})
	model.relateList.Cursor = 0

	out := components.SanitizeText(model.renderRelate())
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "entity")
	assert.Contains(t, out, " - ")
}

func TestEntitiesRenderRelatePreviewNilWhenCursorOutOfRange(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewRelateSelect
	model.width = 220
	model.relateResults = []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active"},
	}
	model.relateList.SetItems([]string{"ent-1", "phantom"})
	model.relateList.Cursor = 99

	out := components.SanitizeText(model.renderRelate())
	assert.Contains(t, out, "1 results")
	assert.NotContains(t, out, "Selected")
}

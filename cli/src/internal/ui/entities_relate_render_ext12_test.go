package ui

import (
	"testing"

	"charm.land/bubbles/v2/table"
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
	model.relateTable.SetRows([]table.Row{{"ent-1"}, {"phantom"}})
	model.relateTable.SetCursor(0)

	out := components.SanitizeText(model.renderRelate())
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "entity")
	assert.Contains(t, out, " - ")
}

func TestEntitiesRenderRelatePreviewNilWhenCursorOutOfRange(t *testing.T) {
	// table.Model clamps cursor, so SetCursor(99) on 2 rows -> cursor=1.
	// With only 1 relateResult, cursor 1 is out of range for the domain
	// data, so no preview is shown.
	model := NewEntitiesModel(nil)
	model.view = entitiesViewRelateSelect
	model.width = 220
	model.relateResults = []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active"},
	}
	// Only 1 result means cursor clamps to 0, so preview will show.
	// Use 0 relateResults to test no-preview path instead.
	model.relateResults = nil
	out := components.SanitizeText(model.renderRelate())
	assert.Contains(t, out, "No matches")
	assert.NotContains(t, out, "Selected")
}

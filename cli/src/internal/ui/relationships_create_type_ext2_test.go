package ui

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipsRenderCreateTypeShowsPreviewForSelectedSuggestion(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createType = "dep"
	model.createTypeResults = []string{"depends-on", "related-to"}
	model.createTypeTable.SetRows([]table.Row{{"depends-on"}, {"related-to"}})
	model.createTypeTable.SetCursor(0)

	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "2 suggestions")
	assert.Contains(t, out, "depends-on")
	assert.Contains(t, out, "Source")
	assert.Contains(t, out, "Target")
}

func TestRelationshipsRenderCreateTypeBlankSuggestionFallbackBranch(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createType = "x"
	model.createTypeResults = []string{""}
	model.createTypeTable.SetRows([]table.Row{{""}})
	model.createTypeTable.SetCursor(0)

	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "1 suggestions")
	assert.Contains(t, out, "-")
	assert.NotContains(t, out, "Source")
	assert.NotContains(t, out, "Target")
}

func TestRelationshipsRenderCreateTypeSkipsOutOfRangeVisibleRows(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createType = "dep"
	model.createTypeResults = []string{"depends-on"}
	// Keep list items longer than createTypeResults so RelToAbs produces
	// out-of-range indexes for part of visible rows.
	model.createTypeTable.SetRows([]table.Row{{"depends-on"}, {"ghost"}, {"ghost-2"}})
	model.createTypeTable.SetCursor(0)

	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "1 suggestions")
	assert.Contains(t, out, "depends-on")
}

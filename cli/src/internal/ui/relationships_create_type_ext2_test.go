package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipsRenderCreateTypeShowsPreviewForSelectedSuggestion(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createType = "dep"
	model.createTypeResults = []string{"depends-on", "related-to"}
	model.createTypeList.SetItems(model.createTypeResults)
	model.createTypeList.Cursor = 0

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
	model.createTypeList.SetItems(model.createTypeResults)
	model.createTypeList.Cursor = 0

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
	model.createTypeList.SetItems([]string{"depends-on", "ghost", "ghost-2"})
	model.createTypeList.Cursor = 0

	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "1 suggestions")
	assert.Contains(t, out, "depends-on")
}

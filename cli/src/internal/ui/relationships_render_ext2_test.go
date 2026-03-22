package ui

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipsRenderCreateNodePreviewGuardAndFallbacks(t *testing.T) {
	model := NewRelationshipsModel(nil)
	assert.Equal(t, "", model.renderCreateNodePreview(relationshipCreateCandidate{}, 0))

	out := components.SanitizeText(model.renderCreateNodePreview(relationshipCreateCandidate{
		ID:       "node-1",
		NodeType: "",
		Name:     "",
		Kind:     "",
		Status:   "",
	}, 48))

	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "node")
	assert.Contains(t, out, "Kind")
	assert.Contains(t, out, "Status")
}

func TestRelationshipsRenderCreateTypePreviewGuardAndFallbacks(t *testing.T) {
	model := NewRelationshipsModel(nil)
	assert.Equal(t, "", model.renderCreateTypePreview("depends-on", 0))

	out := components.SanitizeText(model.renderCreateTypePreview("   ", 48))
	assert.Contains(t, out, "relationship")
	assert.Contains(t, out, "Source")
	assert.Contains(t, out, "Target")
}

func TestRelationshipsRenderCreateSearchWithNarrowWidthStillShowsResults(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 40
	model.createQuery = "alpha"
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "Alpha", Kind: "entity/person", Status: "active"},
	}
	model.createTable.SetRows([]table.Row{{"Alpha"}})

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Alpha")
}

func TestRelationshipsRenderCreateTypeWithSelectedSuggestionPreview(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 88
	model.createType = "dep"
	model.createTypeResults = []string{"depends-on"}
	model.createTypeTable.SetRows([]table.Row{{"depends-on"}})
	model.createTypeTable.SetCursor(0)

	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "1 suggestions")
	// With table.Model, cursor is clamped so preview is always shown.
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "depends-on")
}

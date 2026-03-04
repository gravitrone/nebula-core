package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipsRenderCreateSwitchBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 96

	model.view = relsViewCreateSourceSearch
	_ = components.SanitizeText(model.renderCreate())

	model.view = relsViewCreateTargetSearch
	_ = components.SanitizeText(model.renderCreate())

	model.view = relsViewCreateType
	_ = components.SanitizeText(model.renderCreate())

	model.view = relsViewList
	assert.Equal(t, "", model.renderCreate())
}

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
	model.createList.SetItems([]string{"Alpha"})

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Alpha")
}

func TestRelationshipsRenderCreateTypeWithoutSelectedSuggestionPreview(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 88
	model.createType = "dep"
	model.createTypeResults = []string{"depends-on"}
	model.createTypeList.SetItems([]string{"depends-on"})
	model.createTypeList.Cursor = 9 // out of range, keeps selectedSuggestion empty

	out := components.SanitizeText(model.renderCreateType())
	assert.Contains(t, out, "1 suggestions")
	assert.NotContains(t, out, "Source")
	assert.NotContains(t, out, "Target")
}

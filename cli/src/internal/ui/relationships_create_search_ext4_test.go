package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipsRenderCreateSearchSideBySidePreviewBranch(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createQueryInput.SetValue("alpha")
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "Alpha", Kind: "entity/person", Status: "active"},
		{ID: "ctx-1", NodeType: "context", Name: "Note", Kind: "context/note", Status: "inactive"},
	}
	model.createList.SetItems([]string{"Alpha", "Note"})
	model.createList.Cursor = 0

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "2 results")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "entity/person")
}

func TestRelationshipsRenderCreateSearchNoPreviewWhenCursorOutOfRange(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createQueryInput.SetValue("alpha")
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "Alpha", Kind: "entity/person", Status: "active"},
	}
	model.createList.SetItems([]string{"Alpha", "Ghost"})
	model.createList.Cursor = 99

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Alpha")
	assert.NotContains(t, out, "Selected")
}

func TestRelationshipsRenderCreateSearchVeryNarrowWidthColumnFallbacks(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 30
	model.createQueryInput.SetValue("a")
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "", Name: "", Kind: "", Status: ""},
	}
	model.createList.SetItems([]string{"ent-1"})

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "node")
	assert.Contains(t, out, "-")
}

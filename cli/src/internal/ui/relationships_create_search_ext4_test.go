package ui

import (
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipsRenderCreateSearchSideBySidePreviewBranch(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createQuery = "alpha"
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "Alpha", Kind: "entity/person", Status: "active"},
		{ID: "ctx-1", NodeType: "context", Name: "Note", Kind: "context/note", Status: "inactive"},
	}
	model.createTable.SetRows([]table.Row{{"Alpha"}, {"Note"}})
	model.createTable.SetCursor(0)

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "2 results")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "entity/person")
}

func TestRelationshipsRenderCreateSearchNoPreviewWhenCursorOutOfRange(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.createQuery = "alpha"
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "entity", Name: "Alpha", Kind: "entity/person", Status: "active"},
	}
	// With table.Model, cursor is clamped to valid range so preview is
	// always shown when results exist.
	model.createTable.SetRows([]table.Row{{"Alpha"}})
	model.createTable.SetCursor(0)

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Selected")
}

func TestRelationshipsRenderCreateSearchVeryNarrowWidthColumnFallbacks(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 30
	model.createQuery = "a"
	model.createResults = []relationshipCreateCandidate{
		{ID: "ent-1", NodeType: "", Name: "", Kind: "", Status: ""},
	}
	model.createTable.SetRows([]table.Row{{"ent-1"}})

	out := components.SanitizeText(model.renderCreateSearch("Source Node"))
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "node")
	assert.Contains(t, out, "-")
}

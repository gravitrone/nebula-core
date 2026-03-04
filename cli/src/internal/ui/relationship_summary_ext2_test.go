package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipSummaryEntriesDefaultMaxRowsAndSanitize(t *testing.T) {
	rels := []api.Relationship{
		{Type: "   ", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-2", TargetName: "Two"},
		{Type: "owns", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-3", TargetName: "Three"},
		{Type: "links", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-4", TargetName: "Four"},
		{Type: "refs", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-5", TargetName: "Five"},
		{Type: "tracks", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-6", TargetName: "Six"},
		{Type: "extra", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-7", TargetName: "Seven"},
	}

	entries, extra := relationshipSummaryEntries("entity", "ent-1", rels, 0)
	assert.Len(t, entries, 5)
	assert.Equal(t, 1, extra)
	assert.Equal(t, "-", entries[0].Rel)
	assert.Equal(t, "->", entries[0].Dir)
	assert.Equal(t, "Two", entries[0].Node)
}

func TestRelationshipSummaryColumnWidthsAlwaysRebalanced(t *testing.T) {
	rel, dir, node := relationshipSummaryColumnWidths(10)
	assert.GreaterOrEqual(t, rel, 8)
	assert.GreaterOrEqual(t, dir, 9)
	assert.GreaterOrEqual(t, node, 10)
	assert.Equal(t, 30, rel+dir+node)

	rel, dir, node = relationshipSummaryColumnWidths(220)
	assert.Equal(t, 218, rel+dir+node)
}

func TestRenderRelationshipSummaryTableTinyWidthStillRendersRows(t *testing.T) {
	rels := []api.Relationship{
		{
			Type:       "depends-on",
			SourceType: "entity",
			SourceID:   "ent-1",
			SourceName: "Alpha",
			TargetType: "entity",
			TargetID:   "ent-2",
			TargetName: "Beta",
		},
	}

	out := components.SanitizeText(renderRelationshipSummaryTable("entity", "ent-1", rels, 5, 24))
	assert.Contains(t, out, "depen")
	assert.Contains(t, out, "Direction")
	assert.Contains(t, out, "Node")
}

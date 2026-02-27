package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipSummaryColumnWidthsBounds(t *testing.T) {
	rel, dir, node := relationshipSummaryColumnWidths(10)
	assert.GreaterOrEqual(t, rel, 8)
	assert.GreaterOrEqual(t, dir, 9)
	assert.GreaterOrEqual(t, node, 10)

	rel, dir, node = relationshipSummaryColumnWidths(96)
	assert.Greater(t, rel, 8)
	assert.Greater(t, dir, 9)
	assert.Greater(t, node, 10)
	assert.Equal(t, 94, rel+dir+node) // contentWidth - separators
}

func TestRelationshipNodeLabelMatrix(t *testing.T) {
	assert.Equal(t, "Named Node", relationshipNodeLabel("Named Node", "abcd1234efgh", "entity"))
	assert.Equal(t, "entity:abcd1234", relationshipNodeLabel("", "abcd1234efgh", "entity"))
	assert.Equal(t, "context:abcd1234", relationshipNodeLabel("", "abcd1234efgh", "context"))
	assert.Equal(t, "job:abcd1234", relationshipNodeLabel("", "abcd1234efgh", "job"))
	assert.Equal(t, "log:abcd1234", relationshipNodeLabel("", "abcd1234efgh", "log"))
	assert.Equal(t, "file:abcd1234", relationshipNodeLabel("", "abcd1234efgh", "file"))
	assert.Equal(t, "protocol:abcd1234", relationshipNodeLabel("", "abcd1234efgh", "protocol"))
	assert.Equal(t, "agent:abcd1234", relationshipNodeLabel("", "abcd1234efgh", "agent"))
	assert.Equal(t, "abcd1234", relationshipNodeLabel("", "abcd1234efgh", "other"))
	assert.Equal(t, "unknown", relationshipNodeLabel("", "", "other"))
}

func TestRelationshipDirectionAndEndpointFallback(t *testing.T) {
	rel := api.Relationship{
		SourceType: "entity",
		SourceID:   "ent-1",
		SourceName: "Source",
		TargetType: "job",
		TargetID:   "job-1",
		TargetName: "Target",
	}

	dir, endpoint := relationshipDirectionAndEndpoint("entity", "ent-1", rel)
	assert.Equal(t, "->", dir)
	assert.Equal(t, "Target", endpoint)

	dir, endpoint = relationshipDirectionAndEndpoint("job", "job-1", rel)
	assert.Equal(t, "<-", dir)
	assert.Equal(t, "Source", endpoint)

	dir, endpoint = relationshipDirectionAndEndpoint("protocol", "proto-1", rel)
	assert.Equal(t, "<>", dir)
	assert.Contains(t, endpoint, "Source")
	assert.Contains(t, endpoint, "Target")
}

func TestRenderRelationshipSummaryTableEmptyAndMoreRows(t *testing.T) {
	empty := components.SanitizeText(renderRelationshipSummaryTable("entity", "ent-1", nil, 3, 80))
	assert.Contains(t, empty, "No relationships yet")

	rels := []api.Relationship{
		{Type: "owns", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-2", TargetName: "Two", CreatedAt: time.Now()},
		{Type: "links", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-3", TargetName: "Three", CreatedAt: time.Now()},
		{Type: "refs", SourceType: "entity", SourceID: "ent-1", TargetType: "entity", TargetID: "ent-4", TargetName: "Four", CreatedAt: time.Now()},
	}
	out := stripANSI(renderRelationshipSummaryTable("entity", "ent-1", rels, 2, 90))
	assert.Contains(t, out, "Relationships")
	assert.True(t, strings.Contains(out, "2 more relationships") || strings.Contains(out, "more relationships"))
}

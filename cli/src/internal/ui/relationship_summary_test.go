package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
)

// TestRelationshipSummaryRowsDirection handles test relationship summary rows direction.
func TestRelationshipSummaryRowsDirection(t *testing.T) {
	rels := []api.Relationship{
		{
			Type:       "depends-on",
			SourceType: "job",
			SourceID:   "2026Q1-0001",
			TargetType: "entity",
			TargetID:   "entity-abc",
			TargetName: "Alpha",
		},
	}

	rows := relationshipSummaryRows("job", "2026Q1-0001", rels, 5)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Label != "depends-on 1" {
		t.Fatalf("unexpected label: %q", rows[0].Label)
	}
	if rows[0].Value != "-> Alpha" {
		t.Fatalf("unexpected value: %q", rows[0].Value)
	}
}

// TestRelationshipSummaryRowsShowsMore handles test relationship summary rows shows more.
func TestRelationshipSummaryRowsShowsMore(t *testing.T) {
	rels := []api.Relationship{
		{Type: "rel-a", SourceType: "entity", SourceID: "e-1", TargetType: "entity", TargetID: "e-2"},
		{Type: "rel-b", SourceType: "entity", SourceID: "e-1", TargetType: "entity", TargetID: "e-3"},
		{Type: "rel-c", SourceType: "entity", SourceID: "e-1", TargetType: "entity", TargetID: "e-4"},
	}

	rows := relationshipSummaryRows("entity", "e-1", rels, 2)
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	if rows[2].Label != "More" {
		t.Fatalf("expected trailing More row, got %q", rows[2].Label)
	}
	if rows[2].Value != "+1 relationships" {
		t.Fatalf("unexpected more value: %q", rows[2].Value)
	}
}

// TestRenderRelationshipSummaryTableUsesGridLayout handles test render relationship summary table uses grid layout.
func TestRenderRelationshipSummaryTableUsesGridLayout(t *testing.T) {
	rels := []api.Relationship{
		{
			Type:       "owns",
			SourceType: "entity",
			SourceID:   "ent-1",
			SourceName: "Owner",
			TargetType: "entity",
			TargetID:   "ent-2",
			TargetName: "Target",
		},
	}

	view := renderRelationshipSummaryTable("entity", "ent-1", rels, 5, 120)
	clean := stripANSI(view)

	for _, token := range []string{"Relationships", "Rel", "Direction", "Node", "owns", "->", "Target"} {
		if !strings.Contains(clean, token) {
			t.Fatalf("expected %q in rendered table:\n%s", token, clean)
		}
	}
}

package ui

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRelationshipsRenderListNarrowEdgeCaseBranches(t *testing.T) {
	now := time.Now().UTC()
	model := NewRelationshipsModel(nil)
	model.width = 48
	model.items = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "entity",
			SourceID:   "ent-1",
			TargetType: "entity",
			TargetID:   "ent-2",
			Type:       "",
			Status:     "",
			CreatedAt:  now,
		},
	}
	// Keep one out-of-range row so the absIdx guard branch is exercised.
	model.dataTable.SetRows([]table.Row{{"rel-1"}, {"phantom"}})
	model.dataTable.SetCursor(0)
	model.modeFocus = true
	model.filterBuf = "ent-1"

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "1 total · filter: ent-1")
	assert.Contains(t, out, "unknown entity")
}

func TestRelationshipsRenderListWideSideBySidePreviewBranch(t *testing.T) {
	now := time.Now().UTC()
	model := NewRelationshipsModel(nil)
	model.width = 220
	model.items = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "entity",
			SourceID:   "ent-1",
			SourceName: "Alpha",
			TargetType: "entity",
			TargetID:   "ent-2",
			TargetName: "Beta",
			Type:       "depends-on",
			Status:     "active",
			CreatedAt:  now,
		},
	}
	model.dataTable.SetRows([]table.Row{{"rel-1"}})
	model.dataTable.SetCursor(0)

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "depends-on")
	assert.Contains(t, out, "Alpha -> Beta")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "From:")
	assert.Contains(t, out, "To:")
}

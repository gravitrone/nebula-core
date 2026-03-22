package ui

import (
	"testing"
	"time"

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
	model.list.SetItems([]string{"rel-1", "phantom"})
	model.list.Cursor = 0
	model.modeFocus = true
	model.filterInput.SetValue("ent-1")

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
	model.list.SetItems([]string{"rel-1"})
	model.list.Cursor = 0

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "depends-on")
	assert.Contains(t, out, "Alpha -> Beta")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "From:")
	assert.Contains(t, out, "To:")
}

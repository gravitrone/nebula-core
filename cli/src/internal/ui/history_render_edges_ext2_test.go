package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestHistoryRenderListTinyWidthClampsColumnsAndFallbacksActor(t *testing.T) {
	blankName := "   "
	model := NewHistoryModel(nil)
	model.width = 20
	model, _ = model.Update(historyLoadedMsg{
		items: []api.AuditEntry{
			{
				ID:        "audit-small-1",
				TableName: "",
				Action:    "",
				ActorName: &blankName,
				ChangedAt: time.Now().UTC(),
			},
		},
	})

	out := components.SanitizeText(model.renderList())

	assert.Contains(t, out, "UPDATE")
	assert.Contains(t, out, "system")
	assert.Contains(t, out, "1 total")
}

func TestHistoryRenderAuditPreviewFallsBackForBlankActorAndFields(t *testing.T) {
	blankName := "   "
	model := NewHistoryModel(nil)

	out := components.SanitizeText(model.renderAuditPreview(
		api.AuditEntry{
			ID:        "audit-preview-1",
			TableName: "",
			Action:    "",
			ActorName: &blankName,
			ChangedAt: time.Now().UTC(),
		},
		32,
	))

	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Action")
	assert.Contains(t, out, "UPDATE")
	assert.Contains(t, out, "Table")
	assert.Contains(t, out, "-")
	assert.Contains(t, out, "Actor")
	assert.Contains(t, out, "system")
}

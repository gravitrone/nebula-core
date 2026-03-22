package ui

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesRenderHistoryCoversCompactWidthsAndInvalidVisibleRows(t *testing.T) {
	now := time.Now()
	model := NewEntitiesModel(nil)
	model.width = 28
	model.history = []api.AuditEntry{
		{ID: "a1", Action: "", ChangedAt: now},
	}
	model.historyTable.SetRows([]table.Row{
		{formatHistoryLine(model.history[0])},
		{"orphan-row"},
	})

	out := components.SanitizeText(model.renderHistory())
	assert.Contains(t, out, "1 entries")
	assert.Contains(t, out, "UPDATE")
}

func TestEntitiesRenderHistorySideBySideIncludesReasonPreview(t *testing.T) {
	now := time.Now()
	reason := "manual restore after audit review"
	model := NewEntitiesModel(nil)
	model.width = 220
	model.detail = &api.Entity{Name: "Alpha"}
	model.history = []api.AuditEntry{
		{
			ID:            "a1",
			Action:        "update",
			ChangedAt:     now,
			ChangedFields: []string{"name", "status"},
			ChangeReason:  &reason,
		},
	}
	model.historyTable.SetRows([]table.Row{{formatHistoryLine(model.history[0])}})

	out := components.SanitizeText(model.renderHistory())
	assert.Contains(t, out, "Reason")
	assert.Contains(t, out, "manual restore after audit")
}

func TestRenderEntityHistoryPreviewReturnsEmptyForZeroWidth(t *testing.T) {
	model := NewEntitiesModel(nil)
	out := model.renderEntityHistoryPreview(
		api.AuditEntry{
			ID:        "a1",
			Action:    "update",
			ChangedAt: time.Now(),
		},
		0,
	)
	assert.Equal(t, "", out)
}

func TestEntitiesStartEditNoDetailIsNoop(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.editTagStr = "keep"

	model.startEdit()

	assert.Equal(t, "keep", model.editTagStr)
	require.Nil(t, model.detail)
}

package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryRenderDetailOmitsOptionalSectionsWhenValuesAreEmpty(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 88

	reason := ""
	out := components.SanitizeText(model.renderDetail(api.AuditEntry{
		ID:           "audit-1",
		TableName:    "entities",
		RecordID:     "ent-1",
		Action:       "update",
		ChangedAt:    time.Now().UTC(),
		ChangeReason: &reason,
	}))

	assert.Contains(t, out, "Audit Entry")
	assert.NotContains(t, out, "Fields")
	assert.NotContains(t, out, "Reason")
	assert.NotContains(t, out, "Changes")
}

func TestHistoryRenderDetailKeepsFieldsRowWhenDiffIsEmpty(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 88

	out := components.SanitizeText(model.renderDetail(api.AuditEntry{
		ID:            "audit-2",
		TableName:     "entities",
		RecordID:      "ent-2",
		Action:        "update",
		ChangedAt:     time.Now().UTC(),
		ChangedFields: []string{"status"},
		OldData:       api.JSONMap{"status": "active"},
		NewData:       api.JSONMap{"status": "active"},
	}))

	assert.Contains(t, out, "Audit Entry")
	assert.Contains(t, out, "Fields")
	assert.Contains(t, out, "status")
	assert.NotContains(t, out, "Changes")
}

func TestHistoryRenderListHandlesUnsyncedVisibleRowsAndFallbackValues(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 110
	model.items = []api.AuditEntry{
		{
			ID:        "audit-3",
			TableName: "   ",
			Action:    "   ",
			ChangedAt: time.Now().UTC(),
		},
	}
	// Keep one extra visible row to exercise the out-of-range guard branch.
	model.list.Items = []string{"row-0", "row-1"}
	model.list.Cursor = 0
	model.list.Offset = 0

	out := components.SanitizeText(model.renderList())

	assert.Contains(t, out, "History")
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "UPDATE")
	assert.Contains(t, out, "system")
}

func TestHistoryRenderListSkipsPreviewWhenCursorOutOfRange(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 120
	model, _ = model.Update(historyLoadedMsg{
		items: []api.AuditEntry{
			{
				ID:        "audit-4",
				TableName: "entities",
				RecordID:  "ent-4",
				Action:    "create",
				ChangedAt: time.Now().UTC(),
			},
			{
				ID:        "audit-5",
				TableName: "contexts",
				RecordID:  "ctx-5",
				Action:    "update",
				ChangedAt: time.Now().UTC(),
			},
		},
	})
	model.list.Cursor = 999

	out := components.SanitizeText(model.renderList())

	require.Contains(t, out, "History")
	assert.NotContains(t, out, "Selected")
}

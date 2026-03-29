package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

// TestHistoryViewRendersListAndFiltersLine handles test history view renders list and filters line.
func TestHistoryViewRendersListAndFiltersLine(t *testing.T) {
	now := time.Now()
	model := NewHistoryModel(nil)
	model.width = 80

	model, _ = model.Update(historyLoadedMsg{
		items: []api.AuditEntry{
			{
				ID:        "audit-1",
				TableName: "entities",
				RecordID:  "entity-1",
				Action:    "create",
				OldValues: "",
				NewValues: `{"name": "Alpha"}`,
				ChangedAt: now,
			},
		},
	})

	model.filter = auditFilter{
		tableName: "entities",
		action:    "create",
		scopeID:   "scope-1",
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Filters:")
	assert.Contains(t, out, "table:entities")
	assert.Contains(t, out, "action:create")
	assert.Contains(t, out, "scope:scope-1")
	assert.Contains(t, out, "CREATE")
	assert.Contains(t, out, "entities")
}

// TestHistoryViewRendersScopesActorsAndDetail handles test history view renders scopes actors and detail.
func TestHistoryViewRendersScopesActorsAndDetail(t *testing.T) {
	now := time.Now()
	model := NewHistoryModel(nil)
	model.width = 80

	// Scopes view.
	model.view = historyViewScopes
	model, _ = model.Update(historyScopesLoadedMsg{
		items: []api.AuditScope{
			{ID: "scope-1", Name: "public", AgentCount: 1, EntityCount: 2, ContextCount: 3},
		},
	})
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "public")

	// Actors view.
	model.view = historyViewActors
	model, _ = model.Update(historyActorsLoadedMsg{
		items: []api.AuditActor{
			{ActorType: "agent", ActorID: "agent-1", ActionCount: 42, LastSeen: now},
		},
	})
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "agent:")

	// Detail view with diff.
	model.view = historyViewDetail
	reason := "unit-test"
	model.detail = &api.AuditEntry{
		ID:        "audit-1",
		TableName: "entities",
		RecordID:  "entity-1",
		Action:    "update",
		OldValues: `{"status": "inactive"}`,
		NewValues: `{"status": "active"}`,
		ChangedAt: now,
		ChangeReason: func() *string {
			return &reason
		}(),
	}
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Table")
	assert.Contains(t, out, "entities")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "inactive")
	assert.Contains(t, out, "active")
	assert.Contains(t, out, "Reason")
	assert.Contains(t, out, "unit-test")
}

// TestHistoryViewRendersFilterDialogAndErrorBox handles test history view renders filter dialog and error box.
func TestHistoryViewRendersFilterDialogAndErrorBox(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 80

	model.filtering = true
	model.filterInput.SetValue("table:entities")
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Filter Audit Log")
	assert.Contains(t, out, "table:entities")

	model.filtering = false
	model.errText = "boom"
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Error")
	assert.Contains(t, out, "boom")
}

func TestHistoryViewLoadingAndRevertBranches(t *testing.T) {
	now := time.Now()
	model := NewHistoryModel(nil)
	model.width = 82

	model.loading = true
	model.view = historyViewList
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Loading history...")

	model.view = historyViewScopes
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Loading scopes...")

	model.view = historyViewActors
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Loading actors...")

	model.loading = false
	model.view = historyViewDetail
	model.detail = &api.AuditEntry{
		ID:        "audit-1",
		TableName: "entities",
		RecordID:  "ent-1",
		Action:    "update",
		OldValues: `{"status": "inactive"}`,
		NewValues: `{"status": "active"}`,
		ChangedAt: now,
	}
	model.reverting = true
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Audit Entry")

	// Detail view without selected entry falls back to list rendering branch.
	model.detail = nil
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "No audit entries yet.")
}

// TestHistoryDetailMetadataDiffRendersStructuredRows handles test history detail metadata diff renders structured rows.
func TestHistoryDetailMetadataDiffRendersStructuredRows(t *testing.T) {
	now := time.Now()
	model := NewHistoryModel(nil)
	model.width = 90
	model.view = historyViewDetail
	model.detail = &api.AuditEntry{
		ID:        "audit-2",
		TableName: "entities",
		RecordID:  "ent-1",
		Action:    "update",
		OldValues: `{"metadata": {"context_segments": [{"text": "public", "scopes": ["public"]}]}}`,
		NewValues: `{"metadata": {"context_segments": [{"text": "public", "scopes": ["public"]}, {"text": "secret", "scopes": ["private"]}]}}`,
		ChangedAt: now,
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Metadata")
	assert.Contains(t, out, "context_segments")
	assert.Contains(t, out, "secret")
	assert.NotContains(t, out, "{\"")
}

// TestHistoryActorsViewNormalizesUnknownSystemLabels handles test history actors view normalizes unknown system labels.
func TestHistoryActorsViewNormalizesUnknownSystemLabels(t *testing.T) {
	now := time.Now()
	model := NewHistoryModel(nil)
	model.width = 90
	model.view = historyViewActors
	model, _ = model.Update(historyActorsLoadedMsg{
		items: []api.AuditActor{
			{ActorType: "system", ActorID: "system:", ActionCount: 2, LastSeen: now},
			{ActorType: "agent", ActorID: "agent-1", ActionCount: 1, LastSeen: now},
		},
	})

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "system")
	assert.NotContains(t, strings.ToLower(out), "unknown")
}

// TestHistoryRenderRevertConfirmAndUnknownLabelHelper handles test history render revert confirm and unknown label helper.
func TestHistoryRenderRevertConfirmAndUnknownLabelHelper(t *testing.T) {
	now := time.Now()
	model := NewHistoryModel(nil)
	model.width = 80
	entry := api.AuditEntry{
		ID:        "audit-9f0d",
		TableName: "entities",
		RecordID:  "ent-9f0d",
		Action:    "update",
		OldValues: `{"status": "inactive"}`,
		NewValues: `{"status": "active"}`,
		ChangedAt: now,
	}

	out := components.SanitizeText(model.renderRevertConfirm(entry))
	assert.Contains(t, out, "Action")
	assert.Contains(t, out, "Entity")
	assert.Contains(t, out, "Audit Entry")

	assert.True(t, isUnknownLabel("unknown"))
	assert.True(t, isUnknownLabel("None:"))
	assert.False(t, isUnknownLabel("agent"))
}

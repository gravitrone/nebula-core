package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

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
				OldData:   api.JSONMap{},
				NewData:   api.JSONMap{"name": "Alpha"},
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
	assert.Contains(t, out, "History")
	assert.Contains(t, out, "Filters:")
	assert.Contains(t, out, "table:entities")
	assert.Contains(t, out, "action:create")
	assert.Contains(t, out, "scope:scope-1")
	assert.Contains(t, out, "CREATE")
	assert.Contains(t, out, "entities")
}

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
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "public")

	// Actors view.
	model.view = historyViewActors
	model, _ = model.Update(historyActorsLoadedMsg{
		items: []api.AuditActor{
			{ActorType: "agent", ActorID: "agent-1", ActionCount: 42, LastSeen: now},
		},
	})
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Actors")
	assert.Contains(t, out, "agent:")

	// Detail view with diff.
	model.view = historyViewDetail
	reason := "unit-test"
	model.detail = &api.AuditEntry{
		ID:        "audit-1",
		TableName: "entities",
		RecordID:  "entity-1",
		Action:    "update",
		OldData:   api.JSONMap{"status": "inactive"},
		NewData:   api.JSONMap{"status": "active"},
		ChangedAt: now,
		ChangeReason: func() *string {
			return &reason
		}(),
	}
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Audit Entry")
	assert.Contains(t, out, "Table")
	assert.Contains(t, out, "entities")
	assert.Contains(t, out, "Changes")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "inactive")
	assert.Contains(t, out, "active")
	assert.Contains(t, out, "Reason")
	assert.Contains(t, out, "unit-test")
}

func TestHistoryViewRendersFilterDialogAndErrorBox(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 80

	model.filtering = true
	model.filterBuf = "table:entities"
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Filter Audit Log")
	assert.Contains(t, out, "table:entities")

	model.filtering = false
	model.errText = "boom"
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Error")
	assert.Contains(t, out, "boom")
}

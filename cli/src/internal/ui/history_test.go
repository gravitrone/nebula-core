package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

// TestParseAuditFilter handles test parse audit filter.
func TestParseAuditFilter(t *testing.T) {
	filter := parseAuditFilter("table:entities action:update actor:alice actor_type:agent actor_id:ag-1 record:ent-1 scope:scope-1 extra")
	assert.Equal(t, "entities", filter.tableName)
	assert.Equal(t, "update", filter.action)
	assert.Equal(t, "alice", filter.actor)
	assert.Equal(t, "agent", filter.actorType)
	assert.Equal(t, "ag-1", filter.actorID)
	assert.Equal(t, "ent-1", filter.recordID)
	assert.Equal(t, "scope-1", filter.scopeID)
	assert.Equal(t, []string{"extra"}, filter.terms)
}

// TestFormatAuditActor handles test format audit actor.
func TestFormatAuditActor(t *testing.T) {
	name := "Alxx"
	entry := api.AuditEntry{ActorName: &name}
	assert.Equal(t, "Alxx", formatAuditActor(entry))
}

// TestBuildAuditDiffRows handles test build audit diff rows.
func TestBuildAuditDiffRows(t *testing.T) {
	entry := api.AuditEntry{
		ChangedFields: []string{"name", "status", "unchanged"},
		OldValues:     `{"name": "old", "status": "active", "unchanged": "same"}`,
		NewValues:     `{"name": "new", "status": "archived", "unchanged": "same"}`,
	}
	rows := buildAuditDiffRows(entry)
	assert.Len(t, rows, 2)
}

// TestApplyLocalFilters handles test apply local filters.
func TestApplyLocalFilters(t *testing.T) {
	actor := "Agent Smith"
	now := time.Now()
	items := []api.AuditEntry{{TableName: "entities", RecordID: "ent-1", ActorName: &actor, ChangedAt: now}}
	model := HistoryModel{filter: auditFilter{actor: "smith", terms: []string{"entities"}}}
	filtered := model.applyLocalFilters(items)
	assert.Len(t, filtered, 1)
}

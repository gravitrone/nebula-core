package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestFormatAuditLineActionFallbackAndUppercase(t *testing.T) {
	ts := time.Date(2026, 3, 2, 4, 30, 0, 0, time.UTC)
	entry := api.AuditEntry{
		ChangedAt: ts,
		TableName: "entities",
		Action:    "",
	}

	line := formatAuditLine(entry)
	assert.Contains(t, line, "UPDATE")
	assert.Contains(t, line, "entities")
	assert.Contains(t, line, "system")

	actorType := "agent"
	actorID := "ag-123456789"
	entry = api.AuditEntry{
		ChangedAt:     ts,
		TableName:     "jobs",
		Action:        "archive",
		ChangedByType: &actorType,
		ChangedByID:   &actorID,
	}
	line = formatAuditLine(entry)
	assert.Contains(t, line, "ARCHIVE")
	assert.Contains(t, line, "jobs")
	assert.Contains(t, line, "agent:"+shortID("ag-123456789"))
}

func TestFormatAuditFiltersAllFieldsBranchMatrix(t *testing.T) {
	filter := auditFilter{
		tableName: "entities",
		action:    "update",
		actorType: "agent",
		actorID:   "actor-1234567890",
		recordID:  "rec-1234567890",
		scopeID:   "scope-1234567890",
		actor:     "alxx",
	}

	out := formatAuditFilters(filter)
	assert.Contains(t, out, "Filters:\n")
	assert.Contains(t, out, "table:entities")
	assert.Contains(t, out, "action:update")
	assert.Contains(t, out, "actor_type:agent")
	assert.Contains(t, out, "actor_id:"+shortID("actor-1234567890"))
	assert.Contains(t, out, "record:"+shortID("rec-1234567890"))
	assert.Contains(t, out, "scope:"+shortID("scope-1234567890"))
	assert.Contains(t, out, "actor:alxx")
	assert.Equal(t, 7, strings.Count(out, "\n"))
}

package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestFormatAuditActorFallbackMatrix(t *testing.T) {
	actorType := "agent"
	actorID := "agent-1234567890"

	entry := api.AuditEntry{ChangedByType: &actorType, ChangedByID: &actorID}
	assert.Equal(t, "agent:"+shortID(actorID), formatAuditActor(entry))

	emptyName := ""
	entry = api.AuditEntry{ActorName: &emptyName, ChangedByType: &actorType}
	assert.Equal(t, "agent", formatAuditActor(entry))

	actorID = "  "
	entry = api.AuditEntry{ChangedByType: &actorType, ChangedByID: &actorID}
	assert.Equal(t, "agent", formatAuditActor(entry))

	entry = api.AuditEntry{}
	assert.Equal(t, "system", formatAuditActor(entry))
}

func TestIsUnknownLabelMatrix(t *testing.T) {
	assert.True(t, isUnknownLabel("unknown"))
	assert.True(t, isUnknownLabel("None"))
	assert.True(t, isUnknownLabel("n/a"))
	assert.False(t, isUnknownLabel("alxx"))
}

func TestActorDisplayNameMatrix(t *testing.T) {
	name := "alxx"
	actor := api.AuditActor{ActorName: &name, ActorType: "agent", ActorID: "agent:abc"}
	assert.Equal(t, "alxx", actorDisplayName(actor))

	unknown := "unknown"
	actor = api.AuditActor{ActorName: &unknown, ActorType: "system", ActorID: "entity:ent-123"}
	assert.Equal(t, "entity", actorDisplayName(actor))

	actor = api.AuditActor{ActorType: "system", ActorID: ""}
	assert.Equal(t, "system", actorDisplayName(actor))
}

func TestFormatActorRefMatrix(t *testing.T) {
	actor := api.AuditActor{ActorType: "", ActorID: ""}
	assert.Equal(t, "system", formatActorRef(actor))

	actor = api.AuditActor{ActorType: "system", ActorID: "agent:ag-123456789"}
	assert.Equal(t, "agent:"+shortID("ag-123456789"), formatActorRef(actor))

	actor = api.AuditActor{ActorType: "agent", ActorID: "agent:ag-123456789"}
	assert.Equal(t, "agent:"+shortID("ag-123456789"), formatActorRef(actor))
}

func TestInferActorTypeFromIDMatrix(t *testing.T) {
	assert.Equal(t, "", inferActorTypeFromID(""))
	assert.Equal(t, "", inferActorTypeFromID("agent"))
	assert.Equal(t, "agent", inferActorTypeFromID("agent:abc"))
	assert.Equal(t, "system", inferActorTypeFromID("unknown:abc"))
}

func TestNormalizeActorTypeMatrix(t *testing.T) {
	assert.Equal(t, "system", normalizeActorType(""))
	assert.Equal(t, "system", normalizeActorType("unknown"))
	assert.Equal(t, "system", normalizeActorType("null"))
	assert.Equal(t, "agent", normalizeActorType("agent"))
}

func TestFormatActorDisplayAvoidsDuplicateReference(t *testing.T) {
	actor := api.AuditActor{ActorType: "agent", ActorID: "ag-1"}
	ref := formatActorRef(actor)
	assert.Equal(t, ref, formatActorDisplay(actor, ref))
	assert.Contains(t, formatActorDisplay(actor, "alxx"), "alxx")
}

func TestFormatAuditFiltersMatrix(t *testing.T) {
	assert.Equal(t, "", formatAuditFilters(auditFilter{}))

	single := formatAuditFilters(auditFilter{tableName: "entities"})
	assert.Equal(t, "Filters: table:entities", single)

	multi := formatAuditFilters(auditFilter{tableName: "entities", action: "update", actor: "alxx"})
	assert.Contains(t, multi, "Filters:\n")
	assert.Contains(t, multi, "table:entities")
	assert.Contains(t, multi, "action:update")
	assert.Contains(t, multi, "actor:alxx")
}

func TestFormatAuditValueMatrix(t *testing.T) {
	assert.Equal(t, "None", formatAuditValue(nil))
	assert.Equal(t, "None", formatAuditValue("  "))
	assert.Equal(t, "None", formatAuditValue("<nil>"))
	assert.Equal(t, "None", formatAuditValue("-"))
	assert.Equal(t, "None", formatAuditValue("--"))

	structured := formatAuditValue(`{"owner":"alxx"}`)
	assert.Contains(t, structured, "owner")
	assert.Contains(t, structured, "alxx")

	asMap := formatAuditValue(map[string]any{})
	assert.Equal(t, "None", asMap)

	timestamp := time.Date(2026, 2, 26, 7, 8, 9, 0, time.UTC)
	assert.Contains(t, formatAuditValue(timestamp), "2026")

	asEmptyList := formatAuditValue([]any{})
	assert.Equal(t, "None", asEmptyList)

	asList := formatAuditValue([]any{"a", "b"})
	assert.Contains(t, asList, "a")
	assert.Contains(t, asList, "b")

	// Non-JSON string branch should still produce sanitized text.
	asRaw := formatAuditValue("map[owner:alxx status:active]")
	assert.Contains(t, strings.ToLower(asRaw), "owner")

	// Marshal fallback branch.
	asFallback := formatAuditValue(func() {})
	assert.NotEqual(t, "None", asFallback)
}

func TestBuildAuditDiffRowsUsesUnionWhenChangedFieldsMissing(t *testing.T) {
	entry := api.AuditEntry{
		OldValues: `{"name": "old", "same": "x"}`,
		NewValues: `{"name": "new", "same": "x", "status": "active"}`,
	}
	rows := buildAuditDiffRows(entry)
	assert.Len(t, rows, 2)

	labels := []string{rows[0].Label, rows[1].Label}
	assert.Contains(t, strings.Join(labels, ","), "Name")
	assert.Contains(t, strings.Join(labels, ","), "Status")
}

func TestApplyLocalFiltersRejectsNonMatchingActorOrTerms(t *testing.T) {
	name := "Agent Smith"
	items := []api.AuditEntry{{TableName: "entities", RecordID: "ent-1", ActorName: &name}}

	model := HistoryModel{filter: auditFilter{actor: "alice"}}
	assert.Empty(t, model.applyLocalFilters(items))

	model = HistoryModel{filter: auditFilter{terms: []string{"jobs"}}}
	assert.Empty(t, model.applyLocalFilters(items))
}

func TestParseAuditFilterAliases(t *testing.T) {
	filter := parseAuditFilter("record_id:ent-1 scope_id:scope-1 actor:alxx")
	assert.Equal(t, "ent-1", filter.recordID)
	assert.Equal(t, "scope-1", filter.scopeID)
	assert.Equal(t, "alxx", filter.actor)
}

func TestHumanizeAuditFieldMatrix(t *testing.T) {
	assert.Equal(t, "", humanizeAuditField("   "))
	assert.Equal(t, "Actor ID", humanizeAuditField("actor_id"))
	assert.Equal(t, "Review Notes", humanizeAuditField("review-notes"))
	assert.Equal(t, "ID", humanizeAuditField("id"))
	assert.Equal(t, "Weird Field", humanizeAuditField("___weird_field___"))
}

func TestFormatScopeLineMatrix(t *testing.T) {
	desc := "public-safe"
	scope := api.AuditScope{
		Name:         "public",
		AgentCount:   2,
		EntityCount:  3,
		ContextCount: 1,
		Description:  &desc,
	}
	line := formatScopeLine(scope)
	assert.Contains(t, line, "public")
	assert.Contains(t, line, "agents:2")
	assert.Contains(t, line, "entities:3")
	assert.Contains(t, line, "context:1")
	assert.Contains(t, line, "public-safe")

	scope.Description = nil
	line = formatScopeLine(scope)
	assert.NotContains(t, line, "public-safe")
}

func TestParseAuditFilterEmptyInputReturnsZeroValue(t *testing.T) {
	filter := parseAuditFilter("   ")
	assert.Equal(t, auditFilter{}, filter)
}

func TestBuildAuditDiffRowsSkipsEmptyChangedFieldNames(t *testing.T) {
	entry := api.AuditEntry{
		ChangedFields: []string{"", "name"},
		OldValues:     `{"name": "old"}`,
		NewValues:     `{"name": "new"}`,
	}
	rows := buildAuditDiffRows(entry)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Name", rows[0].Label)
}

func TestFormatAuditValueDefaultMarshalScalarBranch(t *testing.T) {
	assert.Equal(t, "123", formatAuditValue(123))
	assert.Equal(t, "true", formatAuditValue(true))
}

func TestHumanizeAuditFieldSeparatorOnlyFallsBackToRaw(t *testing.T) {
	assert.Equal(t, "___", humanizeAuditField("___"))
}

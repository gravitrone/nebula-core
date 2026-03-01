package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestHistoryRenderListFallbackAndPreviewBranches(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 72
	model.items = []api.AuditEntry{{
		ID:        "audit-1",
		TableName: "",
		Action:    "",
		ChangedAt: time.Time{},
	}}
	model.list.SetItems([]string{"audit-1"})
	model.list.Cursor = 9 // out-of-range selected index => no preview branch

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "History")
	assert.Contains(t, out, "1 total")
	assert.NotContains(t, out, "Filters:")

	model.width = 150
	model.list.Cursor = 0
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "UPDATE")
	assert.Contains(t, out, "system")
}

func TestHistoryRenderScopesAndActorsBranchMatrix(t *testing.T) {
	model := NewHistoryModel(nil)
	model.width = 84

	// Empty list branches.
	assert.Contains(t, components.SanitizeText(model.renderScopes()), "No scopes found")
	assert.Contains(t, components.SanitizeText(model.renderActors()), "No actors found")

	// Scopes with out-of-range cursor to skip preview.
	model.scopes = []api.AuditScope{{ID: "scope-1", Name: "", AgentCount: 1, EntityCount: 2, ContextCount: 3}}
	model.scopeList.SetItems([]string{"scope-1"})
	model.scopeList.Cursor = 7
	out := components.SanitizeText(model.renderScopes())
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "1 total")

	// Scopes with selected row to render preview and fallback title.
	model.width = 150
	model.scopeList.Cursor = 0
	out = components.SanitizeText(model.renderScopes())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "scope")

	// Actors with out-of-range cursor branch.
	model.width = 84
	model.actors = []api.AuditActor{{ActorType: "", ActorID: "", ActionCount: 0}}
	model.actorList.SetItems([]string{"system"})
	model.actorList.Cursor = 5
	out = components.SanitizeText(model.renderActors())
	assert.Contains(t, out, "Actors")
	assert.Contains(t, out, "1 total")

	// Actors with selected row to render preview fallback values.
	model.width = 150
	model.actorList.Cursor = 0
	out = components.SanitizeText(model.renderActors())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "system")
}

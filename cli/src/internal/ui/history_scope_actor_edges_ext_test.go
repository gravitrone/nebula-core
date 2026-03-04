package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestHistoryRenderScopesTinyWidthPreviewAndUnsyncedVisibleRows(t *testing.T) {
	desc := "team-visible-scope"
	model := NewHistoryModel(nil)
	model.width = 24
	model, _ = model.Update(historyScopesLoadedMsg{
		items: []api.AuditScope{
			{
				ID:           "scope-1",
				Name:         "public",
				AgentCount:   2,
				EntityCount:  5,
				ContextCount: 1,
				Description:  &desc,
			},
		},
	})
	// Add an extra visible row without a backing scope to exercise the guard.
	model.scopeList.Items = append(model.scopeList.Items, "orphan-visible-row")

	out := components.SanitizeText(model.renderScopes())

	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "Desc")
	assert.Contains(t, out, "team-")
	assert.NotContains(t, out, "orphan-visible-row")
}

func TestHistoryRenderActorsTinyWidthAndUnsyncedVisibleRows(t *testing.T) {
	now := time.Now().UTC()
	model := NewHistoryModel(nil)
	model.width = 24
	model, _ = model.Update(historyActorsLoadedMsg{
		items: []api.AuditActor{
			{
				ActorType:   "agent",
				ActorID:     "agent:1234567890abcdef",
				ActionCount: 3,
				LastSeen:    now,
			},
		},
	})
	// Add an extra visible row without a backing actor to exercise the guard.
	model.actorList.Items = append(model.actorList.Items, "orphan-visible-row")

	out := components.SanitizeText(model.renderActors())

	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "agent")
	assert.NotContains(t, out, "orphan-visible-row")
}

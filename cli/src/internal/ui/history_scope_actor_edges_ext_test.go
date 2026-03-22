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
	out := components.SanitizeText(model.renderScopes())

	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "Desc")
	assert.Contains(t, out, "team-")
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
	out := components.SanitizeText(model.renderActors())

	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "agent")
}

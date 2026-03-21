package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryHandleScopeKeysNavigationBranches(t *testing.T) {
	model := NewHistoryModel(nil)
	model.view = historyViewScopes
	model.scopes = []api.AuditScope{
		{ID: "scope-1", Name: "public"},
		{ID: "scope-2", Name: "private"},
	}
	model.scopeList.SetItems([]string{"public", "private"})

	updated, cmd := model.handleScopeKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.scopeList.Selected())

	updated, cmd = updated.handleScopeKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.scopeList.Selected())

	// Enter with out-of-range selected index is a no-op.
	updated.scopeList.Cursor = 9
	updated.loading = false
	updated, cmd = updated.handleScopeKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.loading)
	assert.Equal(t, historyViewScopes, updated.view)
}

func TestHistoryHandleActorKeysNavigationBranches(t *testing.T) {
	model := NewHistoryModel(nil)
	model.view = historyViewActors
	model.actors = []api.AuditActor{
		{ActorType: "agent", ActorID: "agent-1"},
		{ActorType: "user", ActorID: "user-1"},
	}
	model.actorList.SetItems([]string{"agent:agent-1", "user:user-1"})

	updated, cmd := model.handleActorKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.actorList.Selected())

	updated, cmd = updated.handleActorKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.actorList.Selected())

	// Enter with out-of-range selected index is a no-op.
	updated.actorList.Cursor = 9
	updated.loading = false
	updated, cmd = updated.handleActorKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.loading)
	assert.Equal(t, historyViewActors, updated.view)
}

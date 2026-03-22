package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
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
	model.scopeTable.SetRows([]table.Row{{"public"}, {"private"}})

	updated, cmd := model.handleScopeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.scopeTable.Cursor())

	updated, cmd = updated.handleScopeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.scopeTable.Cursor())

	// Enter with no scopes is a no-op (cursor -1 via empty table).
	updated.scopes = nil
	updated.scopeTable.SetRows(nil)
	updated.loading = false
	updated, cmd = updated.handleScopeKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	model.actorTable.SetRows([]table.Row{{"agent:agent-1"}, {"user:user-1"}})

	updated, cmd := model.handleActorKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.actorTable.Cursor())

	updated, cmd = updated.handleActorKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.actorTable.Cursor())

	// Enter with no actors is a no-op (cursor -1 via empty table).
	updated.actors = nil
	updated.actorTable.SetRows(nil)
	updated.loading = false
	updated, cmd = updated.handleActorKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.loading)
	assert.Equal(t, historyViewActors, updated.view)
}

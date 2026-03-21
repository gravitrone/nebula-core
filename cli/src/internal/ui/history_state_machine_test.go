package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHistoryModelScopesAndActorsSelectionLoadsHistory handles test history model scopes and actors selection loads history.
func TestHistoryModelScopesAndActorsSelectionLoadsHistory(t *testing.T) {
	var lastPath string
	var lastScopeID string
	var lastActorType string
	var lastActorID string

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		switch r.URL.Path {
		case "/api/audit":
			lastScopeID = r.URL.Query().Get("scope_id")
			lastActorType = r.URL.Query().Get("actor_type")
			lastActorID = r.URL.Query().Get("actor_id")
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "audit-1",
						"table_name": "entities",
						"record_id":  "ent-1",
						"action":     "update",
						"changed_at": time.Now(),
					},
				},
			})
			require.NoError(t, err)
		case "/api/audit/scopes":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-1", "name": "public", "agent_count": 1},
				},
			})
			require.NoError(t, err)
		case "/api/audit/actors":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"changed_by_type": "agent", "changed_by_id": "agent-1", "action_count": 2},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewHistoryModel(client)
	model.width = 80

	// Init loads history.
	cmd := model.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, "/api/audit", lastPath)

	// Load scopes.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, historyViewScopes, model.view)

	// Select scope and verify it is applied to the next history load.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, "scope-1", model.filter.scopeID)
	assert.Equal(t, "scope-1", lastScopeID)

	// Load actors.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, historyViewActors, model.view)

	// Select actor and verify it is applied to the next history load.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, "agent", model.filter.actorType)
	assert.Equal(t, "agent-1", model.filter.actorID)
	assert.Equal(t, "agent", lastActorType)
	assert.Equal(t, "agent-1", lastActorID)
}

// TestHistoryModelFilterPromptAppliesAndLoads handles test history model filter prompt applies and loads.
func TestHistoryModelFilterPromptAppliesAndLoads(t *testing.T) {
	var gotTable string
	var gotAction string

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/audit":
			gotTable = r.URL.Query().Get("table")
			gotAction = r.URL.Query().Get("action")
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewHistoryModel(client)
	model.width = 80

	// Enter filtering mode.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	assert.True(t, model.filtering)

	// Type a filter and apply.
	for _, r := range "table:entities action:update" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.False(t, model.filtering)
	assert.Equal(t, "entities", gotTable)
	assert.Equal(t, "update", gotAction)
}

// TestHistoryDetailRevertFlowExecutesFromSelectedEntry handles test history detail revert flow executes from selected entry.
func TestHistoryDetailRevertFlowExecutesFromSelectedEntry(t *testing.T) {
	var revertEntityID string
	var revertAuditID string

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/audit":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "audit-1",
						"table_name": "entities",
						"record_id":  "ent-1",
						"action":     "update",
						"old_data":   map[string]any{"name": "before"},
						"new_data":   map[string]any{"name": "after"},
						"changed_at": time.Now(),
					},
				},
			})
			require.NoError(t, err)
		case "/api/entities/ent-1/revert":
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			revertEntityID = "ent-1"
			revertAuditID = payload["audit_id"]
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ent-1"}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewHistoryModel(client)
	model.width = 90

	// Load history list.
	cmd := model.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)
	require.Len(t, model.items, 1)

	// Open detail from selected entry.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, historyViewDetail, model.view)
	require.NotNil(t, model.detail)

	// Trigger revert confirm and accept.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.True(t, model.reverting)
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, cmd = model.Update(msg)
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "ent-1", revertEntityID)
	assert.Equal(t, "audit-1", revertAuditID)
	assert.Equal(t, historyViewList, model.view)
	assert.Nil(t, model.detail)
}

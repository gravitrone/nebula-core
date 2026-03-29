package ui

import (
	"encoding/json"
	"net/http"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanRevertAuditEntryGuardMatrix(t *testing.T) {
	assert.False(t, canRevertAuditEntry(nil))
	assert.False(t, canRevertAuditEntry(&api.AuditEntry{ID: "audit-1", TableName: "entities"}))
	assert.False(t, canRevertAuditEntry(&api.AuditEntry{RecordID: "ent-1", TableName: "entities"}))
	assert.False(t, canRevertAuditEntry(&api.AuditEntry{ID: "audit-1", RecordID: "ent-1", TableName: "jobs"}))
	assert.True(t, canRevertAuditEntry(&api.AuditEntry{ID: "audit-1", RecordID: "ent-1", TableName: "Entities"}))
}

func TestHistoryConfirmRevertGuardBranches(t *testing.T) {
	model := NewHistoryModel(nil)
	model.reverting = true

	updated, cmd := model.confirmRevert()
	assert.False(t, updated.reverting)
	assert.Nil(t, cmd)

	model.detail = &api.AuditEntry{ID: "audit-1", RecordID: " "}
	model.reverting = true
	updated, cmd = model.confirmRevert()
	assert.False(t, updated.reverting)
	assert.Nil(t, cmd)

	model.detail = &api.AuditEntry{ID: " ", RecordID: "ent-1"}
	model.reverting = true
	updated, cmd = model.confirmRevert()
	assert.False(t, updated.reverting)
	assert.Nil(t, cmd)
}

func TestHistoryConfirmRevertSuccessReturnsRevertedMsg(t *testing.T) {
	var gotEntityID string
	var gotAuditID string

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities/ent-1/revert" {
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			gotEntityID = "ent-1"
			gotAuditID = body["audit_id"]
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ent-1"}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewHistoryModel(client)
	model.detail = &api.AuditEntry{ID: "audit-1", RecordID: "ent-1"}

	updated, cmd := model.confirmRevert()
	require.NotNil(t, cmd)
	assert.False(t, updated.reverting)

	msg := cmd().(historyRevertedMsg)
	assert.Equal(t, "ent-1", msg.entityID)
	assert.Equal(t, "audit-1", msg.auditID)
	assert.Equal(t, "ent-1", gotEntityID)
	assert.Equal(t, "audit-1", gotAuditID)
}

func TestHistoryConfirmRevertReturnsErrMsgOnAPIError(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities/ent-1/revert" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"revert failed"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewHistoryModel(client)
	model.detail = &api.AuditEntry{ID: "audit-1", RecordID: "ent-1"}

	updated, cmd := model.confirmRevert()
	require.NotNil(t, cmd)
	assert.False(t, updated.reverting)

	msg := cmd()
	errRes, ok := msg.(errMsg)
	require.True(t, ok)
	assert.Contains(t, errRes.err.Error(), "revert failed")
}

func TestHistoryHandleFilterKeysBranchMatrix(t *testing.T) {
	model := NewHistoryModel(nil)
	model.filtering = true
	model.filterInput.SetValue("table:entities")

	updated, cmd := model.handleFilterKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Nil(t, cmd)
	assert.Equal(t, "table:entitie", updated.filterInput.Value())

	updated, cmd = updated.handleFilterKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	assert.Nil(t, cmd)
	assert.Equal(t, "table:entities", updated.filterInput.Value())

	updated, cmd = updated.handleFilterKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "entities", updated.filter.tableName)
	assert.True(t, updated.loading)

	updated.filtering = true
	updated.filterInput.SetValue("actor:alxx")
	updated.filter = auditFilter{tableName: "entities"}
	updated.loading = false
	updated, cmd = updated.handleFilterKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.NotNil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.filterInput.Value())
	assert.Equal(t, auditFilter{}, updated.filter)
	assert.True(t, updated.loading)

	updated.filterInput.SetValue("")
	updated, cmd = updated.handleFilterKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Nil(t, cmd)
	assert.Equal(t, "", updated.filterInput.Value())
}

func TestHistoryHandleScopeAndActorKeysBranchMatrix(t *testing.T) {
	model := NewHistoryModel(nil)
	model.view = historyViewScopes
	model.scopes = []api.AuditScope{{ID: "scope-1", Name: "public"}}
	model.scopeTable.SetRows([]table.Row{{"public"}})

	updated, cmd := model.handleScopeKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, "scope-1", updated.filter.scopeID)
	assert.Equal(t, historyViewList, updated.view)
	assert.True(t, updated.loading)

	updated.view = historyViewScopes
	updated.scopes = nil
	updated.scopeTable.SetRows(nil)
	updated.loading = false
	updated, cmd = updated.handleScopeKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, historyViewScopes, updated.view)
	assert.False(t, updated.loading)

	updated, cmd = updated.handleScopeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Nil(t, cmd)
	assert.Equal(t, historyViewList, updated.view)

	updated.view = historyViewActors
	updated.actors = []api.AuditActor{{ActorType: "agent", ActorID: "agent-1"}}
	updated.actorTable.SetRows([]table.Row{{"agent:agent-1"}})
	updated.actorTable.SetCursor(0)
	updated.loading = false
	updated, cmd = updated.handleActorKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, "agent", updated.filter.actorType)
	assert.Equal(t, "agent-1", updated.filter.actorID)
	assert.Equal(t, historyViewList, updated.view)
	assert.True(t, updated.loading)

	updated.view = historyViewActors
	updated.actors = nil
	updated.actorTable.SetRows(nil)
	updated.loading = false
	updated, cmd = updated.handleActorKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, historyViewActors, updated.view)
	assert.False(t, updated.loading)

	updated, cmd = updated.handleActorKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Nil(t, cmd)
	assert.Equal(t, historyViewList, updated.view)
}

func TestHistoryHandleListKeysBranchMatrix(t *testing.T) {
	model := NewHistoryModel(nil)
	model.items = []api.AuditEntry{{ID: "audit-1", RecordID: "ent-1", TableName: "entities"}}
	model.dataTable.SetRows([]table.Row{{"audit-1"}})

	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Nil(t, cmd)
	assert.Equal(t, 0, updated.dataTable.Cursor())

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Nil(t, cmd)
	assert.Equal(t, 0, updated.dataTable.Cursor())

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, historyViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "audit-1", updated.detail.ID)

	model = NewHistoryModel(nil)
	updated, cmd = model.handleListKeys(tea.KeyPressMsg{Code: 'f', Text: "f"})
	assert.Nil(t, cmd)
	assert.True(t, updated.filtering)

	updated, cmd = model.handleListKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	require.NotNil(t, cmd)
	assert.Equal(t, historyViewScopes, updated.view)
	assert.True(t, updated.loading)

	updated, cmd = model.handleListKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.NotNil(t, cmd)
	assert.Equal(t, historyViewActors, updated.view)
	assert.True(t, updated.loading)
}

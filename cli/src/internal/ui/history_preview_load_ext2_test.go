package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryRenderAuditAndScopePreviewFallbackBranches(t *testing.T) {
	model := NewHistoryModel(nil)

	assert.Equal(t, "", model.renderAuditPreview(api.AuditEntry{}, 0))
	assert.Equal(t, "", model.renderScopePreview(api.AuditScope{}, 0))
	assert.Equal(t, "", model.renderActorPreview(api.AuditActor{}, 0))

	reason := "manual"
	entry := api.AuditEntry{
		TableName:     "",
		Action:        "",
		RecordID:      "ent-1",
		ChangedFields: []string{"status"},
		ChangeReason:  &reason,
	}
	out := components.SanitizeText(model.renderAuditPreview(entry, 48))
	assert.Contains(t, out, "UPDATE")
	assert.Contains(t, out, "Table")
	assert.Contains(t, out, "-")
	assert.Contains(t, out, "Actor")
	assert.Contains(t, out, "system")
	assert.Contains(t, out, "Record")
	assert.Contains(t, out, "Fields")
	assert.Contains(t, out, "Reason")
}

func TestHistoryLoadCommandsErrorBranches(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/audit":
			http.Error(w, `{"error":{"code":"AUDIT_FAIL","message":"audit down"}}`, http.StatusInternalServerError)
		case "/api/audit/scopes":
			http.Error(w, `{"error":{"code":"SCOPES_FAIL","message":"scopes down"}}`, http.StatusInternalServerError)
		case "/api/audit/actors":
			http.Error(w, `{"error":{"code":"ACTORS_FAIL","message":"actors down"}}`, http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewHistoryModel(client)

	msg := model.loadHistory()()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "AUDIT_FAIL")

	msg = model.loadScopes()()
	errOut, ok = msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "SCOPES_FAIL")

	msg = model.loadActors()()
	errOut, ok = msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "ACTORS_FAIL")
}

func TestHistoryUpdateDetailRevertingBranchMatrix(t *testing.T) {
	now := time.Now().UTC()
	entry := api.AuditEntry{
		ID:        "audit-1",
		TableName: "entities",
		RecordID:  "ent-1",
		Action:    "update",
		ChangedAt: now,
	}
	model := NewHistoryModel(nil)
	model.view = historyViewDetail
	model.detail = &entry
	model.reverting = true

	// Unknown key while confirming revert keeps state.
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.Nil(t, cmd)
	assert.True(t, updated.reverting)

	// Cancel revert via "n".
	updated, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	require.Nil(t, cmd)
	assert.False(t, updated.reverting)

	// Back from detail returns list and clears detail.
	updated, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, historyViewList, updated.view)
	assert.Nil(t, updated.detail)

	// "r" on non-revertable entry is ignored.
	updated.view = historyViewDetail
	updated.detail = &api.AuditEntry{ID: "audit-2", TableName: "jobs", RecordID: "job-1"}
	updated, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.Nil(t, cmd)
	assert.False(t, updated.reverting)
}

func TestHistoryUpdateRevertedAndErrMsgBranches(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/audit":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id":         "audit-1",
					"table_name": "entities",
					"record_id":  "ent-1",
					"action":     "update",
					"changed_at": now,
				}},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewHistoryModel(client)
	model.view = historyViewDetail
	model.detail = &api.AuditEntry{ID: "audit-1", RecordID: "ent-1"}
	model.reverting = true
	model.loading = false
	model.errText = "stale"

	updated, cmd := model.Update(historyRevertedMsg{entityID: "ent-1", auditID: "audit-1"})
	require.NotNil(t, cmd)
	assert.False(t, updated.reverting)
	assert.Equal(t, historyViewList, updated.view)
	assert.Nil(t, updated.detail)
	assert.True(t, updated.loading)

	msg := cmd()
	loaded, ok := msg.(historyLoadedMsg)
	require.True(t, ok)
	require.Len(t, loaded.items, 1)

	updated, cmd = updated.Update(errMsg{err: assert.AnError})
	require.Nil(t, cmd)
	assert.False(t, updated.loading)
	assert.False(t, updated.reverting)
	assert.Contains(t, updated.errText, "assert.AnError general error for testing")
}

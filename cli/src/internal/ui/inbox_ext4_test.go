package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInboxItemAtFilteredIndexAndToggleSelectedBranches(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{{ID: "ap-1"}, {ID: "ap-2"}}
	model.filtered = []int{0}
	model.dataTable.SetRows([]table.Row{{"one"}})
	model.dataTable.SetCursor(0)

	_, ok := model.itemAtFilteredIndex(-1)
	assert.False(t, ok)
	_, ok = model.itemAtFilteredIndex(2)
	assert.False(t, ok)

	model.filtered = []int{9}
	_, ok = model.itemAtFilteredIndex(0)
	assert.False(t, ok)

	model.filtered = []int{0}
	item, ok := model.itemAtFilteredIndex(0)
	require.True(t, ok)
	assert.Equal(t, "ap-1", item.ID)

	model.filtered = []int{9}
	model.toggleSelected()
	assert.Empty(t, model.selected)

	model.filtered = []int{0}
	model.toggleSelected()
	assert.Equal(t, map[string]bool{"ap-1": true}, model.selected)
	model.toggleSelected()
	assert.Empty(t, model.selected)
}

func TestInboxApproveSelectedFallbacksAndErrorBranch(t *testing.T) {
	approved := []string{}
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approve"):
			parts := strings.Split(r.URL.Path, "/")
			id := parts[len(parts)-2]
			approved = append(approved, id)
			if id == "ap-err" {
				http.Error(w, `{"error":{"code":"FAILED","message":"boom"}}`, http.StatusInternalServerError)
				return
			}
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	model.items = []api.Approval{{ID: "ap-1"}, {ID: "ap-err"}}
	model.filtered = []int{0}
	model.dataTable.SetRows([]table.Row{{"ap-1"}})
	model.dataTable.SetCursor(0)

	// No selected IDs and no detail falls back to current list item.
	updated, cmd := model.approveSelected()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(approvalDoneMsg)
	require.True(t, ok)
	assert.Nil(t, updated.detail)
	assert.Contains(t, approved, "ap-1")

	// Selected IDs branch should iterate selected and surface API errors.
	model.selected = map[string]bool{"ap-err": true}
	updated, cmd = model.approveSelected()
	require.NotNil(t, cmd)
	msg = cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "FAILED")
	assert.Contains(t, approved, "ap-err")

	// Empty state returns nil command.
	model = NewInboxModel(client)
	updated, cmd = model.approveSelected()
	assert.Nil(t, updated.detail)
	assert.Nil(t, cmd)
}

func TestInboxHandleDetailAndGrantInputBranches(t *testing.T) {
	model := NewInboxModel(nil)
	model.detail = &api.Approval{ID: "ap-1"}
	model.items = []api.Approval{{
		ID:          "ap-1",
		RequestType: "register_agent",
		ChangeDetails: `{"requested_scopes":["public"],"requested_requires_approval":false}`,
	}}
	model.filtered = []int{0}
	model.dataTable.SetRows([]table.Row{{"ap-1"}})
	model.dataTable.SetCursor(0)

	updated, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.True(t, updated.grantEditing)
	assert.Equal(t, "ap-1", updated.grantApproval)
	assert.Equal(t, "public", updated.grantScopes)
	assert.False(t, updated.grantTrusted)

	updated.rejectBuf = "stale"
	updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.Nil(t, cmd)
	assert.True(t, updated.rejecting)
	assert.Equal(t, "", updated.rejectBuf)

	updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Nil(t, updated.detail)

	updated.grantEditing = true
	updated.grantApproval = "ap-1"
	updated.grantScopes = "pub"
	updated.grantTrusted = false

	updated, cmd = updated.handleGrantInput(tea.KeyPressMsg{Code: 't', Text: "t"})
	require.Nil(t, cmd)
	assert.True(t, updated.grantTrusted)

	updated, cmd = updated.handleGrantInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "pu", updated.grantScopes)

	updated, cmd = updated.handleGrantInput(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.Equal(t, "pux", updated.grantScopes)
	updated, cmd = updated.handleGrantInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "pux ", updated.grantScopes)

	updated.grantScopes = ""
	updated, cmd = updated.handleGrantInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "at least one scope is required")

	updated, cmd = updated.handleGrantInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.grantEditing)
	assert.Equal(t, "", updated.grantApproval)
	assert.Equal(t, "", updated.grantScopes)
	assert.False(t, updated.grantTrusted)
}

func TestInboxHandleGrantInputSubmitErrorBranch(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/approve"):
			http.Error(w, `{"error":{"code":"FAILED","message":"approve failed"}}`, http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	model.grantEditing = true
	model.grantApproval = "ap-1"
	model.grantScopes = "public"
	model.grantTrusted = true
	model.detail = &api.Approval{ID: "ap-1", CreatedAt: time.Now()}

	updated, cmd := model.handleGrantInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "FAILED")
	assert.False(t, updated.grantEditing)
	assert.Nil(t, updated.detail)
}

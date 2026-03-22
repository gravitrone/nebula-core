package ui

import (
	"net/http"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextHandleListKeysFilteringDispatchAndTopUpFocus(t *testing.T) {
	model := NewContextModel(nil)
	model.items = []api.Context{{ID: "ctx-1", Title: "Alpha"}}
	model.list.SetItems([]string{"Alpha"})

	model.filtering = true
	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)

	updated.modeFocus = false
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)
}

func TestContextStartEditNilDetailAndNilURLBranch(t *testing.T) {
	model := NewContextModel(nil)
	model.contextEditFields[contextEditFieldURL].value = "keep"
	model.startEdit()
	assert.Equal(t, "keep", model.contextEditFields[contextEditFieldURL].value)

	model.detail = &api.Context{
		ID:         "ctx-1",
		Title:      "Alpha",
		SourceType: "note",
		Status:     "active",
		URL:        nil,
		Content:    nil,
	}
	model.startEdit()
	assert.Equal(t, "", model.contextEditFields[contextEditFieldURL].value)
}

func TestContextSaveEditNilDetailAndUpdateErrorBranch(t *testing.T) {
	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/context/ctx-1" && r.Method == http.MethodPatch {
			http.Error(w, `{"error":{"code":"CTX_UPDATE_FAILED","message":"update failed"}}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewContextModel(client)
	updated, cmd := model.saveEdit()
	require.Nil(t, cmd)
	assert.Nil(t, updated.detail)

	model.detail = &api.Context{
		ID:         "ctx-1",
		Title:      "Alpha",
		SourceType: "note",
		Status:     "active",
	}
	model.startEdit()
	updated, cmd = model.saveEdit()
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)

	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "CTX_UPDATE_FAILED")
}

func TestContextLoadContextDetailGetErrorBranch(t *testing.T) {
	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/context/ctx-bad" {
			http.Error(w, `{"error":{"code":"CTX_GET_FAILED","message":"detail fetch failed"}}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewContextModel(client)
	cmd := model.loadContextDetail("ctx-bad")
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "CTX_GET_FAILED")
}

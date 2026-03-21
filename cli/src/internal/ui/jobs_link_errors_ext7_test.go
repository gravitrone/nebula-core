package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsHandleLinkInputNilDetailAndAPIError(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/relationships" {
			http.Error(w, `{"error":{"code":"REL_CREATE_FAILED","message":"cannot create relationship"}}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewJobsModel(client)
	model.linkingRel = true
	model.linkBuf = "entity ent-1 owns"

	// Detail nil branch should close input with no command.
	updated, cmd := model.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.linkingRel)
	assert.Equal(t, "", updated.linkBuf)

	// API error branch should surface errMsg.
	model.detail = &api.Job{ID: "job-1"}
	model.linkingRel = true
	model.linkBuf = "entity ent-1 owns"
	updated, cmd = model.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "REL_CREATE_FAILED")
	assert.False(t, updated.linkingRel)
	assert.Equal(t, "", updated.linkBuf)
}

func TestJobsHandleUnlinkInputTypingAndAPIError(t *testing.T) {
	var calledPath string
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/relationships/") {
			calledPath = r.URL.Path
			http.Error(w, `{"error":{"code":"REL_UPDATE_FAILED","message":"cannot archive relationship"}}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1"}
	model.unlinkingRel = true

	// Default typing branch appends chars and spaces.
	updated, cmd := model.handleUnlinkInput(tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.Nil(t, cmd)
	assert.Equal(t, "r", updated.unlinkBuf)
	updated, cmd = updated.handleUnlinkInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "r ", updated.unlinkBuf)

	// API error path on submit.
	updated.unlinkBuf = "rel-1"
	updated, cmd = updated.handleUnlinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "REL_UPDATE_FAILED")
	assert.Equal(t, "/api/relationships/rel-1", calledPath)
	assert.False(t, updated.unlinkingRel)
	assert.Equal(t, "", updated.unlinkBuf)
}

func TestJobsHandleStatusAndSubtaskInputErrorPaths(t *testing.T) {
	seen := map[string]int{}
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/status"):
			seen["status"]++
			http.Error(w, `{"error":{"code":"STATUS_FAILED","message":"status update failed"}}`, http.StatusInternalServerError)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/subtasks"):
			seen["subtasks"]++
			http.Error(w, `{"error":{"code":"SUBTASK_FAILED","message":"subtask create failed"}}`, http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1"}
	model.changingSt = true
	model.statusBuf = "active"
	model.statusTargets = []string{"job-1"}

	updated, cmd := model.handleStatusInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "STATUS_FAILED")
	assert.Equal(t, 1, seen["status"])
	assert.Empty(t, updated.selected)
	assert.False(t, updated.changingSt)

	model.creatingSubtask = true
	model.subtaskBuf = "child task"
	updated, cmd = model.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	errOut, ok = msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "SUBTASK_FAILED")
	assert.Equal(t, 1, seen["subtasks"])
	assert.False(t, updated.creatingSubtask)
	assert.Equal(t, "", updated.subtaskBuf)
}

func TestJobsHandleStatusInputAddsSpaceAndCharacters(t *testing.T) {
	model := NewJobsModel(nil)
	model.changingSt = true

	updated, cmd := model.handleStatusInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.statusBuf)

	updated, cmd = updated.handleStatusInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "a ", updated.statusBuf)
}

func TestJobsHandleLinkInputAcceptsMultiWordRelationshipType(t *testing.T) {
	var payload map[string]any
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/relationships" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "rel-1"}})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1"}
	model.linkingRel = true
	model.linkBuf = "entity ent-1 assigned to"

	updated, cmd := model.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(jobRelationshipChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "assigned to", payload["relationship_type"])
	assert.False(t, updated.linkingRel)
	assert.Equal(t, "", updated.linkBuf)
}

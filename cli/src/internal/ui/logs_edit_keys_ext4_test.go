package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsHandleEditKeysAdditionalBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "event", Status: "active", Timestamp: time.Now().UTC()}
	model.startEdit()

	// editSaving guard branch.
	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.True(t, updated.editSaving)

	model.editSaving = false

	// Esc exits edit view.
	updated, cmd = model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewDetail, updated.view)

	// Any other key with non-nil form forwards to form and returns nil or non-nil cmd.
	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "event", Status: "active", Timestamp: time.Now().UTC()}
	model.startEdit()
	require.NotNil(t, model.editForm)

	updated, _ = model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.NotNil(t, updated)

	// Status field is set from startEdit.
	model2 := NewLogsModel(nil)
	model2.detail = &api.Log{ID: "log-1", LogType: "event", Status: "inactive", Timestamp: time.Now().UTC()}
	model2.startEdit()
	assert.Equal(t, "inactive", model2.editStatus)

	// Tags are loaded from detail.
	model3 := NewLogsModel(nil)
	model3.detail = &api.Log{ID: "log-1", Tags: []string{"alpha", "beta"}, Status: "active", Timestamp: time.Now().UTC()}
	model3.startEdit()
	assert.Equal(t, "alpha, beta", model3.editTagStr)
}

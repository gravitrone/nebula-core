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
	model.editFocus = logEditFieldStatus
	model.editStatusIdx = 0

	// status left wrap branch.
	updated, _ = model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, len(logStatusOptions)-1, updated.editStatusIdx)

	// status space branch.
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, 0, updated.editStatusIdx)

	// down wrap branch and up guard branch.
	updated.editFocus = logEditFieldMeta
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, logEditFieldStatus, updated.editFocus)

	updated.editFocus = logEditFieldStatus
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, logEditFieldStatus, updated.editFocus)

	updated.editFocus = logEditFieldTags
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, logEditFieldStatus, updated.editFocus)

	// tags rune append + commit via comma branch.
	updated.editFocus = logEditFieldTags
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Equal(t, "x", updated.editTagInput.Value())
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: ',', Text: ","})
	assert.Empty(t, updated.editTagInput.Value())
	assert.Contains(t, updated.editTags, "x")

	// back key branch exits edit view.
	updated.view = logsViewEdit
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, logsViewDetail, updated.view)
}

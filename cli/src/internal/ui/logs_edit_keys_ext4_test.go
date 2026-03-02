package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.True(t, updated.editSaving)

	model.editSaving = false
	model.editFocus = logEditFieldStatus
	model.editStatusIdx = 0

	// status left wrap branch.
	updated, _ = model.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, len(logStatusOptions)-1, updated.editStatusIdx)

	// status space branch.
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, 0, updated.editStatusIdx)

	// down wrap branch and up guard branch.
	updated.editFocus = logEditFieldMeta
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, logEditFieldStatus, updated.editFocus)

	updated.editFocus = logEditFieldStatus
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, logEditFieldStatus, updated.editFocus)

	updated.editFocus = logEditFieldTags
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, logEditFieldStatus, updated.editFocus)

	// tags rune append + commit via comma branch.
	updated.editFocus = logEditFieldTags
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Equal(t, "x", updated.editTagBuf)
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	assert.Empty(t, updated.editTagBuf)
	assert.Contains(t, updated.editTags, "x")

	// back key branch exits edit view.
	updated.view = logsViewEdit
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, logsViewDetail, updated.view)
}


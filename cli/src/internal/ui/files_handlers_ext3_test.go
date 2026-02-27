package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesHandleAddKeysExtendedBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd

	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	model.addSaving = false
	model.addSaved = true
	model.addName = "Alpha.txt"
	model.addPath = "/tmp/alpha.txt"
	updated, _ = model.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addName)
	assert.Equal(t, "", updated.addPath)

	updated.addMeta.Active = true
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.addMeta.Active)

	updated.addFocus = fileFieldStatus
	updated.addStatusIdx = 0
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, len(fileStatusOptions)-1, updated.addStatusIdx)
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = 0
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, updated.modeFocus)

	updated.modeFocus = false
	updated.addFocus = fileFieldName
	updated, cmd = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
	assert.Nil(t, cmd)
	assert.Equal(t, "Filename is required", updated.addErr)

	updated.addName = "a"
	updated.addPath = "/tmp/a"
	updated.addFocus = fileFieldName
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	assert.Equal(t, "ab", updated.addName)

	updated.addFocus = fileFieldPath
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Contains(t, updated.addPath, "x")

	updated.addFocus = fileFieldMime
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.Equal(t, "t", updated.addMime)

	updated.addFocus = fileFieldSize
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	assert.Equal(t, "1", updated.addSize)

	updated.addFocus = fileFieldChecksum
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	assert.Equal(t, "c", updated.addChecksum)

	updated.addFocus = fileFieldTags
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	assert.Equal(t, []string{"a"}, updated.addTags)

	updated.addFocus = fileFieldMeta
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.addMeta.Active)
}

func TestFilesHandleEditKeysExtendedBranches(t *testing.T) {
	now := time.Now()
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active", CreatedAt: now}
	model.startEdit()

	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	model.editSaving = false
	model.editMeta.Active = true
	updated, _ = model.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.editMeta.Active)

	updated.editFocus = fileFieldStatus
	updated.editStatusIdx = 0
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, len(fileStatusOptions)-1, updated.editStatusIdx)

	updated.editFocus = fileFieldName
	updated.editName = "ab"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", updated.editName)

	updated.editFocus = fileFieldPath
	updated.editPath = "/tmp/ab"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "/tmp/a", updated.editPath)

	updated.editFocus = fileFieldMime
	updated.editMime = "text"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "tex", updated.editMime)

	updated.editFocus = fileFieldSize
	updated.editSize = "10"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "1", updated.editSize)

	updated.editFocus = fileFieldChecksum
	updated.editChecksum = "abcd"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "abc", updated.editChecksum)

	updated.editFocus = fileFieldTags
	updated.editTagBuf = "Z"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"z"}, updated.editTags)

	updated.editFocus = fileFieldMeta
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.editMeta.Active)

	updated.editMeta.Active = false
	updated.editFocus = fileFieldName
	updated.editName = "a"
	updated.editSize = "bad"
	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
	assert.Nil(t, cmd)
	assert.Contains(t, updated.errText, "non-negative")

	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, filesViewDetail, updated.view)
}

package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesHandleAddKeysSavingAndSavedBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd

	// addSaving blocks key input.
	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.addSaving)

	// addSaved + Esc resets form.
	model.addSaving = false
	model.addSaved = true
	model.addName = "Alpha.txt"
	model.addPath = "/tmp/alpha.txt"
	updated, _ = model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addName)
	assert.Equal(t, "", updated.addPath)
}

func TestFilesHandleAddKeysModeToggle(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd
	model.modeFocus = true

	// modeFocus + right is handled by Update routing through handleModeKeys.
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, filesViewList, updated.view)
}

func TestFilesHandleAddKeysNilFormInits(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd
	model.addForm = nil

	// Nil form initializes on first key press.
	_, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.NotNil(t, cmd)
}

func TestFilesHandleAddKeysSaveValidation(t *testing.T) {
	model := NewFilesModel(nil)
	// saveAdd with empty name validates.
	updated, cmd := model.saveAdd()
	assert.Nil(t, cmd)
	assert.Equal(t, "Filename is required", updated.addErr)

	// saveAdd with name but no path.
	model.addName = "alpha.txt"
	updated, cmd = model.saveAdd()
	assert.Nil(t, cmd)
	assert.Equal(t, "File path is required", updated.addErr)
}

func TestFilesHandleEditKeysSavingBranch(t *testing.T) {
	now := time.Now()
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active", CreatedAt: now}
	model.startEdit()

	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.editSaving)
}

func TestFilesHandleEditKeysEscExitsToDetail(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active"}
	model.startEdit()

	model.editSaving = false
	updated, _ := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, filesViewDetail, updated.view)
}

func TestFilesHandleEditKeysSaveValidation(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active"}
	model.startEdit()

	// saveEdit with bad size returns errText.
	model.editSize = "bad"
	updated, cmd := model.saveEdit()
	assert.Nil(t, cmd)
	assert.Contains(t, updated.errText, "non-negative")
}

func TestFilesHandleEditKeysMetadataActiveBranch(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active"}
	model.startEdit()

	// editMeta.Active routes through Update, not handleEditKeys.
	model.editMeta.Active = true
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.editMeta.Active)
}

func TestFilesRenderAddStateBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.width = 90

	// nil form -> Initializing.
	out := model.renderAdd()
	assert.Contains(t, out, "Initializing")

	// saving -> Saving.
	model.addSaving = true
	out = model.renderAdd()
	assert.Contains(t, out, "Saving")

	// saved -> File saved.
	model.addSaving = false
	model.addSaved = true
	out = model.renderAdd()
	assert.Contains(t, out, "File saved")

	// addErr shown with form.
	model.addSaved = false
	model.addForm = nil
	model.initAddForm()
	model.addErr = "bad size"
	out = model.renderAdd()
	assert.Contains(t, out, "bad size")
}

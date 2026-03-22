package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesHandleAddKeysExtendedBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd

	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.addSaving)

	model.addSaving = false
	model.addSaved = true
	model.addName = "Alpha.txt"
	model.addPath = "/tmp/alpha.txt"
	updated, _ = model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addName)
	assert.Equal(t, "", updated.addPath)

	updated.addMeta.Active = true
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.addMeta.Active)

	updated.addFocus = fileFieldStatus
	updated.addStatusIdx = 0
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, len(fileStatusOptions)-1, updated.addStatusIdx)
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = 0
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.True(t, updated.modeFocus)

	updated.modeFocus = false
	updated.addFocus = fileFieldName
	updated, cmd = updated.handleAddKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.Nil(t, cmd)
	assert.Equal(t, "Filename is required", updated.addErr)

	updated.addName = "a"
	updated.addPath = "/tmp/a"
	updated.addFocus = fileFieldName
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.Equal(t, "ab", updated.addName)

	updated.addFocus = fileFieldPath
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Contains(t, updated.addPath, "x")

	updated.addFocus = fileFieldMime
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 't', Text: "t"})
	assert.Equal(t, "t", updated.addMime)

	updated.addFocus = fileFieldSize
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '1', Text: "1"})
	assert.Equal(t, "1", updated.addSize)

	updated.addFocus = fileFieldChecksum
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
	assert.Equal(t, "c", updated.addChecksum)

	updated.addFocus = fileFieldTags
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: ',', Text: ","})
	assert.Equal(t, []string{"a"}, updated.addTags)

	updated.addFocus = fileFieldMeta
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.addMeta.Active)
}

func TestFilesHandleEditKeysExtendedBranches(t *testing.T) {
	now := time.Now()
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active", CreatedAt: now}
	model.startEdit()

	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.editSaving)

	model.editSaving = false
	model.editMeta.Active = true
	updated, _ = model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.editMeta.Active)

	updated.editFocus = fileFieldStatus
	updated.editStatusIdx = 0
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, len(fileStatusOptions)-1, updated.editStatusIdx)

	updated.editFocus = fileFieldName
	updated.editName = "ab"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "a", updated.editName)

	updated.editFocus = fileFieldPath
	updated.editPath = "/tmp/ab"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "/tmp/a", updated.editPath)

	updated.editFocus = fileFieldMime
	updated.editMime = "text"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "tex", updated.editMime)

	updated.editFocus = fileFieldSize
	updated.editSize = "10"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "1", updated.editSize)

	updated.editFocus = fileFieldChecksum
	updated.editChecksum = "abcd"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "abc", updated.editChecksum)

	updated.editFocus = fileFieldTags
	updated.editTagBuf = "Z"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, []string{"z"}, updated.editTags)

	updated.editFocus = fileFieldMeta
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.editMeta.Active)

	updated.editMeta.Active = false
	updated.editFocus = fileFieldName
	updated.editName = "a"
	updated.editSize = "bad"
	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.Nil(t, cmd)
	assert.Contains(t, updated.errText, "non-negative")

	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, filesViewDetail, updated.view)
}

func TestFilesHandleEditKeysMetadataActiveAndUpGuardBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active"}
	model.startEdit()

	model.editMeta.Active = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.editMeta.Active)

	updated.editMeta.Active = false
	updated.editFocus = 0
	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.editFocus)
}

func TestFilesHandleEditKeysNavigationAndAppendBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a", Status: "active"}
	model.startEdit()

	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.editFocus)

	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.editFocus)

	updated.editFocus = fileFieldName
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Equal(t, "Alpha.txtx", updated.editName)

	updated.editFocus = fileFieldPath
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.Equal(t, "/tmp/ab", updated.editPath)

	updated.editFocus = fileFieldMime
	updated.editMime = ""
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 't', Text: "t"})
	assert.Equal(t, "t", updated.editMime)

	updated.editFocus = fileFieldSize
	updated.editSize = ""
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: '1', Text: "1"})
	assert.Equal(t, "1", updated.editSize)

	updated.editFocus = fileFieldChecksum
	updated.editChecksum = ""
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
	assert.Equal(t, "c", updated.editChecksum)
}

func TestFilesHandleAddKeysModeFocusBackspaceAndRenderBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd
	model.modeFocus = true

	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, filesViewList, updated.view)

	updated.view = filesViewAdd
	updated.addFocus = fileFieldPath
	updated.addPath = "/tmp/ab"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "/tmp/a", updated.addPath)

	updated.addFocus = fileFieldMime
	updated.addMime = "text"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "tex", updated.addMime)

	updated.addFocus = fileFieldSize
	updated.addSize = "12"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "1", updated.addSize)

	updated.addFocus = fileFieldChecksum
	updated.addChecksum = "abcd"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "abc", updated.addChecksum)

	updated.addFocus = fileFieldMeta
	updated.addMeta.Buffer = "k: v"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "k: v", updated.addMeta.Buffer)

	updated.addFocus = fileFieldPath
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, fileFieldName, updated.addFocus)

	updated.addName = "Alpha.txt"
	updated.addPath = "/tmp/alpha.txt"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, "", updated.addName)
	assert.Equal(t, "", updated.addPath)

	updated.addErr = "bad size"
	out := updated.renderAdd()
	assert.Contains(t, out, "bad size")

	updated.addSaved = true
	out = updated.renderAdd()
	assert.Contains(t, out, "Saved.")
}

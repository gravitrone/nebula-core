package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsStartEditNilDetailNoMutation(t *testing.T) {
	model := NewJobsModel(nil)
	model.editDesc = "keep-me"
	model.editSaving = true

	// startEdit with nil detail is a no-op
	model.startEdit()

	assert.Equal(t, "keep-me", model.editDesc)
	assert.True(t, model.editSaving)
}

func TestJobsHandleSubtaskInputTypingBackspaceAndEmptyEnter(t *testing.T) {
	model := NewJobsModel(nil)
	model.creatingSubtask = true

	updated, cmd := model.handleSubtaskInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.subtaskInput.Value())

	updated, cmd = updated.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "a ", updated.subtaskInput.Value())

	updated, cmd = updated.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.subtaskInput.Value())

	updated, cmd = updated.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.subtaskInput.Value())

	updated, cmd = updated.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.True(t, updated.creatingSubtask)
	assert.Equal(t, "", updated.subtaskInput.Value())
}

func TestFormatJobLineIncludesPriority(t *testing.T) {
	priority := "high"
	line := formatJobLine(api.Job{
		Title:    "\x1b[31mBuild\x1b[0m",
		Status:   "active",
		Priority: &priority,
	})
	assert.Contains(t, line, "Build")
	assert.Contains(t, line, "active")
	assert.Contains(t, line, "high")
}

func TestJobsHandleLinkInputTypingBackspaceAndBack(t *testing.T) {
	model := NewJobsModel(nil)
	model.detail = &api.Job{ID: "job-1"}
	model.linkingRel = true

	updated, cmd := model.handleLinkInput(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, "e", updated.linkInput.Value())

	updated, cmd = updated.handleLinkInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "e ", updated.linkInput.Value())

	updated, cmd = updated.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "e", updated.linkInput.Value())

	updated, cmd = updated.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.linkingRel)
	assert.Equal(t, "", updated.linkInput.Value())
}

func TestJobsToggleSelectAllNoItemsNoop(t *testing.T) {
	model := NewJobsModel(nil)
	model.selected = map[string]bool{"job-1": true}
	model.toggleSelectAll()
	assert.Equal(t, map[string]bool{"job-1": true}, model.selected)
}

func TestJobsHandleSubtaskInputBackAndNoopKey(t *testing.T) {
	model := NewJobsModel(nil)
	model.creatingSubtask = true
	model.subtaskInput.SetValue("child")

	updated, cmd := model.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.creatingSubtask)
	assert.Equal(t, "", updated.subtaskInput.Value())

	updated.creatingSubtask = true
	updated.subtaskInput.SetValue("x")
	updated, cmd = updated.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Nil(t, cmd)
	assert.True(t, updated.creatingSubtask)
	assert.Equal(t, "x", updated.subtaskInput.Value())
}

func TestJobsHandleLinkInputNoopKeyKeepsBuffer(t *testing.T) {
	model := NewJobsModel(nil)
	model.linkingRel = true
	model.linkInput.SetValue("entity ent-1 owns")

	updated, cmd := model.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Nil(t, cmd)
	assert.True(t, updated.linkingRel)
	assert.Equal(t, "entity ent-1 owns", updated.linkInput.Value())
}

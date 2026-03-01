package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsStartEditNilDetailNoMutation(t *testing.T) {
	model := NewJobsModel(nil)
	model.editFocus = jobEditFieldPriority
	model.editStatusIdx = 2
	model.editPriorityIdx = 1
	model.editDesc = "keep-me"
	model.editSaving = true

	model.startEdit()

	assert.Equal(t, jobEditFieldPriority, model.editFocus)
	assert.Equal(t, 2, model.editStatusIdx)
	assert.Equal(t, 1, model.editPriorityIdx)
	assert.Equal(t, "keep-me", model.editDesc)
	assert.True(t, model.editSaving)
}

func TestJobsHandleSubtaskInputTypingBackspaceAndEmptyEnter(t *testing.T) {
	model := NewJobsModel(nil)
	model.creatingSubtask = true

	updated, cmd := model.handleSubtaskInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.subtaskBuf)

	updated, cmd = updated.handleSubtaskInput(tea.KeyMsg{Type: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "a ", updated.subtaskBuf)

	updated, cmd = updated.handleSubtaskInput(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.subtaskBuf)

	updated, cmd = updated.handleSubtaskInput(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.subtaskBuf)

	updated, cmd = updated.handleSubtaskInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.True(t, updated.creatingSubtask)
	assert.Equal(t, "", updated.subtaskBuf)
}

func TestFormatJobLineIncludesPriorityAndMetadataPreview(t *testing.T) {
	priority := "high"
	line := formatJobLine(api.Job{
		Title:    "\x1b[31mBuild\x1b[0m",
		Status:   "active",
		Priority: &priority,
		Metadata: api.JSONMap{"summary": "  ship clean output  "},
	})
	assert.Contains(t, line, "Build")
	assert.Contains(t, line, "active")
	assert.Contains(t, line, "high")
	assert.Contains(t, line, "ship clean output")
}

func TestFormatJobLineMetadataFallsBackToSortedKey(t *testing.T) {
	line := formatJobLine(api.Job{
		Title:    "Batch",
		Status:   "pending",
		Metadata: api.JSONMap{"z": "later", "a": "first"},
	})
	assert.Contains(t, line, "Batch · pending")
	assert.Contains(t, line, "first")
}

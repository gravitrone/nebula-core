package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsHandleAddKeysAdditionalBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.view = jobsViewAdd

	// addSaving short-circuit branch
	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, model.addFocus, updated.addFocus)

	model.addSaving = false

	// down/up navigation branches while staying in add mode
	model.addFocus = jobFieldTitle
	updated, cmd = model.handleAddKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, jobFieldDescription, updated.addFocus)

	updated, cmd = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, jobFieldTitle, updated.addFocus)

	// status and priority left-wrap branches
	model.addFocus = jobFieldStatus
	model.addStatusIdx = 0
	updated, _ = model.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, len(jobStatusOptions)-1, updated.addStatusIdx)
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = jobFieldPriority
	updated.addPriorityIdx = 0
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, len(jobPriorityOptions)-1, updated.addPriorityIdx)
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.Equal(t, 0, updated.addPriorityIdx)

	// ctrl+s branch (returns save cmd without executing)
	updated.addFocus = jobFieldTitle
	updated.addFields[jobFieldTitle].value = "Ship tests"
	updated, cmd = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)

	// Esc reset branch (when save is not in-flight)
	updated.addSaving = false
	updated.addSaved = true
	updated.addErr = "boom"
	updated.addFields[jobFieldDescription].value = "desc"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addErr)
	assert.Equal(t, "", updated.addFields[jobFieldDescription].value)
}

func TestJobsRenderAddAndEditStateBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.width = 96

	model.addSaving = true
	assert.Contains(t, components.SanitizeText(model.renderAdd()), "Saving")

	model.addSaving = false
	model.addSaved = true
	assert.Contains(t, components.SanitizeText(model.renderAdd()), "Job saved")

	model.addSaved = false
	model.addErr = "bad add"
	model.addFocus = jobFieldStatus
	model.addStatusIdx = 0
	model.addPriorityIdx = 0
	addOut := components.SanitizeText(model.renderAdd())
	assert.Contains(t, addOut, "Error")
	assert.Contains(t, addOut, "bad add")

	model.addErr = ""
	model.addFocus = jobFieldPriority
	model.addPriorityIdx = 0
	addOut = components.SanitizeText(model.renderAdd())
	assert.Contains(t, addOut, "Priority:")
	assert.Contains(t, addOut, "  -")

	model.view = jobsViewEdit
	model.editFocus = jobEditFieldPriority
	model.editPriorityIdx = 0
	model.editDesc = ""
	model.editSaving = true
	editOut := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editOut, "Priority")
	assert.Contains(t, editOut, "Description")
	assert.Contains(t, editOut, "Saving")
	assert.Contains(t, editOut, "-")
}

func TestJobsHandleSubtaskInputNilDetailEnterIsSafe(t *testing.T) {
	model := NewJobsModel(nil)
	model.creatingSubtask = true
	model.subtaskBuf = "Child task"
	model.detail = nil

	require.NotPanics(t, func() {
		updated, cmd := model.handleSubtaskInput(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.False(t, updated.creatingSubtask)
		assert.Equal(t, "", updated.subtaskBuf)
	})
}

func TestJobsRenderEditWithLoadedDetail(t *testing.T) {
	desc := "job details"
	priority := "high"
	model := NewJobsModel(nil)
	model.width = 100
	model.detail = &api.Job{
		ID:          "job-1",
		Status:      "active",
		Priority:    &priority,
		Description: &desc,
	}
	model.startEdit()
	model.editFocus = jobEditFieldDescription

	out := components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Description")
	assert.Contains(t, out, "job details")
}

func TestJobsRenderEditDescriptionFocusBranch(t *testing.T) {
	model := NewJobsModel(nil)
	model.width = 90
	model.detail = &api.Job{ID: "job-1", Status: "pending"}
	model.startEdit()

	model.editFocus = jobEditFieldDescription
	model.editDesc = "ship this"

	out := components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Description:")
	assert.Contains(t, out, "ship this")
	assert.Contains(t, out, "█")
}

func TestJobsHandleEditKeysDownUpNavigationBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.view = jobsViewEdit
	model.detail = &api.Job{ID: "job-1", Status: "pending"}
	model.startEdit()

	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, jobEditFieldDescription, updated.editFocus)

	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, jobEditFieldStatus, updated.editFocus)
}

func TestJobsSaveAddCommandReturnsErrMsgOnCreateFailure(t *testing.T) {
	client := api.NewClient("http://127.0.0.1:9", "test-key", 20*time.Millisecond)
	model := NewJobsModel(client)
	model.addFields[jobFieldTitle].value = "Ship tests"
	model.addFields[jobFieldDescription].value = "desc"
	model.addStatusIdx = 0
	model.addPriorityIdx = 1

	updated, cmd := model.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)

	msg := cmd()
	_, ok := msg.(errMsg)
	assert.True(t, ok)
}

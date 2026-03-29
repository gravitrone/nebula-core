package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.addSaving)

	model.addSaving = false

	// addSaved + Esc resets the form
	model.addSaved = true
	model.addErr = "boom"
	updated, _ = model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addErr)
	assert.NotNil(t, updated.addForm)

	// First key press with nil form initializes it and returns init cmd.
	model2 := NewJobsModel(nil)
	model2.view = jobsViewAdd
	model2.addForm = nil
	_, cmd2 := model2.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.NotNil(t, cmd2)
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
	model.addForm = nil
	addOut := components.SanitizeText(model.renderAdd())
	assert.Contains(t, addOut, "Initializing")

	model.view = jobsViewEdit
	model.editForm = nil
	editOut := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editOut, "Initializing")

	model.detail = &api.Job{ID: "job-1", Status: "pending"}
	model.startEdit()
	model.editSaving = true
	editOut = components.SanitizeText(model.renderEdit())
	assert.Contains(t, editOut, "Saving")
}

func TestJobsHandleSubtaskInputNilDetailEnterIsSafe(t *testing.T) {
	model := NewJobsModel(nil)
	model.creatingSubtask = true
	model.subtaskInput.SetValue("Child task")
	model.detail = nil

	require.NotPanics(t, func() {
		updated, cmd := model.handleSubtaskInput(tea.KeyPressMsg{Code: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.False(t, updated.creatingSubtask)
		assert.Equal(t, "", updated.subtaskInput.Value())
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

	// startEdit initializes editForm; renderEdit returns non-empty output.
	assert.NotNil(t, model.editForm)
	out := components.SanitizeText(model.renderEdit())
	assert.NotEmpty(t, out)
}

func TestJobsRenderEditDescriptionFocusBranch(t *testing.T) {
	model := NewJobsModel(nil)
	model.width = 90
	model.detail = &api.Job{ID: "job-1", Status: "pending"}
	model.startEdit()

	// startEdit initializes editForm; renderEdit returns non-empty output.
	assert.NotNil(t, model.editForm)
	out := components.SanitizeText(model.renderEdit())
	assert.NotEmpty(t, out)
}

func TestJobsHandleEditKeysEarlyReturnBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.view = jobsViewEdit
	model.detail = &api.Job{ID: "job-1", Status: "pending"}
	model.startEdit()

	// editSaving blocks key input
	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.editSaving)

	// Esc goes back to detail
	model.editSaving = false
	updated, cmd = model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, jobsViewDetail, updated.view)
}

func TestJobsSaveAddCommandReturnsErrMsgOnCreateFailure(t *testing.T) {
	client := api.NewClient("http://127.0.0.1:9", "test-key", 20*time.Millisecond)
	model := NewJobsModel(client)
	model.addTitle = "Ship tests"
	model.addDesc = "desc"
	model.addStatus = jobStatusOptions[0]
	model.addPriority = jobPriorityOptions[1]

	updated, cmd := model.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)

	msg := cmd()
	_, ok := msg.(errMsg)
	assert.True(t, ok)
}

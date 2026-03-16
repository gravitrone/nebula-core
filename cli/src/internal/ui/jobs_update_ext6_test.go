package ui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsUpdateMessageBranchesMatrix(t *testing.T) {
	model := NewJobsModel(nil)

	model, cmd := model.Update(jobsLoadedMsg{items: []api.Job{{ID: "job-1", Title: "Alpha"}}})
	assert.False(t, model.loading)
	assert.Len(t, model.allItems, 1)
	require.Nil(t, cmd)

	model.detail = &api.Job{ID: "job-1"}
	model, cmd = model.Update(jobRelationshipsLoadedMsg{id: "job-1", relationships: []api.Relationship{{ID: "rel-1"}}})
	require.Nil(t, cmd)
	assert.Len(t, model.detailRels, 1)

	// Mismatched detail id should keep current relationships.
	model, cmd = model.Update(jobRelationshipsLoadedMsg{id: "job-2", relationships: []api.Relationship{{ID: "rel-2"}}})
	require.Nil(t, cmd)
	assert.Equal(t, "rel-1", model.detailRels[0].ID)

	// Relationship changed with no detail is a no-op.
	model.detail = nil
	model, cmd = model.Update(jobRelationshipChangedMsg{})
	require.Nil(t, cmd)

	// Relationship changed with detail reloads relationships.
	model.detail = &api.Job{ID: "job-1"}
	model, cmd = model.Update(jobRelationshipChangedMsg{})
	require.NotNil(t, cmd)

	model.changingSt = true
	model.statusBuf = "active"
	model.statusTargets = []string{"job-1"}
	model.detail = &api.Job{ID: "job-1"}
	model, cmd = model.Update(jobStatusUpdatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, model.changingSt)
	assert.Nil(t, model.detail)
	assert.Empty(t, model.statusBuf)
	assert.Nil(t, model.statusTargets)

	model.creatingSubtask = true
	model.subtaskBuf = "Subtask"
	model.detail = &api.Job{ID: "job-1"}
	model, cmd = model.Update(subtaskCreatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, model.creatingSubtask)
	assert.Nil(t, model.detail)
	assert.Empty(t, model.subtaskBuf)

	model.addSaving = true
	model, cmd = model.Update(jobCreatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, model.addSaving)
	assert.True(t, model.addSaved)
	assert.True(t, model.loading)

	model.loading = true
	model.addSaving = true
	model.editSaving = true
	model.changingSt = true
	model.statusTargets = []string{"job-1"}
	model.creatingSubtask = true
	model.linkingRel = true
	model.unlinkingRel = true
	model, cmd = model.Update(errMsg{err: errors.New("boom")})
	require.Nil(t, cmd)
	assert.False(t, model.loading)
	assert.False(t, model.addSaving)
	assert.False(t, model.editSaving)
	assert.False(t, model.changingSt)
	assert.Nil(t, model.statusTargets)
	assert.False(t, model.creatingSubtask)
	assert.False(t, model.linkingRel)
	assert.False(t, model.unlinkingRel)
	assert.Equal(t, "boom", model.addErr)
}

func TestJobsUpdateKeyRoutingBranches(t *testing.T) {
	model := NewJobsModel(nil)

	model.creatingSubtask = true
	model.subtaskBuf = "a"
	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Empty(t, updated.subtaskBuf)

	model = NewJobsModel(nil)
	model.linkingRel = true
	model.linkBuf = "ab"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.linkBuf)

	model = NewJobsModel(nil)
	model.unlinkingRel = true
	model.unlinkBuf = "ab"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.unlinkBuf)

	model = NewJobsModel(nil)
	model.changingSt = true
	model.statusBuf = "ab"
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.statusBuf)

	model = NewJobsModel(nil)
	model.modeFocus = true
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	model = NewJobsModel(nil)
	model.view = jobsViewAdd
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	model = NewJobsModel(nil)
	model.view = jobsViewEdit
	model.detail = &api.Job{ID: "job-1", Status: "pending"}
	model.startEdit()
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, jobsViewDetail, updated.view)

	model = NewJobsModel(nil)
	model.view = jobsViewDetail
	model.detail = &api.Job{ID: "job-1"}
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, jobsViewList, updated.view)

	model = NewJobsModel(nil)
	model.view = jobsViewList
	updated, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	require.Nil(t, cmd)
	assert.True(t, updated.filtering)
}

func TestJobsViewBranchMatrix(t *testing.T) {
	model := NewJobsModel(nil)
	model.width = 90

	model = NewJobsModel(nil)
	model.width = 90
	model.creatingSubtask = true
	model.detail = &api.Job{ID: "job-1"}
	model.subtaskBuf = "Subtask"
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "New Subtask Title")

	model = NewJobsModel(nil)
	model.width = 90
	model.linkingRel = true
	model.detail = &api.Job{ID: "job-1"}
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Link Job")

	model = NewJobsModel(nil)
	model.width = 90
	model.unlinkingRel = true
	model.detail = &api.Job{ID: "job-1"}
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Unlink Job")

	model = NewJobsModel(nil)
	model.width = 90
	model.changingSt = true
	model.statusBuf = "active"
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "New Status")

	model = NewJobsModel(nil)
	model.width = 90
	model.filtering = true
	model.view = jobsViewList
	model.searchBuf = "alpha"
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Filter Jobs")
}

package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsRenderModeLineStateVariants(t *testing.T) {
	model := NewJobsModel(nil)

	model.view = jobsViewList
	line := stripANSI(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "List")

	model.view = jobsViewAdd
	line = stripANSI(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "List")

	model.modeFocus = true
	line = stripANSI(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "List")

	model.view = jobsViewList
	line = stripANSI(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "List")
}

func TestJobsHandleFilterInputBranchMatrix(t *testing.T) {
	model := NewJobsModel(nil)
	model.filtering = true
	model.allItems = []api.Job{
		{ID: "job-1", Title: "alpha", Status: "active"},
		{ID: "job-2", Title: "beta", Status: "planning"},
	}
	model.applyJobSearch()

	updated, cmd := model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchBuf)
	require.NotEmpty(t, updated.items)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyDelete})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated.searchBuf = "alpha"
	updated.searchSuggest = "alpha"
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.searchBuf)
	assert.Equal(t, "", updated.searchSuggest)

	updated.filtering = true
	updated.searchBuf = "x"
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "x", updated.searchBuf)
}

func TestJobsHandleListKeysStatusAndSearchBranches(t *testing.T) {
	now := time.Now().UTC()
	model := NewJobsModel(nil)
	model.allItems = []api.Job{
		{ID: "job-1", Title: "alpha", Status: "active", CreatedAt: now},
		{ID: "job-2", Title: "beta", Status: "planning", CreatedAt: now},
	}
	model.applyJobSearch()
	model.list.SetItems([]string{"alpha", "beta"})

	model.searchBuf = "alp"
	model.searchSuggest = "alpha"
	updated, cmd := model.handleListKeys(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "alpha", updated.searchBuf)

	updated.searchBuf = "alpha"
	updated.searchSuggest = "alpha"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "alpha", updated.searchBuf)

	updated.searchBuf = "alpha"
	updated.searchSuggest = "alpha"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)
	assert.Equal(t, "", updated.searchSuggest)

	updated.searchBuf = "ab"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchBuf)

	updated.view = jobsViewList
	updated.items = append([]api.Job{}, updated.allItems...)
	updated.list.SetItems([]string{"alpha", "beta"})
	updated.list.Cursor = 0
	updated.searchBuf = ""
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyEnter})
	_ = cmd
	assert.Equal(t, jobsViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "job-1", updated.detail.ID)

	updated.view = jobsViewList
	updated.selected = map[string]bool{"job-2": true}
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	require.Nil(t, cmd)
	assert.True(t, updated.changingSt)
	assert.Equal(t, []string{"job-2"}, updated.statusTargets)

	updated.selected = map[string]bool{}
	updated.changingSt = false
	updated.statusTargets = nil
	updated.list.Cursor = 0
	updated.view = jobsViewList
	updated.detail = nil
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	require.Nil(t, cmd)
	assert.True(t, updated.changingSt)
	assert.Equal(t, []string{"job-1"}, updated.statusTargets)
	assert.Equal(t, jobsViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "job-1", updated.detail.ID)
}

func TestJobsSaveAddValidationAndSuccess(t *testing.T) {
	model := NewJobsModel(nil)
	updated, cmd := model.saveAdd()
	require.Nil(t, cmd)
	assert.Equal(t, "Title is required", updated.addErr)

	updated.addFields[jobFieldTitle].value = "Ship it"
	var seen api.CreateJobInput
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "job-1",
		}}))
	})

	model = NewJobsModel(client)
	model.addFields[jobFieldTitle].value = "Ship it"
	model.addFields[jobFieldDescription].value = "desc"
	model.addStatusIdx = 1
	model.addPriorityIdx = 2

	updated, cmd = model.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)
	msg := cmd()
	_, ok := msg.(jobCreatedMsg)
	assert.True(t, ok)
	assert.Equal(t, "Ship it", seen.Title)
	assert.Equal(t, "desc", seen.Description)
	assert.Equal(t, jobStatusOptions[1], seen.Status)
	assert.Equal(t, jobPriorityOptions[2], seen.Priority)
}

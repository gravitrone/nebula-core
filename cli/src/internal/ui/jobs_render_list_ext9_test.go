package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsRenderListLoadingEmptyAndPreviewBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.width = 90
	model.loading = true
	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Loading jobs...")

	model.loading = false
	model.items = nil
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "No jobs found.")

	now := time.Now().UTC()
	priority := "high"
	model.items = []api.Job{
		{ID: "job-1", Title: "alpha", Status: "", CreatedAt: now},
		{ID: "job-2", Title: "beta", Status: "active", Priority: &priority, CreatedAt: now},
	}
	model.list.SetItems([]string{"alpha", "beta"})
	model.selected = map[string]bool{"job-1": true}
	model.searchInput.SetValue("a")
	model.searchSuggest = "alpha"
	model.width = 84 // stacked layout branch
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "selected: 1")
	assert.Contains(t, out, "search: a")
	assert.Contains(t, out, "next: alpha")
	assert.Contains(t, out, "Selected")

	model.width = 170 // side-by-side layout branch
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Priority")
}

func TestJobsHandleListKeysAdditionalBranches(t *testing.T) {
	now := time.Now().UTC()
	model := NewJobsModel(nil)
	model.allItems = []api.Job{
		{ID: "job-1", Title: "alpha", Status: "active", CreatedAt: now},
		{ID: "job-2", Title: "beta", Status: "planning", CreatedAt: now},
	}
	model.applyJobSearch()
	model.list.SetItems([]string{"alpha", "beta"})

	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: 'b', Text: "b"})
	require.Nil(t, cmd)
	assert.True(t, updated.selected["job-1"])
	assert.True(t, updated.selected["job-2"])

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 'b', Text: "b"})
	require.Nil(t, cmd)
	assert.Empty(t, updated.selected)

	updated.searchInput.SetValue("alpha")
	updated.searchSuggest = "alpha"
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchInput.Value())
	assert.Equal(t, "", updated.searchSuggest)

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 'f', Text: "f"})
	require.Nil(t, cmd)
	assert.True(t, updated.filtering)

	updated.filtering = false
	updated.searchInput.SetValue("x")
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDelete})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchInput.Value())

	updated.list.Cursor = 9
	updated.changingSt = false
	updated.statusTargets = nil
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	require.Nil(t, cmd)
	assert.False(t, updated.changingSt)
	assert.Nil(t, updated.statusTargets)

	updated.searchInput.SetValue("")
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: ' ', Text: " "})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchInput.Value())
}

func TestJobsSaveEditNilDetailAndSuccess(t *testing.T) {
	model := NewJobsModel(nil)
	updated, cmd := model.saveEdit()
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	desc := "desc"
	var seen api.UpdateJobInput
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/jobs/job-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&seen))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"id": "job-1",
		}}))
	})

	model = NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1", Status: "pending", Description: &desc}
	model.startEdit()
	model.editStatusIdx = 1
	model.editPriorityIdx = 2
	model.editDesc = "updated desc"

	updated, cmd = model.saveEdit()
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)
	msg := cmd()
	_, ok := msg.(jobStatusUpdatedMsg)
	assert.True(t, ok)
	require.NotNil(t, seen.Status)
	require.NotNil(t, seen.Priority)
	require.NotNil(t, seen.Description)
	assert.Equal(t, jobStatusOptions[1], *seen.Status)
	assert.Equal(t, jobPriorityOptions[2], *seen.Priority)
	assert.Equal(t, "updated desc", *seen.Description)
}

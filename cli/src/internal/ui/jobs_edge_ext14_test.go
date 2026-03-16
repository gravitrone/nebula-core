package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsRenderListNarrowWidthStaleRowsAndModeFocus(t *testing.T) {
	now := time.Now().UTC()
	model := NewJobsModel(nil)
	model.width = 10
	model.items = []api.Job{
		{ID: "job-1", Title: "alpha", Status: "pending", CreatedAt: now},
		{ID: "job-2", Title: "beta", Status: "active", CreatedAt: now},
	}
	// Keep one extra stale visual row so renderList hits the absIdx bounds guard.
	model.list.SetItems([]string{"alpha", "beta", "ghost"})
	model.modeFocus = true

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Titl")
}

func TestJobsHandleListKeysDelegatesWhenFiltering(t *testing.T) {
	now := time.Now().UTC()
	model := NewJobsModel(nil)
	model.filtering = true
	model.allItems = []api.Job{{ID: "job-1", Title: "alpha", Status: "active", CreatedAt: now}}
	model.applyJobSearch()

	updated, cmd := model.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchBuf)
	assert.True(t, updated.filtering)
}

func TestJobsHandleStatusInputFallsBackToDetailWhenTargetsEmpty(t *testing.T) {
	var calls int
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/api/jobs/job-1/status" {
			calls++
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"ok": true}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1", Title: "Alpha", Status: "pending"}
	model.statusBuf = "done"
	model.statusTargets = nil
	model.changingSt = true

	updated, cmd := model.handleStatusInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.False(t, updated.changingSt)
	assert.Equal(t, "", updated.statusBuf)
	assert.Nil(t, updated.statusTargets)

	msg := cmd()
	_, ok := msg.(jobStatusUpdatedMsg)
	assert.True(t, ok)
	assert.Equal(t, 1, calls)
}

func TestJobsSaveEditReturnsErrMsgOnUpdateFailure(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/api/jobs/job-1" {
			w.WriteHeader(http.StatusInternalServerError)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "INTERNAL_ERROR",
					"message": "db down",
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1", Title: "Alpha", Status: "pending"}
	model.startEdit()
	model.editDesc = "note"

	updated, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)

	msg := cmd()
	_, ok := msg.(errMsg)
	assert.True(t, ok)
}

func TestJobsRenderDetailFallsBackToListWhenDetailMissing(t *testing.T) {
	now := time.Now().UTC()
	model := NewJobsModel(nil)
	model.width = 88
	model.items = []api.Job{{ID: "job-1", Title: "alpha", Status: "active", CreatedAt: now}}
	model.list.SetItems([]string{"alpha"})

	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "alpha")
}

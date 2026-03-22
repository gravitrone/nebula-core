package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestJobsListSearchSuggestToggleAddSaveAndReset handles test jobs list search suggest toggle add save and reset.
func TestJobsListSearchSuggestToggleAddSaveAndReset(t *testing.T) {
	now := time.Now()
	createCalled := false

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/jobs" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "job-1",
						"title":      "Alpha Job",
						"status":     "pending",
						"priority":   "high",
						"created_at": now,
						"updated_at": now,
					},
				},
			})
			require.NoError(t, err)
		case r.URL.Path == "/api/audit/scopes":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-1", "name": "public", "agent_count": 1},
				},
			})
			require.NoError(t, err)
		case r.URL.Path == "/api/jobs" && r.Method == http.MethodPost:
			createCalled = true
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":         "job-new",
					"title":      "New Job",
					"status":     "pending",
					"created_at": now,
					"updated_at": now,
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewJobsModel(client)
	model.width = 90

	// Init + load jobs + scopes.
	model, _ = model.Update(runCmdFirst(model.Init()))

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "Alpha Job")

	// Search suggest + tab completion.
	model, _ = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.Equal(t, "Alpha Job", model.searchSuggest)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, "Alpha Job", strings.TrimSpace(model.searchInput.Value()))

	// Toggle to Add via modeFocus.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.True(t, model.modeFocus)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, jobsViewAdd, model.view)

	// Enter title.
	for _, r := range "New Job" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// Save.
	var saveCmd tea.Cmd
	model, saveCmd = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.NotNil(t, saveCmd)
	msg := saveCmd()
	model, cmd := model.Update(msg)
	require.NotNil(t, cmd) // reload jobs

	assert.True(t, createCalled)
	assert.True(t, model.addSaved)
	assert.Contains(t, components.SanitizeText(model.View()), "Job saved!")

	// Esc should reset add state.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, model.addSaved)
	assert.Equal(t, "", model.addFields[jobFieldTitle].value)
}

// TestJobsDetailRendersAndEditSaves handles test jobs detail renders and edit saves.
func TestJobsDetailRendersAndEditSaves(t *testing.T) {
	now := time.Now()
	updateCalled := false

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/jobs/") && r.Method == http.MethodPatch:
			updateCalled = true
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":         "job-1",
					"title":      "Alpha Job",
					"status":     "active",
					"created_at": now,
					"updated_at": now,
				},
			})
			require.NoError(t, err)
		case r.URL.Path == "/api/jobs" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		case r.URL.Path == "/api/audit/scopes":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	desc := "hello"
	priority := "high"
	model := NewJobsModel(client)
	model.width = 90
	model.view = jobsViewDetail
	model.detail = &api.Job{
		ID:          "job-1",
		Title:       "Alpha Job",
		Description: &desc,
		Status:      "active",
		Priority:    &priority,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	model.detailContext = []api.Context{{ID: "ctx-1", Title: "Alpha Context", SourceType: "note", Status: "active"}}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Job")
	assert.Contains(t, out, "Alpha Job")
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "Alpha Context")

	// Enter edit mode.
	model, _ = model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	assert.Equal(t, jobsViewEdit, model.view)

	// Edit description and save.
	model.editFocus = jobEditFieldDescription
	for _, r := range " world" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	var saveCmd tea.Cmd
	model, saveCmd = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.NotNil(t, saveCmd)
	msg := saveCmd()
	model, _ = model.Update(msg)

	assert.True(t, updateCalled)
}

// TestJobsDetailRendersRelationshipsSummary handles test jobs detail renders relationships summary.
func TestJobsDetailRendersRelationshipsSummary(t *testing.T) {
	now := time.Now()
	model := NewJobsModel(nil)
	model.width = 100

	model.view = jobsViewDetail
	model.detail = &api.Job{
		ID:        "job-1",
		Title:     "Alpha Job",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	model.detailRels = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "job",
			SourceID:   "job-1",
			SourceName: "Alpha Job",
			TargetType: "entity",
			TargetID:   "ent-1",
			TargetName: "Owner",
			Type:       "assigned-to",
			Status:     "active",
			CreatedAt:  now,
		},
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "assigned-to")
	assert.Contains(t, out, "Owner")
}

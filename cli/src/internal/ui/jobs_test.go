package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testJobsClient handles test jobs client.
func testJobsClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestJobsModelInit handles test jobs model init.
func TestJobsModelInit(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"data": []map[string]any{}}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	assert.NotNil(t, cmd)
}

// TestJobsModelLoadsJobs handles test jobs model loads jobs.
func TestJobsModelLoadsJobs(t *testing.T) {
	priority := "high"
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "job-1", "status": "pending", "title": "Test Job", "priority": priority, "created_at": time.Now()},
				{"id": "job-2", "status": "active", "title": "Another Job", "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)

	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)
	model.applyJobSearch()

	assert.False(t, model.loading)
	assert.Len(t, model.items, 2)
	assert.Equal(t, "job-1", model.items[0].ID)
}

// TestJobsModelNavigationKeys handles test jobs model navigation keys.
func TestJobsModelNavigationKeys(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "job-1", "status": "pending", "title": "Job 1", "created_at": time.Now()},
				{"id": "job-2", "status": "active", "title": "Job 2", "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)
	model.applyJobSearch()

	// Navigate down
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, model.list.Selected())

	// Navigate up
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, model.list.Selected())
}

// TestJobsModelEnterShowsDetail handles test jobs model enter shows detail.
func TestJobsModelEnterShowsDetail(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "job-1", "status": "pending", "title": "Test Job", "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)
	model.applyJobSearch()

	// Press enter
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	assert.NotNil(t, model.detail)
	assert.Equal(t, "job-1", model.detail.ID)
}

// TestJobsModelEscapeBackFromDetail handles test jobs model escape back from detail.
func TestJobsModelEscapeBackFromDetail(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "job-1", "status": "pending", "title": "Test Job", "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)
	model.applyJobSearch()

	// Enter detail
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, model.detail)

	// Escape back
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, model.detail)
}

// TestJobsModelStatusChangeFlow handles test jobs model status change flow.
func TestJobsModelStatusChangeFlow(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "job-1", "status": "pending", "title": "Test Job", "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	// Press 's' to change status
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	assert.True(t, model.changingSt)
	assert.NotNil(t, model.detail)
}

// TestJobsModelStatusInputHandling handles test jobs model status input handling.
func TestJobsModelStatusInputHandling(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "job-1", "status": "pending", "title": "Test Job", "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	// Start status change
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	// Type "active"
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.Equal(t, "act", model.statusBuf)

	// Backspace
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "ac", model.statusBuf)

	// Escape to cancel
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, model.changingSt)
	assert.Equal(t, "", model.statusBuf)
}

// TestJobsModelRenderEmpty handles test jobs model render empty.
func TestJobsModelRenderEmpty(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"data": []map[string]any{}}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	view := model.View()
	assert.Contains(t, view, "No jobs found")
}

// TestJobsListClampsLongRows ensures list rendering stays within the box width.
func TestJobsListClampsLongRows(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"data": []map[string]any{}}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	model.loading = false
	model.width = 60
	model.items = []api.Job{
		{
			ID:        "job-1",
			Title:     strings.Repeat("long-title-", 20),
			Status:    "in-progress",
			CreatedAt: time.Now(),
		},
	}
	model.applyJobSearch()

	view := model.renderList()
	maxWidth := lipgloss.Width(strings.Split(components.Box("x", model.width), "\n")[0])
	for _, line := range strings.Split(view, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), maxWidth)
	}
}

// TestJobsModelRenderLoading handles test jobs model render loading.
func TestJobsModelRenderLoading(t *testing.T) {
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {})

	model := NewJobsModel(client)
	model.loading = true

	view := model.View()
	assert.Contains(t, view, "Loading jobs")
}

// TestJobsModelCreateSubtask handles test jobs model create subtask.
func TestJobsModelCreateSubtask(t *testing.T) {
	var subtaskTitle string
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/jobs":
			resp := map[string]any{
				"data": []map[string]any{
					{"id": "job-1", "status": "pending", "title": "Test Job", "created_at": time.Now()},
				},
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		case r.URL.Path == "/api/jobs/job-1/subtasks" && r.Method == http.MethodPost:
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			subtaskTitle = body["title"]
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "job-1"}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	msg = cmd()
	_, _ = model.Update(msg)

	assert.Equal(t, "foo", subtaskTitle)
}

// TestJobsSearchFiltersList handles test jobs search filters list.
func TestJobsSearchFiltersList(t *testing.T) {
	model := NewJobsModel(nil)
	model.allItems = []api.Job{{ID: "job-1", Title: "Alpha", Status: "pending"}, {ID: "job-2", Title: "Beta", Status: "active"}}
	model.searchBuf = "al"
	model.applyJobSearch()

	assert.Len(t, model.items, 1)
	assert.Equal(t, "job-1", model.items[0].ID)
}

// TestJobsLinkInputCreatesRelationship handles test jobs link input creates relationship.
func TestJobsLinkInputCreatesRelationship(t *testing.T) {
	var received api.CreateRelationshipInput
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/relationships" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		err := json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":          "rel-1",
				"source_type": "job",
				"source_id":   "job-1",
				"target_type": "entity",
				"target_id":   "ent-1",
				"type":        "about",
				"status":      "active",
			},
		})
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1", Title: "Job"}
	model.linkingRel = true
	model.linkBuf = "entity ent-1 about"

	model, cmd := model.handleLinkInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(jobRelationshipChangedMsg)
	require.True(t, ok)
	assert.False(t, model.linkingRel)
	assert.Equal(t, api.CreateRelationshipInput{
		SourceType: "job",
		SourceID:   "job-1",
		TargetType: "entity",
		TargetID:   "ent-1",
		Type:       "about",
	}, received)
}

// TestJobsUnlinkInputSupportsRowIndex handles test jobs unlink input supports row index.
func TestJobsUnlinkInputSupportsRowIndex(t *testing.T) {
	var updatedID string
	var updatedPayload api.UpdateRelationshipInput
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		updatedID = strings.TrimPrefix(r.URL.Path, "/api/relationships/")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&updatedPayload))
		err := json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":     updatedID,
				"status": "archived",
			},
		})
		require.NoError(t, err)
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1", Title: "Job"}
	model.detailRels = []api.Relationship{
		{ID: "rel-1", SourceType: "job", SourceID: "job-1", TargetType: "entity", TargetID: "ent-1", Type: "about"},
	}
	model.unlinkingRel = true
	model.unlinkBuf = "1"

	model, cmd := model.handleUnlinkInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(jobRelationshipChangedMsg)
	require.True(t, ok)
	require.NotNil(t, updatedPayload.Status)
	assert.False(t, model.unlinkingRel)
	assert.Equal(t, "rel-1", updatedID)
	assert.Equal(t, "archived", *updatedPayload.Status)
}

// TestJobsRenderEditShowsFields handles test jobs render edit shows fields.
func TestJobsRenderEditShowsFields(t *testing.T) {
	now := time.Now()
	model := NewJobsModel(nil)
	model.width = 100
	model.detail = &api.Job{
		ID:        "job-1",
		Title:     "Edit Me",
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	}
	model.startEdit()
	model.view = jobsViewEdit

	out := model.renderEdit()
	assert.Contains(t, out, "Status:")
	assert.Contains(t, out, "Description:")
	assert.Contains(t, out, "Priority:")
}

// TestJobsBulkStatusUpdateForSelectedRows handles test jobs bulk status update for selected rows.
func TestJobsBulkStatusUpdateForSelectedRows(t *testing.T) {
	var statusCalls []string
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/jobs" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "job-1", "status": "pending", "title": "Job 1", "created_at": time.Now()},
					{"id": "job-2", "status": "pending", "title": "Job 2", "created_at": time.Now()},
				},
			})
			require.NoError(t, err)
		case strings.HasPrefix(r.URL.Path, "/api/jobs/") && strings.HasSuffix(r.URL.Path, "/status") && r.Method == http.MethodPatch:
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			statusCalls = append(statusCalls, r.URL.Path+"="+payload["status"])
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":     strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/jobs/"), "/status"),
					"status": payload["status"],
					"title":  "updated",
				},
			})
			require.NoError(t, err)
		case r.URL.Path == "/api/audit/scopes":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewJobsModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)
	model.applyJobSearch()

	// Select both jobs.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.Equal(t, 2, model.selectedCount())

	// Trigger bulk status update.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.True(t, model.changingSt)
	for _, r := range "active" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.ElementsMatch(
		t,
		[]string{"/api/jobs/job-1/status=active", "/api/jobs/job-2/status=active"},
		statusCalls,
	)
	assert.Equal(t, 0, model.selectedCount())
}

// TestJobsToggleSelectAllSelectsAndClears handles test jobs toggle select all selects and clears.
func TestJobsToggleSelectAllSelectsAndClears(t *testing.T) {
	model := NewJobsModel(nil)
	model.items = []api.Job{
		{ID: "job-1", Title: "Alpha"},
		{ID: "job-2", Title: "Beta"},
	}

	model.toggleSelectAll()
	assert.Len(t, model.selected, 2)
	assert.True(t, model.selected["job-1"])
	assert.True(t, model.selected["job-2"])

	model.toggleSelectAll()
	assert.Empty(t, model.selected)
}

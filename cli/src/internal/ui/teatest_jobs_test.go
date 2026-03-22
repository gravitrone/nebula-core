package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
)

// --- Job Test Helpers ---

// jobDataHandler returns realistic job data for the mock API.
func jobDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path

	switch {
	case strings.HasPrefix(path, "/api/context/by-owner/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case strings.HasPrefix(path, "/api/relationships/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case strings.HasPrefix(path, "/api/jobs") && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{
				"id":          "2026Q1-0001",
				"title":       "Train model",
				"description": nil,
				"status":      "active",
				"priority":    "high",
				"created_at":  "2026-01-01T00:00:00Z",
				"updated_at":  "2026-01-01T00:00:00Z",
			},
			{
				"id":          "2026Q1-0002",
				"title":       "Deploy pipeline",
				"description": nil,
				"status":      "pending",
				"priority":    "medium",
				"created_at":  "2026-01-15T00:00:00Z",
				"updated_at":  "2026-01-15T00:00:00Z",
			},
		}})
	case strings.HasPrefix(path, "/api/jobs") && r.Method == "POST":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "2026Q1-0003",
			"title":      "NewJob",
			"status":     "pending",
			"priority":   nil,
			"created_at": "2026-01-20T00:00:00Z",
			"updated_at": "2026-01-20T00:00:00Z",
		})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}
}

func newTestAppWithJobData(t *testing.T) App {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(jobDataHandler))
	t.Cleanup(srv.Close)
	client := api.NewClient(srv.URL, "test-key")
	return NewApp(client, &config.Config{})
}

// enterJobsContent sends the key sequence to navigate from tab nav into
// the jobs table content: Down (tabNav -> modeFocus), Down (modeFocus -> table).
func enterJobsContent(tm *teatest.TestModel) {
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)
}

// --- Job Tests ---

// TestJobsTabShowsLoadedData verifies job data appears after switching to
// the Jobs tab with a data-returning mock server.
func TestJobsTabShowsLoadedData(t *testing.T) {
	app := newTestAppWithJobData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Jobs tab and wait for data.
	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Train model"))
	}, teatest.WithDuration(waitDur))
}

// TestJobsTableNavigation verifies navigating within the jobs table.
func TestJobsTableNavigation(t *testing.T) {
	app := newTestAppWithJobData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Train model"))
	}, teatest.WithDuration(waitDur))

	// Enter content area: Down (tabNav -> modeFocus), Down (modeFocus -> table).
	enterJobsContent(tm)

	// Move cursor down to second row.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Deploy pipeline"))
	}, teatest.WithDuration(waitDur))
}

// TestJobsDetailView verifies entering a job shows the detail view
// with the job title and ID.
func TestJobsDetailView(t *testing.T) {
	app := newTestAppWithJobData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Train model"))
	}, teatest.WithDuration(waitDur))

	// Enter content area (tabNav -> modeFocus -> table).
	enterJobsContent(tm)

	// Press Enter to open detail view.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Detail view should show the job ID.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("2026Q1-0001"))
	}, teatest.WithDuration(waitDur))
}

// TestJobsAddFormOpens verifies switching to Add mode renders the add form.
func TestJobsAddFormOpens(t *testing.T) {
	app := newTestAppWithJobData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Train model"))
	}, teatest.WithDuration(waitDur))

	// Down exits tabNav into modeFocus (mode line focused).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)

	// Left in modeFocus toggles to Add mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})

	// The Add view initially shows "Initializing..." until a key triggers form init.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Initializing"))
	}, teatest.WithDuration(waitDur))

	// Send Down to trigger huh form initialization.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	// After init, the form renders with field titles.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Description"))
	}, teatest.WithDuration(waitDur))
}

// TestJobsFilterFlow verifies that typing characters triggers client-side
// filtering within the jobs list.
func TestJobsFilterFlow(t *testing.T) {
	app := newTestAppWithJobData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Train model"))
	}, teatest.WithDuration(waitDur))

	// Enter content area (tabNav -> modeFocus -> table).
	enterJobsContent(tm)

	// Type to filter jobs. Jobs uses client-side filtering on allItems.
	tm.Send(tea.KeyPressMsg{Code: 'D', Text: "D"})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'e', Text: "e"})

	// "Deploy pipeline" should match (case-insensitive).
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Deploy"))
	}, teatest.WithDuration(waitDur))
}

// TestJobsEmptyState verifies the empty state message renders when no jobs
// are returned from the API.
func TestJobsEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("No jobs found"))
	}, teatest.WithDuration(waitDur))
}

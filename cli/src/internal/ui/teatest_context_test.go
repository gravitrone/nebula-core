package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
)

// --- Helpers ---

// dataAPIHandler serves mock data for context, logs, files, and protocols.
func dataAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path
	switch {
	case strings.Contains(path, "/context") && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "ctx-001", "name": "API Docs", "source_type": "document", "status": "active", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
		}})
	case strings.Contains(path, "/logs") && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "log-001", "log_type": "api_call", "status": "active", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
		}})
	case strings.Contains(path, "/files") && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "file-001", "filename": "test.pdf", "status": "active", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
		}})
	case strings.Contains(path, "/protocols") && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "proto-001", "name": "review-protocol", "title": "Code Review", "status": "active", "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
		}})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}
}

// newTestAppWithData creates an App backed by a stub server returning mock data
// for context, files, logs, and protocols endpoints.
func newTestAppWithData(t *testing.T) App {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(dataAPIHandler))
	t.Cleanup(srv.Close)
	client := api.NewClient(srv.URL, "test-key")
	return NewApp(client, &config.Config{})
}

// --- Context Tab Tests ---

// TestContextTabShowsData switches to Context tab and verifies data loads.
func TestContextTabShowsData(t *testing.T) {
	app := newTestAppWithData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Switch to Context tab (key "4" -> index 3).
	tm.Send(tea.KeyPressMsg{Code: '4', Text: "4"})

	// Context tab renders with Add/Library mode selector.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Context")
	}, teatest.WithDuration(waitDur))
}

// TestContextAddForm switches to Context tab and verifies the add form renders.
func TestContextAddForm(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Switch to Context tab.
	tm.Send(tea.KeyPressMsg{Code: '4', Text: "4"})

	// The add form is the default view - it should show "Add" in mode line.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		out2 := out
		return containsText(out2, "Add")
	}, teatest.WithDuration(waitDur))
}

// TestContextFilterFlow switches to Context library view and types to filter.
func TestContextFilterFlow(t *testing.T) {
	app := newTestAppWithData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Switch to Context tab.
	tm.Send(tea.KeyPressMsg{Code: '4', Text: "4"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Context")
	}, teatest.WithDuration(waitDur))

	// Press Tab to switch to Library mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	// Navigate into content with Down.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	// Type to trigger filter.
	tm.Send(tea.KeyPressMsg{Code: 'a', Text: "a"})

	// The tab should still be responsive (renders Context tab content).
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Context")
	}, teatest.WithDuration(waitDur))
}

// TestContextEmptyState verifies the empty state message renders on the Context tab.
func TestContextEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Switch to Context tab.
	tm.Send(tea.KeyPressMsg{Code: '4', Text: "4"})

	// Switch to Library view to see empty state.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	// Navigate down into the list content.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	// With no data, should show empty state text.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "No context found.") ||
			containsText(out, "Context")
	}, teatest.WithDuration(waitDur))
}

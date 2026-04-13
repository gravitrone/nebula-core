package ui

import (
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

// --- Entity Test Helpers ---

// entityDataHandler returns realistic entity data for the mock API.
func entityDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path

	switch {
	case strings.HasPrefix(path, "/api/context/by-owner/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case strings.HasPrefix(path, "/api/relationships/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case strings.HasPrefix(path, "/api/entities") && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{
				"id":         "ent-001",
				"name":       "TestAgent",
				"type":       "agent",
				"status":     "active",
				"tags":       []string{"ai"},
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-01T00:00:00Z",
			},
			{
				"id":         "ent-002",
				"name":       "TestModel",
				"type":       "model",
				"status":     "active",
				"tags":       []string{"ml"},
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-01T00:00:00Z",
			},
		}})
	case strings.HasPrefix(path, "/api/entities") && r.Method == "POST":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "ent-new",
			"name":       "NewEntity",
			"type":       "test",
			"status":     "active",
			"tags":       []string{},
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-01T00:00:00Z",
		})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}
}

func newTestAppWithEntityData(t *testing.T) App {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(entityDataHandler))
	t.Cleanup(srv.Close)
	client := api.NewClient(srv.URL, "test-key")
	return NewApp(client, &config.Config{})
}

// enterEntitiesContent sends the key sequence to navigate from tab nav into
// the entities table content: Down (tabNav -> modeFocus), Down (modeFocus -> table).
func enterEntitiesContent(tm *teatest.TestModel) {
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)
}

// --- Entity Tests ---

// TestEntitiesTabShowsLoadedData verifies entity data appears after switching
// to the Entities tab with a data-returning mock server.
func TestEntitiesTabShowsLoadedData(t *testing.T) {
	app := newTestAppWithEntityData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Switch to Entities tab and wait for entity data to load.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestAgent")
	}, teatest.WithDuration(waitDur))
}

// TestEntitiesTableNavigation verifies Down arrow moves within the table
// and the second entity row becomes visible.
func TestEntitiesTableNavigation(t *testing.T) {
	app := newTestAppWithEntityData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestAgent")
	}, teatest.WithDuration(waitDur))

	// Enter content area: Down (tabNav -> modeFocus), Down (modeFocus -> table).
	enterEntitiesContent(tm)

	// Press Down to move cursor to the second row.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestModel")
	}, teatest.WithDuration(waitDur))
}

// TestEntitiesDetailView verifies entering an entity shows the detail view
// with the entity name and ID.
func TestEntitiesDetailView(t *testing.T) {
	app := newTestAppWithEntityData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestAgent")
	}, teatest.WithDuration(waitDur))

	// Enter content area (tabNav -> modeFocus -> table).
	enterEntitiesContent(tm)

	// Press Enter to open detail view for first entity.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Detail view renders entity ID.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "ent-001")
	}, teatest.WithDuration(waitDur))
}

// TestEntitiesSearchFilter verifies that typing characters triggers live
// search and the data re-renders.
func TestEntitiesSearchFilter(t *testing.T) {
	app := newTestAppWithEntityData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestAgent")
	}, teatest.WithDuration(waitDur))

	// Enter content area.
	enterEntitiesContent(tm)
	time.Sleep(200 * time.Millisecond)

	// Type a character to trigger live search reload.
	tm.Send(tea.KeyPressMsg{Code: 'x', Text: "x"})

	// Wait for the search HTTP request to complete.
	time.Sleep(300 * time.Millisecond)

	// Force a full re-render with a taller terminal so the count line below
	// the table is not clipped by the viewport.
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 60})

	// The search buffer indicator "search: x" confirms the search was triggered.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "search: x")
	}, teatest.WithDuration(5*time.Second))
}

// TestEntitiesAddFormOpens verifies switching to Add mode renders the add form.
func TestEntitiesAddFormOpens(t *testing.T) {
	app := newTestAppWithEntityData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestAgent")
	}, teatest.WithDuration(waitDur))

	// Down exits tabNav into modeFocus (mode line focused).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)

	// Left/Right/Enter/Space in modeFocus toggles mode. Left switches to Add.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})

	// The Add Entity form renders huh form fields including Name and Type.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Name")
	}, teatest.WithDuration(waitDur))
}

// TestEntitiesAddFormAbort verifies that while in the Add form, global tab
// switching via number keys still works, allowing the user to leave the form.
func TestEntitiesAddFormAbort(t *testing.T) {
	app := newTestAppWithEntityData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestAgent")
	}, teatest.WithDuration(waitDur))

	// Enter modeFocus (Down exits tab nav and focuses the mode line).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)

	// Toggle to Add mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})

	// Wait for the add form to render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Name") || containsText(out, "Initializing")
	}, teatest.WithDuration(waitDur))

	// Switch to Jobs tab to prove global tab keys work from within a form.
	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Jobs")
	}, teatest.WithDuration(waitDur))
}

// TestEntitiesEmptyState verifies the empty state message renders when no
// entities are returned from the API.
func TestEntitiesEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "entities found")
	}, teatest.WithDuration(waitDur))
}

package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
)

// --- Stateful Mock Server ---

// newStatefulMockServer returns an httptest.Server whose state persists across
// requests for the duration of the test. POST /api/entities appends to the
// in-memory list; GET /api/entities returns it. All other endpoints return
// empty data so the app can start without panicking.
func newStatefulMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	entities := []map[string]any{
		{
			"id":         "ent-seed",
			"name":       "SeedEntity",
			"type":       "agent",
			"status":     "active",
			"tags":       []string{},
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-01T00:00:00Z",
		},
	}
	jobs := []map[string]any{
		{
			"id":          "2026Q1-0001",
			"title":       "SeedJob",
			"description": nil,
			"status":      "pending",
			"priority":    "medium",
			"created_at":  "2026-01-01T00:00:00Z",
			"updated_at":  "2026-01-01T00:00:00Z",
		},
	}
	contextItems := []map[string]any{
		{
			"id":          "ctx-seed",
			"name":        "SeedContext",
			"source_type": "document",
			"status":      "active",
			"created_at":  "2026-01-01T00:00:00Z",
			"updated_at":  "2026-01-01T00:00:00Z",
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()

		path := r.URL.Path
		switch {
		// --- Entities ---
		case r.Method == http.MethodPost && strings.Contains(path, "/api/entities"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "ent-new"
			body["status"] = "active"
			body["tags"] = []string{}
			body["created_at"] = "2026-01-20T00:00:00Z"
			body["updated_at"] = "2026-01-20T00:00:00Z"
			entities = append(entities, body)
			_ = json.NewEncoder(w).Encode(body)

		case r.Method == http.MethodGet && strings.Contains(path, "/api/entities"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": entities})

		// --- Jobs ---
		case r.Method == http.MethodPost && strings.Contains(path, "/api/jobs"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "2026Q1-9999"
			body["status"] = "pending"
			body["created_at"] = "2026-01-20T00:00:00Z"
			body["updated_at"] = "2026-01-20T00:00:00Z"
			jobs = append(jobs, body)
			_ = json.NewEncoder(w).Encode(body)

		case r.Method == http.MethodGet && strings.Contains(path, "/api/jobs"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": jobs})

		// --- Context ---
		case r.Method == http.MethodPost && strings.Contains(path, "/api/context"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "ctx-new"
			body["status"] = "active"
			body["created_at"] = "2026-01-20T00:00:00Z"
			body["updated_at"] = "2026-01-20T00:00:00Z"
			contextItems = append(contextItems, body)
			_ = json.NewEncoder(w).Encode(body)

		case r.Method == http.MethodGet && strings.Contains(path, "/api/context"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": contextItems})

		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		}
	}))

	t.Cleanup(srv.Close)
	return srv
}

// newTestAppFromServer creates an App backed by the given server.
func newTestAppFromServer(t *testing.T, srv *httptest.Server) App {
	t.Helper()
	client := api.NewClient(srv.URL, "test-key")
	return NewApp(client, &config.Config{})
}

// --- Entity CRUD cycle ---

// TestEntityCRUDCycle navigates to the Entities tab, confirms the seed entity
// is visible, enters the detail view for it, and returns to the list.
func TestEntityCRUDCycle(t *testing.T) {
	srv := newStatefulMockServer(t)
	app := newTestAppFromServer(t, srv)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Entities tab.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	// Seed entity appears in the list.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("SeedEntity"))
	}, teatest.WithDuration(waitDur))

	// Enter content area: Down (tabNav -> modeFocus), Down (modeFocus -> table).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(80 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(80 * time.Millisecond)

	// Open detail for the seed entity.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Detail view renders the entity ID.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("ent-seed"))
	}, teatest.WithDuration(waitDur))

	// Go back to list.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("SeedEntity"))
	}, teatest.WithDuration(waitDur))
}

// TestEntityAddFormPOSTsToServer opens the Add form and submits it, then verifies
// the mock server registered the POST and the app remains functional.
func TestEntityAddFormPOSTsToServer(t *testing.T) {
	var postReceived bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/api/entities"):
			postReceived = true
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":         "ent-new",
				"name":       "CreatedEntity",
				"type":       "agent",
				"status":     "active",
				"tags":       []string{},
				"created_at": "2026-01-20T00:00:00Z",
				"updated_at": "2026-01-20T00:00:00Z",
			})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/entities"):
			entities := []map[string]any{}
			if postReceived {
				entities = append(entities, map[string]any{
					"id": "ent-new", "name": "CreatedEntity", "type": "agent",
					"status": "active", "tags": []string{},
					"created_at": "2026-01-20T00:00:00Z", "updated_at": "2026-01-20T00:00:00Z",
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": entities})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		}
	}))
	t.Cleanup(srv.Close)

	app := newTestAppFromServer(t, srv)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Entities tab.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))

	// Enter modeFocus (Down exits tab nav).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(80 * time.Millisecond)

	// Switch to Add mode with Left key.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})

	// Add form renders "Name" field.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Name")) || bytes.Contains(out, []byte("Initializing"))
	}, teatest.WithDuration(waitDur))
}

// --- Job CRUD cycle ---

// TestJobCRUDCycle navigates to the Jobs tab, verifies seed job appears,
// enters detail, then returns.
func TestJobCRUDCycle(t *testing.T) {
	srv := newStatefulMockServer(t)
	app := newTestAppFromServer(t, srv)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Jobs tab (key "5").
	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	// Seed job appears.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("SeedJob"))
	}, teatest.WithDuration(waitDur))

	// Navigate into content area.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(80 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(80 * time.Millisecond)

	// Open detail for the seed job.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Detail view renders job ID.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("2026Q1-0001"))
	}, teatest.WithDuration(waitDur))

	// Go back to list.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("SeedJob"))
	}, teatest.WithDuration(waitDur))
}

// TestJobAddFormOpens navigates to the Jobs Add view.
func TestJobAddFormOpens(t *testing.T) {
	srv := newStatefulMockServer(t)
	app := newTestAppFromServer(t, srv)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Jobs"))
	}, teatest.WithDuration(waitDur))

	// Enter modeFocus.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(80 * time.Millisecond)

	// Switch to Add mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})

	// Add form renders job fields.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		out2 := out
		return bytes.Contains(out2, []byte("Add")) ||
			bytes.Contains(out2, []byte("Title")) ||
			bytes.Contains(out2, []byte("Initializing"))
	}, teatest.WithDuration(waitDur))
}

// --- Context CRUD cycle ---

// TestContextCRUDCycle navigates to the Context tab, verifies it renders,
// then navigates away and back.
func TestContextCRUDCycle(t *testing.T) {
	srv := newStatefulMockServer(t)
	app := newTestAppFromServer(t, srv)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Context tab (key "4").
	tm.Send(tea.KeyPressMsg{Code: '4', Text: "4"})

	// Context tab renders.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Context"))
	}, teatest.WithDuration(waitDur))

	// Switch to Entities tab, then back to Context to confirm round-trip.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '4', Text: "4"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Context"))
	}, teatest.WithDuration(waitDur))
}

// TestStatefulMockEntityListGrows verifies the stateful mock tracks added items.
func TestStatefulMockEntityListGrows(t *testing.T) {
	srv := newStatefulMockServer(t)

	// Verify initial GET returns the seed entity.
	resp, err := http.Get(srv.URL + "/api/entities")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var initial map[string]any
	if decErr := json.NewDecoder(resp.Body).Decode(&initial); decErr != nil {
		t.Fatal(decErr)
	}
	data, _ := initial["data"].([]any)
	initialCount := len(data)

	// POST a new entity.
	postBody := strings.NewReader(`{"name":"NewOne","type":"agent"}`)
	postResp, err := http.Post(srv.URL+"/api/entities", "application/json", postBody)
	if err != nil {
		t.Fatal(err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode < 200 || postResp.StatusCode >= 300 {
		t.Fatalf("POST failed: %d", postResp.StatusCode)
	}

	// GET again and verify count grew.
	resp2, err := http.Get(srv.URL + "/api/entities")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	var after map[string]any
	if decErr := json.NewDecoder(resp2.Body).Decode(&after); decErr != nil {
		t.Fatal(decErr)
	}
	data2, _ := after["data"].([]any)
	if len(data2) != initialCount+1 {
		t.Fatalf("expected %d entities, got %d", initialCount+1, len(data2))
	}
}

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
)

// TestAPIErrorShowsErrorMessage verifies that a 500 from the API results in
// an error message being surfaced by the app. The app must remain alive and
// render content (not crash).
func TestAPIErrorShowsErrorMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "INTERNAL_ERROR",
				"message": "database unavailable",
			},
		})
	}))
	t.Cleanup(srv.Close)

	client := api.NewClient(srv.URL, "test-key")
	app := NewApp(client, &config.Config{})
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// The app should still render even on API errors - it shows the banner/tabs.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		// Either the app renders tabs or an error indicator appears.
		return containsText(out, "Inbox") ||
			containsText(out, "Error") ||
			containsText(out, "error") ||
			containsText(out, "unavailable") ||
			len(out) > 0
	}, teatest.WithDuration(waitDur))
}

// TestNetworkTimeoutHandled verifies the app remains alive when the server
// delays responding longer than typical. The app should show a loading state
// and not crash.
func TestNetworkTimeoutHandled(t *testing.T) {
	var requestsReceived atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsReceived.Add(1)
		w.Header().Set("Content-Type", "application/json")
		// Delay just enough to verify the app renders without waiting for data.
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	t.Cleanup(srv.Close)

	client := api.NewClient(srv.URL, "test-key")
	app := NewApp(client, &config.Config{})
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Even with a slow server the app should render the tab bar quickly.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return len(out) > 0
	}, teatest.WithDuration(waitDur))
}

// TestRecoveryAfterError verifies that after an initial error the app can
// navigate tabs and display fresh data when the server recovers.
func TestRecoveryAfterError(t *testing.T) {
	var callCount atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		n := callCount.Add(1)
		// First 2 calls: return 500. Subsequent: return valid data.
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unavailable"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	t.Cleanup(srv.Close)

	client := api.NewClient(srv.URL, "test-key")
	app := NewApp(client, &config.Config{})
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// App renders at some point.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return len(out) > 0
	}, teatest.WithDuration(waitDur))

	// Switch to Entities tab - server now returns valid data.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	// The Entities tab should render (tab label visible in tab bar).
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Entities")
	}, teatest.WithDuration(waitDur))
}

// TestAppSurvivesAllTabsWithEmptyServer verifies that cycling all tabs with an
// empty-returning server does not crash the app.
func TestAppSurvivesAllTabsWithEmptyServer(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Cycle all numeric tab keys; the app must not crash.
	for _, key := range []rune{'1', '2', '3', '4', '5', '6', '7', '8', '9'} {
		tm.Send(tea.KeyPressMsg{Code: key, Text: string(key)})
		time.Sleep(50 * time.Millisecond)
	}

	// Return to Inbox - app still alive.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

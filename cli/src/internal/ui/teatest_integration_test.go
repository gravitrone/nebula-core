package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
)

// --- Helpers ---

const waitDur = 3 * time.Second

// ansiRe matches ANSI escape sequences that lipgloss/bubbletea insert into
// rendered output. These sequences can split text tokens, breaking naive
// bytes.Contains checks.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x1b]*\x1b\\`)

// containsText returns true if out contains needle after stripping ANSI
// escape sequences from out.
func containsText(out []byte, needle string) bool {
	clean := ansiRe.ReplaceAll(out, nil)
	return bytes.Contains(clean, []byte(needle))
}

// emptyAPIHandler returns empty JSON arrays for all API endpoints so the app
// can initialize without panicking on nil client dereferences.
func emptyAPIHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
}

// newTestApp creates a minimal App suitable for teatest integration tests.
// Spins up a stub HTTP server so the client is non-nil and all Init commands
// resolve cleanly with empty data.
func newTestApp(t *testing.T) App {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(emptyAPIHandler))
	t.Cleanup(srv.Close)
	client := api.NewClient(srv.URL, "test-key")
	return NewApp(client, &config.Config{})
}

// --- Tests ---

// TestAppRendersOnStartup verifies the app renders with tabs and banner content.
func TestAppRendersOnStartup(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))
}

// TestAppRendersNebulaBanner verifies the ASCII banner appears on startup.
func TestAppRendersNebulaBanner(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// The banner subtitle is rendered below the ASCII art.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Context Infrastructure for Agents"))
	}, teatest.WithDuration(waitDur))
}

// TestTabNavigationByNumber verifies tab switching via number keys.
func TestTabNavigationByNumber(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Entities tab (key "2").
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))

	// Switch to Jobs tab (key "5").
	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Jobs"))
	}, teatest.WithDuration(waitDur))
}

// TestTabNavigationWithArrows verifies left/right arrow tab navigation.
func TestTabNavigationWithArrows(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render (starts on Inbox).
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Press right arrow to move from Inbox to Entities.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))
}

// TestCommandPaletteOpens verifies / opens the command palette.
func TestCommandPaletteOpens(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Open command palette with "/".
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Command"))
	}, teatest.WithDuration(waitDur))
}

// TestCommandPaletteCloses verifies the command palette closes on Esc and the
// app continues to render normally (no crash, program still alive).
func TestCommandPaletteCloses(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Open command palette.
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Command"))
	}, teatest.WithDuration(waitDur))

	// Close palette with Escape.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})

	// After closing the palette, switching tabs should still work.
	// This proves the palette closed and the app is functional.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))
}

// TestHelpOverlayOpens verifies ? opens the help overlay.
func TestHelpOverlayOpens(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Open help with "?".
	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("esc to close"))
	}, teatest.WithDuration(waitDur))
}

// TestHelpOverlayCloses verifies help can be closed and the app remains
// functional afterwards.
func TestHelpOverlayCloses(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Open and close help.
	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("esc to close"))
	}, teatest.WithDuration(waitDur))

	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})

	// After closing help, switch tabs to prove the app is still responsive.
	tm.Send(tea.KeyPressMsg{Code: '3', Text: "3"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Relationships"))
	}, teatest.WithDuration(waitDur))
}

// TestQuitWithNoUnsavedExitsImmediately verifies q quits when no unsaved changes.
func TestQuitWithNoUnsavedExitsImmediately(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Send "q" - with no unsaved changes, the app should quit immediately.
	tm.Send(tea.KeyPressMsg{Code: 'q', Text: "q"})

	tm.WaitFinished(t, teatest.WithFinalTimeout(waitDur))
}

// TestEmptyStateRendering verifies the Entities tab renders when no data is loaded.
func TestEmptyStateRendering(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Entities tab.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})

	// The entities tab should render. With empty data, "Entities" still appears in the tab bar.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))
}

// TestMultipleTabSwitches verifies rapid tab switching works correctly.
func TestMultipleTabSwitches(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Cycle through several tabs.
	tabs := []struct {
		key  rune
		name string
	}{
		{'3', "Relationships"},
		{'4', "Context"},
		{'6', "Logs"},
		{'1', "Inbox"},
	}

	for _, tc := range tabs {
		tm.Send(tea.KeyPressMsg{Code: tc.key, Text: string(tc.key)})
		name := tc.name
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte(name))
		}, teatest.WithDuration(waitDur))
	}
}

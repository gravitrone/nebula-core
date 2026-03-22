package ui

import (
	"bytes"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

// --- Inbox Tests ---

// TestInboxTabRendersOnStart verifies inbox renders as the default tab on startup.
func TestInboxTabRendersOnStart(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))
}

// TestInboxEmptyState verifies the empty state renders when no approvals exist.
// The inbox is the default tab, so we just wait for the empty state to appear.
func TestInboxEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for the empty inbox state - approvals load quickly from the stub server.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("No pending approvals"))
	}, teatest.WithDuration(waitDur))
}

// TestInboxTableNavigation verifies row navigation works when data is loaded.
func TestInboxTableNavigation(t *testing.T) {
	app := newTestAppWithData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for inbox to render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Navigate with j/down - should not crash.
	tm.Send(tea.KeyPressMsg{Code: 'j', Text: "j"})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	// App should still be rendering inbox.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))
}

// --- Relationships Tests ---

// TestRelationshipsTabShowsData verifies switching to the Relationships tab renders.
func TestRelationshipsTabShowsData(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Relationships tab (key "3" = tab index 2).
	tm.Send(tea.KeyPressMsg{Code: '3', Text: "3"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Relationships"))
	}, teatest.WithDuration(waitDur))
}

// TestRelationshipsEmptyState verifies empty state renders in the relationships tab.
func TestRelationshipsEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Relationships tab.
	tm.Send(tea.KeyPressMsg{Code: '3', Text: "3"})

	// With empty data the tab header still shows.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Relationships"))
	}, teatest.WithDuration(waitDur))
}

// TestRelationshipsTableNavigation verifies j/k navigation works in the relationships tab.
func TestRelationshipsTableNavigation(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Relationships tab.
	tm.Send(tea.KeyPressMsg{Code: '3', Text: "3"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Relationships"))
	}, teatest.WithDuration(waitDur))

	// Navigate with j/k (vi keys) - these are consumed by the tab content, not tab nav.
	tm.Send(tea.KeyPressMsg{Code: 'j', Text: "j"})
	tm.Send(tea.KeyPressMsg{Code: 'k', Text: "k"})

	// App should still be on Relationships tab.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Relationships"))
	}, teatest.WithDuration(waitDur))
}

// --- History Tests ---

// TestHistoryTabShowsData verifies switching to the History tab renders.
func TestHistoryTabShowsData(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to History tab (key "9" = tab index 8).
	tm.Send(tea.KeyPressMsg{Code: '9', Text: "9"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("History"))
	}, teatest.WithDuration(waitDur))
}

// TestHistoryEmptyState verifies the history tab renders without crashing when empty.
func TestHistoryEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to History tab.
	tm.Send(tea.KeyPressMsg{Code: '9', Text: "9"})

	// History tab header should appear.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("History"))
	}, teatest.WithDuration(waitDur))
}

// --- Cross-Cutting Tests ---

// TestScrollingWorks verifies PgDown/PgUp sends without crashing the app.
func TestScrollingWorks(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render then send scroll keys.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Send PgDown/PgUp - should not crash.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyPgDown})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyPgUp})

	// Navigate to a named tab to confirm the app is still alive and renders.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))
}

// TestImportExportOpens verifies the command palette opens and closes without crashing.
func TestImportExportOpens(t *testing.T) {
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

	// Close palette with ESC, then navigate to prove app is still live.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	tm.Send(tea.KeyPressMsg{Code: '3', Text: "3"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Relationships"))
	}, teatest.WithDuration(waitDur))
}

// TestWindowResizeHandling verifies the app handles WindowSizeMsg without crashing.
func TestWindowResizeHandling(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Send a window resize.
	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 30})

	// Navigate to confirm the app still responds after resize.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))
}

// TestRapidKeyInput verifies the app handles many keys quickly without crashing.
func TestRapidKeyInput(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Rapidly send tab switching keys.
	for _, key := range []rune{'2', '3', '4', '5', '6', '7', '8', '9', '0'} {
		tm.Send(tea.KeyPressMsg{Code: key, Text: string(key)})
	}

	// Send a known-good key and wait for it to be processed.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Entities"))
	}, teatest.WithDuration(waitDur))
}

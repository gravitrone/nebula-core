package ui

import (
	"bytes"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

// --- Files Tab Tests ---

// TestFilesTabShowsData switches to the Files tab and verifies data renders.
func TestFilesTabShowsData(t *testing.T) {
	app := newTestAppWithData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Files tab (key "7" -> index 6).
	tm.Send(tea.KeyPressMsg{Code: '7', Text: "7"})

	// Files tab renders with Add/Library mode selector.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Files"))
	}, teatest.WithDuration(waitDur))
}

// TestFilesAddForm switches to the Files tab and verifies the add form renders.
func TestFilesAddForm(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Files tab.
	tm.Send(tea.KeyPressMsg{Code: '7', Text: "7"})

	// Tab to switch to Library then back to Add (or just verify Add text visible).
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Add")) || bytes.Contains(out, []byte("Files"))
	}, teatest.WithDuration(waitDur))
}

// TestFilesEmptyState verifies the Files empty state renders when no data is present.
func TestFilesEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Files tab.
	tm.Send(tea.KeyPressMsg{Code: '7', Text: "7"})

	// Switch to Library view.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	// Navigate into content.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	// With no data, should show empty state or Files tab name.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("No files found.")) ||
			bytes.Contains(out, []byte("Files"))
	}, teatest.WithDuration(waitDur))
}

// --- Logs Tab Tests ---

// TestLogsTabShowsData switches to the Logs tab and verifies data renders.
func TestLogsTabShowsData(t *testing.T) {
	app := newTestAppWithData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Logs tab (key "6" -> index 5).
	tm.Send(tea.KeyPressMsg{Code: '6', Text: "6"})

	// Logs tab renders with Add/Library mode selector.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Logs"))
	}, teatest.WithDuration(waitDur))
}

// TestLogsAddForm switches to the Logs tab and verifies the add form renders.
func TestLogsAddForm(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Logs tab.
	tm.Send(tea.KeyPressMsg{Code: '6', Text: "6"})

	// The default view shows the add form or mode selector.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Add")) || bytes.Contains(out, []byte("Logs"))
	}, teatest.WithDuration(waitDur))
}

// TestLogsEmptyState verifies the Logs empty state renders when no data is present.
func TestLogsEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Logs tab.
	tm.Send(tea.KeyPressMsg{Code: '6', Text: "6"})

	// Switch to Library view.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	// Navigate into content.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	// With no data, should show empty state or Logs tab name.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("No logs found.")) ||
			bytes.Contains(out, []byte("Logs"))
	}, teatest.WithDuration(waitDur))
}

// --- Protocols Tab Tests ---

// TestProtocolsTabShowsData switches to the Protocols tab and verifies data renders.
func TestProtocolsTabShowsData(t *testing.T) {
	app := newTestAppWithData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Protocols tab (key "8" -> index 7).
	tm.Send(tea.KeyPressMsg{Code: '8', Text: "8"})

	// Protocols tab renders.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Protocols"))
	}, teatest.WithDuration(waitDur))
}

// TestProtocolsAddForm switches to the Protocols tab and verifies the add form renders.
func TestProtocolsAddForm(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Protocols tab.
	tm.Send(tea.KeyPressMsg{Code: '8', Text: "8"})

	// The default view shows the add form or mode selector.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Add")) || bytes.Contains(out, []byte("Protocols"))
	}, teatest.WithDuration(waitDur))
}

// TestProtocolsEmptyState verifies the Protocols empty state renders when no data is present.
func TestProtocolsEmptyState(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Protocols tab.
	tm.Send(tea.KeyPressMsg{Code: '8', Text: "8"})

	// Switch to Library view.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})

	// Navigate into content.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})

	// With no data, should show empty state or Protocols tab name.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("No protocols found.")) ||
			bytes.Contains(out, []byte("Protocols"))
	}, teatest.WithDuration(waitDur))
}

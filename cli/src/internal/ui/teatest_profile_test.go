package ui

import (
	"bytes"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

// --- Profile / Settings Tests ---

// TestProfileTabRendersSettings verifies the Settings tab renders on key "0".
func TestProfileTabRendersSettings(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Profile/Settings tab (key "0" = tab index 9).
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Settings"))
	}, teatest.WithDuration(waitDur))
}

// TestProfileShowsAPIKeySection verifies the API Keys section label renders.
func TestProfileShowsAPIKeySection(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Settings tab.
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})

	// API Keys section should appear in the profile view.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("API Keys"))
	}, teatest.WithDuration(waitDur))
}

// TestProfileShowsTaxonomy verifies the Taxonomy section label renders in Settings.
func TestProfileShowsTaxonomy(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	// Wait for initial render.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Inbox"))
	}, teatest.WithDuration(waitDur))

	// Switch to Settings tab.
	tm.Send(tea.KeyPressMsg{Code: '0', Text: "0"})

	// Taxonomy section should appear in the profile view.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return bytes.Contains(out, []byte("Taxonomy"))
	}, teatest.WithDuration(waitDur))
}

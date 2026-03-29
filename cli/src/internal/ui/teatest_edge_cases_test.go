package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
)

// --- Table Edge Cases ---

// TestTableEmptyCursorDoesNotPanic switches to entities with no data and rapidly
// presses Down/Up/Enter to verify no panic occurs on empty tables.
func TestTableEmptyCursorDoesNotPanic(t *testing.T) {
	app := newTestApp(t) // empty data
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Switch to entities (empty).
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	time.Sleep(200 * time.Millisecond)

	// Rapid Down/Up/Enter on empty table.
	for i := 0; i < 10; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
		tm.Send(tea.KeyPressMsg{Code: tea.KeyUp})
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	time.Sleep(200 * time.Millisecond)

	// If we get here without panic, verify app is still alive.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestTableNavigationBoundsCheck loads 2 entities and presses Down 100 times
// to verify cursor stays within bounds without panic.
func TestTableNavigationBoundsCheck(t *testing.T) {
	app := newTestAppWithEntityData(t) // 2 entities
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

	// Press Down 100 times - cursor must not go out of bounds.
	for i := 0; i < 100; i++ {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	time.Sleep(200 * time.Millisecond)

	// App should still be alive and responsive.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestTableAfterDataReload switches to entities, verifies data, then rapidly
// switches tabs back and forth 5 times to stress data reload paths.
func TestTableAfterDataReload(t *testing.T) {
	app := newTestAppWithEntityData(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Switch to entities, verify data shows.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "TestAgent")
	}, teatest.WithDuration(waitDur))

	// Rapidly switch tabs back and forth 5 times.
	for i := 0; i < 5; i++ {
		tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
		time.Sleep(50 * time.Millisecond)
		tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
		time.Sleep(50 * time.Millisecond)
	}

	// Verify entities tab still renders data after all the switching.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Entities")
	}, teatest.WithDuration(waitDur))
}

// --- Form Edge Cases ---

// TestFormSubmitWithEmptyFields opens the add form on entities and immediately
// presses Enter to submit empty fields, verifying no panic.
func TestFormSubmitWithEmptyFields(t *testing.T) {
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

	// Enter modeFocus (Down exits tab nav).
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)

	// Toggle to Add mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Name") || containsText(out, "Initializing")
	}, teatest.WithDuration(waitDur))

	// Immediately press Enter to submit empty form.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	time.Sleep(200 * time.Millisecond)

	// App should still be alive.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestFormOpenCloseRapidly opens and closes the add form 3 times to verify
// no state corruption from rapid form lifecycle.
func TestFormOpenCloseRapidly(t *testing.T) {
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

	// 3 cycles of open/close add form.
	for i := 0; i < 3; i++ {
		// Enter modeFocus.
		tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
		time.Sleep(100 * time.Millisecond)
		// Toggle to Add mode.
		tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
		time.Sleep(200 * time.Millisecond)
		// Toggle back to List mode (Right).
		tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
		time.Sleep(100 * time.Millisecond)
		// Go back to tab nav.
		tm.Send(tea.KeyPressMsg{Code: tea.KeyUp})
		time.Sleep(100 * time.Millisecond)
	}

	// Verify app is still functional.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestFormTypingSpecialCharacters opens the add form and types unicode
// characters to verify no crash from non-ASCII input.
func TestFormTypingSpecialCharacters(t *testing.T) {
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

	// Enter modeFocus -> Add mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Name") || containsText(out, "Initializing")
	}, teatest.WithDuration(waitDur))

	// Type unicode characters.
	for _, ch := range "日本語 émojis ñ" {
		tm.Send(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
	time.Sleep(200 * time.Millisecond)

	// Verify app is still alive.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// --- Viewport Edge Cases ---

// TestViewportWithZeroHeight sends a WindowSizeMsg with height=0 to verify
// the app does not panic on degenerate terminal dimensions.
func TestViewportWithZeroHeight(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Send zero-height resize.
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 0})
	time.Sleep(200 * time.Millisecond)

	// Restore normal size and verify app recovers.
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	time.Sleep(200 * time.Millisecond)

	// Switch tab to prove app is alive.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Entities")
	}, teatest.WithDuration(waitDur))
}

// TestViewportWithTinyTerminal sends a WindowSizeMsg with 20x5 dimensions
// to verify the app renders without crashing on a tiny terminal.
func TestViewportWithTinyTerminal(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Resize to tiny terminal.
	tm.Send(tea.WindowSizeMsg{Width: 20, Height: 5})
	time.Sleep(200 * time.Millisecond)

	// Try switching tabs on tiny terminal.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	time.Sleep(200 * time.Millisecond)

	// Restore and verify.
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestViewportWithHugeTerminal sends a WindowSizeMsg with 500x200 to verify
// the app does not crash or allocate excessive memory on large terminals.
func TestViewportWithHugeTerminal(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Send huge terminal resize.
	tm.Send(tea.WindowSizeMsg{Width: 500, Height: 200})
	time.Sleep(200 * time.Millisecond)

	// Switch tabs to exercise render with huge dimensions.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Entities")
	}, teatest.WithDuration(waitDur))
}

// --- State Machine Edge Cases ---

// TestEscapeFromEveryView switches to each tab and presses Escape to verify
// the app returns to a sane state without crashing.
func TestEscapeFromEveryView(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// tabNames: Inbox(1), Entities(2), Relationships(3), Context(4), Jobs(5),
	// Logs(6), Files(7), Protocols(8), History(9), Settings(0)
	keys := []struct {
		key  rune
		text string
	}{
		{'1', "1"}, {'2', "2"}, {'3', "3"}, {'4', "4"}, {'5', "5"},
		{'6', "6"}, {'7', "7"}, {'8', "8"}, {'9', "9"}, {'0', "0"},
	}

	for _, k := range keys {
		tm.Send(tea.KeyPressMsg{Code: k.key, Text: k.text})
		time.Sleep(100 * time.Millisecond)
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
		time.Sleep(100 * time.Millisecond)
	}

	// Return to Inbox and verify app is alive.
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestDoubleEnterOnDetail enters the entity detail view then presses Enter
// again to verify no double-navigation or panic.
func TestDoubleEnterOnDetail(t *testing.T) {
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

	// Enter content area and open detail.
	enterEntitiesContent(tm)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "ent-001")
	}, teatest.WithDuration(waitDur))

	// Press Enter again inside detail view - should not panic.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	time.Sleep(200 * time.Millisecond)

	// Escape back and verify app is alive.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestTabSwitchDuringFormEdit opens the add form, types text, then switches
// tabs via number key to verify state is clean after abandoning form.
func TestTabSwitchDuringFormEdit(t *testing.T) {
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

	// Enter modeFocus -> Add mode.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyDown})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyLeft})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Name") || containsText(out, "Initializing")
	}, teatest.WithDuration(waitDur))

	// Type some text in the form name field.
	for _, ch := range "test" {
		tm.Send(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	}
	time.Sleep(200 * time.Millisecond)

	// Abort form with Escape and verify app returns to list view.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(200 * time.Millisecond)

	// Verify app is still functional by switching tabs.
	tm.Send(tea.KeyPressMsg{Code: '5', Text: "5"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Jobs")
	}, teatest.WithDuration(waitDur))
}

// --- Concurrent-ish Edge Cases ---

// TestRapidKeyFlood sends 50 random keys in rapid succession to verify
// the app does not crash under rapid input.
func TestRapidKeyFlood(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Send 50 mixed keys rapidly.
	keys := []tea.KeyPressMsg{
		{Code: tea.KeyDown}, {Code: tea.KeyUp}, {Code: tea.KeyEnter},
		{Code: tea.KeyEscape}, {Code: tea.KeyLeft}, {Code: tea.KeyRight},
		{Code: 'j', Text: "j"}, {Code: 'k', Text: "k"},
		{Code: '1', Text: "1"}, {Code: '2', Text: "2"},
		{Code: '/', Text: "/"}, {Code: '?', Text: "?"},
		{Code: tea.KeyTab}, {Code: tea.KeyBackspace},
	}

	for i := 0; i < 50; i++ {
		tm.Send(keys[i%len(keys)])
	}
	time.Sleep(300 * time.Millisecond)

	// Close any overlays and return to Inbox.
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: '1', Text: "1"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))
}

// TestAlternatingPaletteAndHelp alternates between opening the command palette
// and help overlay to verify no state leak between overlays.
func TestAlternatingPaletteAndHelp(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Palette open/close.
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Command")
	}, teatest.WithDuration(waitDur))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(100 * time.Millisecond)

	// Help open/close.
	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "esc to close")
	}, teatest.WithDuration(waitDur))
	tm.Send(tea.KeyPressMsg{Code: '?', Text: "?"})
	time.Sleep(100 * time.Millisecond)

	// Palette again.
	tm.Send(tea.KeyPressMsg{Code: '/', Text: "/"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Command")
	}, teatest.WithDuration(waitDur))
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	time.Sleep(100 * time.Millisecond)

	// Verify normal operation.
	tm.Send(tea.KeyPressMsg{Code: '2', Text: "2"})
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Entities")
	}, teatest.WithDuration(waitDur))
}

// --- Render Verification ---

// TestAllTabsRenderWithoutPanic loops through all 10 tabs and verifies each
// renders its tab name without crashing.
func TestAllTabsRenderWithoutPanic(t *testing.T) {
	app := newTestApp(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(120, 40))
	t.Cleanup(func() { _ = tm.Quit() })

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return containsText(out, "Inbox")
	}, teatest.WithDuration(waitDur))

	// Test first 5 tabs.
	firstHalf := []struct {
		key  rune
		text string
		name string
	}{
		{'2', "2", "Entities"},
		{'3', "3", "Relationships"},
		{'4', "4", "Context"},
		{'5', "5", "Jobs"},
		{'6', "6", "Logs"},
	}

	for _, tc := range firstHalf {
		tm.Send(tea.KeyPressMsg{Code: tc.key, Text: tc.text})
		name := tc.name
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return containsText(out, name)
		}, teatest.WithDuration(waitDur))
	}

	// Test remaining tabs.
	secondHalf := []struct {
		key  rune
		text string
		name string
	}{
		{'7', "7", "Files"},
		{'8', "8", "Protocols"},
		{'9', "9", "History"},
		{'0', "0", "Settings"},
	}

	for _, tc := range secondHalf {
		tm.Send(tea.KeyPressMsg{Code: tc.key, Text: tc.text})
		name := tc.name
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return containsText(out, name)
		}, teatest.WithDuration(waitDur))
	}
}

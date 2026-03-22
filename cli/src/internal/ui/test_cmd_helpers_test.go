package ui

import (
	"reflect"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// runCmd executes a tea.Cmd and recursively expands any tea.BatchMsg,
// returning all non-nil leaf messages.
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	switch m := msg.(type) {
	case tea.BatchMsg:
		var msgs []tea.Msg
		for _, c := range m {
			msgs = append(msgs, runCmd(c)...)
		}
		return msgs
	default:
		return []tea.Msg{msg}
	}
}

// isFrameworkMsg returns true for messages produced by charm framework
// internals (spinner ticks, cursor blinks, etc.) that tests should skip.
func isFrameworkMsg(msg tea.Msg) bool {
	if _, ok := msg.(spinner.TickMsg); ok {
		return true
	}
	// Cursor blink messages are unexported types from charm.land/bubbles/v2/cursor.
	// Detect them by package path to avoid importing the package.
	pkg := reflect.TypeOf(msg).PkgPath()
	return strings.Contains(pkg, "charm.land/bubbles") || strings.Contains(pkg, "charm.land/bubbletea")
}

// runCmdFirst executes a tea.Cmd (recursively expanding BatchMsg) and returns
// the first message that is not a framework internal (spinner tick, cursor
// blink, etc.). Falls back to the first message if all are framework internals.
func runCmdFirst(cmd tea.Cmd) tea.Msg {
	msgs := runCmd(cmd)
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if isFrameworkMsg(msg) {
			continue
		}
		return msg
	}
	if len(msgs) > 0 {
		return msgs[0]
	}
	return nil
}

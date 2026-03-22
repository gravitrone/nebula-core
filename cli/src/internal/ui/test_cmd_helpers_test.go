package ui

import (
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

// runCmdFirst executes a tea.Cmd (recursively expanding BatchMsg) and returns
// the first message that is not a spinner.TickMsg. Falls back to the first
// message if all are spinner ticks.
func runCmdFirst(cmd tea.Cmd) tea.Msg {
	msgs := runCmd(cmd)
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		if _, ok := msg.(spinner.TickMsg); ok {
			continue
		}
		return msg
	}
	if len(msgs) > 0 {
		return msgs[0]
	}
	return nil
}

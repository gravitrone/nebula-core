package ui

import "github.com/charmbracelet/bubbletea"

// --- Key Constants ---

func isKey(msg tea.KeyMsg, keys ...string) bool {
	for _, k := range keys {
		if msg.String() == k {
			return true
		}
	}
	return false
}

// isQuit handles is quit.
func isQuit(msg tea.KeyMsg) bool {
	return isKey(msg, "q", "ctrl+c")
}

// isBack handles is back.
func isBack(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyEsc {
		return true
	}
	return isKey(msg, "esc", "escape", "ctrl+[")
}

// isUp handles is up.
func isUp(msg tea.KeyMsg) bool {
	return isKey(msg, "up")
}

// isDown handles is down.
func isDown(msg tea.KeyMsg) bool {
	return isKey(msg, "down")
}

// isEnter handles is enter.
func isEnter(msg tea.KeyMsg) bool {
	return isKey(msg, "enter", "return")
}

// isSpace handles is space.
func isSpace(msg tea.KeyMsg) bool {
	return isKey(msg, " ")
}

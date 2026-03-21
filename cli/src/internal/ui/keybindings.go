package ui

import tea "charm.land/bubbletea/v2"

// --- Key Constants ---

func isKey(msg tea.KeyPressMsg, keys ...string) bool {
	for _, k := range keys {
		if msg.String() == k {
			return true
		}
	}
	return false
}

// isQuit handles is quit.
func isQuit(msg tea.KeyPressMsg) bool {
	return isKey(msg, "q", "ctrl+c")
}

// isBack handles is back.
func isBack(msg tea.KeyPressMsg) bool {
	return isKey(msg, "esc", "escape", "ctrl+[")
}

// isUp handles is up.
func isUp(msg tea.KeyPressMsg) bool {
	return isKey(msg, "up")
}

// isDown handles is down.
func isDown(msg tea.KeyPressMsg) bool {
	return isKey(msg, "down")
}

// isEnter handles is enter.
func isEnter(msg tea.KeyPressMsg) bool {
	return isKey(msg, "enter", "return")
}

// isSpace handles is space.
func isSpace(msg tea.KeyPressMsg) bool {
	return isKey(msg, " ", "space")
}

// keyText returns the printable text for a key press.
// In bubbletea v2, msg.Text is empty for space; this helper normalizes that.
func keyText(msg tea.KeyPressMsg) string {
	if msg.Code == tea.KeySpace {
		return " "
	}
	return msg.Text
}

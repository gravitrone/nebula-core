package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

// TestIsQuit handles test is quit.
func TestIsQuit(t *testing.T) {
	assert.True(t, isQuit(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}))
	assert.True(t, isQuit(tea.KeyPressMsg{Code: 'q', Text: "q"}))
	assert.False(t, isQuit(tea.KeyPressMsg{Code: 'a', Text: "a"}))
}

// TestIsEnter handles test is enter.
func TestIsEnter(t *testing.T) {
	assert.True(t, isEnter(tea.KeyPressMsg{Code: tea.KeyEnter}))
	assert.False(t, isEnter(tea.KeyPressMsg{Code: tea.KeySpace}))
}

// TestIsSpace handles test is space.
func TestIsSpace(t *testing.T) {
	assert.True(t, isSpace(tea.KeyPressMsg{Code: tea.KeySpace}))
	assert.False(t, isSpace(tea.KeyPressMsg{Code: tea.KeyEnter}))
}

// TestIsBack handles test is back.
func TestIsBack(t *testing.T) {
	assert.True(t, isBack(tea.KeyPressMsg{Code: tea.KeyEscape}))
	assert.False(t, isBack(tea.KeyPressMsg{Code: tea.KeyEnter}))
}

// TestIsDown handles test is down.
func TestIsDown(t *testing.T) {
	assert.True(t, isDown(tea.KeyPressMsg{Code: tea.KeyDown}))
	assert.False(t, isDown(tea.KeyPressMsg{Code: tea.KeyUp}))
	assert.False(t, isDown(tea.KeyPressMsg{Code: 'j', Text: "j"}))
}

// TestIsUp handles test is up.
func TestIsUp(t *testing.T) {
	assert.True(t, isUp(tea.KeyPressMsg{Code: tea.KeyUp}))
	assert.False(t, isUp(tea.KeyPressMsg{Code: tea.KeyDown}))
	assert.False(t, isUp(tea.KeyPressMsg{Code: 'k', Text: "k"}))
}

// TestIsKey handles test is key.
func TestIsKey(t *testing.T) {
	assert.True(t, isKey(tea.KeyPressMsg{Code: 's', Text: "s"}, "s"))
	assert.True(t, isKey(tea.KeyPressMsg{Code: 'a', Text: "a"}, "a"))
	assert.True(t, isKey(tea.KeyPressMsg{Code: tea.KeyBackspace}, "backspace"))
	assert.True(t, isKey(tea.KeyPressMsg{Code: tea.KeyLeft}, "left"))
	assert.True(t, isKey(tea.KeyPressMsg{Code: tea.KeyRight}, "right"))
	assert.False(t, isKey(tea.KeyPressMsg{Code: 's', Text: "s"}, "a"))
	assert.False(t, isKey(tea.KeyPressMsg{Code: tea.KeyLeft}, "right"))
}

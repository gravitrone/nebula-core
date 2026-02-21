package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// TestIsQuit handles test is quit.
func TestIsQuit(t *testing.T) {
	assert.True(t, isQuit(tea.KeyMsg{Type: tea.KeyCtrlC}))
	assert.True(t, isQuit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}))
	assert.False(t, isQuit(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}))
}

// TestIsEnter handles test is enter.
func TestIsEnter(t *testing.T) {
	assert.True(t, isEnter(tea.KeyMsg{Type: tea.KeyEnter}))
	assert.False(t, isEnter(tea.KeyMsg{Type: tea.KeySpace}))
}

// TestIsSpace handles test is space.
func TestIsSpace(t *testing.T) {
	assert.True(t, isSpace(tea.KeyMsg{Type: tea.KeySpace}))
	assert.False(t, isSpace(tea.KeyMsg{Type: tea.KeyEnter}))
}

// TestIsBack handles test is back.
func TestIsBack(t *testing.T) {
	assert.True(t, isBack(tea.KeyMsg{Type: tea.KeyEsc}))
	assert.False(t, isBack(tea.KeyMsg{Type: tea.KeyEnter}))
}

// TestIsDown handles test is down.
func TestIsDown(t *testing.T) {
	assert.True(t, isDown(tea.KeyMsg{Type: tea.KeyDown}))
	assert.False(t, isDown(tea.KeyMsg{Type: tea.KeyUp}))
	assert.False(t, isDown(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}))
}

// TestIsUp handles test is up.
func TestIsUp(t *testing.T) {
	assert.True(t, isUp(tea.KeyMsg{Type: tea.KeyUp}))
	assert.False(t, isUp(tea.KeyMsg{Type: tea.KeyDown}))
	assert.False(t, isUp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}))
}

// TestIsKey handles test is key.
func TestIsKey(t *testing.T) {
	assert.True(t, isKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}, "s"))
	assert.True(t, isKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, "a"))
	assert.True(t, isKey(tea.KeyMsg{Type: tea.KeyBackspace}, "backspace"))
	assert.True(t, isKey(tea.KeyMsg{Type: tea.KeyLeft}, "left"))
	assert.True(t, isKey(tea.KeyMsg{Type: tea.KeyRight}, "right"))
	assert.False(t, isKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}, "a"))
	assert.False(t, isKey(tea.KeyMsg{Type: tea.KeyLeft}, "right"))
}

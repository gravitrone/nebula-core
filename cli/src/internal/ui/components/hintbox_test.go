package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewHintBoxReturnsEmptyViewWhenNoHints(t *testing.T) {
	h := NewHintBox(nil)
	assert.Equal(t, "", h.View())

	h2 := NewHintBox([]string{})
	assert.Equal(t, "", h2.View())
}

func TestNewHintBoxRendersHints(t *testing.T) {
	h := NewHintBox([]string{"esc back", "enter confirm"})
	out := h.View()
	assert.Contains(t, out, "esc")
	assert.Contains(t, out, "back")
	assert.Contains(t, out, "enter")
	assert.Contains(t, out, "confirm")
	// Separator dot should appear between hints.
	assert.Contains(t, out, "\u00b7")
}

func TestNewHintBoxHandlesSingleWordHints(t *testing.T) {
	h := NewHintBox([]string{"quit"})
	out := h.View()
	assert.Contains(t, out, "quit")
}

func TestHintBoxSetWidthDoesNotPanic(t *testing.T) {
	h := NewHintBox([]string{"esc back"})
	h.SetWidth(40)
	out := h.View()
	assert.NotEmpty(t, out)
}

func TestHintBoxSkipsEmptyHints(t *testing.T) {
	h := NewHintBox([]string{"", "  ", "esc back"})
	out := h.View()
	assert.Contains(t, out, "esc")
	// Should not have a separator before esc since empty hints are skipped.
	assert.Equal(t, 1, strings.Count(out, "esc"))
}

func TestSplitHint(t *testing.T) {
	key, action := splitHint("esc back")
	assert.Equal(t, "esc", key)
	assert.Equal(t, "back", action)

	key2, action2 := splitHint("quit")
	assert.Equal(t, "quit", key2)
	assert.Equal(t, "", action2)

	key3, action3 := splitHint("ctrl+c force quit")
	assert.Equal(t, "ctrl+c", key3)
	assert.Equal(t, "force quit", action3)
}

package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSanitizeOneLineStripsOscAndNewlines handles test sanitize one line strips osc and newlines.
func TestSanitizeOneLineStripsOscAndNewlines(t *testing.T) {
	input := "\x1b]8;;https://evil\x07click\x1b]8;;\x07\nline\tmore"
	out := SanitizeOneLine(input)

	assert.False(t, strings.Contains(out, "\x1b"))
	assert.False(t, strings.Contains(out, "\n"))
	assert.False(t, strings.Contains(out, "\t"))
}

// TestSanitizeTextRemovesBidiControls handles test sanitize text removes bidi controls.
func TestSanitizeTextRemovesBidiControls(t *testing.T) {
	input := "safe\u202eexe.txt"
	out := SanitizeText(input)

	assert.NotContains(t, out, "\u202e")
}

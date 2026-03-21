package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderMarkdown_ValidMarkdown(t *testing.T) {
	input := "# Hello\n\nThis is **bold** text."
	out := RenderMarkdown(input, 80)
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "Hello")
	assert.Contains(t, out, "bold")
	// Glamour adds ANSI styling, so output should differ from raw input.
	assert.NotEqual(t, input, strings.TrimSpace(out))
}

func TestRenderMarkdown_EmptyInput(t *testing.T) {
	out := RenderMarkdown("", 80)
	assert.Equal(t, "", out)
}

func TestRenderMarkdown_PlainText(t *testing.T) {
	input := "just some plain text with no markdown"
	out := RenderMarkdown(input, 80)
	assert.NotEmpty(t, out)
	assert.Contains(t, out, "just some plain text")
}

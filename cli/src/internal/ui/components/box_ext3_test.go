package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestRenderBoxHandlesBorderWiderThanTargetWidth(t *testing.T) {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	out := renderBox(style, 1, "x")

	assert.NotEmpty(t, out)
	lines := strings.Split(out, "\n")
	assert.GreaterOrEqual(t, len(lines), 1)
}

func TestTitledBoxWithStyleTruncatesLongHeaderToFit(t *testing.T) {
	out := titledBoxWithStyle(
		strings.Repeat("header-", 8),
		"body",
		22,
		boxBorder,
		lipgloss.NewStyle().Bold(true),
		lipgloss.Color("#273540"),
	)

	clean := SanitizeText(out)
	assert.Contains(t, clean, "body")
	assert.NotContains(t, clean, "[")

	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 22)
	}
}

func TestRenderMetadataLinesNestedAndInlineScopeBranches(t *testing.T) {
	lines := renderMetadataLines(
		map[string]any{
			"nested": map[string]any{
				"k": "v",
			},
			"segments": []any{
				map[string]any{
					"text":   "hello",
					"scopes": []any{"public", "admin"},
				},
			},
		},
		0,
	)

	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "nested:")
	assert.Contains(t, joined, "k: v")
	assert.Contains(t, joined, "- [public, admin] hello")
}

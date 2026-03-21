package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestTitledBoxWithStyleHeaderWidthBranchMatrix(t *testing.T) {
	// Header style with fixed tiny width can make rendered header narrower than
	// titleText width, exercising the right-pad branch in titledBoxWithStyle.
	narrowHeader := lipgloss.NewStyle().Width(1)
	out := titledBoxWithStyle(
		"title",
		"body",
		60,
		boxBorder,
		narrowHeader,
		lipgloss.Color("#273540"),
	)
	lines := strings.Split(out, "\n")
	assert.NotEmpty(t, lines)
	assert.Greater(t, lipgloss.Width(lines[0]), 0)
	assert.LessOrEqual(t, lipgloss.Width(lines[0]), 54)

	// Header style with horizontal padding makes the rendered header wider than
	// titleText width, exercising the truncate branch.
	wideHeader := lipgloss.NewStyle().Padding(0, 4)
	out = titledBoxWithStyle(
		"title",
		"body",
		60,
		boxBorder,
		wideHeader,
		lipgloss.Color("#273540"),
	)
	lines = strings.Split(out, "\n")
	assert.NotEmpty(t, lines)
	assert.Greater(t, lipgloss.Width(lines[0]), 0)
	assert.LessOrEqual(t, lipgloss.Width(lines[0]), 54)
	assert.Contains(t, SanitizeText(out), "body")
}

func TestMetadataTableHandlesNilScalarAndNestedEmptyMap(t *testing.T) {
	out := MetadataTable(map[string]any{
		"owner":  nil,
		"nested": map[string]any{},
	}, 70)

	clean := SanitizeText(out)
	assert.Contains(t, clean, "owner: None")
	assert.Contains(t, clean, "nested: {}")
}

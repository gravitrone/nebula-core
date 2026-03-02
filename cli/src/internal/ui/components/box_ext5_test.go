package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableGuardAndWidthFallbackBranches(t *testing.T) {
	assert.Equal(t, "", Table("Any", nil, 80))

	out := Table("Tiny", []TableRow{{Label: "a", Value: "value"}}, 0)
	clean := SanitizeText(out)
	assert.Contains(t, clean, "Tiny")
	assert.Contains(t, clean, "a")
	assert.Contains(t, clean, "value")
}

func TestWrapTableWordsAndCenterLineAdditionalBranches(t *testing.T) {
	assert.Equal(t, []string{""}, wrapTableWords("   ", 10))
	assert.Equal(t, []string{"alpha"}, wrapTableWords("alpha", 0))
	assert.Equal(t, []string{"alpha"}, wrapTableWords("alpha", 10))

	assert.Equal(t, "abc", CenterLine("abc", 4))
}

func TestDiffTableAdditionalNormalizationAndSeparatorBranches(t *testing.T) {
	out := DiffTable(
		"Diff",
		[]DiffRow{
			{Label: "owner", From: " ", To: "--"},
			{Label: "status", From: "pending", To: "approved"},
		},
		12,
	)

	clean := SanitizeText(out)
	assert.Contains(t, clean, "Diff")
	assert.Contains(t, clean, "owner")
	assert.Contains(t, clean, "None")
	assert.Contains(t, clean, "status")
	assert.Contains(t, clean, "approv")
}

func TestRenderMetadataValueLinesTextWithoutScopesAndMapFormatting(t *testing.T) {
	lines := renderMetadataValueLines(
		[]any{
			map[string]any{
				"text": "hello",
			},
		},
		2,
	)
	assert.Equal(t, []string{"  - hello"}, lines)

	formatted := formatMetadataValue(map[string]any{"k": "v"})
	assert.True(t, strings.Contains(formatted, `"k":"v"`))
}

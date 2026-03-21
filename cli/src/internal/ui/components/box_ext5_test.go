package components

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTableGuardAndWidthFallbackBranches(t *testing.T) {
	assert.Equal(t, "", Table("Any", nil, 80))

	out := Table("Tiny", []TableRow{{Label: "a", Value: "value"}}, 0)
	clean := SanitizeText(out)
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
	// At very tight widths, lipgloss v2 wraps words across lines.
	assert.Contains(t, clean, "owne")
	assert.Contains(t, clean, "None")
	assert.Contains(t, clean, "stat")
	assert.Contains(t, clean, "pend")
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

func TestParseMetadataScopesInlineSkipsNonStringEntries(t *testing.T) {
	out := parseMetadataScopesInline([]any{" public ", nil, 42, "admin", "   "})
	assert.Equal(t, "public, admin", out)
}

func TestRenderMetadataValueLinesScopeBadgeSkipsInvalidScopeEntries(t *testing.T) {
	lines := renderMetadataValueLines(
		[]any{
			map[string]any{
				"text":   "hello",
				"scopes": []any{"public", nil, 99, " admin "},
			},
		},
		2,
	)

	assert.Equal(t, []string{"  - [public, admin] hello"}, lines)
}

func TestDiffSectionForLabelMatrix(t *testing.T) {
	assert.Equal(t, "Content", diffSectionForLabel("Content"))
	assert.Equal(t, "Scopes", diffSectionForLabel("privacy_scope_ids"))
	assert.Equal(t, "Tags", diffSectionForLabel("tags"))
	assert.Equal(t, "Source", diffSectionForLabel("Source Type"))
	assert.Equal(t, "Core", diffSectionForLabel("title"))
	assert.Equal(t, "Metadata", diffSectionForLabel("metadata.preview"))
	assert.Equal(t, "Other", diffSectionForLabel("custom_field"))
}

func TestWrapDiffCellValueCollapsesLongOutputByDefault(t *testing.T) {
	t.Setenv("NEBULA_DIFF_FULL", "")
	lines := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	out := wrapDiffCellValue(strings.Join(lines, "\n"), 40)
	assert.Len(t, out, 7)
	assert.Contains(t, out[6], "+2 more lines")
}

func TestWrapDiffCellValueFullModeKeepsAllLines(t *testing.T) {
	t.Setenv("NEBULA_DIFF_FULL", "1")
	lines := []string{"1", "2", "3", "4", "5", "6", "7", "8"}
	out := wrapDiffCellValue(strings.Join(lines, "\n"), 40)
	assert.Len(t, out, 8)
}

func TestDiffFullModeEnabledMatrix(t *testing.T) {
	prev := os.Getenv("NEBULA_DIFF_FULL")
	t.Cleanup(func() {
		_ = os.Setenv("NEBULA_DIFF_FULL", prev)
	})
	_ = os.Setenv("NEBULA_DIFF_FULL", "true")
	assert.True(t, diffFullModeEnabled())
	_ = os.Setenv("NEBULA_DIFF_FULL", "yes")
	assert.True(t, diffFullModeEnabled())
	_ = os.Setenv("NEBULA_DIFF_FULL", "0")
	assert.False(t, diffFullModeEnabled())
}

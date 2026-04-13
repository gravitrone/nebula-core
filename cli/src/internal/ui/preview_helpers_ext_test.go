package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreferredPreviewWidthAndContentWidthBounds(t *testing.T) {
	assert.Equal(t, previewMinWidth, preferredPreviewWidth(0))
	assert.Equal(t, previewMinWidth, preferredPreviewWidth(40))
	assert.Equal(t, previewMaxWidth, preferredPreviewWidth(400))

	assert.Equal(t, 0, previewBoxContentWidth(0))
	assert.Equal(t, 10, previewBoxContentWidth(1))
	assert.GreaterOrEqual(t, previewBoxContentWidth(80), 10)
}

func TestRenderPreviewBoxWrapAndPadHelpers(t *testing.T) {
	assert.Equal(t, "", renderPreviewBox("hello", 0))
	assert.NotEmpty(t, renderPreviewBox("x", 1))

	boxed := renderPreviewBox("hello", 24)
	assert.NotEmpty(t, boxed)
	assert.Contains(t, components.SanitizeText(boxed), "hello")

	assert.Nil(t, wrapPreviewText("", 20))
	assert.Nil(t, wrapPreviewText("hello", 0))
	assert.Equal(t, []string{"hello"}, wrapPreviewText("hello", 20))
	wrapped := wrapPreviewText("alpha beta gamma", 6)
	assert.GreaterOrEqual(t, len(wrapped), 2)

	padded := padPreviewLines([]string{"ab", "cdef"}, 4)
	lines := strings.Split(padded, "\n")
	assert.Len(t, lines, 2)
	assert.Equal(t, 4, len(lines[0]))
	assert.Equal(t, 4, len(lines[1]))

	assert.Equal(t, "", padPreviewLines(nil, 4))
	assert.Equal(t, "", padPreviewLines([]string{"abc"}, 0))

	clamped := padPreviewLines([]string{"\x1b[31mabcdef\x1b[0m"}, 4)
	clampedLines := strings.Split(clamped, "\n")
	require.Len(t, clampedLines, 1)
	assert.Equal(t, 4, len(clampedLines[0]))
}

func TestRenderPreviewRowScopeVariants(t *testing.T) {
	row := renderPreviewRow("Scopes", "public, admin", 80)
	clean := components.SanitizeText(row)
	assert.Contains(t, clean, "Scopes:")
	assert.Contains(t, clean, "public")
	assert.Contains(t, clean, "admin")

	row = renderPreviewRow("Scopes", "", 40)
	assert.Contains(t, components.SanitizeText(row), "Scopes: -")

	row = renderPreviewRow("Scope", "public private sensitive admin", 12)
	clean = stripANSI(row)
	assert.Contains(t, clean, "Scope:")
	assert.NotEmpty(t, strings.TrimSpace(clean))

	// Scope fallback branch when the first token cannot fit.
	row = renderPreviewRow("Scope", "verylongscope", 8)
	clean = components.SanitizeText(row)
	assert.Contains(t, clean, "Scope:")
	assert.Contains(t, clean, "...")

	// Scope ellipsis branch when one token fits but the next does not.
	row = renderPreviewRow("Scopes", "public admin private", 18)
	clean = components.SanitizeText(row)
	assert.Contains(t, clean, "Scopes:")
	assert.Contains(t, clean, "...")

	// Scope tiny-budget fallback after first badge is dropped for ellipsis fit.
	row = renderPreviewRow("Scope", "a b", 10)
	clean = components.SanitizeText(row)
	assert.Contains(t, clean, "Scope:")
	assert.Contains(t, clean, "...")

	valueRow := renderPreviewRow("Status", "active", 20)
	assert.Contains(t, components.SanitizeText(valueRow), "Status: active")

	// Non-scope value clamp path with tiny width.
	valueRow = renderPreviewRow("Status", "very-long-status-value", 3)
	assert.Contains(t, components.SanitizeText(valueRow), "Status:")

	// Prefix-pressure branch keeps minimum clamp budget of 4 chars.
	valueRow = renderPreviewRow("VeryLongLabel", "abcdef", 4)
	assert.Contains(t, components.SanitizeText(valueRow), "VeryLongLabel:")
}

func TestParseScopePreviewTokensAndFormatScopePreviewEdgeCases(t *testing.T) {
	assert.Nil(t, parseScopePreviewTokens(""))
	assert.Nil(t, parseScopePreviewTokens("-"))
	assert.Equal(
		t,
		[]string{"public", "admin", "private"},
		parseScopePreviewTokens("[public], admin | public private"),
	)

	assert.Equal(t, "-", formatScopePreview([]string{"", "  "}))
	assert.Equal(t, "[public] [admin]", formatScopePreview([]string{" public ", "admin"}))
}

func TestWrapPreviewTextSkipsLeadingSpaceOnWrappedLine(t *testing.T) {
	wrapped := wrapPreviewText("aa bb cc", 2)
	require.GreaterOrEqual(t, len(wrapped), 3)
	for _, line := range wrapped {
		assert.False(t, strings.HasPrefix(line, " "))
	}
}

func TestWrapPreviewTextHandlesCombiningAndWideRunes(t *testing.T) {
	// Combining mark path exercises rune width fallback (rw < 1 => rw = 1).
	combining := wrapPreviewText("\u0301a", 1)
	require.NotEmpty(t, combining)

	// Wide rune at column 0 path (lineW == 0 while rune width > target width).
	wide := wrapPreviewText("界x", 1)
	require.NotEmpty(t, wide)
	assert.Contains(t, wide[0], "界")
}

func TestWrapPreviewTextHandlesZeroWidthRuneDuringWrap(t *testing.T) {
	// Keep width tighter than text so the loop executes and zero-width runes are normalized.
	wrapped := wrapPreviewText("a\u0301b", 1)
	require.GreaterOrEqual(t, len(wrapped), 2)
	assert.Equal(t, "a", wrapped[0])
	assert.Equal(t, "b", wrapped[len(wrapped)-1])
	assert.Contains(t, strings.Join(wrapped, ""), "\u0301")
}

func TestPreviewStringAndListValueMatrix(t *testing.T) {
	assert.Equal(t, "", previewStringValue(nil, "note"))
	assert.Equal(t, "", previewListValue(nil, "tags"))

	m := api.JSONMap{}
	assert.Equal(t, "", previewStringValue(m, "note"))
	assert.Equal(t, "", previewListValue(m, "tags"))

	m["note"] = "  hello  "
	assert.Equal(t, "hello", previewStringValue(m, "note"))

	m["note"] = "   "
	assert.Equal(t, "", previewStringValue(m, "note"))

	m["tags"] = "not-a-list"
	assert.Equal(t, "", previewListValue(m, "tags"))

	m["tags"] = []any{"alpha", " ", nil, "beta"}
	assert.Equal(t, "alpha, beta", previewListValue(m, "tags"))
}

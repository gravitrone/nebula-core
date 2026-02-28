package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

// TestBoxWidthBounds handles test box width bounds.
func TestBoxWidthBounds(t *testing.T) {
	assert.Equal(t, 40, boxWidth(10))
	assert.Equal(t, 194, boxWidth(200))
	assert.Equal(t, 94, boxWidth(100))
}

// TestBoxNarrowTerminalClampsWidth handles test box narrow terminal clamps width.
func TestBoxNarrowTerminalClampsWidth(t *testing.T) {
	out := TitledBox("Inbox", "line", 20)
	overflow := false
	for _, line := range strings.Split(out, "\n") {
		if lipgloss.Width(line) > 20 {
			overflow = true
			break
		}
	}
	assert.False(t, overflow)
}

// TestTitledBoxIncludesTitle handles test titled box includes title.
func TestTitledBoxIncludesTitle(t *testing.T) {
	out := TitledBox("My Title", "Content", 80)
	assert.True(t, strings.Contains(out, "My Title"))
}

// TestTitledBoxEmptyTitleFallsBack handles test titled box empty title falls back.
func TestTitledBoxEmptyTitleFallsBack(t *testing.T) {
	out := TitledBox("", "Content", 80)
	assert.True(t, strings.Contains(out, "Content"))
}

// TestErrorBoxIncludesMessage handles test error box includes message.
func TestErrorBoxIncludesMessage(t *testing.T) {
	out := ErrorBox("Error", "Something broke", 80)
	assert.True(t, strings.Contains(out, "Something broke"))
}

// TestEmptyStateBoxIncludesActions handles test empty state box includes actions.
func TestEmptyStateBoxIncludesActions(t *testing.T) {
	out := EmptyStateBox("Entities", "No entities found.", []string{"Press n to create", "Press / to search"}, 80)
	clean := SanitizeText(out)

	assert.Contains(t, clean, "Entities")
	assert.Contains(t, clean, "No entities found.")
	assert.Contains(t, clean, "Try:")
	assert.Contains(t, clean, "Press n to create")
	assert.Contains(t, clean, "Press / to search")
}

// TestTruncateRunes handles test truncate runes.
func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "", truncateRunes("hello", 0))
	assert.Equal(t, "he", truncateRunes("hello", 2))
	assert.Equal(t, "你", truncateRunes("你好", 1))
}

// TestTableClampsLongValues ensures table rows stay within the box width.
func TestTableClampsLongValues(t *testing.T) {
	rows := []TableRow{
		{
			Label: strings.Repeat("Label", 8),
			Value: strings.Repeat("value", 40),
		},
	}
	out := Table("Table", rows, 60)
	maxWidth := lipgloss.Width(strings.Split(Box("x", 60), "\n")[0])
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), maxWidth)
	}
}

// TestWrapTableValueWrapsLongText handles test wrap table value wraps long text.
func TestWrapTableValueWrapsLongText(t *testing.T) {
	lines := wrapTableValue("alpha beta gamma delta epsilon zeta", 12)
	assert.GreaterOrEqual(t, len(lines), 3)
	assert.Equal(t, "alpha beta", lines[0])
	assert.Equal(t, "gamma delta", lines[1])
	assert.Equal(t, "epsilon zeta", lines[2])
}

// TestWrapTableWordsHandlesOversizedToken handles test wrap table words handles oversized token.
func TestWrapTableWordsHandlesOversizedToken(t *testing.T) {
	lines := wrapTableWords("supercalifragilisticexpialidocious test", 10)
	assert.Len(t, lines, 2)
	assert.Equal(t, "superca...", lines[0])
	assert.Equal(t, "test", lines[1])
}

// TestActiveBoxClampsWidth handles test active box clamps width.
func TestActiveBoxClampsWidth(t *testing.T) {
	out := ActiveBox("hello\nworld", 40)
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 40)
	}
}

// TestInfoRowSanitizesLabelAndValue handles test info row sanitizes label and value.
func TestInfoRowSanitizesLabelAndValue(t *testing.T) {
	out := InfoRow("na\u202Eme\x1b]0;evil\x07", "va\x1b[2Jlu\u202Ee")
	assert.NotContains(t, out, "\u202E")
	assert.NotContains(t, out, "\x1b]")
	assert.NotContains(t, out, "\x1b[2J")

	clean := SanitizeText(out)
	assert.Contains(t, clean, "name: value")
}

// TestIndentPreservesLineCountAndAddsPadding handles test indent preserves line count and adds padding.
func TestIndentPreservesLineCountAndAddsPadding(t *testing.T) {
	src := "a\nb\nc"
	out := Indent(src, 2)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 3)
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "  "))
	}
}

// TestCenterLineAddsLeftPadding handles test center line adds left padding.
func TestCenterLineAddsLeftPadding(t *testing.T) {
	out := CenterLine("hi", 80)
	pad := (safeBoxWidth(80) - lipgloss.Width("hi")) / 2
	assert.True(t, strings.HasPrefix(out, strings.Repeat(" ", pad)))
}

// TestDiffTableRendersMultilineValuesAndSanitizes handles test diff table renders multiline values and sanitizes.
func TestDiffTableRendersMultilineValuesAndSanitizes(t *testing.T) {
	out := DiffTable("Changes", []DiffRow{
		{
			Label: "Field\u202E\x1b]0;bad\x07",
			From:  "from1\n\x1b[2Jfrom2 and a very very long metadata value that should wrap safely in the diff area",
			To:    "to1\n\u202Eto2 and another very very long metadata value that should also wrap safely",
		},
	}, 60)

	assert.NotContains(t, out, "\x1b]")
	assert.NotContains(t, out, "\u202E")
	assert.NotContains(t, out, "\x1b[2J")

	clean := SanitizeText(out)
	assert.Contains(t, clean, "Changes")
	assert.Contains(t, clean, "- from1")
	assert.Contains(t, clean, "from2")
	assert.Contains(t, clean, "+ to1")
	assert.Contains(t, clean, "to2")

	maxWidth := lipgloss.Width(strings.Split(Box("x", 60), "\n")[0])
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), maxWidth)
	}
}

// TestMetadataTableRendersNestedStructures handles test metadata table renders nested structures.
func TestMetadataTableRendersNestedStructures(t *testing.T) {
	out := MetadataTable(map[string]any{
		"b": "two",
		"a": map[string]any{
			"nested": []any{"x", map[string]any{"k": "v"}},
		},
	}, 60)

	clean := SanitizeText(out)
	assert.Contains(t, clean, "Metadata")
	assert.Contains(t, clean, "a:")
	assert.Contains(t, clean, "nested:")
}

// TestRenderMetadataLinesSortsKeys handles test render metadata lines sorts keys.
func TestRenderMetadataLinesSortsKeys(t *testing.T) {
	lines := renderMetadataLines(map[string]any{"b": 1, "a": 2}, 0)
	assert.GreaterOrEqual(t, len(lines), 2)
	assert.True(t, strings.HasPrefix(lines[0], "a:"))
}

// TestFormatMetadataValueEncodesMapsInArrays handles test format metadata value encodes maps in arrays.
func TestFormatMetadataValueEncodesMapsInArrays(t *testing.T) {
	val := formatMetadataValue([]any{
		map[string]any{"a": 1},
		"x",
	})
	assert.Contains(t, val, `{"a":1}`)
	assert.Contains(t, val, "x")
}

// TestMaxIntReturnsLarger handles test max int returns larger.
func TestMaxIntReturnsLarger(t *testing.T) {
	assert.Equal(t, 2, maxInt(1, 2))
	assert.Equal(t, 2, maxInt(2, 1))
}

// TestClampTextWidthEllipsisHandlesTightWidths handles test clamp text width ellipsis handles tight widths.
func TestClampTextWidthEllipsisHandlesTightWidths(t *testing.T) {
	assert.Equal(t, "", ClampTextWidthEllipsis("hello", 0))
	assert.Equal(t, "he", ClampTextWidthEllipsis("hello", 2))
	assert.Equal(t, "hel...", ClampTextWidthEllipsis("hello world", 6))
}

// TestTitledBoxWithHeaderStyleRendersCustomTitle handles test titled box with header style renders custom title.
func TestTitledBoxWithHeaderStyleRendersCustomTitle(t *testing.T) {
	header := lipgloss.NewStyle().Bold(true)
	out := TitledBoxWithHeaderStyle("Custom Header", "body", 70, header)
	clean := SanitizeText(out)
	assert.Contains(t, clean, "Custom Header")
	assert.Contains(t, clean, "body")
}

// TestParseMetadataScopesInlineHandlesSupportedShapes handles test parse metadata scopes inline handles supported shapes.
func TestParseMetadataScopesInlineHandlesSupportedShapes(t *testing.T) {
	assert.Equal(t, "public, private", parseMetadataScopesInline([]string{"public", "private"}))
	assert.Equal(t, "public, admin", parseMetadataScopesInline([]any{"public", "admin"}))
	assert.Equal(t, "sensitive", parseMetadataScopesInline(" sensitive "))
	assert.Equal(t, "", parseMetadataScopesInline(map[string]any{"k": "v"}))
}

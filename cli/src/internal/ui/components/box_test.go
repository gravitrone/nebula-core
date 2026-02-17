package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestBoxWidthBounds(t *testing.T) {
	assert.Equal(t, 40, boxWidth(10))
	assert.Equal(t, 194, boxWidth(200))
	assert.Equal(t, 94, boxWidth(100))
}

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

func TestTitledBoxIncludesTitle(t *testing.T) {
	out := TitledBox("My Title", "Content", 80)
	assert.True(t, strings.Contains(out, "My Title"))
}

func TestTitledBoxEmptyTitleFallsBack(t *testing.T) {
	out := TitledBox("", "Content", 80)
	assert.True(t, strings.Contains(out, "Content"))
}

func TestErrorBoxIncludesMessage(t *testing.T) {
	out := ErrorBox("Error", "Something broke", 80)
	assert.True(t, strings.Contains(out, "Something broke"))
}

func TestEmptyStateBoxIncludesActions(t *testing.T) {
	out := EmptyStateBox("Entities", "No entities found.", []string{"Press n to create", "Press / to search"}, 80)
	clean := SanitizeText(out)

	assert.Contains(t, clean, "Entities")
	assert.Contains(t, clean, "No entities found.")
	assert.Contains(t, clean, "Try:")
	assert.Contains(t, clean, "Press n to create")
	assert.Contains(t, clean, "Press / to search")
}

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

func TestActiveBoxClampsWidth(t *testing.T) {
	out := ActiveBox("hello\nworld", 40)
	for _, line := range strings.Split(out, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 40)
	}
}

func TestInfoRowSanitizesLabelAndValue(t *testing.T) {
	out := InfoRow("na\u202Eme\x1b]0;evil\x07", "va\x1b[2Jlu\u202Ee")
	assert.NotContains(t, out, "\u202E")
	assert.NotContains(t, out, "\x1b]")
	assert.NotContains(t, out, "\x1b[2J")

	clean := SanitizeText(out)
	assert.Contains(t, clean, "name: value")
}

func TestIndentPreservesLineCountAndAddsPadding(t *testing.T) {
	src := "a\nb\nc"
	out := Indent(src, 2)
	lines := strings.Split(out, "\n")
	assert.Len(t, lines, 3)
	for _, line := range lines {
		assert.True(t, strings.HasPrefix(line, "  "))
	}
}

func TestCenterLineAddsLeftPadding(t *testing.T) {
	out := CenterLine("hi", 80)
	pad := (safeBoxWidth(80) - lipgloss.Width("hi")) / 2
	assert.True(t, strings.HasPrefix(out, strings.Repeat(" ", pad)))
}

func TestDiffTableRendersMultilineValuesAndSanitizes(t *testing.T) {
	out := DiffTable("Changes", []DiffRow{
		{
			Label: "Field\u202E\x1b]0;bad\x07",
			From:  "from1\n\x1b[2Jfrom2",
			To:    "to1\n\u202Eto2",
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
}

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

func TestRenderMetadataLinesSortsKeys(t *testing.T) {
	lines := renderMetadataLines(map[string]any{"b": 1, "a": 2}, 0)
	assert.GreaterOrEqual(t, len(lines), 2)
	assert.True(t, strings.HasPrefix(lines[0], "a:"))
}

func TestFormatMetadataValueEncodesMapsInArrays(t *testing.T) {
	val := formatMetadataValue([]any{
		map[string]any{"a": 1},
		"x",
	})
	assert.Contains(t, val, `{"a":1}`)
	assert.Contains(t, val, "x")
}

func TestMaxIntReturnsLarger(t *testing.T) {
	assert.Equal(t, 2, maxInt(1, 2))
	assert.Equal(t, 2, maxInt(2, 1))
}

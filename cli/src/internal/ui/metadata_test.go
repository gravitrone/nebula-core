package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestParseMetadataInput(t *testing.T) {
	input := "name: alex\nprofile:\n  age: 17\n  tags: [ai, ml]"
	got, err := parseMetadataInput(input)
	assert.NoError(t, err)
	assert.Equal(t, "alex", got["name"])

	profile, ok := got["profile"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "17", profile["age"])

	tags, ok := profile["tags"].([]any)
	assert.True(t, ok)
	assert.Equal(t, []any{"ai", "ml"}, tags)
}

func TestParseMetadataInputPipeRows(t *testing.T) {
	input := strings.Join([]string{
		"profile | timezone | europe/warsaw",
		"profile | website | https://bro.dev",
		"owner | alxx",
	}, "\n")

	got, err := parseMetadataInput(input)
	assert.NoError(t, err)

	profile, ok := got["profile"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "europe/warsaw", profile["timezone"])
	assert.Equal(t, "https://bro.dev", profile["website"])
	assert.Equal(t, "alxx", got["owner"])
}

func TestParseMetadataInputErrors(t *testing.T) {
	_, err := parseMetadataInput("name alex")
	assert.Error(t, err)

	_, err = parseMetadataInput(" name: bad")
	assert.Error(t, err)
}

func TestMetadataToInput(t *testing.T) {
	data := map[string]any{
		"name": "alex",
		"profile": map[string]any{
			"age": 17,
		},
	}
	out := metadataToInput(data)
	assert.Contains(t, out, "name: alex")
	assert.Contains(t, out, "profile:")
	assert.Contains(t, out, "  age: 17")
}

func TestMetadataScopeHelpers(t *testing.T) {
	input := map[string]any{
		"scopes": []any{"Public", "private"},
		"name":   "alex",
	}
	scopes := extractMetadataScopes(input)
	assert.Equal(t, []string{"public", "private"}, scopes)

	stripped := stripMetadataScopes(input)
	_, ok := stripped["scopes"]
	assert.False(t, ok)

	merged := mergeMetadataScopes(stripped, []string{"private"})
	assert.Equal(t, []string{"private"}, merged["scopes"])
}

func TestNormalizeStructuredMetadataValueParsesStringSlices(t *testing.T) {
	raw := map[string]any{
		"context_segments": []string{
			`{"scopes":["private"],"text":"internal note"}`,
		},
	}

	normalized := normalizeStructuredMetadataValue(raw).(map[string]any) //nolint:forcetypeassert
	segments, ok := normalized["context_segments"].([]any)
	assert.True(t, ok)
	if assert.Len(t, segments, 1) {
		segment, ok := segments[0].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "internal note", segment["text"])
	}
}

func TestMetadataLinesStyledRenderContextSegmentsCleanly(t *testing.T) {
	data := map[string]any{
		"context_segments": []string{
			`{"scopes":["public"],"text":"Public profile block."}`,
		},
	}

	lines := metadataLinesStyled(data, 0)
	rendered := ""
	if len(lines) > 0 {
		rendered = lines[0]
	}
	assert.Contains(t, rendered, "context_segments")

	joined := ""
	for i, line := range lines {
		if i > 0 {
			joined += "\n"
		}
		joined += line
	}
	assert.Contains(t, joined, "[public] Public profile block.")
}

func TestMetadataDisplayRowsFlattensNestedMapsAndLists(t *testing.T) {
	data := map[string]any{
		"owner": "alxx",
		"profile": map[string]any{
			"timezone": "Europe/Warsaw",
		},
		"context_segments": []any{
			map[string]any{
				"scopes": []any{"public", "private"},
				"text":   "Scoped note",
			},
		},
	}

	rows := metadataDisplayRows(data)
	assert.NotEmpty(t, rows)

	var fields []string
	for _, row := range rows {
		fields = append(fields, row.field)
	}
	assert.Contains(t, fields, "owner")
	assert.Contains(t, fields, "profile.timezone")
	assert.Contains(t, fields, "context_segments[0]")
}

func TestRenderMetadataBlockWithTitleUsesTableLayoutAndScopes(t *testing.T) {
	data := map[string]any{
		"scopes": []any{"public", "admin"},
		"profile": map[string]any{
			"timezone": "Europe/Warsaw",
		},
	}

	out := renderMetadataBlockWithTitle("Metadata", data, 80, false)
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Group")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, clean, "profile")
	assert.Contains(t, clean, "timezone")
	assert.NotContains(t, clean, "profile.timezone")
	assert.Contains(t, clean, "[public]")
	assert.Contains(t, clean, "[admin]")
}

func TestFormatMetadataInlineSanitizesStructuredValues(t *testing.T) {
	inline := formatMetadataInline(map[string]any{
		"k\x1b]0;bad\x07": "v\u202E",
	})
	assert.NotContains(t, inline, "\x1b]")
	assert.NotContains(t, inline, "\u202E")
	assert.Contains(t, inline, "\"k\"")
	assert.Contains(t, inline, "\"v\"")
}

func TestSanitizeMetadataValueRecursesNestedCollections(t *testing.T) {
	sanitized := sanitizeMetadataValue(map[string]any{
		"ke\u202Ey": []any{"va\x1b]0;bad\x07l", map[string]any{"n\x1b]x": "v"}},
	}).(map[string]any)

	_, hasUnsafeKey := sanitized["ke\u202Ey"]
	assert.False(t, hasUnsafeKey)
	assert.Contains(t, sanitized, "key")
}

func TestWrapMetadataDisplayLineAndWordsWrapLongInput(t *testing.T) {
	line := "  - this is a very long metadata line that should wrap safely across columns"
	wrapped := wrapMetadataDisplayLine(line, 24)
	assert.Greater(t, len(wrapped), 1)
	for _, segment := range wrapped {
		assert.LessOrEqual(t, len(stripANSI(segment)), 28)
	}

	words := wrapMetadataWords("alpha beta gamma delta epsilon", 10)
	assert.GreaterOrEqual(t, len(words), 2)
}

func TestWrapMetadataDisplayLinesPreservesBlankRows(t *testing.T) {
	lines := wrapMetadataDisplayLines([]string{"", "hello world"}, 6)
	assert.NotEmpty(t, lines)
	assert.Equal(t, "", lines[0])
	assert.GreaterOrEqual(t, len(lines), 2)
}

func TestRenderMetadataSelectableBlockOmitsActiveChevronPrefix(t *testing.T) {
	rows := []metadataDisplayRow{
		{field: "note", value: "hello"},
	}
	list := components.NewList(metadataPanelPageSize(false))
	syncMetadataList(list, rows, metadataPanelPageSize(false))

	out := renderMetadataSelectableBlockWithTitle("Metadata", rows, 80, list, map[int]bool{})
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "[ ]")
	assert.NotContains(t, clean, "›[ ]")
	assert.NotContains(t, clean, ">[ ]")
}

func TestRenderMetadataSelectableBlockHumanizesContextSegmentField(t *testing.T) {
	rows := []metadataDisplayRow{
		{field: "context_segments[0]", value: "[public] hello"},
	}
	list := components.NewList(metadataPanelPageSize(false))
	syncMetadataList(list, rows, metadataPanelPageSize(false))

	out := renderMetadataSelectableBlockWithTitle("Metadata", rows, 80, list, map[int]bool{})
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "context")
	assert.Contains(t, clean, "segment 1")
	assert.NotContains(t, clean, "context_segments[0]")
}

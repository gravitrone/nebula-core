package ui

import (
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
		"scopes": []any{"Public", "work"},
		"name":   "alex",
	}
	scopes := extractMetadataScopes(input)
	assert.Equal(t, []string{"public", "work"}, scopes)

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
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, clean, "profile.timezone")
	assert.Contains(t, clean, "[public]")
	assert.Contains(t, clean, "[admin]")
}

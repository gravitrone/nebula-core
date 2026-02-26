package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRenderMetadataInputHandlesEmptyAndPlainText(t *testing.T) {
	assert.Equal(t, "-", renderMetadataInput(""))

	plain := components.SanitizeText(renderMetadataInput("hello world"))
	assert.Contains(t, plain, "hello world")
}

func TestRenderMetadataInputPipeRowsAndListRows(t *testing.T) {
	input := strings.Join([]string{
		"profile | timezone | Europe/Warsaw",
		"- first tag",
	}, "\n")

	out := components.SanitizeText(renderMetadataInput(input))
	assert.Contains(t, out, "profile")
	assert.Contains(t, out, "timezone")
	assert.Contains(t, out, "Europe/Warsaw")
	assert.Contains(t, out, "- first tag")
}

func TestRenderMetadataInputHandlesKeyValueRows(t *testing.T) {
	out := components.SanitizeText(renderMetadataInput("owner: alxx"))
	assert.Contains(t, out, "owner")
	assert.Contains(t, out, "alxx")
}

func TestRenderMetadataInputPreservesIndentation(t *testing.T) {
	out := renderMetadataInput("  owner: alxx")
	assert.True(t, strings.HasPrefix(out, "  "))
}

func TestFormatMetadataInlineCoversShapeMatrix(t *testing.T) {
	assert.Equal(t, "None", formatMetadataInline(nil))
	assert.Equal(t, "None", formatMetadataInline("   "))
	assert.Contains(t, formatMetadataInline(map[string]any{"x": "y"}), "\"x\"")
	assert.Equal(t, "[a, None]", formatMetadataInline([]any{"a", nil}))
}

func TestHumanizeGoMapStringMatrix(t *testing.T) {
	assert.Equal(
		t,
		"[public private] hello",
		humanizeGoMapString("map[scopes:[public private] text:hello]"),
	)
	assert.Equal(t, "hello", humanizeGoMapString("map[text:hello]"))
	assert.Equal(t, "raw", humanizeGoMapString("raw"))
}

func TestRenderMetadataEditorPreviewFallbackOnParseError(t *testing.T) {
	buffer := "name alxx" // invalid metadata input format
	out := components.SanitizeText(renderMetadataEditorPreview(buffer, nil, 90, 3))
	assert.Contains(t, out, "name alxx")
}

func TestRenderMetadataEditorPreviewShowsMoreRowsIndicator(t *testing.T) {
	buffer := strings.Join([]string{
		"profile | timezone | Europe/Warsaw",
		"profile | locale | pl_PL",
		"owner | alxx",
	}, "\n")
	out := components.SanitizeText(renderMetadataEditorPreview(buffer, []string{"public"}, 90, 1))
	assert.Contains(t, out, "+3 more rows")
}

func TestMetadataPreviewPrefersKnownKeysAndTruncates(t *testing.T) {
	data := map[string]any{
		"summary": "This is a long summary used for preview generation.",
		"z":       "fallback",
	}
	preview := metadataPreview(data, 16)
	assert.NotEmpty(t, preview)
	assert.LessOrEqual(t, len(stripANSI(preview)), 16+3)
}

func TestMetadataValuePreviewCoversCompositeValues(t *testing.T) {
	assert.Contains(t, metadataValuePreview([]any{"alpha", "beta"}, 20), "alpha")
	assert.Contains(
		t,
		metadataValuePreview(map[string]any{"text": "note", "scopes": []any{"public"}}, 30),
		"note",
	)
	assert.Equal(t, "<nil>", metadataValuePreview(nil, 20))
}

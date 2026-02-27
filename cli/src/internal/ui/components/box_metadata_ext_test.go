package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeStructuredValueParsesJSONAndDoubleEncoded(t *testing.T) {
	parsed := normalizeStructuredValue(`{"a":1}`)
	_, ok := parsed.(map[string]any)
	assert.True(t, ok)

	parsed = normalizeStructuredValue(`["a", "b"]`)
	_, ok = parsed.([]any)
	assert.True(t, ok)

	parsed = normalizeStructuredValue(`"{\"nested\":true}"`)
	obj, ok := parsed.(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, true, obj["nested"])

	assert.Equal(t, "raw", normalizeStructuredValue("raw"))
}

func TestRenderMetadataValueLinesCoversNestedShapes(t *testing.T) {
	lines := renderMetadataValueLines(map[string]any{}, 0)
	assert.Equal(t, []string{"{}"}, lines)

	lines = renderMetadataValueLines([]any{}, 2)
	assert.Equal(t, []string{"[]"}, lines)

	lines = renderMetadataValueLines(
		[]any{
			map[string]any{"text": "hello", "scopes": []any{"public", "admin"}},
			map[string]any{"k": "v"},
			[]any{"x"},
			nil,
		},
		2,
	)
	assert.NotEmpty(t, lines)
	joined := SanitizeText(joinLines(lines))
	assert.Contains(t, joined, "[public, admin] hello")
	assert.Contains(t, joined, "k:")
	assert.Contains(t, joined, "- [x]")
	assert.Contains(t, joined, "- None")
}

func TestFormatMetadataValueFallbackBranches(t *testing.T) {
	assert.Equal(t, "None", formatMetadataValue(nil))
	assert.Equal(t, "None", formatMetadataValue("   "))
	assert.Equal(t, "[]", formatMetadataValue([]any{}))
	assert.Equal(t, "{}", formatMetadataValue(map[string]any{}))

	bad := map[string]any{"bad": func() {}}
	fallback := formatMetadataValue(bad)
	assert.Contains(t, fallback, "map")
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for i := 1; i < len(lines); i++ {
		out += "\n" + lines[i]
	}
	return out
}

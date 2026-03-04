package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMetadataEditorPreviewAndBlockHandleEmptyNestedMaps(t *testing.T) {
	preview := components.SanitizeText(renderMetadataEditorPreview("profile:", nil, 96, 6))
	assert.Contains(t, preview, "profile")
	assert.Contains(t, preview, "None")

	block := components.SanitizeText(
		renderMetadataBlockWithTitle("Metadata", map[string]any{"profile": map[string]any{}}, 96, false),
	)
	assert.Contains(t, block, "Metadata")
	assert.Contains(t, block, "None")
}

func TestFlattenMetadataListRowsMixedNestedAndScalarBranches(t *testing.T) {
	rows := make([]metadataDisplayRow, 0)
	flattenMetadataListRows("items", []any{[]any{"alpha"}, 7}, &rows)

	require.Len(t, rows, 2)
	assert.Equal(t, "items[0]", rows[0].field)
	assert.Equal(t, "alpha", rows[0].value)
	assert.Equal(t, "items[1]", rows[1].field)
	assert.Equal(t, "7", rows[1].value)
}

func TestParseJSONStructuredStringQuotedObjectAndArray(t *testing.T) {
	parsed, ok := parseJSONStructuredString(`"{\"kind\":\"entity\"}"`)
	require.True(t, ok)
	obj, ok := parsed.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "entity", obj["kind"])

	parsed, ok = parseJSONStructuredString("[1,2,3]")
	require.True(t, ok)
	list, ok := parsed.([]any)
	require.True(t, ok)
	require.Len(t, list, 3)
}

func TestMetadataGroupAndFieldSeparatorOnlyPath(t *testing.T) {
	group, field := metadataGroupAndField("[]")
	assert.Equal(t, "-", group)
	assert.Equal(t, "[]", field)
}

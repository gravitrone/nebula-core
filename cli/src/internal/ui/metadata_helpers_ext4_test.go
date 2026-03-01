package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataValueWrappedLinesBranchMatrix(t *testing.T) {
	assert.Equal(t, []string{"None"}, metadataValueWrappedLines("   ", 20))
	assert.Equal(t, []string{"alpha"}, metadataValueWrappedLines(" alpha ", 0))

	lines := metadataValueWrappedLines("alpha\n\nbeta", 20)
	require.Len(t, lines, 3)
	assert.Equal(t, "alpha", lines[0])
	assert.Equal(t, "", lines[1])
	assert.Equal(t, "beta", lines[2])

	lines = metadataValueWrappedLines("alpha\nbeta", 4)
	assert.NotEmpty(t, lines)
}

func TestMetadataLinesStyledAndListStyledBranchMatrix(t *testing.T) {
	lines := metadataLinesStyled(
		map[string]any{
			"scalars": []any{"a", 1},
			"empty":   []any{},
			"nested":  map[string]any{"k": "v"},
			"value":   "x",
		},
		0,
	)
	text := ""
	for _, line := range lines {
		text += line + "\n"
	}
	assert.Contains(t, text, "nested")
	assert.Contains(t, text, "empty")
	assert.Contains(t, text, "value")

	assert.Nil(t, metadataListLinesStyled([]any{}, 0))

	listLines := metadataListLinesStyled(
		[]any{
			map[string]any{"text": "hello", "scopes": []any{"public", "admin"}},
			map[string]any{"text": "   ", "k": "v"},
			[]any{"child"},
			"scalar",
		},
		0,
	)
	require.NotEmpty(t, listLines)
	joined := ""
	for _, line := range listLines {
		joined += line + "\n"
	}
	assert.Contains(t, joined, "hello")
	assert.Contains(t, joined, "{...}")
	assert.Contains(t, joined, "[...]")
	assert.Contains(t, joined, "scalar")
}

func TestExtractMetadataScopesBranchMatrix(t *testing.T) {
	assert.Nil(t, extractMetadataScopes(nil))
	assert.Nil(t, extractMetadataScopes(map[string]any{"x": "y"}))

	assert.Equal(
		t,
		[]string{"public", "admin"},
		extractMetadataScopes(map[string]any{"scopes": []string{" PUBLIC ", "#admin", "public"}}),
	)
	assert.Equal(
		t,
		[]string{"private", "2"},
		extractMetadataScopes(map[string]any{"scopes": []any{"private", nil, 2}}),
	)
	assert.Equal(
		t,
		[]string{"sensitive"},
		extractMetadataScopes(map[string]any{"scopes": "  sensitive "}),
	)
	assert.Equal(t, []string{}, extractMetadataScopes(map[string]any{"scopes": "   "}))
}

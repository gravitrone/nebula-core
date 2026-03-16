package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMetadataValueBranchMatrix(t *testing.T) {
	value, err := parseMetadataValue("", 1)
	require.NoError(t, err)
	assert.Equal(t, "", value)

	value, err = parseMetadataValue("[]", 2)
	require.NoError(t, err)
	assert.Equal(t, []any{}, value)

	value, err = parseMetadataValue("[alpha, 'beta', \"gamma\"]", 3)
	require.NoError(t, err)
	assert.Equal(t, []any{"alpha", "beta", "gamma"}, value)

	_, err = parseMetadataValue("{\"k\":\"v\"}", 9)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inline objects not supported")
	assert.Contains(t, err.Error(), "line 9")
}

func TestRenderMetadataEditorPreviewHandlesEmptyAndRootRows(t *testing.T) {
	assert.Equal(t, "-", renderMetadataEditorPreview("", nil, 90, 3))

	out := components.SanitizeText(renderMetadataEditorPreview("owner:\n  ", nil, 120, 3))
	assert.True(t, strings.Contains(out, "root | owner | None") || strings.Contains(out, "root | owner | {}"))
}

package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestMetadataGroupAndFieldAdditionalBranches(t *testing.T) {
	group, field := metadataGroupAndField("...")
	assert.Equal(t, "-", group)
	assert.Equal(t, "...", field)

	group, field = metadataGroupAndField("profile.context_segments")
	assert.Equal(t, "profile", group)
	assert.Equal(t, "context", field)

	group, field = metadataGroupAndField("profile.context_segments.kind")
	assert.Equal(t, "profile", group)
	assert.Equal(t, "context.kind", field)

	group, field = metadataGroupAndField("0.name")
	assert.Equal(t, "item 1", group)
	assert.Equal(t, "name", field)
}

func TestRenderMetadataEditorPreviewWidthClampBranches(t *testing.T) {
	input := strings.Join([]string{
		"profile | note | " + strings.Repeat("x", 220),
		"profile | region | eu",
	}, "\n")

	wide := components.SanitizeText(renderMetadataEditorPreview(input, nil, 600, 2))
	assert.Contains(t, wide, "profile | note |")
	assert.Contains(t, wide, "profile | region | eu")

	narrow := components.SanitizeText(renderMetadataEditorPreview(input, nil, 24, 1))
	assert.Contains(t, narrow, "profile | note |")
	assert.Contains(t, narrow, "+1 more rows")
}

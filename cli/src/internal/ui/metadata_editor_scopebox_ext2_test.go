package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestMetadataEditorRenderScopeBoxBranchMatrix(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{})

	ed.scopeSelecting = true
	ed.scopeOptions = nil
	ed.Scopes = nil
	out := components.SanitizeText(ed.renderScopeBox(80))
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "no scopes available")

	ed.scopeSelecting = false
	ed.Scopes = []string{"public", "admin"}
	out = components.SanitizeText(ed.renderScopeBox(80))
	assert.Contains(t, out, "[public]")
	assert.Contains(t, out, "[admin]")
}

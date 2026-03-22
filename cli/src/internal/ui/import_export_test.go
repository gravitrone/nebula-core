package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

// TestImportExportResources handles test import export resources.
func TestImportExportResources(t *testing.T) {
	importResources := importExportResourcesForMode(importMode)
	exportResources := importExportResourcesForMode(exportMode)

	assert.Greater(t, len(importResources), 0)
	assert.Greater(t, len(exportResources), 0)
	assert.Equal(t, "entities", importResources[0].value)

	hasContext := false
	for _, r := range exportResources {
		if r.value == "context" {
			hasContext = true
		}
	}
	assert.True(t, hasContext)
}

// TestImportExportStartResets handles test import export start resets.
func TestImportExportStartResets(t *testing.T) {
	model := NewImportExportModel(&api.Client{})
	model.pathInput.SetValue("tmp")
	model.summary = "old"
	model.Start(importMode)

	assert.Equal(t, stepResource, model.step)
	assert.Equal(t, "", model.pathInput.Value())
	assert.Equal(t, "", model.summary)
	assert.False(t, model.closed)
}

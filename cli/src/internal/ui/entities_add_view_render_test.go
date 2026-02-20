package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestEntitiesAddViewRendersTagsScopesAndMetadataPreview(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 80
	model.view = entitiesViewAdd

	model.addFields[addFieldName].value = "Alpha"
	model.addFields[addFieldType].value = "person"
	model.addStatusIdx = statusIndex(entityStatusOptions, "active")
	model.addTags = []string{"demo"}
	model.addScopes = []string{"public", "work"}
	model.addMeta.Buffer = "profile | role | founder\nprofile | location | warsaw"

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Add Entity")
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Type")
	assert.Contains(t, out, "person")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "demo")
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "work")
	assert.Contains(t, out, "Metadata")
	assert.Contains(t, out, "Group")
	assert.Contains(t, out, "Field")
	assert.Contains(t, out, "Value")
	assert.Contains(t, out, "profile")
	assert.Contains(t, out, "role")
	assert.Contains(t, out, "founder")
}

func TestEntitiesCommitAddScopeNormalizesAndDedupes(t *testing.T) {
	model := NewEntitiesModel(nil)

	model.addScopeBuf = " Public "
	model.commitAddScope()
	assert.Equal(t, []string{"public"}, model.addScopes)
	assert.Equal(t, "", model.addScopeBuf)

	// Duplicate should be ignored.
	model.addScopeBuf = "public"
	model.commitAddScope()
	assert.Equal(t, []string{"public"}, model.addScopes)
}

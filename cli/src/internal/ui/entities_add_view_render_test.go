package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

// TestEntitiesAddViewRendersFormFields verifies the add form renders all expected fields.
func TestEntitiesAddViewRendersFormFields(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 80
	model.view = entitiesViewAdd

	model.addName = "Alpha"
	model.addType = "person"
	model.addStatus = "active"
	model.addTagStr = "demo"
	model.addScopeStr = "public, private"
	model.initAddForm()
	_ = model.addForm.Init()

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Type")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "Scopes")
	assert.NotContains(t, out, "Metadata")
}

// TestEntitiesInitAddFormCreatesHuhForm verifies initAddForm creates a non-nil form.
func TestEntitiesInitAddFormCreatesHuhForm(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.initAddForm()
	assert.NotNil(t, model.addForm)
}

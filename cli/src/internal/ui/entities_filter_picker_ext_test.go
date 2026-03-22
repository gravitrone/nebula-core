package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRenderFilterPickerShowsFallbackWhenFacetIsEmpty(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.filterFacet = entitiesFilterFacetType
	model.filterTypeSet = nil
	model.filterStatSet = nil
	model.filterScopeSet = nil

	out := components.SanitizeText(model.renderFilterPicker())
	assert.Contains(t, out, "No values in current list")
	assert.Contains(t, out, "No active filters")
}

func TestRenderFilterPickerShowsActiveFacetCountsAndSummary(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 110
	model.filterFacet = entitiesFilterFacetStatus
	model.filterTypeSet = []string{"person", "tool"}
	model.filterStatSet = []string{"active", "inactive"}
	model.filterScopeSet = []string{"public", "private"}
	model.filterCursor[entitiesFilterFacetStatus] = 1
	model.filterTypes = map[string]bool{"person": true}
	model.filterStatus = map[string]bool{"active": true}
	model.filterScopes = map[string]bool{"public": true}

	out := components.SanitizeText(model.renderFilterPicker())
	assert.Contains(t, out, "Type (1)")
	assert.Contains(t, out, "Status (1)")
	assert.Contains(t, out, "Scope (1)")
	assert.Contains(t, out, "Active: type=1, status=1, scope=1")
	assert.Contains(t, out, "[X]")
}

func TestEntitiesHandleModeKeysDownAndUpClearModeFocus(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.modeFocus = true
	model.view = entitiesViewAdd

	updated, _ := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, _ = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.False(t, updated.modeFocus)
}

func TestEntitiesHandleModeKeysBackClearsModeFocus(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.modeFocus = true

	updated, _ := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.modeFocus)
}

func TestEntitiesHandleModeKeysToggleBindings(t *testing.T) {
	keys := []tea.KeyPressMsg{
		{Code: tea.KeyEnter},
		{Code: tea.KeyLeft},
		{Code: tea.KeyRight},
		{Code: tea.KeySpace},
	}
	for _, key := range keys {
		model := NewEntitiesModel(nil)
		model.modeFocus = true
		model.view = entitiesViewList

		updated, _ := model.handleModeKeys(key)
		assert.False(t, updated.modeFocus)
		assert.Equal(t, entitiesViewAdd, updated.view)
		assert.False(t, updated.addSaved)
	}
}

func TestEntitiesToggleModeSwitchesAddAndListViews(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewList
	model.modeFocus = true
	model.addSaved = true

	updated, _ := model.toggleMode()
	assert.Equal(t, entitiesViewAdd, updated.view)
	assert.False(t, updated.modeFocus)
	assert.False(t, updated.addSaved)
	assert.NotNil(t, updated.addForm)

	updated.modeFocus = true
	back, _ := updated.toggleMode()
	assert.Equal(t, entitiesViewList, back.view)
	assert.False(t, back.modeFocus)
}

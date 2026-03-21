package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func testEntitiesFilterModel() EntitiesModel {
	model := NewEntitiesModel(nil)
	model.scopeNames = map[string]string{
		"scope-public":  "public",
		"scope-private": "private",
	}
	model.allItems = []api.Entity{
		{ID: "ent-1", Type: "person", Status: "active", PrivacyScopeIDs: []string{"scope-public"}},
		{ID: "ent-2", Type: "tool", Status: "inactive", PrivacyScopeIDs: []string{"scope-private"}},
	}
	model.refreshFilterSets()
	model.applyEntityFilters()
	return model
}

func TestEntitiesHandleFilterInputEnterAndBackPaths(t *testing.T) {
	model := testEntitiesFilterModel()
	model.filtering = true

	updated, _ := model.handleFilterInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, updated.filtering)

	updated.filtering = true
	updated.filterTypes = map[string]bool{"person": true}
	updated.applyEntityFilters()
	assert.Len(t, updated.items, 1)

	updated, _ = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.filtering)
	assert.Empty(t, updated.filterTypes)
	assert.Empty(t, updated.filterStatus)
	assert.Empty(t, updated.filterScopes)
	assert.Len(t, updated.items, 2)
}

func TestEntitiesHandleFilterInputFacetNavigationWraps(t *testing.T) {
	model := testEntitiesFilterModel()
	model.filterFacet = entitiesFilterFacetType

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, entitiesFilterFacetScope, model.filterFacet)

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, entitiesFilterFacetType, model.filterFacet)
}

func TestEntitiesHandleFilterInputCursorMovementWrapsWithinFacet(t *testing.T) {
	model := testEntitiesFilterModel()
	model.filterFacet = entitiesFilterFacetType
	model.filterCursor[entitiesFilterFacetType] = 0

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, model.filterCursor[entitiesFilterFacetType])

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 0, model.filterCursor[entitiesFilterFacetType])

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, model.filterCursor[entitiesFilterFacetType])
}

func TestEntitiesHandleFilterInputSpaceTogglesSelectionAndApplies(t *testing.T) {
	model := testEntitiesFilterModel()
	model.filterFacet = entitiesFilterFacetType
	model.filterCursor[entitiesFilterFacetType] = 0 // person

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.True(t, model.filterTypes["person"])
	assert.Len(t, model.items, 1)
	assert.Equal(t, "ent-1", model.items[0].ID)

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.False(t, model.filterTypes["person"])
	assert.Len(t, model.items, 2)
}

func TestEntitiesHandleFilterInputBulkToggleSelectsAndClearsFacet(t *testing.T) {
	model := testEntitiesFilterModel()
	model.filterFacet = entitiesFilterFacetStatus

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	assert.Len(t, model.filterStatus, len(model.filterStatSet))
	assert.True(t, model.filterStatus["active"])
	assert.True(t, model.filterStatus["inactive"])

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	assert.Empty(t, model.filterStatus)
}

func TestEntitiesHandleFilterInputClearCommandResetsAllFacets(t *testing.T) {
	model := testEntitiesFilterModel()
	model.filterTypes = map[string]bool{"person": true}
	model.filterStatus = map[string]bool{"active": true}
	model.filterScopes = map[string]bool{"public": true}
	model.applyEntityFilters()
	assert.Len(t, model.items, 1)

	model, _ = model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	assert.Empty(t, model.filterTypes)
	assert.Empty(t, model.filterStatus)
	assert.Empty(t, model.filterScopes)
	assert.Len(t, model.items, 2)
}

func TestEntitiesHandleFilterInputIgnoresUnknownKeys(t *testing.T) {
	model := testEntitiesFilterModel()
	model.filterFacet = entitiesFilterFacetStatus
	model.filterCursor[entitiesFilterFacetStatus] = 1

	updated, _ := model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Equal(t, model.filterFacet, updated.filterFacet)
	assert.Equal(t, model.filterCursor, updated.filterCursor)
	assert.Equal(t, model.items, updated.items)
}

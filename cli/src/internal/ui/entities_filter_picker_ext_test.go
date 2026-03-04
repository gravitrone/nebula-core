package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
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
	model.addFocus = addFieldTags

	updated, _ := model.handleModeKeys(tea.KeyMsg{Type: tea.KeyDown})
	assert.False(t, updated.modeFocus)
	assert.Equal(t, 0, updated.addFocus)

	updated.modeFocus = true
	updated, _ = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.False(t, updated.modeFocus)
}

func TestEntitiesHandleModeKeysBackClearsModeFocus(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.modeFocus = true

	updated, _ := model.handleModeKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.modeFocus)
}

func TestEntitiesHandleModeKeysToggleBindings(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
		{Type: tea.KeySpace},
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

	updated.modeFocus = true
	updated, _ = updated.toggleMode()
	assert.Equal(t, entitiesViewList, updated.view)
	assert.False(t, updated.modeFocus)
}

func TestFilterMapForFacetInitializesMissingMaps(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.filterTypes = nil
	model.filterStatus = nil
	model.filterScopes = nil

	types := model.filterMapForFacet(entitiesFilterFacetType)
	status := model.filterMapForFacet(entitiesFilterFacetStatus)
	scopes := model.filterMapForFacet(entitiesFilterFacetScope)
	unknown := model.filterMapForFacet(entitiesFilterFacet(99))

	assert.NotNil(t, types)
	assert.NotNil(t, status)
	assert.NotNil(t, scopes)
	assert.NotNil(t, unknown)
	assert.Empty(t, unknown)
}

func TestFilterOptionsForFacetRoutesSets(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.filterTypeSet = []string{"person"}
	model.filterStatSet = []string{"active"}
	model.filterScopeSet = []string{"public"}

	assert.Equal(t, []string{"person"}, model.filterOptionsForFacet(entitiesFilterFacetType))
	assert.Equal(t, []string{"active"}, model.filterOptionsForFacet(entitiesFilterFacetStatus))
	assert.Equal(t, []string{"public"}, model.filterOptionsForFacet(entitiesFilterFacetScope))
	assert.Nil(t, model.filterOptionsForFacet(entitiesFilterFacet(99)))
}

func TestRefreshFilterSetsBuildsFacetOptionsAndRetainsSelection(t *testing.T) {
	publicID := "scope-public"
	privateID := "scope-private"

	model := NewEntitiesModel(nil)
	model.scopeNames = map[string]string{
		publicID:  "public",
		privateID: "private",
	}
	model.allItems = []api.Entity{
		{Type: "person", Status: "active", PrivacyScopeIDs: []string{publicID}},
		{Type: "tool", Status: "inactive", PrivacyScopeIDs: []string{privateID}},
	}
	model.filterTypes = map[string]bool{"person": true, "removed": true}
	model.filterStatus = map[string]bool{"active": true}
	model.filterScopes = map[string]bool{"public": true}
	model.filterCursor[entitiesFilterFacetType] = 9

	model.refreshFilterSets()

	assert.Equal(t, []string{"person", "tool"}, model.filterTypeSet)
	assert.Equal(t, []string{"active", "inactive"}, model.filterStatSet)
	assert.Equal(t, []string{"private", "public"}, model.filterScopeSet)
	assert.True(t, model.filterTypes["person"])
	assert.False(t, model.filterTypes["removed"])
	assert.Equal(t, 1, model.filterCursor[entitiesFilterFacetType])
}

func TestRetainEntityFilterSelectionAndSortedKeysHelpers(t *testing.T) {
	assert.Empty(t, retainEntityFilterSelection(nil, []string{"public"}))

	current := map[string]bool{
		"public":  true,
		"private": false,
		"ghost":   true,
	}
	next := retainEntityFilterSelection(current, []string{"public", "private"})
	assert.Equal(t, map[string]bool{"public": true}, next)

	assert.Nil(t, sortedFilterKeys(map[string]struct{}{}))
	assert.Equal(
		t,
		[]string{"alpha", "zeta"},
		sortedFilterKeys(map[string]struct{}{"zeta": {}, "alpha": {}}),
	)
}

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesAddTagScopeHelpersBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)

	assert.Equal(t, "-", model.renderAddTags(false))
	assert.Contains(t, stripANSI(model.renderAddTags(true)), "█")

	model.addTags = []string{"alpha"}
	model.addTagInput.SetValue("beta")
	assert.Contains(t, stripANSI(model.renderAddTags(false)), "alpha")
	assert.Contains(t, stripANSI(model.renderAddTags(false)), "beta")
	assert.Contains(t, stripANSI(model.renderAddTags(true)), "█")

	model.addTagInput.SetValue("   ")
	model.commitAddTag()
	assert.Equal(t, "", model.addTagInput.Value())

	model.addTagInput.SetValue("ALPHA")
	model.commitAddTag()
	assert.Equal(t, []string{"alpha"}, model.addTags)

	model.addTagInput.SetValue("gamma_tag")
	model.commitAddTag()
	assert.Equal(t, []string{"alpha", "gamma-tag"}, model.addTags)

	model.addScopeInput.SetValue("  ")
	model.commitAddScope()
	assert.Equal(t, "", model.addScopeInput.Value())

	model.addScopes = []string{"public"}
	model.addScopeInput.SetValue(" PUBLIC ")
	model.commitAddScope()
	assert.Equal(t, []string{"public"}, model.addScopes)

	model.addScopeInput.SetValue("sensitive")
	model.commitAddScope()
	assert.Equal(t, []string{"public", "sensitive"}, model.addScopes)
}

func TestEntitiesSearchInputAndBulkSelectionBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewSearch

	updated, cmd := model.handleSearchInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchInput.Value())

	updated, _ = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "a ", updated.searchInput.Value())

	updated, _ = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "a", updated.searchInput.Value())

	updated, _ = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, entitiesViewList, updated.view)
	assert.Equal(t, "", updated.searchInput.Value())

	updated.view = entitiesViewSearch
	updated.searchInput.SetValue("  alpha ")
	updated, cmd = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewList, updated.view)
	assert.Equal(t, "", updated.searchInput.Value())
	assert.True(t, updated.loading)

	updated.items = []api.Entity{
		{ID: "ent-1", Name: "Alpha"},
		{ID: "", Name: "NoID"},
	}

	updated.toggleBulkSelection(-1)
	assert.Equal(t, 0, updated.bulkCount())

	updated.toggleBulkSelection(1)
	assert.Equal(t, 0, updated.bulkCount())

	updated.toggleBulkSelection(0)
	assert.Equal(t, 1, updated.bulkCount())
	assert.True(t, updated.isBulkSelected(0))
	assert.False(t, updated.isBulkSelected(1))
	assert.False(t, updated.isBulkSelected(9))

	ids := updated.bulkSelectedIDs()
	assert.Equal(t, []string{"ent-1"}, ids)

	updated.toggleBulkSelection(0)
	assert.Equal(t, 0, updated.bulkCount())

	updated.bulkSelected = map[string]bool{"ent-1": true}
	updated.clearBulkSelection()
	assert.Empty(t, updated.bulkSelected)
}

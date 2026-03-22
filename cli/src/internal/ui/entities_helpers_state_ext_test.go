package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesParseCommaSeparatedHelpers(t *testing.T) {
	// Tag normalization is tested via normalizeTag in helpers.
	tags := parseCommaSeparated("alpha, BETA, gamma_tag")
	assert.Equal(t, []string{"alpha", "BETA", "gamma_tag"}, tags)

	tags = parseCommaSeparated("")
	assert.Nil(t, tags)

	tags = parseCommaSeparated("  ,  ,  ")
	assert.Nil(t, tags)

	// Dedup preserves order and removes empty.
	assert.Equal(t, []string{"alpha"}, dedup([]string{"alpha", "alpha"}))
	assert.Equal(t, []string{"a", "b"}, dedup([]string{"a", "b", "a"}))
}

func TestEntitiesSearchInputAndBulkSelectionBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewSearch

	updated, cmd := model.handleSearchInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchBuf)

	updated, _ = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "a ", updated.searchBuf)

	updated, _ = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "a", updated.searchBuf)

	updated, _ = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, entitiesViewList, updated.view)
	assert.Equal(t, "", updated.searchBuf)

	updated.view = entitiesViewSearch
	updated.searchBuf = "  alpha "
	updated, cmd = updated.handleSearchInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewList, updated.view)
	assert.Equal(t, "", updated.searchBuf)
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

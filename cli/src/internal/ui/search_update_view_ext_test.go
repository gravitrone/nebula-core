package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchUpdateIgnoresStaleAndModeMismatchResults(t *testing.T) {
	model := NewSearchModel(nil)
	model.query = "alpha"
	model.mode = searchModeText
	model.loading = true

	updated, cmd := model.Update(searchResultsMsg{
		query:    "beta",
		mode:     searchModeText,
		entities: []api.Entity{{ID: "ent-1", Name: "ignored"}},
	})
	require.Nil(t, cmd)
	assert.True(t, updated.loading)
	assert.Empty(t, updated.items)

	updated, cmd = updated.Update(searchResultsMsg{
		query: "alpha",
		mode:  searchModeSemantic,
		semantic: []api.SemanticSearchResult{
			{Kind: "entity", ID: "ent-1", Title: "ignored", Score: 0.9},
		},
	})
	require.Nil(t, cmd)
	assert.True(t, updated.loading)
	assert.Empty(t, updated.items)
}

func TestSearchUpdateBackspaceAndDeleteSearchBranches(t *testing.T) {
	model := NewSearchModel(nil)
	model.query = "ab"

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	require.NotNil(t, cmd)
	assert.Equal(t, "a", updated.query)
	assert.True(t, updated.loading)

	updated.query = "a"
	updated.loading = true
	updated.items = []searchEntry{{id: "ent-1"}}
	updated.list.SetItems([]string{"ent-1"})

	updated, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyDelete})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.query)
	assert.False(t, updated.loading)
	assert.Empty(t, updated.items)
	assert.Empty(t, updated.list.Items)
}

func TestSearchUpdateTabTogglePaths(t *testing.T) {
	model := NewSearchModel(nil)
	model.mode = searchModeText
	model.loading = true
	model.items = []searchEntry{{id: "ent-1"}}
	model.list.SetItems([]string{"ent-1"})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, searchModeSemantic, updated.mode)
	assert.False(t, updated.loading)
	assert.Empty(t, updated.items)
	assert.Empty(t, updated.list.Items)

	updated.query = "alpha"
	updated, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, cmd)
	assert.Equal(t, searchModeText, updated.mode)
	assert.True(t, updated.loading)
}

func TestSearchUpdateEnterOutOfRangeReturnsNil(t *testing.T) {
	model := NewSearchModel(nil)
	model.items = []searchEntry{{kind: "entity", id: "ent-1"}}
	model.list.SetItems([]string{"ent-1"})
	model.list.Cursor = 5

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, 5, updated.list.Cursor)
}

func TestSearchUpdateArrowNavigation(t *testing.T) {
	model := NewSearchModel(nil)
	model.list.SetItems([]string{"one", "two"})

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.list.Cursor)

	updated, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.list.Cursor)
}

func TestSearchViewLoadingAndNoMatchStates(t *testing.T) {
	model := NewSearchModel(nil)
	model.width = 90
	model.query = "alpha"
	model.loading = true

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Searching...")

	model.loading = false
	model.items = nil
	model.list.SetItems(nil)
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "No matches.")
}

func TestSearchViewRendersPreviewWhenSelectionExists(t *testing.T) {
	model := NewSearchModel(nil)
	model.width = 130
	model.query = "alpha"
	model.items = []searchEntry{
		{
			kind:  "entity",
			id:    "ent-1",
			label: "Alpha Node",
			desc:  "desc",
			entity: &api.Entity{
				Type:   "tool",
				Status: "active",
			},
		},
	}
	model.list.SetItems([]string{"Alpha Node"})

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Alpha Node")
	assert.Contains(t, out, "Search")
}

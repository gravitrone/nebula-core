package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsRenderListLoadingEmptyAndLayoutBranches(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 90
	model.loading = true
	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Loading logs...")

	model.loading = false
	model.items = nil
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "No logs found.")

	now := time.Now().UTC()
	model.items = []api.Log{
		{
			ID:        "log-1",
			LogType:   "",
			Status:    "",
			Value:     api.JSONMap{},
			CreatedAt: now,
		},
		{
			ID:        "log-2",
			LogType:   "deploy",
			Status:    "active",
			Value:     api.JSONMap{"group": map[string]any{"field": "value"}},
			Timestamp: now.Add(time.Minute),
			CreatedAt: now,
		},
	}
	model.list.SetItems([]string{"log-1", "log-2"})
	model.width = 84 // stacked preview layout
	model.searchBuf = "dep"
	model.searchSuggest = "deploy"
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "2 total")
	assert.Contains(t, out, "search: dep")
	assert.Contains(t, out, "next: deploy")
	assert.Contains(t, out, "log")
	assert.Contains(t, out, "Selected")

	model.searchBuf = "deploy"
	model.searchSuggest = "deploy"
	out = components.SanitizeText(model.renderList())
	assert.NotContains(t, out, "next: ")

	model.list.Cursor = -1 // no selected row preview
	out = components.SanitizeText(model.renderList())
	assert.NotContains(t, out, "Selected")

	model.width = 170 // side-by-side preview layout
	model.list.Cursor = 0
	model.modeFocus = true
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Status")
}

func TestLogsHandleListKeysTabBackAndSpaceOutOfRange(t *testing.T) {
	now := time.Now().UTC()
	model := NewLogsModel(nil)
	model.allItems = []api.Log{
		{ID: "log-1", LogType: "workout", Status: "active", CreatedAt: now},
		{ID: "log-2", LogType: "deploy", Status: "planning", CreatedAt: now},
	}
	model.applyLogSearch()
	model.list.SetItems([]string{"workout", "deploy"})

	model.searchBuf = "wo"
	model.searchSuggest = "workout"
	updated, cmd := model.handleListKeys(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "workout", updated.searchBuf)

	updated.searchBuf = "workout"
	updated.searchSuggest = "workout"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "workout", updated.searchBuf)

	updated.searchBuf = "abc"
	updated.searchSuggest = "abc"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)
	assert.Equal(t, "", updated.searchSuggest)

	updated.searchBuf = ""
	updated.searchSuggest = ""
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated.list.Cursor = 9
	updated.view = logsViewList
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)
	assert.Nil(t, updated.detail)

	updated.filtering = true
	updated.searchBuf = "x"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "x", updated.searchBuf)
}

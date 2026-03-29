package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
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
			Content: "",
			CreatedAt: now,
		},
		{
			ID:        "log-2",
			LogType:   "deploy",
			Status:    "active",
			Content: "group field value",
			Timestamp: now.Add(time.Minute),
			CreatedAt: now,
		},
	}
	model.dataTable.SetRows([]table.Row{{"log-1"}, {"log-2"}})
	model.dataTable.SetCursor(0)
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

	model.dataTable.SetRows(nil) // no rows = cursor goes to -1, no selected row preview
	model.dataTable.SetCursor(0)
	out = components.SanitizeText(model.renderList())
	assert.NotContains(t, out, "Selected")

	model.dataTable.SetRows([]table.Row{{"log-1"}, {"log-2"}})
	model.dataTable.SetCursor(0)
	model.width = 170 // side-by-side preview layout
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

	model.searchBuf = "wo"
	model.searchSuggest = "workout"
	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "workout", updated.searchBuf)

	updated.searchBuf = "workout"
	updated.searchSuggest = "workout"
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "workout", updated.searchBuf)

	updated.searchBuf = "abc"
	updated.searchSuggest = "abc"
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)
	assert.Equal(t, "", updated.searchSuggest)

	updated.searchBuf = ""
	updated.searchSuggest = ""
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated.items = nil
	updated.dataTable.SetRows(nil)
	updated.view = logsViewList
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: ' ', Text: " "})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)
	assert.Nil(t, updated.detail)

	updated.filtering = true
	updated.searchBuf = "x"
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "x", updated.searchBuf)
}

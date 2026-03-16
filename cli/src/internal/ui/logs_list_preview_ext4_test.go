package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsRenderPreviewBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	assert.Equal(t, "", model.renderLogPreview(api.Log{}, 0))

	now := time.Now().UTC()
	value := api.JSONMap{"group": map[string]any{"field": "value"}}
	meta := api.JSONMap{"note": "alpha"}
	out := components.SanitizeText(model.renderLogPreview(api.Log{
		ID:        "log-1",
		LogType:   "",
		Status:    "",
		CreatedAt: now,
		Value:     value,
		Metadata:  meta,
	}, 44))
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "log")
	assert.Contains(t, out, "Status: -")
	assert.Contains(t, out, "Value:")
	assert.Contains(t, out, "Meta:")

	ts := now.Add(-time.Hour)
	updated := now.Add(-2 * time.Hour)
	out = components.SanitizeText(model.renderLogPreview(api.Log{
		ID:        "log-2",
		LogType:   "deploy",
		Status:    "active",
		Timestamp: ts,
		UpdatedAt: updated,
		CreatedAt: now,
		Tags:      []string{"ops", "prod"},
	}, 44))
	assert.Contains(t, out, "deploy")
	assert.Contains(t, out, "Status: active")
	assert.Contains(t, out, "Tags: ops, prod")
}

func TestLogsHandleListKeysAdditionalBranches(t *testing.T) {
	now := time.Now().UTC()
	model := NewLogsModel(nil)
	model.allItems = []api.Log{
		{ID: "log-1", LogType: "workout", Status: "active", Timestamp: now},
		{ID: "log-2", LogType: "study", Status: "active", Timestamp: now},
	}
	model.applyLogSearch()
	model.list.SetItems([]string{"workout", "study"})

	model.filtering = true
	updated, cmd := model.handleListKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)

	updated.filtering = false
	updated.searchBuf = "workout"
	updated.searchSuggest = "workout"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)
	assert.Equal(t, "", updated.searchSuggest)

	updated.list.Cursor = 8
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)
	assert.Nil(t, updated.detail)

	updated.list.Cursor = 0
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, logsViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "log-1", updated.detail.ID)

	updated.view = logsViewList
	updated.list.Cursor = 1
	updated.modeFocus = false
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.list.Cursor)
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)
	updated.modeFocus = false
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.list.Cursor)

	updated.searchBuf = "wo"
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "w", updated.searchBuf)
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyDelete})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	require.Nil(t, cmd)
	assert.True(t, updated.filtering)
	updated.filtering = false

	updated.searchBuf = ""
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated.searchBuf = ""
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	require.Nil(t, cmd)
	assert.Equal(t, "w", updated.searchBuf)

	updated.searchBuf = ""
	updated.list.Cursor = 0
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeySpace})
	require.NotNil(t, cmd)
	assert.Equal(t, logsViewDetail, updated.view)
}

func TestLogsHandleDetailAndRenderDetailAdditionalBranches(t *testing.T) {
	now := time.Now().UTC()
	model := NewLogsModel(nil)
	model.width = 88
	model.items = []api.Log{{ID: "log-1", LogType: "build", Status: "active", Timestamp: now}}
	model.list.SetItems([]string{"build"})

	model.view = logsViewDetail
	model.detail = &api.Log{
		ID:        "log-1",
		LogType:   "build",
		Timestamp: now,
		CreatedAt: now,
	}
	out := components.SanitizeText(model.renderDetail())
	assert.NotContains(t, out, "Updated")

	model.detail.UpdatedAt = now.Add(time.Minute)
	model.detail.Status = "active"
	model.detail.Tags = []string{"ci"}
	model.detail.Value = api.JSONMap{"key": "value"}
	model.detail.Metadata = api.JSONMap{"meta": "yes"}
	model.detailRels = []api.Relationship{{ID: "rel-1", Type: "linked", SourceID: "log-1", SourceType: "log", TargetID: "ent-1", TargetType: "entity", Status: "active", CreatedAt: now}}
	out = components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "Updated")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "Value")

	updated, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated, _ = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	assert.True(t, updated.valueExpanded)
	updated, _ = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	assert.True(t, updated.metaExpanded)

	updated, _ = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	assert.Equal(t, logsViewEdit, updated.view)

	updated.view = logsViewDetail
	updated, _ = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, logsViewList, updated.view)
	assert.Nil(t, updated.detail)
	assert.Nil(t, updated.detailRels)
	assert.False(t, updated.valueExpanded)
	assert.False(t, updated.metaExpanded)
}

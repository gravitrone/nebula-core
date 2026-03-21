package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolsHandleListKeysAdditionalBranches(t *testing.T) {
	now := time.Now().UTC()
	model := NewProtocolsModel(nil)
	model.allItems = []api.Protocol{
		{ID: "proto-1", Name: "alpha", Title: "Alpha", Status: "active", CreatedAt: now},
		{ID: "proto-2", Name: "beta", Title: "Beta", Status: "active", CreatedAt: now},
	}
	model.applySearch()
	model.list.SetItems([]string{"alpha", "beta"})

	model.filtering = true
	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)

	updated.filtering = false
	updated.list.Cursor = 5
	updated.view = protocolsViewList
	updated.detail = nil
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, protocolsViewList, updated.view)
	assert.Nil(t, updated.detail)

	updated.searchBuf = ""
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, protocolsViewAdd, updated.view)

	updated.view = protocolsViewList
	updated.searchBuf = ""
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchBuf)
	require.NotEmpty(t, updated.items)
}

func TestProtocolsRenderListFallbackAndPreviewLayout(t *testing.T) {
	now := time.Now().UTC()
	model := NewProtocolsModel(nil)
	model.width = 84 // forces stacked table+preview layout.
	model.searchBuf = "  alpha  "
	model.items = []api.Protocol{
		{
			ID:        "proto-1",
			Name:      " ",
			Title:     "",
			Status:    "",
			CreatedAt: now,
		},
	}
	model.list.SetItems([]string{"placeholder"})
	model.list.Cursor = -1
	model.modeFocus = true

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "search: alpha")
	assert.Contains(t, strings.ToLower(out), "protocol")
	assert.NotContains(t, out, "Selected")

	model.width = 170 // enables side-by-side layout path.
	model.list.Cursor = 0
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Status")
}

func TestProtocolsRenderListLoadingEmptyAndOutOfRangeSelection(t *testing.T) {
	now := time.Now().UTC()
	model := NewProtocolsModel(nil)
	model.width = 120

	model.loading = true
	loading := components.SanitizeText(model.renderList())
	assert.Contains(t, loading, "Loading protocols...")

	model.loading = false
	empty := components.SanitizeText(model.renderList())
	assert.Contains(t, empty, "No protocols found.")

	model.items = []api.Protocol{
		{ID: "proto-1", Name: "alpha", Title: "Alpha", Status: "active", CreatedAt: now},
	}
	// Extra visible item ensures one RelToAbs path exceeds item bounds.
	model.list.SetItems([]string{"alpha", "orphan-row"})
	model.list.Cursor = 1 // selected index is out of range for m.items

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "alpha")
	// No selected preview should render when selected index is out of range.
	assert.NotContains(t, out, "Selected")
}

func TestProtocolsRenderListTinyWidthStillRenders(t *testing.T) {
	now := time.Now().UTC()
	model := NewProtocolsModel(nil)
	model.width = 32
	model.items = []api.Protocol{
		{
			ID:        "proto-1",
			Name:      "alpha",
			Title:     "A",
			Status:    "active",
			CreatedAt: now,
		},
	}
	model.list.SetItems([]string{"alpha"})
	model.list.Cursor = 0

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "At")
}

func TestProtocolsHandleAddKeysAdditionalBranches(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.view = protocolsViewAdd

	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	model.addSaving = false
	model.addMeta.Active = true
	updated, cmd = model.handleAddKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.True(t, updated.addMeta.Active)

	updated, cmd = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.addMeta.Active)

	updated.addFocus = protoFieldStatus
	updated.addStatusIdx = 0
	updated, cmd = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, len(protocolStatusOptions)-1, updated.addStatusIdx)

	updated.addFocus = protoFieldName
	updated.addFields[protoFieldName].value = ""
	updated, cmd = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.addFields[protoFieldName].value)

	updated.addFocus = protoFieldMetadata
	updated, cmd = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.True(t, updated.addMeta.Active)

	updated.addMeta.Active = false
	updated.addFocus = protoFieldName
	updated.addFields[protoFieldName].value = "alpha"
	updated.addFields[protoFieldTitle].value = "Alpha"
	updated.addFields[protoFieldContent].value = "rules"
	updated, cmd = updated.handleAddKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)
}

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestProtocolsHandleModeKeysDownUpBackClearModeFocus(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.modeFocus = true

	updated, _ := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, _ = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, _ = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.modeFocus)
}

func TestProtocolsHandleModeKeysToggleBindings(t *testing.T) {
	keys := []tea.KeyPressMsg{
		{Code: tea.KeyLeft},
		{Code: tea.KeyRight},
		{Code: tea.KeySpace},
		{Code: tea.KeyEnter},
	}
	for _, key := range keys {
		model := NewProtocolsModel(nil)
		model.modeFocus = true
		model.view = protocolsViewList

		updated, _ := model.handleModeKeys(key)
		assert.False(t, updated.modeFocus)
		assert.Equal(t, protocolsViewAdd, updated.view)
	}
}

func TestProtocolsToggleModeSwitchesBetweenListAndAdd(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.view = protocolsViewList
	model.modeFocus = true

	updated, _ := model.toggleMode()
	assert.Equal(t, protocolsViewAdd, updated.view)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, _ = updated.toggleMode()
	assert.Equal(t, protocolsViewList, updated.view)
	assert.False(t, updated.modeFocus)
}

func TestProtocolsRenderModeLineReflectsState(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.view = protocolsViewList
	line := model.renderModeLine()
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")

	model.modeFocus = true
	line = model.renderModeLine()
	assert.Contains(t, line, "Library")

	model.view = protocolsViewAdd
	model.modeFocus = true
	line = model.renderModeLine()
	assert.Contains(t, line, "Add")
}

func TestProtocolsApplySearchByNameAndTitle(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.allItems = []api.Protocol{
		{Name: "alpha", Title: "one"},
		{Name: "beta", Title: "ops policy"},
	}

	model.searchInput.SetValue("ops")
	model.applySearch()
	assert.Len(t, model.items, 1)
	assert.Equal(t, "beta", model.items[0].Name)

	model.searchInput.SetValue("")
	model.applySearch()
	assert.Len(t, model.items, 2)
}

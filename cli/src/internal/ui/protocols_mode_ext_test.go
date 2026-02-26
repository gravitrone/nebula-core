package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestProtocolsHandleModeKeysDownUpBackClearModeFocus(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.modeFocus = true

	updated, _ := model.handleModeKeys(tea.KeyMsg{Type: tea.KeyDown})
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, _ = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, _ = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.modeFocus)
}

func TestProtocolsHandleModeKeysToggleBindings(t *testing.T) {
	keys := []tea.KeyMsg{
		{Type: tea.KeyLeft},
		{Type: tea.KeyRight},
		{Type: tea.KeySpace},
		{Type: tea.KeyEnter},
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

	model.searchBuf = "ops"
	model.applySearch()
	assert.Len(t, model.items, 1)
	assert.Equal(t, "beta", model.items[0].Name)

	model.searchBuf = ""
	model.applySearch()
	assert.Len(t, model.items, 2)
}

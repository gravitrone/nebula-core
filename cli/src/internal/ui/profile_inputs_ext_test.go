package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileHandleAPIKeyInputBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_old"})
	model.editAPIKey = true

	updated, cmd := model.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	require.Nil(t, cmd)
	updated, _ = updated.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	updated, _ = updated.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, "nb ", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, "nb ", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "nb", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeyDelete})
	assert.Equal(t, "n", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.editAPIKey)
	assert.Equal(t, "", updated.apiKeyBuf)
}

func TestProfileHandleAPIKeyInputEnterEmptyReturnsErrMsg(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_old"})
	model.editAPIKey = true
	model.apiKeyBuf = "   "

	updated, cmd := model.handleAPIKeyInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.editAPIKey)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestProfileHandlePendingLimitInputBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_old"})
	model.editPendingLimit = true

	updated, cmd := model.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.pendingLimitBuf)

	updated, _ = updated.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	updated, _ = updated.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	assert.Equal(t, "42", updated.pendingLimitBuf)

	updated, _ = updated.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyDelete})
	assert.Equal(t, "4", updated.pendingLimitBuf)

	updated.pendingLimitBuf = "0"
	updated, cmd = updated.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)

	updated.pendingLimitBuf = "123"
	updated, _ = updated.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.editPendingLimit)
	assert.Equal(t, "", updated.pendingLimitBuf)
}

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

	updated, cmd := model.handleAPIKeyInput(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	updated, _ = updated.handleAPIKeyInput(tea.KeyPressMsg{Code: 'b', Text: "b"})
	updated, _ = updated.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "nb ", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, "nb ", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "nb", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeyDelete})
	assert.Equal(t, "n", updated.apiKeyBuf)

	updated, _ = updated.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.editAPIKey)
	assert.Equal(t, "", updated.apiKeyBuf)
}

func TestProfileHandleAPIKeyInputEnterEmptyReturnsErrMsg(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_old"})
	model.editAPIKey = true
	model.apiKeyBuf = "   "

	updated, cmd := model.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.editAPIKey)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestProfileHandlePendingLimitInputBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_old"})
	model.editPendingLimit = true

	updated, cmd := model.handlePendingLimitInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.pendingLimitBuf)

	updated, _ = updated.handlePendingLimitInput(tea.KeyPressMsg{Code: '4', Text: "4"})
	updated, _ = updated.handlePendingLimitInput(tea.KeyPressMsg{Code: '2', Text: "2"})
	assert.Equal(t, "42", updated.pendingLimitBuf)

	updated, _ = updated.handlePendingLimitInput(tea.KeyPressMsg{Code: tea.KeyDelete})
	assert.Equal(t, "4", updated.pendingLimitBuf)

	updated.pendingLimitBuf = "0"
	updated, cmd = updated.handlePendingLimitInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)

	updated.pendingLimitBuf = "123"
	updated, _ = updated.handlePendingLimitInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.editPendingLimit)
	assert.Equal(t, "", updated.pendingLimitBuf)
}

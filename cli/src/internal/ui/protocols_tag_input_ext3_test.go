package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolsHandleTagInputAdditionalBranchMatrix(t *testing.T) {
	model := NewProtocolsModel(nil)

	// delete/backspace branch in edit mode with existing buffer
	model.editTagInput.SetValue("ab")
	updated, cmd := model.handleTagInput(tea.KeyPressMsg{Code: tea.KeyDelete}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.editTagInput.Value())

	// backspace branch with empty buffer keeps state stable
	updated, cmd = updated.handleTagInput(tea.KeyPressMsg{Code: tea.KeyBackspace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.editTagInput.Value())
	updated, cmd = updated.handleTagInput(tea.KeyPressMsg{Code: tea.KeyBackspace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.editTagInput.Value())

	// comma/space commit branches
	updated.addTagInput.SetValue("Tag-One")
	updated, cmd = updated.handleTagInput(tea.KeyPressMsg{Code: ',', Text: ","}, true)
	require.Nil(t, cmd)
	assert.Equal(t, []string{"tag-one"}, updated.addTags)
	assert.Equal(t, "", updated.addTagInput.Value())

	updated.editTagInput.SetValue("Tag Two")
	updated, cmd = updated.handleTagInput(tea.KeyPressMsg{Code: tea.KeySpace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, []string{"tag-two"}, updated.editTags)
	assert.Equal(t, "", updated.editTagInput.Value())

	// non-printable/no-op branch (len(msg.String()) != 1 and not handled key)
	tagsBefore := updated.addTags
	updated, cmd = updated.handleTagInput(tea.KeyPressMsg{Code: tea.KeyTab}, true)
	require.Nil(t, cmd)
	assert.Equal(t, tagsBefore, updated.addTags)

	// printable branch in edit mode appends rune
	updated, cmd = updated.handleTagInput(tea.KeyPressMsg{Code: 'x', Text: "x"}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "x", updated.editTagInput.Value())
}

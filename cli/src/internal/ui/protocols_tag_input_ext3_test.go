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
	model.editTagBuf = "ab"
	updated, cmd := model.handleTagInput(tea.KeyMsg{Type: tea.KeyDelete}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.editTagBuf)

	// backspace branch with empty buffer keeps state stable
	updated, cmd = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyBackspace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.editTagBuf)
	updated, cmd = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyBackspace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.editTagBuf)

	// comma/space commit branches
	updated.addTagBuf = "Tag-One"
	updated, cmd = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}}, true)
	require.Nil(t, cmd)
	assert.Equal(t, []string{"tag-one"}, updated.addTags)
	assert.Equal(t, "", updated.addTagBuf)

	updated.editTagBuf = "Tag Two"
	updated, cmd = updated.handleTagInput(tea.KeyMsg{Type: tea.KeySpace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, []string{"tag-two"}, updated.editTags)
	assert.Equal(t, "", updated.editTagBuf)

	// non-printable/no-op branch (len(msg.String()) != 1 and not handled key)
	before := updated
	updated, cmd = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyTab}, true)
	require.Nil(t, cmd)
	assert.Equal(t, before, updated)

	// printable branch in edit mode appends rune
	updated, cmd = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "x", updated.editTagBuf)
}

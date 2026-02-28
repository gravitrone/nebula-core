package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextCommitScopeBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.scopes = []string{"public"}

	model.scopeBuf = "   "
	model.commitScope()
	assert.Equal(t, []string{"public"}, model.scopes)
	assert.Equal(t, "", model.scopeBuf)

	model.scopeBuf = "#"
	model.commitScope()
	assert.Equal(t, []string{"public"}, model.scopes)
	assert.Equal(t, "", model.scopeBuf)

	model.scopeBuf = " Public "
	model.commitScope()
	assert.Equal(t, []string{"public"}, model.scopes)
	assert.Equal(t, "", model.scopeBuf)

	model.scopeBuf = "Team Scope"
	model.commitScope()
	assert.Equal(t, []string{"public", "team-scope"}, model.scopes)
	assert.Equal(t, "", model.scopeBuf)
}

func TestContextCommitEditScopeBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.editScopes = []string{"private"}

	model.editScopeBuf = "   "
	model.commitEditScope()
	assert.Equal(t, []string{"private"}, model.editScopes)
	assert.Equal(t, "", model.editScopeBuf)

	model.editScopeBuf = "#"
	model.commitEditScope()
	assert.Equal(t, []string{"private"}, model.editScopes)
	assert.Equal(t, "", model.editScopeBuf)

	model.editScopeBuf = " PRIVATE "
	model.commitEditScope()
	assert.Equal(t, []string{"private"}, model.editScopes)
	assert.Equal(t, "", model.editScopeBuf)

	model.editScopeBuf = "Admin Team"
	model.commitEditScope()
	assert.Equal(t, []string{"private", "admin-team"}, model.editScopes)
	assert.Equal(t, "", model.editScopeBuf)
}

package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextCommitScopeBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.scopes = []string{"public"}

	model.scopeInput.SetValue("   ")
	model.commitScope()
	assert.Equal(t, []string{"public"}, model.scopes)
	assert.Equal(t, "", model.scopeInput.Value())

	model.scopeInput.SetValue("#")
	model.commitScope()
	assert.Equal(t, []string{"public"}, model.scopes)
	assert.Equal(t, "", model.scopeInput.Value())

	model.scopeInput.SetValue(" Public ")
	model.commitScope()
	assert.Equal(t, []string{"public"}, model.scopes)
	assert.Equal(t, "", model.scopeInput.Value())

	model.scopeInput.SetValue("Team Scope")
	model.commitScope()
	assert.Equal(t, []string{"public", "team-scope"}, model.scopes)
	assert.Equal(t, "", model.scopeInput.Value())
}

func TestContextCommitEditScopeBranchMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.editScopes = []string{"private"}

	model.editScopeInput.SetValue("   ")
	model.commitEditScope()
	assert.Equal(t, []string{"private"}, model.editScopes)
	assert.Equal(t, "", model.editScopeInput.Value())

	model.editScopeInput.SetValue("#")
	model.commitEditScope()
	assert.Equal(t, []string{"private"}, model.editScopes)
	assert.Equal(t, "", model.editScopeInput.Value())

	model.editScopeInput.SetValue(" PRIVATE ")
	model.commitEditScope()
	assert.Equal(t, []string{"private"}, model.editScopes)
	assert.Equal(t, "", model.editScopeInput.Value())

	model.editScopeInput.SetValue("Admin Team")
	model.commitEditScope()
	assert.Equal(t, []string{"private", "admin-team"}, model.editScopes)
	assert.Equal(t, "", model.editScopeInput.Value())
}

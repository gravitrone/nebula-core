package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextCommitTagAndToggleModeAdditionalBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.tags = []string{"alpha-tag"}

	model.tagInput.SetValue("   ")
	model.commitTag()
	assert.Equal(t, []string{"alpha-tag"}, model.tags)
	assert.Equal(t, "", model.tagInput.Value())

	model.tagInput.SetValue("#")
	model.commitTag()
	assert.Equal(t, []string{"alpha-tag"}, model.tags)
	assert.Equal(t, "", model.tagInput.Value())

	model.tagInput.SetValue("Beta Tag")
	model.commitTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, model.tags)
	assert.Equal(t, "", model.tagInput.Value())

	model.editTags = []string{"alpha-tag"}
	model.editTagInput.SetValue("#")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())

	model.editTagInput.SetValue("Beta_Tag")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())

	model.view = contextViewDetail
	model.modeFocus = true
	model.detail = &apiContextFixture
	model.contentExpanded = true
	model.sourcePathExpanded = true
	updated, cmd := model.toggleMode()
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Nil(t, updated.detail)
	assert.False(t, updated.contentExpanded)
	assert.False(t, updated.sourcePathExpanded)
	assert.Equal(t, contextViewList, updated.view)

	model.view = contextViewList
	updated, cmd = model.toggleMode()
	require.Nil(t, cmd)
	assert.Equal(t, contextViewAdd, updated.view)
}

func TestEntityAndFileTagScopeCommitAdditionalBranches(t *testing.T) {
	entities := NewEntitiesModel(nil)
	entities.addTags = []string{"alpha-tag"}
	entities.addTagInput.SetValue("#")
	entities.commitAddTag()
	assert.Equal(t, []string{"alpha-tag"}, entities.addTags)
	assert.Equal(t, "", entities.addTagInput.Value())

	entities.addTagInput.SetValue("Beta Tag")
	entities.commitAddTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, entities.addTags)
	assert.Equal(t, "", entities.addTagInput.Value())

	entities.addScopes = []string{"public"}
	entities.addScopeInput.SetValue("#")
	entities.commitAddScope()
	assert.Equal(t, []string{"public"}, entities.addScopes)
	assert.Equal(t, "", entities.addScopeInput.Value())

	entities.addScopeInput.SetValue("Team Scope")
	entities.commitAddScope()
	assert.Equal(t, []string{"public", "team-scope"}, entities.addScopes)
	assert.Equal(t, "", entities.addScopeInput.Value())

	entities.editTags = []string{"alpha-tag"}
	entities.editTagInput.SetValue("#")
	entities.commitEditTag()
	assert.Equal(t, []string{"alpha-tag"}, entities.editTags)
	assert.Equal(t, "", entities.editTagInput.Value())

	entities.editScopeInput.SetValue("#")
	entities.editScopes = []string{"private"}
	entities.editScopesDirty = false
	entities.commitEditScope()
	assert.Equal(t, []string{"private"}, entities.editScopes)
	assert.False(t, entities.editScopesDirty)

	entities.editScopeInput.SetValue("Admin Team")
	entities.commitEditScope()
	assert.Equal(t, []string{"private", "admin-team"}, entities.editScopes)
	assert.True(t, entities.editScopesDirty)

	files := NewFilesModel(nil)
	files.addTags = []string{"alpha-tag"}
	files.addTagInput.SetValue("#")
	files.commitAddTag()
	assert.Equal(t, []string{"alpha-tag"}, files.addTags)
	assert.Equal(t, "", files.addTagInput.Value())

	files.addTagInput.SetValue("Beta Tag")
	files.commitAddTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, files.addTags)
	assert.Equal(t, "", files.addTagInput.Value())

	files.editTags = []string{"alpha-tag"}
	files.editTagInput.SetValue("#")
	files.commitEditTag()
	assert.Equal(t, []string{"alpha-tag"}, files.editTags)
	assert.Equal(t, "", files.editTagInput.Value())

	files.editTagInput.SetValue("Beta Tag")
	files.commitEditTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, files.editTags)
	assert.Equal(t, "", files.editTagInput.Value())
}

var apiContextFixture = apiContextForToggleMode()

func apiContextForToggleMode() api.Context {
	return api.Context{ID: "ctx-1", Title: "Alpha"}
}

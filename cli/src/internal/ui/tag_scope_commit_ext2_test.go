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

	model.tagBuf = "   "
	model.commitTag()
	assert.Equal(t, []string{"alpha-tag"}, model.tags)
	assert.Equal(t, "", model.tagBuf)

	model.tagBuf = "#"
	model.commitTag()
	assert.Equal(t, []string{"alpha-tag"}, model.tags)
	assert.Equal(t, "", model.tagBuf)

	model.tagBuf = "Beta Tag"
	model.commitTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, model.tags)
	assert.Equal(t, "", model.tagBuf)

	model.editTags = []string{"alpha-tag"}
	model.editTagBuf = "#"
	model.commitEditTag()
	assert.Equal(t, []string{"alpha-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)

	model.editTagBuf = "Beta_Tag"
	model.commitEditTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)

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
	entities.addTagBuf = "#"
	entities.commitAddTag()
	assert.Equal(t, []string{"alpha-tag"}, entities.addTags)
	assert.Equal(t, "", entities.addTagBuf)

	entities.addTagBuf = "Beta Tag"
	entities.commitAddTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, entities.addTags)
	assert.Equal(t, "", entities.addTagBuf)

	entities.addScopes = []string{"public"}
	entities.addScopeBuf = "#"
	entities.commitAddScope()
	assert.Equal(t, []string{"public"}, entities.addScopes)
	assert.Equal(t, "", entities.addScopeBuf)

	entities.addScopeBuf = "Team Scope"
	entities.commitAddScope()
	assert.Equal(t, []string{"public", "team-scope"}, entities.addScopes)
	assert.Equal(t, "", entities.addScopeBuf)

	entities.editTags = []string{"alpha-tag"}
	entities.editTagBuf = "#"
	entities.commitEditTag()
	assert.Equal(t, []string{"alpha-tag"}, entities.editTags)
	assert.Equal(t, "", entities.editTagBuf)

	entities.editScopeBuf = "#"
	entities.editScopes = []string{"private"}
	entities.editScopesDirty = false
	entities.commitEditScope()
	assert.Equal(t, []string{"private"}, entities.editScopes)
	assert.False(t, entities.editScopesDirty)

	entities.editScopeBuf = "Admin Team"
	entities.commitEditScope()
	assert.Equal(t, []string{"private", "admin-team"}, entities.editScopes)
	assert.True(t, entities.editScopesDirty)

	files := NewFilesModel(nil)
	files.addTags = []string{"alpha-tag"}
	files.addTagBuf = "#"
	files.commitAddTag()
	assert.Equal(t, []string{"alpha-tag"}, files.addTags)
	assert.Equal(t, "", files.addTagBuf)

	files.addTagBuf = "Beta Tag"
	files.commitAddTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, files.addTags)
	assert.Equal(t, "", files.addTagBuf)

	files.editTags = []string{"alpha-tag"}
	files.editTagBuf = "#"
	files.commitEditTag()
	assert.Equal(t, []string{"alpha-tag"}, files.editTags)
	assert.Equal(t, "", files.editTagBuf)

	files.editTagBuf = "Beta Tag"
	files.commitEditTag()
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, files.editTags)
	assert.Equal(t, "", files.editTagBuf)
}

var apiContextFixture = apiContextForToggleMode()

func apiContextForToggleMode() api.Context {
	return api.Context{ID: "ctx-1", Title: "Alpha"}
}

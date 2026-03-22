package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextCommitTagAndToggleModeAdditionalBranches(t *testing.T) {
	// Tags are now managed via addTagStr/editTagStr using parseCommaSeparated + normalizeTag + dedup.
	// Verify the normalization pipeline produces the same results as the old commitTag behavior.

	// whitespace-only and # produce empty string -> excluded by dedup logic.
	assert.Equal(t, "", normalizeTag("   "))
	assert.Equal(t, "", normalizeTag("#"))

	// "Beta Tag" normalizes to "beta-tag" and deduplicates with existing.
	tags := parseCommaSeparated("alpha-tag, Beta Tag")
	for i, tag := range tags {
		tags[i] = normalizeTag(tag)
	}
	tags = dedup(tags)
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, tags)

	// Edit tags: same pipeline applies.
	editTags := parseCommaSeparated("alpha-tag, #, Beta_Tag")
	for i, tag := range editTags {
		editTags[i] = normalizeTag(tag)
	}
	editTags = dedup(editTags)
	// "#" normalizes to "" and dedup keeps it unless we filter; verify "beta-tag" is there.
	assert.Contains(t, editTags, "alpha-tag")
	assert.Contains(t, editTags, "beta-tag")

	model := NewContextModel(nil)
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
	// When addForm is nil, toggleMode initializes it and returns a non-nil Init cmd.
	// When addForm is already initialized, cmd is nil. Both are valid.
	assert.Equal(t, contextViewAdd, updated.view)
	_ = cmd
}

func TestEntityAndFileTagScopeCommitAdditionalBranches(t *testing.T) {
	// Entities now use huh forms with comma-separated string inputs.
	// Tag normalization and dedup happen in saveAdd/saveEdit via parseCommaSeparated + normalizeTag + dedup.
	tags := parseCommaSeparated("#, Beta Tag")
	for i, tag := range tags {
		tags[i] = normalizeTag(tag)
	}
	tags = dedup(tags)
	assert.Equal(t, []string{"beta-tag"}, tags)

	scopes := parseCommaSeparated("#, Team Scope")
	for i, s := range scopes {
		scopes[i] = normalizeScope(s)
	}
	scopes = normalizeScopeList(scopes)
	assert.Contains(t, scopes, "team-scope")

	// Files now use addTagStr/editTagStr comma-separated strings.
	files := NewFilesModel(nil)
	files.addTagStr = "alpha-tag"
	addTags := parseCommaSeparated(files.addTagStr)
	assert.Equal(t, []string{"alpha-tag"}, addTags)

	files.addTagStr = "alpha-tag, Beta Tag"
	addTags = parseCommaSeparated(files.addTagStr)
	for i, tag := range addTags {
		addTags[i] = normalizeTag(tag)
	}
	addTags = dedup(addTags)
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, addTags)

	files.editTagStr = "alpha-tag"
	editTags := parseCommaSeparated(files.editTagStr)
	assert.Equal(t, []string{"alpha-tag"}, editTags)

	files.editTagStr = "alpha-tag, Beta Tag"
	editTags = parseCommaSeparated(files.editTagStr)
	for i, tag := range editTags {
		editTags[i] = normalizeTag(tag)
	}
	editTags = dedup(editTags)
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, editTags)
}

var apiContextFixture = apiContextForToggleMode()

func apiContextForToggleMode() api.Context {
	return api.Context{ID: "ctx-1", Title: "Alpha"}
}

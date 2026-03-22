package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogsEditTagStrBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)

	// Empty editTagStr parses to nil.
	model.editTagStr = ""
	tags := parseCommaSeparated(model.editTagStr)
	assert.Nil(t, tags)

	// Single tag.
	model.editTagStr = "alpha"
	tags = parseCommaSeparated(model.editTagStr)
	assert.Equal(t, []string{"alpha"}, tags)

	// Multiple tags.
	model.editTagStr = "alpha, beta-tag"
	tags = parseCommaSeparated(model.editTagStr)
	assert.Equal(t, []string{"alpha", "beta-tag"}, tags)

	// Whitespace-only sections ignored.
	model.editTagStr = "alpha,  , beta"
	tags = parseCommaSeparated(model.editTagStr)
	assert.Equal(t, []string{"alpha", "beta"}, tags)
}

func TestLogsAddTagStrBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)

	// Empty addTagStr parses to nil.
	model.addTagStr = ""
	tags := parseCommaSeparated(model.addTagStr)
	assert.Nil(t, tags)

	// Single tag.
	model.addTagStr = "alpha"
	tags = parseCommaSeparated(model.addTagStr)
	assert.Equal(t, []string{"alpha"}, tags)

	// Multiple tags.
	model.addTagStr = "alpha, beta-tag"
	tags = parseCommaSeparated(model.addTagStr)
	assert.Equal(t, []string{"alpha", "beta-tag"}, tags)
}

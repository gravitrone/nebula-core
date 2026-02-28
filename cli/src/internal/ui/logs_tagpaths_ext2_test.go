package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogsCommitEditTagBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)

	model.editTags = []string{"alpha"}
	model.editTagBuf = "   "
	model.commitEditTag()
	assert.Equal(t, []string{"alpha"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)

	model.editTagBuf = "#"
	model.commitEditTag()
	assert.Equal(t, []string{"alpha"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)

	model.editTagBuf = "alpha"
	model.commitEditTag()
	assert.Equal(t, []string{"alpha"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)

	model.editTagBuf = "beta tag"
	model.commitEditTag()
	assert.Equal(t, []string{"alpha", "beta-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)
}

func TestLogsRenderEditTagsBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)

	assert.Equal(t, "-", model.renderEditTags(false))

	model.editTags = []string{"alpha"}
	out := stripANSI(model.renderEditTags(false))
	assert.Contains(t, out, "[alpha]")
	assert.NotContains(t, out, "█")

	model.editTagBuf = "beta"
	out = stripANSI(model.renderEditTags(false))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "beta")
	assert.NotContains(t, out, "█")

	out = stripANSI(model.renderEditTags(true))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "beta")
	assert.Contains(t, out, "█")

	model.editTagBuf = ""
	out = stripANSI(model.renderEditTags(true))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "█")
}

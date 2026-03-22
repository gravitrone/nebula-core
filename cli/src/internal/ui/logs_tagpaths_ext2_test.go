package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogsCommitEditTagBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)

	model.editTags = []string{"alpha"}
	model.editTagInput.SetValue("   ")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())

	model.editTagInput.SetValue("#")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())

	model.editTagInput.SetValue("alpha")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())

	model.editTagInput.SetValue("beta tag")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha", "beta-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())
}

func TestLogsRenderEditTagsBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)

	assert.Equal(t, "-", model.renderEditTags(false))

	model.editTags = []string{"alpha"}
	out := stripANSI(model.renderEditTags(false))
	assert.Contains(t, out, "[alpha]")
	assert.NotContains(t, out, "█")

	model.editTagInput.SetValue("beta")
	out = stripANSI(model.renderEditTags(false))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "beta")
	assert.NotContains(t, out, "█")

	out = stripANSI(model.renderEditTags(true))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "beta")
	assert.Contains(t, out, "█")

	model.editTagInput.SetValue("")
	out = stripANSI(model.renderEditTags(true))
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "█")
}

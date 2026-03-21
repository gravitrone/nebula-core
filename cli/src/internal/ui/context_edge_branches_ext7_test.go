package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextUpdateBackspaceTagBufferBeforeRemovingTags(t *testing.T) {
	model := NewContextModel(nil)
	model.focus = fieldTags
	model.tags = []string{"alpha"}
	model.tagBuf = "xy"

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "x", updated.tagBuf)
	assert.Equal(t, []string{"alpha"}, updated.tags)
}

func TestContextRenderListHandlesNarrowColumnsAndOutOfRangeVisibleRows(t *testing.T) {
	now := time.Now().UTC()
	model := NewContextModel(nil)
	model.width = 24 // force availableCols/titleWidth clamp branches
	model.items = []api.Context{{
		ID:         "ctx-1",
		Title:      "Alpha Context",
		SourceType: "note",
		Status:     "active",
		CreatedAt:  now,
	}}
	// The second visible row has no backing item and should be skipped.
	model.list.SetItems([]string{"alpha", "ghost"})

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Context")
	assert.Contains(t, out, "Alpha Context")
	assert.NotContains(t, out, "ghost")
}

func TestContextRenderDetailFallsBackToListWhenDetailMissing(t *testing.T) {
	now := time.Now().UTC()
	model := NewContextModel(nil)
	model.width = 80
	model.items = []api.Context{{
		ID:         "ctx-1",
		Title:      "Detail Fallback",
		SourceType: "note",
		Status:     "active",
		CreatedAt:  now,
	}}
	model.list.SetItems([]string{formatContextLine(model.items[0])})

	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "Detail Fallback")
}

func TestContextPreviewAndLineUseContentFallbackWhenMetadataMissing(t *testing.T) {
	content := "content fallback branch text"
	item := api.Context{
		ID:      "ctx-1",
		Title:   "Alpha",
		Content: &content,
	}

	model := NewContextModel(nil)
	preview := components.SanitizeText(model.renderContextPreview(item, 48))
	assert.Contains(t, preview, "Preview")
	assert.Contains(t, preview, "content fallback")

	line := components.SanitizeText(formatContextLine(item))
	assert.Contains(t, line, "content fallback")
}

func TestContextRenderTagsSpacingForMultipleValues(t *testing.T) {
	model := NewContextModel(nil)
	model.tags = []string{"alpha", "beta"}

	out := components.SanitizeText(model.renderTags(false))
	assert.Contains(t, out, "[alpha] [beta]")

	model.editTags = []string{"one", "two"}
	out = components.SanitizeText(model.renderEditTags(false))
	assert.Contains(t, out, "[one] [two]")
}

func TestContextHandleLinkSearchMovesCursorWhenListPresent(t *testing.T) {
	model := NewContextModel(nil)
	model.startLinkSearch()
	model.linkResults = []api.Entity{{ID: "ent-1"}, {ID: "ent-2"}}
	model.linkList.SetItems([]string{"ent-1", "ent-2"})

	assert.Equal(t, 0, model.linkList.Selected())

	updated, cmd := model.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.linkList.Selected())

	updated, cmd = updated.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.linkList.Selected())
}

package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextAddTagStrNormalizesViaParseAndNormalize(t *testing.T) {
	model := NewContextModel(nil)
	model.addTagStr = "#Foo Bar, beta_tag, "
	tags := parseCommaSeparated(model.addTagStr)
	for i, t := range tags {
		tags[i] = normalizeTag(t)
	}
	tags = dedup(tags)
	assert.Equal(t, []string{"foo-bar", "beta-tag"}, tags)
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
	model.dataTable.SetRows([]table.Row{{"alpha"}, {"ghost"}})

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
	model.dataTable.SetRows([]table.Row{{formatContextLine(model.items[0])}})

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

func TestContextRenderLinkedEntitiesFallbackAndValues(t *testing.T) {
	model := NewContextModel(nil)

	// No entities: renderLinkedEntities returns empty string.
	out := model.renderLinkedEntities()
	assert.Equal(t, "", out)

	// With entities: rendered as [Name] pills.
	model.linkEntities = []api.Entity{
		{ID: "ent-1", Name: "Alpha"},
		{ID: "ent-2", Name: "Beta"},
	}
	out = components.SanitizeText(model.renderLinkedEntities())
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Beta")
}

func TestContextHandleLinkSearchMovesCursorWhenListPresent(t *testing.T) {
	model := NewContextModel(nil)
	model.startLinkSearch()
	model.linkResults = []api.Entity{{ID: "ent-1"}, {ID: "ent-2"}}
	model.linkTable.SetRows([]table.Row{{"ent-1"}, {"ent-2"}})
	model.linkTable.SetCursor(0)

	assert.Equal(t, 0, model.linkTable.Cursor())

	updated, cmd := model.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.linkTable.Cursor())

	updated, cmd = updated.handleLinkSearch(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.linkTable.Cursor())
}

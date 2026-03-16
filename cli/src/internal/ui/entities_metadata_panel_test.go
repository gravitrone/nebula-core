package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRenderContextSummaryTableEmpty(t *testing.T) {
	out := renderContextSummaryTable(nil, 6, 80)
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "No context items yet.")
}

func TestContextSummaryEntriesDefaults(t *testing.T) {
	entries, extra := contextSummaryEntries([]api.Context{{}}, 3)
	assert.Equal(t, 1, len(entries))
	assert.Equal(t, 0, extra)
	assert.Equal(t, "(untitled)", entries[0].Title)
	assert.Equal(t, "note", entries[0].Type)
	assert.Equal(t, "-", entries[0].Status)
}

func TestRenderContextSummaryTableMoreRow(t *testing.T) {
	items := []api.Context{
		{Title: "Alpha", SourceType: "note", Status: "active"},
		{Title: "Beta", SourceType: "article", Status: "active"},
		{Title: "Gamma", SourceType: "paper", Status: "active"},
		{Title: "Delta", SourceType: "video", Status: "active"},
		{Title: "Epsilon", SourceType: "note", Status: "active"},
	}

	out := renderContextSummaryTable(items, 3, 80)
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "2 more")
	assert.True(t, strings.Contains(clean, "Alpha"))
}

package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestContextSummaryColumnWidthsMinimum(t *testing.T) {
	title, typ, status := contextSummaryColumnWidths(20)
	assert.Equal(t, 12, title)
	assert.Equal(t, 10, typ)
	assert.Equal(t, 8, status)
}

func TestContextSummaryEntriesExtraCount(t *testing.T) {
	items := []api.Context{
		{Title: "one"},
		{Title: "two"},
		{Title: "three"},
		{Title: "four"},
		{Title: "five"},
		{Title: "six"},
		{Title: "seven"},
	}
	entries, extra := contextSummaryEntries(items, 5)
	assert.Equal(t, 5, len(entries))
	assert.Equal(t, 2, extra)
}

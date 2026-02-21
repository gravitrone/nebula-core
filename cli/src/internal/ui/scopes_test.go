package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestScopeSelectedReportsMembership handles test scope selected reports membership.
func TestScopeSelectedReportsMembership(t *testing.T) {
	assert.True(t, scopeSelected([]string{"public", "private"}, "private"))
	assert.False(t, scopeSelected([]string{"public", "private"}, "admin"))
}

// TestRenderScopeOptionsShowsSelectionAndCursor handles test render scope options shows selection and cursor.
func TestRenderScopeOptionsShowsSelectionAndCursor(t *testing.T) {
	out := renderScopeOptions(
		[]string{"private"},
		[]string{"public", "private", "admin"},
		1,
	)
	clean := stripANSI(out)

	assert.Contains(t, clean, "[private]")
	assert.Contains(t, clean, "public")
	assert.Contains(t, clean, "admin")
}

// TestRenderScopeOptionsFallbacksToSelectedWhenOptionsEmpty handles test render scope options fallbacks to selected when options empty.
func TestRenderScopeOptionsFallbacksToSelectedWhenOptionsEmpty(t *testing.T) {
	out := renderScopeOptions([]string{"sensitive"}, nil, 0)
	clean := stripANSI(out)
	assert.Contains(t, clean, "[sensitive]")
}

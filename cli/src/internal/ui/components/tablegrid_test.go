package components

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestFitGridColumnsPrefersShrinkingWideColumns(t *testing.T) {
	columns := []TableColumn{
		{Header: "Rel", Width: 12, Align: lipgloss.Left},
		{Header: "Edge", Width: 42, Align: lipgloss.Left},
		{Header: "Status", Width: 9, Align: lipgloss.Left},
		{Header: "At", Width: 11, Align: lipgloss.Left},
	}

	// Force deficit so at least one column must shrink.
	fitted := fitGridColumns(columns, "|", 56)

	assert.Equal(t, 12, fitted[0].Width, "short system columns should remain stable")
	assert.Less(t, fitted[1].Width, 42, "wide edge column should absorb shrink first")
	assert.Equal(t, 9, fitted[2].Width, "status column should remain readable")
	assert.Equal(t, 11, fitted[3].Width, "time column should remain readable")
}

func TestShrinkColumnsStopsAtMinimums(t *testing.T) {
	columns := []TableColumn{
		{Header: "A", Width: 4},
		{Header: "B", Width: 4},
	}
	remaining := shrinkColumns(columns, []int{4, 4}, 10)
	assert.Equal(t, 10, remaining)
	assert.Equal(t, 4, columns[0].Width)
	assert.Equal(t, 4, columns[1].Width)
}

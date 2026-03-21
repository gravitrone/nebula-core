package components

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestFitGridColumnsNormalizesZeroWidthsAndEmptyHeaderMinimums(t *testing.T) {
	columns := []TableColumn{
		{Header: "", Width: 0, Align: lipgloss.Left},
		{Header: "Type", Width: 0, Align: lipgloss.Left},
	}

	fitted := fitGridColumns(columns, "|", 18)
	assert.Len(t, fitted, 2)
	assert.GreaterOrEqual(t, fitted[0].Width, 1)
	assert.GreaterOrEqual(t, fitted[1].Width, 1)
}

func TestShrinkColumnsNoopBranches(t *testing.T) {
	cols := []TableColumn{{Header: "A", Width: 5}}
	assert.Equal(t, 0, shrinkColumns(cols, []int{2}, 0))
	assert.Equal(t, 0, shrinkColumns(nil, nil, 3))
}

func TestRenderGridRowPadsWhenTableWidthIsWiderThanContent(t *testing.T) {
	columns := []TableColumn{
		{Header: "Name", Width: 6, Align: lipgloss.Left},
	}
	line := renderGridRow(columns, []string{"a"}, "|", 24, false, false)

	assert.Equal(t, 24, lipgloss.Width(line))
	assert.Contains(t, SanitizeText(line), "a")
}

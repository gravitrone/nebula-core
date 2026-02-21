package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

func renderFormGrid(title string, rows [][2]string, activeRow int, width int) string {
	contentWidth := components.BoxContentWidth(width) - 2
	if contentWidth < 48 {
		contentWidth = 48
	}
	fieldWidth := contentWidth * 30 / 100
	if fieldWidth < 16 {
		fieldWidth = 16
	}
	valueWidth := contentWidth - fieldWidth - 1
	if valueWidth < 24 {
		valueWidth = 24
	}

	columns := []components.TableColumn{
		{Header: "Field", Width: fieldWidth, Align: lipgloss.Left},
		{Header: "Value", Width: valueWidth, Align: lipgloss.Left},
	}

	gridRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		value := strings.TrimSpace(row[1])
		if value == "" {
			value = "-"
		}
		value = strings.ReplaceAll(value, "\n", " · ")
		gridRows = append(gridRows, []string{
			components.SanitizeOneLine(row[0]),
			components.SanitizeText(value),
		})
	}

	table := components.TableGridWithActiveRow(columns, gridRows, contentWidth, activeRow)
	return components.TitledBox(title, colorizeScopeBadges(table), width)
}

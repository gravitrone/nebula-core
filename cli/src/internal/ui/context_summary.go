package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type contextSummaryEntry struct {
	Title  string
	Type   string
	Status string
}

// renderContextSummaryTable renders context items summary table.
func renderContextSummaryTable(items []api.Context, maxRows, width int) string {
	entries, extra := contextSummaryEntries(items, maxRows)
	if len(entries) == 0 && extra == 0 {
		return components.TitledBox("Context Items", "No context items yet.", width)
	}

	contentWidth := components.BoxContentWidth(width) - 2
	if contentWidth < 32 {
		contentWidth = 32
	}
	titleWidth, typeWidth, statusWidth := contextSummaryColumnWidths(contentWidth)
	columns := []components.TableColumn{
		{Header: "Title", Width: titleWidth, Align: lipgloss.Left},
		{Header: "Type", Width: typeWidth, Align: lipgloss.Left},
		{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
	}

	gridRows := make([][]string, 0, len(entries)+1)
	for _, entry := range entries {
		gridRows = append(gridRows, []string{
			entry.Title,
			entry.Type,
			entry.Status,
		})
	}
	if extra > 0 {
		gridRows = append(gridRows, []string{
			"More",
			"+",
			fmt.Sprintf("%d more context items", extra),
		})
	}

	content := components.TableGrid(columns, gridRows, contentWidth)
	return components.TitledBox("Context Items", content, width)
}

func contextSummaryEntries(items []api.Context, maxRows int) ([]contextSummaryEntry, int) {
	if maxRows <= 0 {
		maxRows = 5
	}
	entries := make([]contextSummaryEntry, 0, maxRows)
	for i, item := range items {
		if i >= maxRows {
			break
		}
		title := strings.TrimSpace(components.SanitizeOneLine(contextTitle(item)))
		if title == "" {
			title = "(untitled)"
		}
		kind := strings.TrimSpace(components.SanitizeOneLine(item.SourceType))
		if kind == "" {
			kind = "note"
		}
		status := strings.TrimSpace(components.SanitizeOneLine(item.Status))
		if status == "" {
			status = "-"
		}
		entries = append(entries, contextSummaryEntry{
			Title:  title,
			Type:   kind,
			Status: status,
		})
	}
	extra := len(items) - maxRows
	if extra < 0 {
		extra = 0
	}
	return entries, extra
}

func contextSummaryColumnWidths(contentWidth int) (int, int, int) {
	if contentWidth < 32 {
		contentWidth = 32
	}
	usable := contentWidth - 2

	title := usable * 62 / 100
	typ := usable * 20 / 100
	status := usable - title - typ

	if typ < 10 {
		typ = 10
		status = usable - title - typ
	}
	if status < 8 {
		status = 8
		title = usable - typ - status
	}
	return title, typ, status
}

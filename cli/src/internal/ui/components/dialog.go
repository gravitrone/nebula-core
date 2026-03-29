package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

var dialogStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#273540")).
	Padding(1, 2).
	Width(40)

// ConfirmDialog renders a yes/no confirmation.
func ConfirmDialog(title, message string) string {
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7f57b4")).
		Bold(true).
		Render(title)

	body := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ba0bf")).
		Render(message)

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ba0bf")).
		Render("\nenter: confirm | esc: cancel | y/n: alias")

	return dialogStyle.Render(header + "\n\n" + body + hint)
}

// InputDialog renders a text input prompt.
func InputDialog(title, input string) string {
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7f57b4")).
		Bold(true).
		Render(title)

	field := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#436b77")).
		Render("> " + input + "█")

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ba0bf")).
		Render("\nenter: submit | esc: cancel")

	return dialogStyle.Render(header + "\n\n" + field + hint)
}

// TextInputDialog renders a dialog with a bubbles textinput view.
func TextInputDialog(title, inputView string) string {
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7f57b4")).
		Bold(true).
		Render(title)

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ba0bf")).
		Render("\nenter: submit | esc: cancel")

	return dialogStyle.Render(header + "\n\n" + inputView + hint)
}

// ConfirmPreviewDialog renders a confirmation with summary rows and optional diffs.
func ConfirmPreviewDialog(title string, summary []TableRow, diffs []DiffRow, width int) string {
	sections := make([]string, 0, 4)
	if len(summary) > 0 {
		sections = append(
			sections,
			boxHeaderStyle.Render("Summary"),
			renderSummaryRows(summary, width),
		)
	}
	if len(diffs) > 0 {
		sections = append(
			sections,
			boxHeaderStyle.Render("Changes"),
			renderDiffRows(diffs, width),
		)
	}

	return TitledBox(title, strings.Join(sections, "\n\n"), width)
}

// renderSummaryRows renders render summary rows.
func renderSummaryRows(rows []TableRow, width int) string {
	if len(rows) == 0 {
		return ""
	}

	maxLabel := 0
	safeRows := make([]TableRow, len(rows))
	for i, row := range rows {
		safeRows[i] = TableRow{
			Label: SanitizeOneLine(row.Label),
			Value: SanitizeOneLine(row.Value),
		}
		if lw := lipgloss.Width(safeRows[i].Label); lw > maxLabel {
			maxLabel = lw
		}
	}

	contentWidth := BoxContentWidth(width)
	if contentWidth <= 0 {
		contentWidth = maxLabel + 8
	}

	labelWidth := maxLabel
	if labelWidth > 24 {
		labelWidth = 24
	}
	if contentWidth > 0 {
		maxLabelWidth := contentWidth / 2
		if maxLabelWidth < 8 {
			maxLabelWidth = contentWidth
		}
		if labelWidth > maxLabelWidth {
			labelWidth = maxLabelWidth
		}
	}
	if labelWidth < 4 {
		labelWidth = maxLabel
	}
	valueWidth := contentWidth - labelWidth - 2
	if valueWidth < 4 {
		valueWidth = 4
	}

	var b strings.Builder
	for i, row := range safeRows {
		label := boxLabelStyle.Render(padRight(ClampTextWidth(row.Label, labelWidth), labelWidth))
		value := boxValueStyle.Render(ClampTextWidth(row.Value, valueWidth))
		b.WriteString(label + "  " + value)
		if i < len(safeRows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderDiffRows renders render diff rows.
func renderDiffRows(rows []DiffRow, width int) string {
	lines := DiffRowsToLines(rows)
	w := BoxContentWidth(width)
	if w <= 0 {
		w = 60
	}
	return RenderDiffView(lines, w)
}

// renderDiffValue is kept for test/backward compatibility with prefix-based diff formatting.
func renderDiffValue(style lipgloss.Style, prefix, value string, valueWidth int) string {
	lines := wrapDiffCellValue(value, maxInt(4, valueWidth))
	if len(lines) == 0 {
		lines = []string{"None"}
	}
	pad := strings.Repeat(" ", lipgloss.Width(prefix))

	var out strings.Builder
	for i, line := range lines {
		leader := pad
		if i == 0 {
			leader = prefix
		}
		out.WriteString(style.Render(leader + line))
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	return out.String()
}

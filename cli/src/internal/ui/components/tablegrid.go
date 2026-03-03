package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TableColumn defines a single column for TableGrid.
//
// Width is the visual width of the column content (excluding separators).
// Align controls how cell text is aligned within the column.
type TableColumn struct {
	Header string
	Width  int
	Align  lipgloss.Position
}

const (
	tableGridInset      = 0
	tableGridLeftOffset = 2
)

var gridLineStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#273540"))

var gridActiveRowStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#eef2ff")).
	Background(lipgloss.Color("#2a3348")).
	Bold(true)

var gridActiveSepStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#38506b")).
	Background(lipgloss.Color("#2a3348"))

var gridSelectedMarkStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#d1606b")).
	Bold(true)

var tableGridActiveRowsEnabled = true

// SetTableGridActiveRowsEnabled toggles active-row highlighting globally.
// App view rendering uses this to hide row focus while top tab navigation
// is focused, so only one focus target is visible at a time.
func SetTableGridActiveRowsEnabled(enabled bool) {
	tableGridActiveRowsEnabled = enabled
}

// TableGrid renders a table-like layout using the same rounded border glyphs
// used by Nebula's box components.
//
// The returned string has a visual width equal to tableWidth.
// Callers should pass a tableWidth that fits inside a box content area
// (typically components.BoxContentWidth(termWidth)).
func TableGrid(columns []TableColumn, rows [][]string, tableWidth int) string {
	return TableGridWithActiveRow(columns, rows, tableWidth, -1)
}

// TableGridWithActiveRow is like TableGrid but highlights one data row by index.
// activeRow is a 0-based index into rows; pass -1 to disable highlighting.
func TableGridWithActiveRow(columns []TableColumn, rows [][]string, tableWidth int, activeRow int) string {
	if !tableGridActiveRowsEnabled {
		activeRow = -1
	}
	if tableWidth <= 0 {
		return ""
	}
	if len(columns) == 0 {
		return padRight("", tableWidth)
	}

	border := lipgloss.RoundedBorder()
	v := border.Left
	h := border.Top
	cross := border.Middle

	cols := fitGridColumns(columns, v, tableWidth)

	// Build header and row lines.
	var out []string
	out = append(out, renderGridRow(cols, headerCells(cols), v, tableWidth, true, false))
	out = append(out, renderGridRule(cols, cross, h, tableWidth))

	for i, row := range rows {
		out = append(out, renderGridRow(cols, row, v, tableWidth, false, i == activeRow))
	}

	return strings.Join(out, "\n")
}

// headerCells handles header cells.
func headerCells(columns []TableColumn) []string {
	hdr := make([]string, len(columns))
	for i, c := range columns {
		hdr[i] = SanitizeOneLine(c.Header)
	}
	return hdr
}

// fitGridColumns handles fit grid columns.
func fitGridColumns(columns []TableColumn, sep string, tableWidth int) []TableColumn {
	fitted := make([]TableColumn, len(columns))
	copy(fitted, columns)

	sepW := lipgloss.Width(sep)
	if sepW < 1 {
		sepW = 1
	}
	contentWidth := tableWidth - tableGridLeftOffset - (tableGridInset * 2)
	if contentWidth < len(fitted) {
		contentWidth = len(fitted)
	}

	sum := 0
	for i := range fitted {
		if fitted[i].Width < 1 {
			fitted[i].Width = 1
		}
		sum += fitted[i].Width
	}
	// n columns => n-1 separators (only between columns, no outer table border).
	expected := sum
	if len(fitted) > 1 {
		expected += (len(fitted) - 1) * sepW
	}
	delta := contentWidth - expected
	if len(fitted) > 0 && delta > 0 {
		// Grow the widest column to absorb extra room.
		widest := 0
		for i := 1; i < len(fitted); i++ {
			if fitted[i].Width > fitted[widest].Width {
				widest = i
			}
		}
		fitted[widest].Width += delta
	} else if len(fitted) > 0 && delta < 0 {
		// Shrink wide columns first to avoid clipping narrow/system columns
		// like status/time that should remain readable.
		deficit := -delta
		minWidthLoose := make([]int, len(fitted))
		minWidthStrict := make([]int, len(fitted))
		for i := range fitted {
			headerMin := lipgloss.Width(SanitizeOneLine(fitted[i].Header)) + 1
			if headerMin < 2 {
				headerMin = 2
			}
			minWidthLoose[i] = headerMin
			minWidthStrict[i] = headerMin
			// Preserve compact system columns (for example: status/time) whenever possible.
			if fitted[i].Width <= 12 && fitted[i].Width > minWidthStrict[i] {
				minWidthStrict[i] = fitted[i].Width
			}
			// Keep timestamp-like columns readable and shift truncation pressure
			// to wider descriptive columns first.
			if isPinnedSystemHeader(fitted[i].Header) && fitted[i].Width > minWidthStrict[i] {
				minWidthStrict[i] = fitted[i].Width
			}
		}
		deficit = shrinkColumns(fitted, minWidthStrict, deficit)
		if deficit > 0 {
			shrinkColumns(fitted, minWidthLoose, deficit)
		}
	}
	return fitted
}

// isPinnedSystemHeader handles is pinned system header.
func isPinnedSystemHeader(header string) bool {
	switch strings.ToLower(strings.TrimSpace(header)) {
	case "at", "last", "created", "updated", "timestamp", "when":
		return true
	default:
		return false
	}
}

// shrinkColumns handles shrink columns.
func shrinkColumns(columns []TableColumn, mins []int, deficit int) int {
	if deficit <= 0 || len(columns) == 0 {
		return 0
	}
	for deficit > 0 {
		best := -1
		bestSpare := 0
		for i := range columns {
			spare := columns[i].Width - mins[i]
			if spare > bestSpare {
				bestSpare = spare
				best = i
			}
		}
		if best == -1 || bestSpare <= 0 {
			break
		}
		columns[best].Width--
		deficit--
	}
	return deficit
}

// renderGridRow renders render grid row.
func renderGridRow(columns []TableColumn, cells []string, sep string, tableWidth int, header bool, active bool) string {
	headerStyle := boxLabelStyle
	if header {
		headerStyle = boxLabelStyle.Bold(true)
	}

	sepStyle := gridLineStyle
	cellStyle := lipgloss.NewStyle()
	if active {
		sepStyle = gridActiveSepStyle
		cellStyle = gridActiveRowStyle
	}
	sepStyled := sepStyle.Inline(true).Render(sep)

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", tableGridLeftOffset+tableGridInset))
	for i, col := range columns {
		if i > 0 {
			b.WriteString(sepStyled)
		}
		w := col.Width
		text := ""
		if i < len(cells) {
			text = cells[i]
		}

		rendered := renderGridCell(text, w, col.Align)
		if header {
			// Inline keeps this cell as exactly one rendered line.
			rendered = headerStyle.Inline(true).Render(rendered)
		} else if active {
			rendered = cellStyle.Inline(true).Render(rendered)
		}
		if !header {
			rendered = highlightSelectionMarkers(rendered)
		}
		b.WriteString(rendered)
	}

	line := b.String()
	if lipgloss.Width(line) < tableWidth {
		line = padRight(line, tableWidth)
	}
	return line
}

// renderGridRule renders render grid rule.
func renderGridRule(columns []TableColumn, cross, horiz string, tableWidth int) string {
	if horiz == "" {
		horiz = "-"
	}
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", tableGridLeftOffset+tableGridInset))
	for i, col := range columns {
		w := col.Width
		if w < 1 {
			w = 1
		}
		b.WriteString(strings.Repeat(horiz, w))
		if i < len(columns)-1 {
			b.WriteString(cross)
		}
	}
	line := b.String()
	if lipgloss.Width(line) < tableWidth {
		line = padRight(line, tableWidth)
	}
	return gridLineStyle.Inline(true).Render(line)
}

// renderGridCell renders render grid cell.
func renderGridCell(text string, width int, align lipgloss.Position) string {
	if width <= 0 {
		return ""
	}

	clamped := ClampTextWidthEllipsis(text, width)
	w := lipgloss.Width(clamped)
	if w >= width {
		return truncateRunes(clamped, width)
	}

	pad := width - w
	switch align {
	case lipgloss.Right:
		return strings.Repeat(" ", pad) + clamped
	case lipgloss.Center:
		left := pad / 2
		right := pad - left
		return strings.Repeat(" ", left) + clamped + strings.Repeat(" ", right)
	default:
		return clamped + strings.Repeat(" ", pad)
	}
}

// highlightSelectionMarkers handles highlight selection markers.
func highlightSelectionMarkers(value string) string {
	highlighted := strings.ReplaceAll(value, "[X]", gridSelectedMarkStyle.Render("[X]"))
	highlighted = strings.ReplaceAll(highlighted, "[x]", gridSelectedMarkStyle.Render("[x]"))
	return highlighted
}

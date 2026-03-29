package components

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// Nebula theme colors (mirrored from styles.go for components package use).
var (
	themePrimary    = lipgloss.Color("#7f57b4")
	themeMuted      = lipgloss.Color("#9ba0bf")
	themeText       = lipgloss.Color("#d7d9da")
	themeBorder     = lipgloss.Color("#273540")
	themeCursorLine = lipgloss.Color("#2a3348")
)

// NewNebulaTextInput returns a textinput.Model styled to match the Nebula theme.
// The input is focused by default so it immediately accepts key events.
func NewNebulaTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()

	styles := textinput.DefaultDarkStyles()
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(themeMuted)
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(themePrimary)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(themeText)
	styles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(themeMuted)
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(themeMuted)
	styles.Cursor.Color = themePrimary
	ti.SetStyles(styles)

	return ti
}

// NewNebulaSpinner returns a spinner.Model styled to match the Nebula theme.
func NewNebulaSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(themePrimary)
	return s
}

// TableBaseStyle wraps a table.View() in a bordered box matching the
// bubbles table example pattern with nebula border color.
var TableBaseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(themeBorder)

// NewNebulaTable returns a table.Model styled like the charmbracelet
// bubbles table example with nebula theme colors. Pattern:
//   - DefaultStyles() as base (Cell has Padding(0,1))
//   - Header: bottom border separator, not bold
//   - Selected: nebula purple background, white foreground, full-width
//   - Keybindings disabled (nebula tabs handle their own nav)
//
// When cols is nil a single placeholder column is used so that SetRows
// does not panic before the caller sets proper columns.
func NewNebulaTable(cols []table.Column, height int) table.Model {
	if cols == nil {
		cols = []table.Column{{Title: "", Width: 40}}
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeBorder).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("#ffffff")).
		Background(themePrimary).
		Bold(false)

	t := table.New(
		table.WithColumns(cols),
		table.WithHeight(height),
		table.WithStyles(s),
		table.WithFocused(true),
	)
	return t
}

// InfoTableRow is a key-value pair for RenderInfoTable.
type InfoTableRow struct {
	Key   string
	Value string
}

// RenderInfoTable renders key-value pairs as a read-only 2-column bubbles table
// with no selection highlight. This replaces the old components.Table() pattern.
func RenderInfoTable(rows []InfoTableRow, width int) string {
	if len(rows) == 0 || width <= 0 {
		return ""
	}

	// Subtract border overhead from TableBaseStyle (2 for left+right border).
	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	keyWidth := 0
	for _, r := range rows {
		w := lipgloss.Width(SanitizeOneLine(r.Key))
		if w > keyWidth {
			keyWidth = w
		}
	}
	if keyWidth > 24 {
		keyWidth = 24
	}
	if keyWidth < 6 {
		keyWidth = 6
	}

	valWidth := innerWidth - keyWidth - (2 * 2) // 2 columns * 2 cell padding
	if valWidth < 10 {
		valWidth = 10
	}

	tableRows := make([]table.Row, len(rows))
	for i, r := range rows {
		tableRows[i] = table.Row{
			ClampTextWidthEllipsis(SanitizeOneLine(r.Key), keyWidth),
			ClampTextWidthEllipsis(SanitizeOneLine(r.Value), valWidth),
		}
	}

	cols := []table.Column{
		{Title: "Field", Width: keyWidth},
		{Title: "Value", Width: valWidth},
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeBorder).
		BorderBottom(true).
		Bold(false)
	s.Selected = lipgloss.NewStyle()

	actualW := keyWidth + valWidth + (2 * 2)
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(tableRows),
		table.WithHeight(len(rows)+1),
		table.WithWidth(actualW),
		table.WithStyles(s),
	)
	t.Blur()

	return TableBaseStyle.Render(t.View())
}

// RenderDiffInfoTable renders a before/after diff as a read-only 4-column bubbles table.
// This replaces the old components.DiffTable() pattern.
func RenderDiffInfoTable(rows []DiffRow, width int) string {
	if len(rows) == 0 || width <= 0 {
		return ""
	}

	innerWidth := width - 2
	if innerWidth < 40 {
		innerWidth = 40
	}

	fieldWidth := innerWidth / 6
	if fieldWidth < 8 {
		fieldWidth = 8
	}
	if fieldWidth > 20 {
		fieldWidth = 20
	}
	changeWidth := 9
	remaining := innerWidth - fieldWidth - changeWidth - (4 * 2) // 4 columns * 2 padding
	valWidth := remaining / 2
	if valWidth < 8 {
		valWidth = 8
	}

	tableRows := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		from := SanitizeOneLine(r.From)
		to := SanitizeOneLine(r.To)
		kind := diffChangeKind(from, to)
		tableRows = append(tableRows, table.Row{
			ClampTextWidthEllipsis(SanitizeOneLine(r.Label), fieldWidth),
			kind,
			ClampTextWidthEllipsis(from, valWidth),
			ClampTextWidthEllipsis(to, valWidth),
		})
	}

	cols := []table.Column{
		{Title: "Field", Width: fieldWidth},
		{Title: "Change", Width: changeWidth},
		{Title: "Before", Width: valWidth},
		{Title: "After", Width: valWidth},
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeBorder).
		BorderBottom(true).
		Bold(false)
	s.Selected = lipgloss.NewStyle()

	actualW := fieldWidth + changeWidth + (2 * valWidth) + (4 * 2)
	t := table.New(
		table.WithColumns(cols),
		table.WithRows(tableRows),
		table.WithHeight(len(rows)+1),
		table.WithWidth(actualW),
		table.WithStyles(s),
	)
	t.Blur()

	return TableBaseStyle.Render(t.View())
}

// RenderGridTable renders a multi-column read-only bubbles table from column
// definitions and string rows. This replaces the old TableGrid() pattern.
func RenderGridTable(columns []TableColumn, rows [][]string, width int) string {
	if len(columns) == 0 || width <= 0 {
		return ""
	}

	cols := make([]table.Column, len(columns))
	for i, c := range columns {
		cols[i] = table.Column{Title: c.Header, Width: c.Width}
	}

	tableRows := make([]table.Row, len(rows))
	for i, row := range rows {
		cells := make(table.Row, len(columns))
		for j := range columns {
			if j < len(row) {
				cells[j] = ClampTextWidthEllipsis(SanitizeOneLine(row[j]), columns[j].Width)
			}
		}
		tableRows[i] = cells
	}

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(themeBorder).
		BorderBottom(true).
		Bold(false)
	s.Selected = lipgloss.NewStyle()

	actualW := 0
	for _, c := range columns {
		actualW += c.Width + 2 // cell padding
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(tableRows),
		table.WithHeight(len(rows)+1),
		table.WithWidth(actualW),
		table.WithStyles(s),
	)
	t.Blur()

	return t.View()
}

// RenderCompactBox renders content inside a compact bordered box (width 66).
// This replaces the old components.Box() pattern for loading/empty states.
func RenderCompactBox(content string) string {
	return renderBox(boxBorder, 66, content)
}

// NewNebulaViewport returns a viewport.Model with the given dimensions.
func NewNebulaViewport(width, height int) viewport.Model {
	return viewport.New(
		viewport.WithWidth(width),
		viewport.WithHeight(height),
	)
}

// NewNebulaTextarea returns a textarea.Model styled to match the Nebula theme.
// The textarea uses rounded borders, line numbers, and no character limit.
func NewNebulaTextarea(width, height int) textarea.Model {
	ta := textarea.New()
	ta.SetWidth(width)
	ta.SetHeight(height)
	ta.ShowLineNumbers = true
	ta.CharLimit = 0

	styles := textarea.DefaultDarkStyles()

	// Focused state: primary border, highlighted cursor line, muted line numbers.
	styles.Focused.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(themePrimary).
		Padding(0, 1)
	styles.Focused.Text = lipgloss.NewStyle().Foreground(themeText)
	styles.Focused.CursorLine = lipgloss.NewStyle().Background(themeCursorLine)
	styles.Focused.LineNumber = lipgloss.NewStyle().Foreground(themeMuted)
	styles.Focused.CursorLineNumber = lipgloss.NewStyle().Foreground(themePrimary)
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(themeMuted)
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(themePrimary)
	styles.Focused.EndOfBuffer = lipgloss.NewStyle().Foreground(themeMuted)

	// Blurred state: dim border, muted text.
	styles.Blurred.Base = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(themeBorder).
		Padding(0, 1)
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(themeMuted)
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	styles.Blurred.LineNumber = lipgloss.NewStyle().Foreground(themeBorder)
	styles.Blurred.CursorLineNumber = lipgloss.NewStyle().Foreground(themeBorder)
	styles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(themeBorder)
	styles.Blurred.Prompt = lipgloss.NewStyle().Foreground(themeBorder)
	styles.Blurred.EndOfBuffer = lipgloss.NewStyle().Foreground(themeBorder)

	styles.Cursor.Color = themePrimary

	ta.SetStyles(styles)

	return ta
}

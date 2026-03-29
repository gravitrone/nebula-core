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
// bubbles table example pattern (NormalBorder + visible gray border).
var TableBaseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

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
		BorderForeground(lipgloss.Color("240")).
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

package components

import (
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

// Nebula theme colors (mirrored from styles.go for components package use).
var (
	themePrimary = lipgloss.Color("#7f57b4")
	themeMuted   = lipgloss.Color("#9ba0bf")
	themeText    = lipgloss.Color("#d7d9da")
)

// NewNebulaTextInput returns a textinput.Model styled to match the Nebula theme.
func NewNebulaTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder

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

// NewNebulaTable returns a table.Model styled to match the Nebula theme.
// Keybindings are disabled by default since nebula tabs handle their own nav.
// When cols is nil a single placeholder column is used so that SetRows
// does not panic before the caller sets proper columns.
func NewNebulaTable(cols []table.Column, height int) table.Model {
	if cols == nil {
		cols = []table.Column{{Title: "", Width: 40}}
	}
	s := table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(themeText).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Foreground(themeText).
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#eef2ff")).
			Background(lipgloss.Color("#2a3348")).
			Padding(0, 1),
	}
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

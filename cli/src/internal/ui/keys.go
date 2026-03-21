package ui

import (
	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
)

// --- Key Map ---

// KeyMap defines all keybindings for the nebula TUI.
type KeyMap struct {
	Quit       key.Binding
	Help       key.Binding
	Command    key.Binding
	Tabs       key.Binding
	TabLeft    key.Binding
	TabRight   key.Binding
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Back       key.Binding
	Space      key.Binding
	ScrollUp   key.Binding
	ScrollDown key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Command: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "command"),
		),
		Tabs: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9", "0"),
			key.WithHelp("1-9/0", "tabs"),
		),
		TabLeft: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "prev tab"),
		),
		TabRight: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "next tab"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "toggle"),
		),
		ScrollUp: key.NewBinding(
			key.WithKeys("ctrl+u", "pgup"),
			key.WithHelp("ctrl+u", "scroll up"),
		),
		ScrollDown: key.NewBinding(
			key.WithKeys("ctrl+d", "pgdown"),
			key.WithHelp("ctrl+d", "scroll down"),
		),
	}
}

// ShortHelp returns keybindings for the short help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit, k.Command, k.Enter, k.Back}
}

// FullHelp returns keybindings for the full help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Back},
		{k.Tabs, k.TabLeft, k.TabRight, k.Command},
		{k.ScrollUp, k.ScrollDown, k.Space, k.Help, k.Quit},
	}
}

// newHelpModel creates a help.Model styled to match nebula's theme.
func newHelpModel() help.Model {
	h := help.New()
	h.Styles = help.Styles{
		ShortKey:       lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true),
		ShortDesc:      lipgloss.NewStyle().Foreground(ColorMuted),
		ShortSeparator: lipgloss.NewStyle().Foreground(ColorBorder),
		FullKey:        lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true),
		FullDesc:       lipgloss.NewStyle().Foreground(ColorMuted),
		FullSeparator:  lipgloss.NewStyle().Foreground(ColorBorder),
		Ellipsis:       lipgloss.NewStyle().Foreground(ColorMuted),
	}
	return h
}

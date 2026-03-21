package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// --- Theme Colors ---

var (
	ColorPrimary    = lipgloss.Color("#7f57b4") // purple
	ColorSecondary  = lipgloss.Color("#436b77") // teal
	ColorAccent     = lipgloss.Color("#a7754e") // warm
	ColorBackground = lipgloss.Color("#16161d") // dark
	ColorText       = lipgloss.Color("#d7d9da") // main text
	ColorMuted      = lipgloss.Color("#9ba0bf") // muted text
	ColorSuccess    = lipgloss.Color("#3f866b") // green
	ColorError      = lipgloss.Color("#6d424b") // red
	ColorWarning    = lipgloss.Color("#c78854") // warning
	ColorBorder     = lipgloss.Color("#273540") // border
	ColorBlue       = lipgloss.Color("#436b77") // blue-teal
)

// --- Reusable Styles ---

var (
	BannerStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	BannerAccentStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	TabActiveStyle = lipgloss.NewStyle().
			Foreground(ColorBackground).
			Background(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	TabFocusStyle = lipgloss.NewStyle().
			Foreground(ColorBackground).
			Background(lipgloss.Color("#9972cf")).
			Bold(true).
			Padding(0, 1)

	TabInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorMuted).
				Padding(0, 1)

	TabCurrentStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)

	TabSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Bold(true).
				Padding(0, 1)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			PaddingTop(1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#c79bff")).
			Bold(true)

	NormalStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	AccentStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)

	BlueStyle = lipgloss.NewStyle().
			Foreground(ColorBlue)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			PaddingBottom(1)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	TypeBadgeStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Border(lipgloss.NormalBorder()).
			BorderForeground(ColorBorder).
			BorderTop(false).
			BorderBottom(false).
			Bold(true).
			Padding(0, 1)

	DividerStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	MetaKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	MetaValueStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	MetaPunctStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)
)

// Divider returns a horizontal line.
func Divider(width int) string {
	if width <= 0 {
		return ""
	}
	return DividerStyle.Render(strings.Repeat("─", width))
}

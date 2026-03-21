package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const bannerArt = `
 ██████   █████ ██████████ ███████████  █████  █████ █████        █████████
░░██████ ░░███ ░░███░░░░░█░░███░░░░░███░░███  ░░███ ░░███        ███░░░░░███
 ░███░███ ░███  ░███  █ ░  ░███    ░███ ░███   ░███  ░███       ░███    ░███
 ░███░░███░███  ░██████    ░██████████  ░███   ░███  ░███       ░███████████
 ░███ ░░██████  ░███░░█    ░███░░░░░███ ░███   ░███  ░███       ░███░░░░░███
 ░███  ░░█████  ░███ ░   █ ░███    ░███ ░███   ░███  ░███      █░███    ░███
 █████  ░░█████ ██████████ ███████████  ░░████████   ███████████░███    ░███
░░░░░    ░░░░░ ░░░░░░░░░░ ░░░░░░░░░░░   ░░░░░░░░   ░░░░░░░░░░░░░░░    ░░░`

// RenderBanner returns the styled ASCII banner with gradient colors.
func RenderBanner() string {
	lines := splitLines(bannerArt)
	rendered := ""

	baseStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	maxWidth := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > maxWidth {
			maxWidth = w
		}
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		rendered += baseStyle.Render(line) + "\n"
	}

	subtitleText := "Context Infrastructure for Agents • Command-Line Interface"
	subtitleWidth := lipgloss.Width(subtitleText)
	blockWidth := max(maxWidth, subtitleWidth)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(blockWidth).
		Align(lipgloss.Center)
	subtitle := subtitleStyle.Render(subtitleText)

	underlineStyle := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Width(blockWidth).
		Align(lipgloss.Center)
	underline := underlineStyle.Render(strings.Repeat("─", subtitleWidth))

	return "\n" + rendered + "\n" + subtitle + "\n" + underline + "\n"
}

// splitLines handles split lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

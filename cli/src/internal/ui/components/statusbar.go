package components

import "charm.land/lipgloss/v2"

var (
	hintDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ba0bf"))
	keyCapStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#16161d")).
			Background(lipgloss.Color("#888ba4")).
			Bold(true).
			Padding(0, 1)
	segmentStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#273540")).
			Padding(0, 1).
			MarginRight(1)
	statusBarBorder = lipgloss.NewStyle()
)

// StatusBar renders the bottom hint bar separated from content by a border line.
func StatusBar(hints []string, width int) string {
	segments := make([]string, 0, len(hints))
	for _, h := range hints {
		segments = append(segments, segmentStyle.Render(h))
	}
	if width <= 0 {
		content := lipgloss.JoinHorizontal(lipgloss.Top, segments...)
		return statusBarBorder.Render(content)
	}
	available := width - statusBarBorder.GetHorizontalFrameSize()
	if available <= 0 {
		available = width
	}
	clamped := clampStatusSegments(segments, available)
	content := lipgloss.JoinHorizontal(lipgloss.Top, clamped...)
	return statusBarBorder.Width(width).Align(lipgloss.Center).Render(content)
}

// Hint formats a single keybind hint like "↑/↓ Scroll".
func Hint(key, desc string) string {
	keyText := keyCapStyle.Render(key)
	return hintDescStyle.Render(desc+" ") + keyText
}

// wrapSegments handles wrap segments.
func wrapSegments(segments []string, width int) []string {
	if width <= 0 {
		return []string{lipgloss.JoinHorizontal(lipgloss.Top, segments...)}
	}
	rows := make([]string, 0, 2)
	var current []string
	currentWidth := 0
	for _, seg := range segments {
		segWidth := lipgloss.Width(seg)
		if currentWidth > 0 && currentWidth+segWidth > width {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, current...))
			current = []string{seg}
			currentWidth = segWidth
			continue
		}
		current = append(current, seg)
		currentWidth += segWidth
	}
	if len(current) > 0 {
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, current...))
	}
	return rows
}

// clampStatusSegments handles clamp status segments.
func clampStatusSegments(segments []string, width int) []string {
	if len(segments) == 0 || width <= 0 {
		return segments
	}

	out := make([]string, 0, len(segments))
	fitAll := true
	for _, seg := range segments {
		candidate := append(append([]string{}, out...), seg)
		if statusSegmentsWidth(candidate) > width {
			fitAll = false
			break
		}
		out = append(out, seg)
	}
	if fitAll {
		return out
	}

	// Keep status bar on one visual row under resize and show explicit overflow.
	overflow := segmentStyle.Render(hintDescStyle.Render("More ") + keyCapStyle.Render("..."))
	for len(out) > 0 && statusSegmentsWidth(append(append([]string{}, out...), overflow)) > width {
		out = out[:len(out)-1]
	}
	if statusSegmentsWidth([]string{overflow}) <= width {
		out = append(out, overflow)
	}
	if len(out) > 0 {
		return out
	}

	// Degenerate tiny width: render at least one hint segment clipped.
	return []string{lipgloss.NewStyle().MaxWidth(width).Render(segments[0])}
}

// statusSegmentsWidth handles status segments width.
func statusSegmentsWidth(segments []string) int {
	if len(segments) == 0 {
		return 0
	}
	return lipgloss.Width(lipgloss.JoinHorizontal(lipgloss.Top, segments...))
}

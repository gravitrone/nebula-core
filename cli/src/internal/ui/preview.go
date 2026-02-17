package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

var previewBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ColorBorder).
	Padding(1, 2)

const (
	previewWidthPercent       = 20
	previewMinWidth           = 28
	previewMaxWidth           = 40
	minSideBySideContentWidth = 138
)

func preferredPreviewWidth(contentWidth int) int {
	if contentWidth <= 0 {
		return previewMinWidth
	}
	previewWidth := contentWidth * previewWidthPercent / 100
	if previewWidth < previewMinWidth {
		previewWidth = previewMinWidth
	}
	if previewWidth > previewMaxWidth {
		previewWidth = previewMaxWidth
	}
	return previewWidth
}

func previewBoxContentWidth(width int) int {
	if width <= 0 {
		return 0
	}
	contentWidth := width - previewBoxStyle.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}
	return contentWidth
}

func renderPreviewBox(content string, width int) string {
	if width <= 0 {
		return ""
	}
	// lipgloss.Style.Width includes padding but excludes borders, so we only
	// subtract left/right border widths to hit the target outer width.
	borderW := previewBoxStyle.GetBorderLeftSize() + previewBoxStyle.GetBorderRightSize()
	inner := width - borderW
	if inner < 1 {
		inner = 1
	}
	return previewBoxStyle.Width(inner).Render(content)
}

func wrapPreviewText(text string, width int) []string {
	text = components.SanitizeOneLine(text)
	if width <= 0 || text == "" {
		return nil
	}
	if lipgloss.Width(text) <= width {
		return []string{text}
	}

	var out []string
	var line strings.Builder
	lineW := 0
	for _, r := range text {
		rw := lipgloss.Width(string(r))
		if rw < 1 {
			rw = 1
		}
		if lineW+rw > width && lineW > 0 {
			out = append(out, strings.TrimRight(line.String(), " "))
			line.Reset()
			lineW = 0
			if r == ' ' {
				continue
			}
		}
		line.WriteRune(r)
		lineW += rw
	}
	if line.Len() > 0 {
		out = append(out, strings.TrimRight(line.String(), " "))
	}
	return out
}

func renderPreviewRow(label, value string, width int) string {
	label = components.SanitizeOneLine(label)
	value = components.SanitizeOneLine(value)

	prefixWidth := lipgloss.Width(label) + 2 // ": "
	maxValue := width - prefixWidth
	if maxValue < 4 {
		maxValue = 4
	}
	value = components.ClampTextWidthEllipsis(value, maxValue)
	return MetaKeyStyle.Render(label) + MetaPunctStyle.Render(": ") + MetaValueStyle.Render(value)
}

func previewStringValue(m api.JSONMap, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "" || s == "<nil>" {
		return ""
	}
	return components.SanitizeOneLine(s)
}

func previewListValue(m api.JSONMap, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s := strings.TrimSpace(fmt.Sprintf("%v", item))
		if s == "" || s == "<nil>" {
			continue
		}
		out = append(out, components.SanitizeOneLine(s))
	}
	return strings.Join(out, ", ")
}

func padPreviewLines(lines []string, width int) string {
	if width <= 0 || len(lines) == 0 {
		return ""
	}
	padded := make([]string, 0, len(lines))
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			// This should be rare because all preview rows clamp before styling.
			// If it happens, strip ANSI so we don't break the layout.
			line = components.ClampTextWidth(components.SanitizeText(line), width)
		}
		if w := lipgloss.Width(line); w < width {
			line += strings.Repeat(" ", width-w)
		}
		padded = append(padded, line)
	}
	return strings.Join(padded, "\n")
}

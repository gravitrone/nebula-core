package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

var previewBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ColorBorder).
	Padding(1, 2)

const (
	previewWidthPercent       = 24
	previewMinWidth           = 26
	previewMaxWidth           = 44
	minSideBySideContentWidth = 138
)

// preferredPreviewWidth handles preferred preview width.
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

// previewBoxContentWidth handles preview box content width.
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

// renderPreviewBox renders render preview box.
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

// wrapPreviewText handles wrap preview text.
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

// renderPreviewRow renders render preview row.
func renderPreviewRow(label, value string, width int) string {
	label = components.SanitizeOneLine(label)
	value = components.SanitizeOneLine(value)
	if strings.EqualFold(label, "scope") || strings.EqualFold(label, "scopes") {
		return renderPreviewScopeRow(label, value, width)
	}

	prefixWidth := lipgloss.Width(label) + 2 // ": "
	maxValue := width - prefixWidth
	if maxValue < 4 {
		maxValue = 4
	}
	value = components.ClampTextWidthEllipsis(value, maxValue)
	return MetaKeyStyle.Render(label) + MetaPunctStyle.Render(": ") + MetaValueStyle.Render(value)
}

// renderPreviewScopeRow renders render preview scope row.
func renderPreviewScopeRow(label, value string, width int) string {
	prefixWidth := lipgloss.Width(label) + 2 // ": "
	maxValue := width - prefixWidth
	if maxValue < 4 {
		maxValue = 4
	}
	scopes := parseScopePreviewTokens(value)
	if len(scopes) == 0 {
		return MetaKeyStyle.Render(label) + MetaPunctStyle.Render(": ") + MetaValueStyle.Render("-")
	}
	parts := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		badge := renderScopeBadge(scope)
		candidate := strings.Join(append(parts, badge), " ")
		if lipgloss.Width(candidate) <= maxValue {
			parts = append(parts, badge)
			continue
		}
		if len(parts) == 0 {
			fallback := components.ClampTextWidthEllipsis("["+scope+"]", maxValue)
			return MetaKeyStyle.Render(label) + MetaPunctStyle.Render(": ") + MetaValueStyle.Render(fallback)
		}
		ellipsis := MutedStyle.Render("...")
		for len(parts) > 0 {
			candidate = strings.Join(append(parts, ellipsis), " ")
			if lipgloss.Width(candidate) <= maxValue {
				parts = append(parts, ellipsis)
				break
			}
			parts = parts[:len(parts)-1]
		}
		if len(parts) == 0 {
			parts = []string{MetaValueStyle.Render(components.ClampTextWidthEllipsis("...", maxValue))}
		}
		break
	}
	return MetaKeyStyle.Render(label) + MetaPunctStyle.Render(": ") + strings.Join(parts, " ")
}

// parseScopePreviewTokens parses parse scope preview tokens.
func parseScopePreviewTokens(value string) []string {
	value = strings.TrimSpace(components.SanitizeOneLine(value))
	if value == "" || value == "-" {
		return nil
	}
	value = strings.ReplaceAll(value, "[", "")
	value = strings.ReplaceAll(value, "]", "")
	raw := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '|' || r == ' '
	})
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, token := range raw {
		token = strings.TrimSpace(token)
		key := strings.ToLower(token)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, token)
	}
	return out
}

// formatScopePreview handles format scope preview.
func formatScopePreview(scopes []string) string {
	if len(scopes) == 0 {
		return "-"
	}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		clean := strings.TrimSpace(components.SanitizeOneLine(scope))
		if clean == "" {
			continue
		}
		out = append(out, "["+clean+"]")
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, " ")
}

// previewStringValue handles preview string value.
func previewStringValue(m map[string]any, key string) string {
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

// previewListValue handles preview list value.
func previewListValue(m map[string]any, key string) string {
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

// padPreviewLines handles pad preview lines.
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

// previewKV is a key-value pair for the preview table.
type previewKV struct {
	key   string
	value string
}

// renderPreviewTable renders a preview panel as a 2-column bubbles table
// with the same styling as the main data tables.
func renderPreviewTable(title string, kvs []previewKV, width int) string {
	if width <= 0 || len(kvs) == 0 {
		return ""
	}

	// Subtract border overhead from TableBaseStyle (2 for left+right border).
	innerWidth := width - 2
	if innerWidth < 20 {
		innerWidth = 20
	}

	keyWidth := 10
	valWidth := innerWidth - keyWidth - (2 * 2) // 2 columns * 2 padding each
	if valWidth < 10 {
		valWidth = 10
	}

	rows := make([]table.Row, len(kvs))
	for i, kv := range kvs {
		rows[i] = table.Row{
			components.ClampTextWidthEllipsis(kv.key, keyWidth),
			components.ClampTextWidthEllipsis(kv.value, valWidth),
		}
	}

	cols := []table.Column{
		{Title: "Field", Width: keyWidth},
		{Title: title, Width: valWidth},
	}

	t := components.NewNebulaTable(cols, len(kvs)+1)
	t.SetColumns(cols)
	t.SetRows(rows)
	actualW := keyWidth + valWidth + (2 * 2)
	t.SetWidth(actualW)
	t.Blur() // Preview table is not interactive.

	return components.TableBaseStyle.Render(t.View())
}

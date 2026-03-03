package components

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

var (
	boxBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#273540")).
			Padding(1, 2)

	boxBorderActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7f57b4")).
			Padding(1, 2)

	boxHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7f57b4")).
			Bold(true)

	diffLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#a9c4ff")).
			Bold(true)

	boxMutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ba0bf"))

	boxValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d7d9da"))

	boxLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#436b77")).
			Bold(true)

	errorBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7a2f3a")).
			Padding(1, 2)

	errorHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#e06c75")).
				Bold(true)

	errorBodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d6b5b5"))
)

// boxWidth handles box width.
func boxWidth(width int) int {
	// Prefer near-full width so dense tables do not clip on common laptop sizes.
	if width <= 0 {
		return 0
	}
	w := width - 6
	if w < 40 {
		w = 40
	}
	return w
}

// safeBoxWidth handles safe box width.
func safeBoxWidth(width int) int {
	if width <= 0 {
		return boxWidth(width)
	}
	w := boxWidth(width)
	if w > width {
		return width
	}
	return w
}

// renderBox renders render box.
func renderBox(style lipgloss.Style, targetWidth int, content string) string {
	width := safeBoxWidth(targetWidth)
	if width <= 0 {
		return style.Render(content)
	}
	// lipgloss.Style.Width includes padding but excludes borders, so we only
	// subtract left/right border widths to hit the target outer width.
	borderW := style.GetBorderLeftSize() + style.GetBorderRightSize()
	inner := width - borderW
	if inner < 1 {
		inner = 1
	}
	return style.Width(inner).Render(content)
}

// Box renders content inside a bordered box.
func Box(content string, width int) string {
	return renderBox(boxBorder, width, content)
}

// BoxContentWidth returns the inner content width excluding border and padding.
func BoxContentWidth(width int) int {
	w := safeBoxWidth(width)
	if w <= 0 {
		return 0
	}
	// Border adds 2, padding adds 4 (left+right).
	inner := w - 6
	if inner < 0 {
		return 0
	}
	return inner
}

// ClampTextWidth truncates text to the given visual width (ANSI-aware).
func ClampTextWidth(text string, width int) string {
	if width <= 0 {
		return text
	}
	cleaned := SanitizeOneLine(text)
	if lipgloss.Width(cleaned) <= width {
		return cleaned
	}
	return truncateRunes(cleaned, width)
}

// ClampTextWidthEllipsis truncates text to the given visual width and adds "..."
// when truncation occurs (ANSI-aware).
func ClampTextWidthEllipsis(text string, width int) string {
	if width <= 0 {
		return ""
	}
	cleaned := SanitizeOneLine(text)
	if lipgloss.Width(cleaned) <= width {
		return cleaned
	}
	if width <= 3 {
		return truncateRunes(cleaned, width)
	}
	return truncateRunes(cleaned, width-3) + "..."
}

// ActiveBox renders content inside a highlighted bordered box.
func ActiveBox(content string, width int) string {
	return renderBox(boxBorderActive, width, content)
}

// ErrorBox renders a red bordered box for errors.
func ErrorBox(title, message string, width int) string {
	header := ""
	if title != "" {
		header = errorHeaderStyle.Render(title) + "\n\n"
	}
	body := errorBodyStyle.Render(message)
	return renderBox(errorBorder, width, header+body)
}

// EmptyStateBox renders a titled empty-state with suggested actions.
func EmptyStateBox(title, message string, actions []string, width int) string {
	var b strings.Builder
	b.WriteString(boxMutedStyle.Render(SanitizeOneLine(message)))

	cleanActions := make([]string, 0, len(actions))
	for _, action := range actions {
		item := strings.TrimSpace(SanitizeOneLine(action))
		if item == "" {
			continue
		}
		cleanActions = append(cleanActions, item)
	}
	if len(cleanActions) > 0 {
		b.WriteString("\n\n")
		b.WriteString(boxLabelStyle.Render("Try:"))
		for _, action := range cleanActions {
			b.WriteString("\n")
			b.WriteString(boxMutedStyle.Render("  - " + action))
		}
	}

	return TitledBox(SanitizeOneLine(title), b.String(), width)
}

// TitledBox renders a box with a header title.
func TitledBox(title, content string, width int) string {
	return titledBoxWithStyle(title, content, width, boxBorder, boxHeaderStyle, lipgloss.Color("#273540"))
}

// TitledBoxWithHeaderStyle renders a titled box using the default border style but a custom title style.
func TitledBoxWithHeaderStyle(title, content string, width int, headerStyle lipgloss.Style) string {
	return titledBoxWithStyle(title, content, width, boxBorder, headerStyle, lipgloss.Color("#273540"))
}

// titledBoxWithStyle handles titled box with style.
func titledBoxWithStyle(title, content string, width int, boxStyle, headerStyle lipgloss.Style, borderColor lipgloss.Color) string {
	if title == "" {
		return renderBox(boxStyle, width, content)
	}
	boxed := renderBox(boxStyle, width, content)
	lines := strings.Split(boxed, "\n")

	lineWidth := lipgloss.Width(lines[0])
	if lineWidth < 4 {
		return boxed
	}

	border := lipgloss.RoundedBorder()
	middleLen := lineWidth - 2
	titleText := fmt.Sprintf(" [ %s ] ", title)
	if lipgloss.Width(titleText) > middleLen {
		titleText = truncateRunes(titleText, middleLen)
	}

	titleWidth := lipgloss.Width(titleText)
	left := (middleLen - titleWidth) / 2
	right := middleLen - titleWidth - left

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	leftSeg := borderStyle.Render(border.TopLeft + strings.Repeat(border.Top, left))
	rightSeg := borderStyle.Render(strings.Repeat(border.Top, right) + border.TopRight)
	line := leftSeg + headerStyle.Render(titleText) + rightSeg
	if w := lipgloss.Width(line); w < lineWidth {
		line += borderStyle.Render(strings.Repeat(border.Top, lineWidth-w))
	} else if w > lineWidth {
		line = truncateRunes(line, lineWidth)
	}

	lines[0] = line
	return strings.Join(lines, "\n")
}

// truncateRunes handles truncate runes.
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	var b strings.Builder
	b.Grow(max)
	n := 0
	for _, r := range s {
		if n >= max {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// maxInt handles max int.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// padRight handles pad right.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// InfoRow renders a label: value row for detail views.
func InfoRow(label, value string) string {
	safeLabel := SanitizeOneLine(label)
	safeValue := SanitizeOneLine(value)
	return boxMutedStyle.Render(safeLabel+": ") + boxValueStyle.Render(safeValue)
}

// Table renders a key-value table with aligned columns inside a bordered box.
func Table(title string, rows []TableRow, width int) string {
	if len(rows) == 0 {
		return ""
	}

	// Find max label width for alignment
	maxLabel := 0
	safeRows := make([]TableRow, len(rows))
	for i, r := range rows {
		safeRows[i] = TableRow{
			Label:      SanitizeOneLine(r.Label),
			Value:      SanitizeOneLine(r.Value),
			ValueColor: r.ValueColor,
		}
		if lipgloss.Width(safeRows[i].Label) > maxLabel {
			maxLabel = lipgloss.Width(safeRows[i].Label)
		}
	}

	contentWidth := BoxContentWidth(width)
	if contentWidth <= 0 {
		contentWidth = maxLabel + 8
	}

	labelWidth := maxLabel
	if labelWidth > 24 {
		labelWidth = 24
	}
	if contentWidth > 0 {
		maxLabelWidth := contentWidth / 2
		if maxLabelWidth < 8 {
			maxLabelWidth = contentWidth
		}
		if labelWidth > maxLabelWidth {
			labelWidth = maxLabelWidth
		}
	}
	if labelWidth < 4 {
		labelWidth = maxLabel
	}
	valueWidth := contentWidth - labelWidth - 2
	if valueWidth < 4 {
		valueWidth = 4
		if contentWidth > 0 {
			labelWidth = maxInt(4, contentWidth-valueWidth-2)
		}
	}

	lines := make([]string, 0, len(safeRows))
	for _, r := range safeRows {
		labelText := ClampTextWidth(r.Label, labelWidth)
		valueLines := wrapTableValue(r.Value, valueWidth)
		label := boxLabelStyle.Render(padRight(labelText, labelWidth))
		valueStyle := boxValueStyle
		if strings.TrimSpace(r.ValueColor) != "" {
			// Only allow internal callers to apply styling; user-provided content is sanitized above.
			valueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(r.ValueColor)).Bold(true)
		}
		for idx, valueLine := range valueLines {
			currentLabel := label
			if idx > 0 {
				currentLabel = boxLabelStyle.Render(strings.Repeat(" ", labelWidth))
			}
			valueText := ClampTextWidth(valueLine, valueWidth)
			lines = append(lines, currentLabel+"  "+valueStyle.Render(valueText))
		}
	}
	content := strings.Join(lines, "\n")

	if title != "" {
		return TitledBox(title, content, width)
	}
	return Box(content, width)
}

// TableRow is a single row in a key-value table.
type TableRow struct {
	Label string
	Value string
	// ValueColor overrides the default value color for this row.
	// It must be a lipgloss-compatible color string (e.g. "#RRGGBB").
	ValueColor string
}

// wrapTableValue handles wrap table value.
func wrapTableValue(value string, width int) []string {
	value = SanitizeText(value)
	if width <= 0 {
		return []string{value}
	}
	if strings.TrimSpace(value) == "" {
		return []string{""}
	}
	rawLines := strings.Split(value, "\n")
	out := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			out = append(out, "")
			continue
		}
		if lipgloss.Width(line) <= width {
			out = append(out, line)
			continue
		}
		out = append(out, wrapTableWords(line, width)...)
	}
	return out
}

// wrapTableWords handles wrap table words.
func wrapTableWords(text string, width int) []string {
	text = strings.TrimSpace(SanitizeOneLine(text))
	if text == "" || width <= 0 {
		return []string{text}
	}
	if lipgloss.Width(text) <= width {
		return []string{text}
	}
	words := strings.Fields(text)
	out := make([]string, 0, len(words))
	current := ""
	for _, word := range words {
		if lipgloss.Width(word) > width {
			word = ClampTextWidthEllipsis(word, width)
		}
		if current == "" {
			current = word
			continue
		}
		candidate := current + " " + word
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		out = append(out, current)
		current = word
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

// Indent adds left padding to every line of a multi-line string.
func Indent(s string, spaces int) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join(lines, "\n")
}

// CenterLine centers a single line within the standard box width.
func CenterLine(s string, width int) string {
	w := safeBoxWidth(width)
	if w <= 0 {
		return s
	}
	lineWidth := lipgloss.Width(s)
	if lineWidth >= w {
		return s
	}
	pad := (w - lineWidth) / 2
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

// DiffRow represents a single change with from/to values.
type DiffRow struct {
	Label string
	From  string
	To    string
}

// DiffTable renders a from/to diff table with - (red) and + (yellow) lines.
func DiffTable(title string, rows []DiffRow, width int) string {
	if len(rows) == 0 {
		return ""
	}

	removeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff4d6d"))
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#3f866b"))
	valueWidth := BoxContentWidth(width) - 4
	if valueWidth < 24 {
		valueWidth = 24
	}
	renderValue := func(style lipgloss.Style, prefix string, value string) string {
		value = SanitizeText(value)
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || trimmed == "<nil>" || trimmed == "-" || trimmed == "--" {
			value = "None"
		} else {
			value = trimmed
		}
		lines := strings.Split(value, "\n")
		var out strings.Builder
		for i, line := range lines {
			wrapped := wrapDiffLine(line, valueWidth-len(prefix))
			for j, chunk := range wrapped {
				if i == 0 && j == 0 {
					out.WriteString(style.Render(prefix + chunk))
				} else {
					out.WriteString(style.Render(strings.Repeat(" ", len(prefix)) + chunk))
				}
				if i < len(lines)-1 || j < len(wrapped)-1 {
					out.WriteString("\n")
				}
			}
		}
		return out.String()
	}

	var b strings.Builder
	for i, r := range rows {
		label := SanitizeOneLine(r.Label)
		b.WriteString(diffLabelStyle.Render(label))
		b.WriteString("\n")
		b.WriteString(renderValue(removeStyle, "  - ", r.From))
		b.WriteString("\n")
		b.WriteString(renderValue(addStyle, "  + ", r.To))
		if i < len(rows)-1 {
			b.WriteString("\n\n")
		}
	}

	return TitledBox(title, b.String(), width)
}

// wrapDiffLine handles wrap diff line.
func wrapDiffLine(line string, width int) []string {
	line = strings.TrimSpace(SanitizeText(line))
	if line == "" {
		return []string{"None"}
	}
	if width <= 0 {
		return []string{line}
	}
	if lipgloss.Width(line) <= width {
		return []string{line}
	}
	words := strings.Fields(line)
	out := make([]string, 0, len(words))
	current := ""
	for _, word := range words {
		if lipgloss.Width(word) > width {
			if strings.TrimSpace(current) != "" {
				out = append(out, strings.TrimSpace(current))
				current = ""
			}
			out = append(out, ClampTextWidthEllipsis(word, width))
			continue
		}
		if strings.TrimSpace(current) == "" {
			current = word
			continue
		}
		candidate := current + " " + word
		if lipgloss.Width(candidate) <= width {
			current = candidate
			continue
		}
		out = append(out, strings.TrimSpace(current))
		current = word
	}
	if strings.TrimSpace(current) != "" {
		out = append(out, strings.TrimSpace(current))
	}
	return out
}

// MetadataTable renders a nested metadata map as a bordered table.
func MetadataTable(data map[string]any, width int) string {
	if len(data) == 0 {
		return ""
	}

	lines := renderMetadataLines(data, 0)
	return TitledBox("Metadata", strings.Join(lines, "\n"), width)
}

// renderMetadataLines renders render metadata lines.
func renderMetadataLines(data map[string]any, indent int) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	pad := strings.Repeat(" ", indent)
	for _, k := range keys {
		key := SanitizeOneLine(k)
		rendered := renderMetadataValueLines(data[k], indent+2)
		if len(rendered) == 1 {
			lines = append(lines, fmt.Sprintf("%s%s: %s", pad, key, strings.TrimSpace(rendered[0])))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s%s:", pad, key))
		lines = append(lines, rendered...)
	}
	return lines
}

// renderMetadataValueLines renders render metadata value lines.
func renderMetadataValueLines(val any, indent int) []string {
	pad := strings.Repeat(" ", indent)
	val = normalizeStructuredValue(val)
	switch typed := val.(type) {
	case map[string]any:
		if len(typed) == 0 {
			return []string{"{}"}
		}
		return renderMetadataLines(typed, indent)
	case []any:
		if len(typed) == 0 {
			return []string{"[]"}
		}
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			item = normalizeStructuredValue(item)
			switch nested := item.(type) {
			case map[string]any:
				if textRaw, ok := nested["text"]; ok {
					text := strings.TrimSpace(SanitizeOneLine(fmt.Sprintf("%v", textRaw)))
					if text != "" {
						scopes := parseMetadataScopesInline(nested["scopes"])
						if scopes != "" {
							out = append(out, fmt.Sprintf("%s- [%s] %s", pad, scopes, text))
						} else {
							out = append(out, fmt.Sprintf("%s- %s", pad, text))
						}
						continue
					}
				}
				out = append(out, pad+"-")
				out = append(out, renderMetadataLines(nested, indent+2)...)
			case []any:
				out = append(out, fmt.Sprintf("%s- %s", pad, formatMetadataValue(nested)))
			default:
				out = append(out, fmt.Sprintf("%s- %s", pad, formatMetadataValue(nested)))
			}
		}
		return out
	default:
		return []string{formatMetadataValue(typed)}
	}
}

// normalizeStructuredValue handles normalize structured value.
func normalizeStructuredValue(val any) any {
	switch typed := val.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if len(trimmed) >= 2 {
			if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
				(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
				var parsed any
				if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
					switch parsed.(type) {
					case map[string]any, []any:
						return parsed
					}
				}
			}
			// Handle double-encoded JSON strings.
			if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
				var unquoted string
				if err := json.Unmarshal([]byte(trimmed), &unquoted); err == nil {
					return normalizeStructuredValue(unquoted)
				}
			}
		}
	}
	return val
}

// parseMetadataScopesInline parses parse metadata scopes inline.
func parseMetadataScopesInline(value any) string {
	switch typed := value.(type) {
	case []string:
		clean := make([]string, 0, len(typed))
		for _, item := range typed {
			s := strings.TrimSpace(SanitizeOneLine(item))
			if s != "" {
				clean = append(clean, s)
			}
		}
		return strings.Join(clean, ", ")
	case []any:
		clean := make([]string, 0, len(typed))
		for _, item := range typed {
			strItem, ok := item.(string)
			if !ok {
				continue
			}
			s := strings.TrimSpace(SanitizeOneLine(strItem))
			if s != "" {
				clean = append(clean, s)
			}
		}
		return strings.Join(clean, ", ")
	case string:
		s := strings.TrimSpace(SanitizeOneLine(typed))
		return s
	default:
		return ""
	}
}

// formatMetadataValue handles format metadata value.
func formatMetadataValue(val any) string {
	val = normalizeStructuredValue(val)
	switch typed := val.(type) {
	case []any:
		if len(typed) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			switch sub := item.(type) {
			case map[string]any:
				encoded, err := json.Marshal(sub)
				if err != nil {
					parts = append(parts, fmt.Sprintf("%v", sub))
				} else {
					parts = append(parts, string(encoded))
				}
			default:
				parts = append(parts, fmt.Sprintf("%v", sub))
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		if len(typed) == 0 {
			return "{}"
		}
		encoded, err := json.Marshal(typed)
		if err != nil {
			return SanitizeOneLine(fmt.Sprintf("%v", typed))
		}
		return SanitizeOneLine(string(encoded))
	case nil:
		return "None"
	default:
		s := strings.TrimSpace(SanitizeOneLine(fmt.Sprintf("%v", typed)))
		if s == "" || s == "<nil>" {
			return "None"
		}
		return s
	}
}

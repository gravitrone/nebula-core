package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

func parseMetadataInput(input string) (map[string]any, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	root := map[string]any{}
	stack := []map[string]any{root}

	lines := strings.Split(input, "\n")
	for idx, raw := range lines {
		lineNum := idx + 1
		line := strings.TrimRight(raw, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		spaces := leadingSpaces(line)
		if spaces%2 != 0 {
			return nil, fmt.Errorf("line %d: indent must use 2 spaces (tab inserts 2)", lineNum)
		}
		level := spaces / 2
		if level > len(stack)-1 {
			return nil, fmt.Errorf("line %d: indent has no parent key, add a parent line first", lineNum)
		}
		if level < len(stack)-1 {
			stack = stack[:level+1]
		}
		content := strings.TrimSpace(line)
		if strings.HasPrefix(content, "- ") {
			return nil, fmt.Errorf("line %d: list items not supported, use key: [a, b]", lineNum)
		}
		if strings.Contains(content, "|") {
			keyPath, valueRaw, err := parseMetadataPipeLine(content, lineNum)
			if err != nil {
				return nil, err
			}
			value, err := parseMetadataValue(valueRaw, lineNum)
			if err != nil {
				return nil, err
			}
			if err := setMetadataPath(root, keyPath, value, lineNum); err != nil {
				return nil, err
			}
			// Pipe rows are absolute entries and don't use indentation stack.
			stack = stack[:1]
			continue
		}
		parts := strings.SplitN(content, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: expected 'key: value' or 'group | field | value'", lineNum)
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			return nil, fmt.Errorf("line %d: key is empty", lineNum)
		}
		valueRaw := strings.TrimSpace(parts[1])
		current := stack[len(stack)-1]
		if valueRaw == "" {
			child := map[string]any{}
			current[key] = child
			stack = append(stack, child)
			continue
		}
		value, err := parseMetadataValue(valueRaw, lineNum)
		if err != nil {
			return nil, err
		}
		current[key] = value
	}
	return root, nil
}

func parseMetadataValue(raw string, lineNum int) (any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "["), "]"))
		if inner == "" {
			return []any{}, nil
		}
		parts := strings.Split(inner, ",")
		items := make([]any, 0, len(parts))
		for _, part := range parts {
			items = append(items, parseMetadataScalar(strings.TrimSpace(part)))
		}
		return items, nil
	}
	if strings.HasPrefix(raw, "{") && strings.HasSuffix(raw, "}") {
		return nil, fmt.Errorf("line %d: inline objects not supported, use nested keys", lineNum)
	}
	return parseMetadataScalar(raw), nil
}

func parseMetadataPipeLine(content string, lineNum int) (string, string, error) {
	parts := strings.Split(content, "|")
	trimmed := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s != "" {
			trimmed = append(trimmed, s)
		}
	}
	if len(trimmed) < 2 {
		return "", "", fmt.Errorf("line %d: expected at least 'field | value'", lineNum)
	}
	value := trimmed[len(trimmed)-1]
	pathParts := trimmed[:len(trimmed)-1]
	for _, part := range pathParts {
		if strings.TrimSpace(part) == "" {
			return "", "", fmt.Errorf("line %d: empty key segment in pipe row", lineNum)
		}
	}
	return strings.Join(pathParts, "."), value, nil
}

func setMetadataPath(root map[string]any, path string, value any, lineNum int) error {
	segments := strings.Split(path, ".")
	if len(segments) == 0 {
		return fmt.Errorf("line %d: empty metadata path", lineNum)
	}
	current := root
	for idx, raw := range segments {
		segment := strings.TrimSpace(raw)
		if segment == "" {
			return fmt.Errorf("line %d: empty key segment in '%s'", lineNum, path)
		}
		isLast := idx == len(segments)-1
		if isLast {
			current[segment] = value
			return nil
		}
		next, ok := current[segment]
		if !ok {
			child := map[string]any{}
			current[segment] = child
			current = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("line %d: key '%s' is already set as a value", lineNum, strings.Join(segments[:idx+1], "."))
		}
		current = child
	}
	return nil
}

func parseMetadataScalar(raw string) any {
	if raw == "" {
		return ""
	}
	if (strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"")) ||
		(strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'")) {
		return strings.Trim(raw, "\"'")
	}
	return raw
}

func metadataToInput(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	lines := metadataInputLines(data, 0)
	return strings.Join(lines, "\n")
}

func metadataInputLines(data map[string]any, indent int) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	pad := strings.Repeat(" ", indent)
	for _, k := range keys {
		switch typed := data[k].(type) {
		case map[string]any:
			lines = append(lines, pad+components.SanitizeText(k)+":")
			lines = append(lines, metadataInputLines(typed, indent+2)...)
		default:
			value := formatMetadataValue(typed)
			lines = append(
				lines,
				fmt.Sprintf("%s%s: %s", pad, components.SanitizeText(k), value),
			)
		}
	}
	return lines
}

func formatMetadataValue(value any) string {
	value = normalizeStructuredMetadataValue(value)
	switch typed := value.(type) {
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, formatMetadataInline(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		b, err := json.Marshal(sanitizeMetadataValue(typed))
		if err != nil {
			return components.SanitizeText(fmt.Sprintf("%v", typed))
		}
		return components.SanitizeText(string(b))
	case string:
		trimmed := strings.TrimSpace(humanizeGoMapString(typed))
		if trimmed == "" || trimmed == "<nil>" {
			return "None"
		}
		return components.SanitizeText(trimmed)
	case nil:
		return "None"
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", typed))
		if s == "" || s == "<nil>" {
			return "None"
		}
		return components.SanitizeText(s)
	}
}

func formatMetadataInline(value any) string {
	value = normalizeStructuredMetadataValue(value)
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" || trimmed == "<nil>" {
			return "None"
		}
		return components.SanitizeText(trimmed)
	case map[string]any:
		b, err := json.Marshal(sanitizeMetadataValue(typed))
		if err != nil {
			return components.SanitizeText(fmt.Sprintf("%v", typed))
		}
		return components.SanitizeText(string(b))
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, formatMetadataInline(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case nil:
		return "None"
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", typed))
		if s == "" || s == "<nil>" {
			return "None"
		}
		return components.SanitizeText(s)
	}
}

func renderMetadataInput(input string) string {
	if strings.TrimSpace(input) == "" {
		return "-"
	}
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		spaces := leadingSpaces(line)
		content := strings.TrimSpace(line)
		pad := strings.Repeat(" ", spaces)

		if strings.Contains(content, "|") {
			parts := strings.Split(content, "|")
			trimmed := make([]string, 0, len(parts))
			for _, p := range parts {
				s := components.SanitizeText(strings.TrimSpace(p))
				if s != "" {
					trimmed = append(trimmed, s)
				}
			}
			if len(trimmed) >= 2 {
				value := trimmed[len(trimmed)-1]
				path := strings.Join(trimmed[:len(trimmed)-1], ".")
				group, field := metadataGroupAndField(path)
				row := MetaKeyStyle.Render(group) +
					MetaPunctStyle.Render(" | ") +
					MetaKeyStyle.Render(field) +
					MetaPunctStyle.Render(" | ") +
					MetaValueStyle.Render(value)
				lines[i] = pad + row
				continue
			}
		}

		if strings.HasPrefix(content, "- ") {
			value := strings.TrimSpace(strings.TrimPrefix(content, "- "))
			lines[i] = pad + MetaPunctStyle.Render("- ") +
				MetaValueStyle.Render(components.SanitizeText(value))
			continue
		}

		if idx := strings.Index(content, ":"); idx != -1 {
			key := components.SanitizeText(strings.TrimSpace(content[:idx]))
			rest := components.SanitizeText(strings.TrimSpace(content[idx+1:]))
			rendered := MetaKeyStyle.Render(key) + MetaPunctStyle.Render(":")
			if rest != "" {
				rendered += " " + MetaValueStyle.Render(rest)
			}
			lines[i] = pad + rendered
			continue
		}

		lines[i] = pad + MetaValueStyle.Render(components.SanitizeText(content))
	}
	return strings.Join(lines, "\n")
}

func renderMetadataEditorPreview(buffer string, scopes []string, width int, maxRows int) string {
	if maxRows < 1 {
		maxRows = 1
	}

	data, err := parseMetadataInput(buffer)
	if err != nil {
		return renderMetadataInput(buffer)
	}
	data = mergeMetadataScopes(data, scopes)
	if len(data) == 0 {
		return "-"
	}

	rows := metadataDisplayRows(data)
	if len(rows) == 0 {
		return "-"
	}
	remaining := 0
	if len(rows) > maxRows {
		remaining = len(rows) - maxRows
		rows = rows[:maxRows]
	}

	contentWidth := components.BoxContentWidth(width) - 8
	if contentWidth < 36 {
		contentWidth = 36
	}
	groupWidth, fieldWidth, valueWidth := metadataColumnWidths(contentWidth)
	columns := []components.TableColumn{
		{Header: "Group", Width: groupWidth, Align: lipgloss.Left},
		{Header: "Field", Width: fieldWidth, Align: lipgloss.Left},
		{Header: "Value", Width: valueWidth, Align: lipgloss.Left},
	}

	gridRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		group, field := metadataGroupAndField(row.field)
		gridRows = append(gridRows, []string{group, field, row.value})
	}

	rendered := components.TableGrid(columns, gridRows, contentWidth)
	if remaining > 0 {
		rendered += "\n" + MutedStyle.Render(fmt.Sprintf("+%d more rows", remaining))
	}
	return colorizeScopeBadges(rendered)
}

func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r != ' ' {
			break
		}
		count++
	}
	return count
}

func metadataPreview(data map[string]any, maxLen int) string {
	if len(data) == 0 || maxLen <= 0 {
		return ""
	}
	keys := []string{
		"summary",
		"notes",
		"content",
		"text",
		"context_segments",
		"url",
		"author",
		"title",
	}
	for _, key := range keys {
		if val, ok := data[key]; ok {
			if preview := metadataValuePreview(val, maxLen); preview != "" {
				return preview
			}
		}
	}
	sorted := make([]string, 0, len(data))
	for k := range data {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	if len(sorted) == 0 {
		return ""
	}
	return metadataValuePreview(data[sorted[0]], maxLen)
}

func metadataValuePreview(value any, maxLen int) string {
	value = normalizeStructuredMetadataValue(value)
	if maxLen <= 0 {
		return ""
	}
	switch typed := value.(type) {
	case string:
		pretty := strings.TrimSpace(humanizeGoMapString(typed))
		return truncateString(strings.TrimSpace(components.SanitizeText(pretty)), maxLen)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			snippet := metadataValuePreview(item, maxLen)
			if snippet == "" {
				continue
			}
			parts = append(parts, snippet)
			joined := strings.Join(parts, " | ")
			if lipgloss.Width(joined) >= maxLen {
				break
			}
		}
		return truncateString(strings.Join(parts, " | "), maxLen)
	case map[string]any:
		if textRaw, ok := typed["text"]; ok {
			text := strings.TrimSpace(components.SanitizeText(fmt.Sprintf("%v", textRaw)))
			if text != "" {
				if scopesRaw, hasScopes := typed["scopes"]; hasScopes {
					scopes := metadataValuePreview(scopesRaw, maxLen/3)
					if scopes != "" {
						return truncateString("["+scopes+"] "+text, maxLen)
					}
				}
				return truncateString(text, maxLen)
			}
		}
		for _, key := range []string{"summary", "notes", "content", "title", "name"} {
			if v, ok := typed[key]; ok {
				if snippet := metadataValuePreview(v, maxLen); snippet != "" {
					return snippet
				}
			}
		}
		keys := make([]string, 0, len(typed))
		for k := range typed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if snippet := metadataValuePreview(typed[key], maxLen); snippet != "" {
				return truncateString(key+": "+snippet, maxLen)
			}
		}
		return ""
	default:
		return truncateString(components.SanitizeText(fmt.Sprintf("%v", value)), maxLen)
	}
}

func humanizeGoMapString(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "map[") || !strings.HasSuffix(trimmed, "]") {
		return raw
	}

	scopes := ""
	if start := strings.Index(trimmed, "scopes:["); start >= 0 {
		start += len("scopes:[")
		if end := strings.Index(trimmed[start:], "]"); end >= 0 {
			scopes = strings.TrimSpace(trimmed[start : start+end])
		}
	}

	text := ""
	if start := strings.Index(trimmed, "text:"); start >= 0 {
		start += len("text:")
		text = strings.TrimSpace(trimmed[start:])
		text = strings.TrimSuffix(text, "]")
		text = strings.TrimSpace(text)
	}

	switch {
	case text != "" && scopes != "":
		return "[" + scopes + "] " + text
	case text != "":
		return text
	default:
		return raw
	}
}

func sanitizeMetadataValue(value any) any {
	switch typed := value.(type) {
	case string:
		return components.SanitizeText(typed)
	case map[string]any:
		cleaned := make(map[string]any, len(typed))
		for key, val := range typed {
			cleaned[components.SanitizeText(key)] = sanitizeMetadataValue(val)
		}
		return cleaned
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeMetadataValue(item))
		}
		return out
	default:
		return value
	}
}

func renderMetadataBlock(data map[string]any, width int, expanded bool) string {
	return renderMetadataBlockWithTitle("Metadata", data, width, expanded)
}

func renderMetadataBlockWithTitle(title string, data map[string]any, width int, expanded bool) string {
	if len(data) == 0 {
		return ""
	}
	contentWidth := components.BoxContentWidth(width) - 2
	if contentWidth < 24 {
		contentWidth = 24
	}
	rows := metadataDisplayRows(data)
	if len(rows) == 0 {
		return components.TitledBox(title, MetaValueStyle.Render("None"), width)
	}
	maxRows := 12
	if expanded {
		maxRows = 26
	}
	if len(rows) > maxRows {
		remaining := len(rows) - maxRows
		rows = append(rows[:maxRows], metadataDisplayRow{
			field: "...",
			value: fmt.Sprintf("+%d more rows (press m to expand)", remaining),
		})
	}
	groupWidth, fieldWidth, valueWidth := metadataColumnWidths(contentWidth)
	columns := []components.TableColumn{
		{Header: "Group", Width: groupWidth, Align: lipgloss.Left},
		{Header: "Field", Width: fieldWidth, Align: lipgloss.Left},
		{Header: "Value", Width: valueWidth, Align: lipgloss.Left},
	}
	gridRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		group, field := metadataGroupAndField(row.field)
		gridRows = append(gridRows, []string{group, field, row.value})
	}
	rendered := components.TableGrid(columns, gridRows, contentWidth)
	return components.TitledBox(title, colorizeScopeBadges(rendered), width)
}

type metadataDisplayRow struct {
	field string
	value string
}

func metadataPanelPageSize(expanded bool) int {
	if expanded {
		return 24
	}
	return 12
}

func syncMetadataList(list *components.List, rows []metadataDisplayRow, pageSize int) {
	if list == nil {
		return
	}
	if pageSize < 1 {
		pageSize = 1
	}
	list.PageSize = pageSize

	items := make([]string, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.field)
	}

	prevCursor := list.Cursor
	prevOffset := list.Offset
	list.Items = items
	if len(items) == 0 {
		list.Cursor = 0
		list.Offset = 0
		return
	}
	if prevCursor < 0 {
		prevCursor = 0
	}
	if prevCursor >= len(items) {
		prevCursor = len(items) - 1
	}
	maxOffset := len(items) - list.PageSize
	if maxOffset < 0 {
		maxOffset = 0
	}
	if prevOffset < 0 {
		prevOffset = 0
	}
	if prevOffset > maxOffset {
		prevOffset = maxOffset
	}
	if prevCursor < prevOffset {
		prevOffset = prevCursor
	}
	if prevCursor >= prevOffset+list.PageSize {
		prevOffset = prevCursor - list.PageSize + 1
		if prevOffset < 0 {
			prevOffset = 0
		}
	}
	list.Cursor = prevCursor
	list.Offset = prevOffset
}

func renderMetadataSelectableBlockWithTitle(
	title string,
	rows []metadataDisplayRow,
	width int,
	list *components.List,
	selected map[int]bool,
) string {
	if len(rows) == 0 {
		return components.TitledBox(title, MetaValueStyle.Render("None"), width)
	}
	contentWidth := components.BoxContentWidth(width) - 2
	if contentWidth < 40 {
		contentWidth = 40
	}

	if list == nil {
		fallback := components.NewList(metadataPanelPageSize(false))
		list = fallback
		syncMetadataList(list, rows, metadataPanelPageSize(false))
	}
	visible := list.Visible()
	if len(visible) == 0 {
		return components.TitledBox(title, MetaValueStyle.Render("None"), width)
	}

	groupWidth, fieldWidth, valueWidth := metadataColumnWidths(contentWidth - 5)

	columns := []components.TableColumn{
		{Header: "Sel", Width: 4, Align: lipgloss.Left},
		{Header: "Group", Width: groupWidth, Align: lipgloss.Left},
		{Header: "Field", Width: fieldWidth, Align: lipgloss.Left},
		{Header: "Value", Width: valueWidth, Align: lipgloss.Left},
	}

	gridRows := make([][]string, 0, len(visible))
	activeVisible := -1
	for relIdx := range visible {
		absIdx := list.RelToAbs(relIdx)
		if absIdx < 0 || absIdx >= len(rows) {
			continue
		}
		row := rows[absIdx]
		mark := "[ ]"
		if selected != nil && selected[absIdx] {
			mark = "[X]"
		}
		if list.IsSelected(absIdx) {
			activeVisible = len(gridRows)
		}
		group, field := metadataGroupAndField(row.field)
		gridRows = append(gridRows, []string{
			mark,
			group,
			field,
			row.value,
		})
	}
	rendered := colorizeScopeBadges(
		components.TableGridWithActiveRow(columns, gridRows, contentWidth, activeVisible),
	)

	start := list.Offset + 1
	end := list.Offset + len(gridRows)
	if start < 1 {
		start = 1
	}
	selectedCount := 0
	for _, v := range selected {
		if v {
			selectedCount++
		}
	}
	metaLine := fmt.Sprintf("Rows %d-%d of %d", start, end, len(rows))
	if selectedCount > 0 {
		metaLine += fmt.Sprintf(" · selected %d", selectedCount)
	}
	hintLine := "↑/↓ navigate · space select · b all · enter inspect · c copy selected"

	content := rendered + "\n\n" + MutedStyle.Render(metaLine) + "\n" + MutedStyle.Render(hintLine)
	return components.TitledBox(title, content, width)
}

func metadataDisplayRows(data map[string]any) []metadataDisplayRow {
	rows := make([]metadataDisplayRow, 0, len(data)*2)
	flattenMetadataMapRows("", data, &rows)
	return rows
}

func metadataGroupAndField(path string) (string, string) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "-", "-"
	}
	parts := splitMetadataPath(trimmed)
	if len(parts) == 0 {
		return "-", trimmed
	}
	if len(parts) == 1 {
		return "-", parts[0]
	}

	group := parts[0]
	fieldParts := append([]string{}, parts[1:]...)

	for i := 0; i < len(fieldParts); i++ {
		if fieldParts[i] != "context_segments" {
			continue
		}
		if i+1 >= len(fieldParts) {
			fieldParts[i] = "context"
			continue
		}
		if idx, err := strconv.Atoi(fieldParts[i+1]); err == nil {
			segment := fmt.Sprintf("context segment %d", idx+1)
			fieldParts = append(fieldParts[:i], append([]string{segment}, fieldParts[i+2:]...)...)
		} else {
			fieldParts[i] = "context"
		}
	}

	if group == "context_segments" {
		group = "context"
		if len(fieldParts) > 0 {
			if idx, err := strconv.Atoi(fieldParts[0]); err == nil {
				fieldParts[0] = fmt.Sprintf("segment %d", idx+1)
			}
		}
	}
	if idx, err := strconv.Atoi(group); err == nil {
		group = fmt.Sprintf("segment %d", idx+1)
	}

	field := strings.Join(fieldParts, ".")
	field = strings.TrimSpace(field)
	if field == "" {
		field = "-"
	}
	return group, field
}

func splitMetadataPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	normalized := strings.ReplaceAll(path, "]", "")
	normalized = strings.ReplaceAll(normalized, "[", ".")
	raw := strings.Split(normalized, ".")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func metadataColumnWidths(contentWidth int) (int, int, int) {
	if contentWidth < 34 {
		contentWidth = 34
	}
	usable := contentWidth - 2 // separators
	if usable < 24 {
		usable = 24
	}

	groupWidth := usable * 22 / 100
	fieldWidth := usable * 30 / 100
	valueWidth := usable - groupWidth - fieldWidth

	if groupWidth < 10 {
		groupWidth = 10
	}
	if fieldWidth < 14 {
		fieldWidth = 14
	}
	if valueWidth < 14 {
		valueWidth = 14
	}
	used := groupWidth + fieldWidth + valueWidth
	if used < usable {
		valueWidth += usable - used
	} else if used > usable {
		overflow := used - usable
		if valueWidth-overflow >= 14 {
			valueWidth -= overflow
		} else if fieldWidth-overflow >= 14 {
			fieldWidth -= overflow
		}
	}
	return groupWidth, fieldWidth, valueWidth
}

func flattenMetadataMapRows(prefix string, data map[string]any, rows *[]metadataDisplayRow) {
	if len(data) == 0 {
		if prefix != "" {
			*rows = append(*rows, metadataDisplayRow{field: prefix, value: "None"})
		}
		return
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		value := normalizeStructuredMetadataValue(data[key])
		if strings.EqualFold(strings.TrimSpace(key), "scopes") {
			scopes := parseStringSlice(value)
			if len(scopes) == 0 {
				*rows = append(*rows, metadataDisplayRow{field: path, value: "None"})
			} else {
				*rows = append(*rows, metadataDisplayRow{
					field: path,
					value: strings.Join(scopeBadgesText(scopes), " "),
				})
			}
			continue
		}

		switch typed := value.(type) {
		case map[string]any:
			flattenMetadataMapRows(path, typed, rows)
		case []any:
			flattenMetadataListRows(path, typed, rows)
		default:
			*rows = append(*rows, metadataDisplayRow{
				field: path,
				value: formatMetadataValue(typed),
			})
		}
	}
}

func flattenMetadataListRows(prefix string, items []any, rows *[]metadataDisplayRow) {
	if len(items) == 0 {
		*rows = append(*rows, metadataDisplayRow{field: prefix, value: "None"})
		return
	}

	// Compact simple scalar lists into one row.
	allScalars := true
	for _, raw := range items {
		switch normalizeStructuredMetadataValue(raw).(type) {
		case map[string]any, []any:
			allScalars = false
		}
		if !allScalars {
			break
		}
	}
	if allScalars {
		values := make([]string, 0, len(items))
		for _, raw := range items {
			values = append(values, formatMetadataValue(raw))
		}
		*rows = append(*rows, metadataDisplayRow{
			field: prefix,
			value: strings.Join(values, ", "),
		})
		return
	}

	for idx, raw := range items {
		path := fmt.Sprintf("%s[%d]", prefix, idx)
		value := normalizeStructuredMetadataValue(raw)
		switch typed := value.(type) {
		case map[string]any:
			if textRaw, ok := typed["text"]; ok {
				text := strings.TrimSpace(components.SanitizeText(fmt.Sprintf("%v", textRaw)))
				if text != "" {
					scopePrefix := ""
					if scopesRaw, hasScopes := typed["scopes"]; hasScopes {
						scopes := parseStringSlice(scopesRaw)
						if len(scopes) > 0 {
							scopePrefix = strings.Join(scopeBadgesText(scopes), " ") + " "
						}
					}
					*rows = append(*rows, metadataDisplayRow{
						field: path,
						value: scopePrefix + text,
					})
					continue
				}
			}
			flattenMetadataMapRows(path, typed, rows)
		case []any:
			flattenMetadataListRows(path, typed, rows)
		default:
			*rows = append(*rows, metadataDisplayRow{
				field: path,
				value: formatMetadataValue(typed),
			})
		}
	}
}

func metadataLinesStyled(data map[string]any, indent int) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	pad := strings.Repeat(" ", indent)
	for _, k := range keys {
		value := normalizeStructuredMetadataValue(data[k])
		switch typed := value.(type) {
		case map[string]any:
			lines = append(
				lines,
				pad+MetaKeyStyle.Render(components.SanitizeText(k))+
					MetaPunctStyle.Render(":"),
			)
			lines = append(lines, metadataLinesStyled(typed, indent+2)...)
		case []any:
			if len(typed) == 0 {
				lines = append(
					lines,
					fmt.Sprintf(
						"%s%s%s %s",
						pad,
						MetaKeyStyle.Render(components.SanitizeText(k)),
						MetaPunctStyle.Render(":"),
						MetaValueStyle.Render("[]"),
					),
				)
				continue
			}
			lines = append(
				lines,
				pad+MetaKeyStyle.Render(components.SanitizeText(k))+
					MetaPunctStyle.Render(":"),
			)
			lines = append(lines, metadataListLinesStyled(typed, indent+2)...)
		default:
			value := formatMetadataValue(typed)
			lines = append(
				lines,
				fmt.Sprintf(
					"%s%s%s %s",
					pad,
					MetaKeyStyle.Render(components.SanitizeText(k)),
					MetaPunctStyle.Render(":"),
					MetaValueStyle.Render(value),
				),
			)
		}
	}
	return lines
}

func metadataLinesPlain(data map[string]any, indent int) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	pad := strings.Repeat(" ", indent)
	for _, k := range keys {
		value := normalizeStructuredMetadataValue(data[k])
		switch typed := value.(type) {
		case map[string]any:
			lines = append(lines, pad+components.SanitizeText(k)+":")
			lines = append(lines, metadataLinesPlain(typed, indent+2)...)
		case []any:
			if strings.EqualFold(strings.TrimSpace(k), "scopes") {
				scopes := parseStringSlice(typed)
				if len(scopes) == 0 {
					lines = append(lines, fmt.Sprintf("%s%s: None", pad, components.SanitizeText(k)))
				} else {
					lines = append(lines, fmt.Sprintf("%s%s: %s", pad, components.SanitizeText(k), strings.Join(scopeBadgesText(scopes), " ")))
				}
				continue
			}
			lines = append(lines, pad+components.SanitizeText(k)+":")
			lines = append(lines, metadataListLinesPlain(typed, indent+2)...)
		default:
			lines = append(
				lines,
				fmt.Sprintf(
					"%s%s: %s",
					pad,
					components.SanitizeText(k),
					formatMetadataValue(typed),
				),
			)
		}
	}
	return lines
}

func extractMetadataScopes(data map[string]any) []string {
	if data == nil {
		return nil
	}
	raw, ok := data["scopes"]
	if !ok {
		return nil
	}
	var out []string
	switch typed := raw.(type) {
	case []string:
		out = append(out, typed...)
	case []any:
		for _, item := range typed {
			if item == nil {
				continue
			}
			out = append(out, fmt.Sprintf("%v", item))
		}
	case string:
		if strings.TrimSpace(typed) != "" {
			out = append(out, typed)
		}
	}
	return normalizeScopeList(out)
}

func stripMetadataScopes(data map[string]any) map[string]any {
	if len(data) == 0 {
		return data
	}
	clean := make(map[string]any, len(data))
	for k, v := range data {
		if k == "scopes" {
			continue
		}
		clean[k] = v
	}
	return clean
}

func mergeMetadataScopes(data map[string]any, scopes []string) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	scopes = normalizeScopeList(scopes)
	if len(scopes) == 0 {
		delete(data, "scopes")
		return data
	}
	data["scopes"] = scopes
	return data
}

func normalizeScopeList(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		scope := strings.ToLower(strings.TrimSpace(v))
		scope = strings.TrimPrefix(scope, "#")
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	return out
}

func metadataListLinesStyled(items []any, indent int) []string {
	if len(items) == 0 {
		return nil
	}
	pad := strings.Repeat(" ", indent)
	var lines []string
	for _, rawItem := range items {
		item := normalizeStructuredMetadataValue(rawItem)
		switch typed := item.(type) {
		case map[string]any:
			if textRaw, ok := typed["text"]; ok {
				text := strings.TrimSpace(components.SanitizeText(fmt.Sprintf("%v", textRaw)))
				if text != "" {
					scopeText := ""
					if scopesRaw, hasScopes := typed["scopes"]; hasScopes {
						scopes := parseStringSlice(scopesRaw)
						if len(scopes) > 0 {
							scopeText = "[" + strings.Join(scopes, ", ") + "] "
						}
					}
					lines = append(
						lines,
						pad+MetaPunctStyle.Render("- ")+MetaValueStyle.Render(scopeText+text),
					)
					continue
				}
			}
			lines = append(lines, pad+MetaPunctStyle.Render("- ")+MetaValueStyle.Render("{...}"))
			lines = append(lines, metadataLinesStyled(typed, indent+2)...)
		case []any:
			lines = append(lines, pad+MetaPunctStyle.Render("- ")+MetaValueStyle.Render("[...]"))
			lines = append(lines, metadataListLinesStyled(typed, indent+2)...)
		default:
			lines = append(
				lines,
				pad+MetaPunctStyle.Render("- ")+MetaValueStyle.Render(formatMetadataValue(typed)),
			)
		}
	}
	return lines
}

func metadataListLinesPlain(items []any, indent int) []string {
	if len(items) == 0 {
		return nil
	}
	pad := strings.Repeat(" ", indent)
	var lines []string
	for _, rawItem := range items {
		item := normalizeStructuredMetadataValue(rawItem)
		switch typed := item.(type) {
		case map[string]any:
			if textRaw, ok := typed["text"]; ok {
				text := strings.TrimSpace(components.SanitizeText(fmt.Sprintf("%v", textRaw)))
				if text != "" {
					scopePrefix := ""
					if scopesRaw, hasScopes := typed["scopes"]; hasScopes {
						scopes := parseStringSlice(scopesRaw)
						if len(scopes) > 0 {
							scopePrefix = strings.Join(scopeBadgesText(scopes), " ") + " "
						}
					}
					lines = append(lines, pad+"- "+scopePrefix+text)
					continue
				}
			}
			lines = append(lines, pad+"-")
			lines = append(lines, metadataLinesPlain(typed, indent+2)...)
		case []any:
			lines = append(lines, pad+"-")
			lines = append(lines, metadataListLinesPlain(typed, indent+2)...)
		default:
			lines = append(lines, pad+"- "+formatMetadataValue(typed))
		}
	}
	return lines
}

func normalizeStructuredMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, raw := range typed {
			out[key] = normalizeStructuredMetadataValue(raw)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, raw := range typed {
			out[key] = normalizeStructuredMetadataValue(raw)
		}
		return out
	case []map[string]any:
		out := make([]any, 0, len(typed))
		for _, raw := range typed {
			out = append(out, normalizeStructuredMetadataValue(raw))
		}
		return out
	case []map[string]string:
		out := make([]any, 0, len(typed))
		for _, raw := range typed {
			out = append(out, normalizeStructuredMetadataValue(raw))
		}
		return out
	case []string:
		out := make([]any, 0, len(typed))
		for _, raw := range typed {
			out = append(out, normalizeStructuredMetadataValue(raw))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, raw := range typed {
			out = append(out, normalizeStructuredMetadataValue(raw))
		}
		return out
	case string:
		if parsed, ok := parseJSONStructuredString(typed); ok {
			return normalizeStructuredMetadataValue(parsed)
		}
		return typed
	default:
		return value
	}
}

func parseJSONStructuredString(raw string) (any, bool) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 2 {
		return nil, false
	}
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		var unquoted string
		if err := json.Unmarshal([]byte(trimmed), &unquoted); err == nil {
			trimmed = strings.TrimSpace(unquoted)
		}
	}
	if !((strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))) {
		return nil, false
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, false
	}
	switch parsed.(type) {
	case map[string]any, []any:
		return parsed, true
	default:
		return nil, false
	}
}

func parseStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return normalizeScopeList(typed)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				out = append(out, text)
			}
		}
		return normalizeScopeList(out)
	case string:
		parts := strings.Split(typed, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			text := strings.TrimSpace(part)
			if text != "" {
				out = append(out, text)
			}
		}
		return normalizeScopeList(out)
	default:
		return nil
	}
}

func scopeBadgesText(scopes []string) []string {
	if len(scopes) == 0 {
		return nil
	}
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		out = append(out, "["+scope+"]")
	}
	return out
}

func wrapMetadataDisplayLines(lines []string, width int) []string {
	if width <= 0 || len(lines) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, wrapMetadataDisplayLine(line, width)...)
	}
	return out
}

func wrapMetadataDisplayLine(line string, width int) []string {
	clean := strings.TrimRight(components.SanitizeText(line), " ")
	if clean == "" {
		return []string{""}
	}
	if lipgloss.Width(clean) <= width {
		return []string{clean}
	}

	indentSize := leadingSpaces(clean)
	prefix := strings.Repeat(" ", indentSize)
	trimmed := strings.TrimSpace(clean)

	if strings.HasPrefix(trimmed, "- ") {
		bulletPrefix := prefix + "- "
		chunks := wrapMetadataWords(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), width-lipgloss.Width(bulletPrefix))
		if len(chunks) == 0 {
			return []string{components.ClampTextWidthEllipsis(clean, width)}
		}
		out := make([]string, 0, len(chunks))
		out = append(out, bulletPrefix+chunks[0])
		contPrefix := prefix + "  "
		for _, chunk := range chunks[1:] {
			out = append(out, contPrefix+chunk)
		}
		return out
	}

	chunks := wrapMetadataWords(trimmed, width-lipgloss.Width(prefix))
	if len(chunks) == 0 {
		return []string{components.ClampTextWidthEllipsis(clean, width)}
	}
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, prefix+chunk)
	}
	return out
}

func wrapMetadataWords(text string, width int) []string {
	if width <= 0 {
		return []string{components.SanitizeOneLine(text)}
	}
	text = strings.TrimSpace(components.SanitizeText(text))
	if text == "" {
		return nil
	}
	if lipgloss.Width(text) <= width {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{components.ClampTextWidthEllipsis(text, width)}
	}
	out := make([]string, 0, len(words))
	current := ""
	for _, word := range words {
		if lipgloss.Width(word) > width {
			word = components.ClampTextWidthEllipsis(word, width)
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

func colorizeScopeBadges(text string) string {
	rendered := text
	for _, scope := range []string{"public", "private", "sensitive", "admin"} {
		token := "[" + scope + "]"
		rendered = strings.ReplaceAll(rendered, token, renderScopeBadge(scope))
	}
	return rendered
}

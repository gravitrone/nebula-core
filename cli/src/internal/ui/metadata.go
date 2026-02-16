package ui

import (
	"encoding/json"
	"fmt"
	"sort"
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
		parts := strings.SplitN(content, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: expected 'key: value'", lineNum)
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
		return components.SanitizeText(typed)
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
	switch typed := value.(type) {
	case string:
		return components.SanitizeText(typed)
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
	if maxLen <= 0 {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return truncateString(strings.TrimSpace(components.SanitizeText(typed)), maxLen)
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
	lines := metadataLinesStyled(data, 0)
	maxLines := 6
	if !expanded && len(lines) > maxLines {
		lines = append(lines[:maxLines], MutedStyle.Render("..."))
	}
	return components.TitledBox(title, strings.Join(lines, "\n"), width)
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
		switch typed := data[k].(type) {
		case map[string]any:
			lines = append(
				lines,
				pad+MetaKeyStyle.Render(components.SanitizeText(k))+
					MetaPunctStyle.Render(":"),
			)
			lines = append(lines, metadataLinesStyled(typed, indent+2)...)
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
		switch typed := data[k].(type) {
		case map[string]any:
			lines = append(lines, pad+components.SanitizeText(k)+":")
			lines = append(lines, metadataLinesPlain(typed, indent+2)...)
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

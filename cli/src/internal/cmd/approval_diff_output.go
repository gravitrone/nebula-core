package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// approvalDiffViewOptions controls non-interactive approval diff focus behavior.
type approvalDiffViewOptions struct {
	OnlyChanged bool
	Sections    map[string]struct{}
	MaxLines    int
	RawOnly     []string
}

// parseApprovalDiffViewOptions parses --only and --max-lines flags for approval diff rendering.
func parseApprovalDiffViewOptions(only []string, maxLines int) (approvalDiffViewOptions, error) {
	if maxLines < 1 {
		return approvalDiffViewOptions{}, fmt.Errorf("invalid --max-lines %d (expected >= 1)", maxLines)
	}
	opts := approvalDiffViewOptions{
		MaxLines: maxLines,
		RawOnly:  append([]string(nil), only...),
	}
	for _, raw := range only {
		token := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case token == "":
			continue
		case token == "changed":
			opts.OnlyChanged = true
		case strings.HasPrefix(token, "section="):
			section := strings.TrimSpace(strings.TrimPrefix(token, "section="))
			if section == "" {
				return approvalDiffViewOptions{}, fmt.Errorf("invalid --only value %q (missing section name)", raw)
			}
			if opts.Sections == nil {
				opts.Sections = make(map[string]struct{})
			}
			section = canonicalDiffSection(section)
			opts.Sections[section] = struct{}{}
		default:
			return approvalDiffViewOptions{}, fmt.Errorf(
				"invalid --only value %q (expected changed or section=<name>)",
				raw,
			)
		}
	}
	return opts, nil
}

// approvalDiffRows converts the server diff map into ordered rows.
func approvalDiffRows(changes map[string]any, maxLines int) []components.DiffRow {
	if len(changes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(changes))
	for key := range changes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rows := make([]components.DiffRow, 0, len(keys))
	for _, key := range keys {
		from, to := approvalDiffFieldValues(changes[key])
		from = clampDiffValueLines(from, maxLines)
		to = clampDiffValueLines(to, maxLines)
		rows = append(rows, components.DiffRow{
			Label: key,
			From:  from,
			To:    to,
		})
	}
	return rows
}

// approvalDiffFieldValues handles shape-tolerant parsing for server diff values.
func approvalDiffFieldValues(value any) (string, string) {
	diff, ok := value.(map[string]any)
	if !ok {
		return "None", approvalDiffAnyValue(value)
	}
	from := approvalDiffAnyValue(diff["from"])
	to := approvalDiffAnyValue(diff["to"])
	return from, to
}

// approvalDiffAnyValue formats scalar/structured diff values with stable readable output.
func approvalDiffAnyValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "None"
	case string:
		clean := strings.TrimSpace(typed)
		if clean == "" {
			return "None"
		}
		return components.SanitizeText(clean)
	default:
		raw, err := json.MarshalIndent(typed, "", "  ")
		if err != nil {
			clean := strings.TrimSpace(fmt.Sprintf("%v", typed))
			if clean == "" || clean == "<nil>" {
				return "None"
			}
			return components.SanitizeText(clean)
		}
		clean := strings.TrimSpace(string(raw))
		if clean == "" || clean == "null" {
			return "None"
		}
		return components.SanitizeText(clean)
	}
}

// applyApprovalDiffFilters narrows rows using parsed --only focus options.
func applyApprovalDiffFilters(rows []components.DiffRow, opts approvalDiffViewOptions) []components.DiffRow {
	if len(rows) == 0 {
		return nil
	}
	filtered := make([]components.DiffRow, 0, len(rows))
	for _, row := range rows {
		if opts.OnlyChanged && !approvalDiffRowChanged(row) {
			continue
		}
		if len(opts.Sections) > 0 {
			section := approvalDiffSectionForLabel(row.Label)
			if _, ok := opts.Sections[section]; !ok {
				continue
			}
		}
		filtered = append(filtered, row)
	}
	return filtered
}

// approvalDiffRowChanged treats placeholder-normalized rows as unchanged.
func approvalDiffRowChanged(row components.DiffRow) bool {
	return normalizeDiffCompareValue(row.From) != normalizeDiffCompareValue(row.To)
}

// normalizeDiffCompareValue normalizes placeholders for changed/same classification.
func normalizeDiffCompareValue(value string) string {
	clean := strings.TrimSpace(strings.ToLower(components.SanitizeText(value)))
	switch clean {
	case "", "none", "null", "<nil>", "-", "--":
		return "none"
	default:
		return clean
	}
}

// approvalDiffSectionForLabel classifies fields into stable lowercase section keys.
func approvalDiffSectionForLabel(label string) string {
	key := strings.ToLower(strings.TrimSpace(label))
	switch {
	case key == "content":
		return "content"
	case key == "scopes" || strings.Contains(key, "scope"):
		return "scopes"
	case key == "tags" || strings.Contains(key, "tag"):
		return "tags"
	case key == "source type" || strings.Contains(key, "source"):
		return "source"
	case key == "title" || key == "name" || key == "status" || key == "type":
		return "core"
	case strings.Contains(key, "metadata"):
		return "metadata"
	default:
		return "other"
	}
}

// canonicalDiffSection handles accepted section aliases in --only section=<name>.
func canonicalDiffSection(section string) string {
	switch strings.ToLower(strings.TrimSpace(section)) {
	case "core":
		return "core"
	case "metadata", "meta":
		return "metadata"
	case "tags", "tag":
		return "tags"
	case "scopes", "scope":
		return "scopes"
	case "content":
		return "content"
	case "source":
		return "source"
	default:
		return "other"
	}
}

// clampDiffValueLines enforces the command-level --max-lines cap before UI wrapping.
func clampDiffValueLines(value string, maxLines int) string {
	if maxLines <= 0 {
		return value
	}
	lines := strings.Split(components.SanitizeText(value), "\n")
	nonEmpty := make([]string, 0, len(lines))
	for _, line := range lines {
		nonEmpty = append(nonEmpty, strings.TrimRight(line, " \t"))
	}
	if len(nonEmpty) <= maxLines {
		return strings.Join(nonEmpty, "\n")
	}
	hidden := len(nonEmpty) - maxLines
	clamped := append([]string{}, nonEmpty[:maxLines]...)
	clamped = append(clamped, fmt.Sprintf("... (+%d more lines)", hidden))
	return strings.Join(clamped, "\n")
}

// buildApprovalDiffResponse returns the filtered machine payload for json/plain output modes.
func buildApprovalDiffResponse(
	item *api.ApprovalDiff,
	rows []components.DiffRow,
	opts approvalDiffViewOptions,
) map[string]any {
	changes := make(map[string]any, len(rows))
	for _, row := range rows {
		changes[row.Label] = map[string]any{
			"from":    row.From,
			"to":      row.To,
			"section": approvalDiffSectionForLabel(row.Label),
			"changed": approvalDiffRowChanged(row),
		}
	}
	sections := make([]string, 0, len(opts.Sections))
	for section := range opts.Sections {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	return map[string]any{
		"approval_id":    item.ApprovalID,
		"request_type":   item.RequestType,
		"changes":        changes,
		"filtered_count": len(rows),
		"filters": map[string]any{
			"only":      opts.RawOnly,
			"sections":  sections,
			"changed":   opts.OnlyChanged,
			"max_lines": opts.MaxLines,
		},
	}
}

// renderApprovalDiffTable handles table-mode output for approval diff command.
func renderApprovalDiffTable(
	out io.Writer,
	item *api.ApprovalDiff,
	rows []components.DiffRow,
	opts approvalDiffViewOptions,
) {
	summary := []components.TableRow{
		{Label: "approval_id", Value: item.ApprovalID},
		{Label: "request_type", Value: item.RequestType},
		{Label: "fields", Value: fmt.Sprintf("%d", len(rows))},
	}
	if len(opts.RawOnly) > 0 {
		summary = append(summary, components.TableRow{
			Label: "filter",
			Value: strings.Join(opts.RawOnly, ", "),
		})
	}
	summary = append(summary, components.TableRow{
		Label: "max_lines",
		Value: fmt.Sprintf("%d", opts.MaxLines),
	})
	renderCommandPanel(out, "Approval Diff", summary)

	if len(rows) == 0 {
		renderCommandMessage(out, "Changes", "No fields matched selected filters.")
		return
	}

	prev, hadPrev := os.LookupEnv("NEBULA_DIFF_FULL")
	_ = os.Setenv("NEBULA_DIFF_FULL", "1")
	defer func() {
		if hadPrev {
			_ = os.Setenv("NEBULA_DIFF_FULL", prev)
			return
		}
		_ = os.Unsetenv("NEBULA_DIFF_FULL")
	}()

	box := components.DiffTable("Changes", rows, commandWidth(out))
	_, _ = fmt.Fprintf(out, "%s\n", box)
}

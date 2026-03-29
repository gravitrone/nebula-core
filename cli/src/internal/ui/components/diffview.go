package components

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// --- Diff View Types ---

// DiffLineKind classifies a line in a unified diff.
type DiffLineKind int

const (
	// DiffContext is an unchanged context line.
	DiffContext DiffLineKind = iota
	// DiffAdd is an added line.
	DiffAdd
	// DiffDelete is a removed line.
	DiffDelete
	// DiffHunk is a @@ hunk header line.
	DiffHunk
)

// DiffLine represents a single line in a unified diff.
type DiffLine struct {
	Kind   DiffLineKind
	Before int    // line number in old file (0 = absent)
	After  int    // line number in new file (0 = absent)
	Text   string // line content without leading +/-/space
}

// --- Diff View Styles ---

// diffColors holds the color scheme for diff rendering, adapted from
// crush's dark theme with nebula accent colors.
var diffColors = struct {
	addNumFg    color.Color
	addNumBg    color.Color
	addSymbolFg color.Color
	addCodeBg   color.Color
	delNumFg    color.Color
	delNumBg    color.Color
	delSymbolFg color.Color
	delCodeBg   color.Color
	ctxNumFg    color.Color
	ctxCodeFg   color.Color
	ctxBg       color.Color
	hunkFg      color.Color
	hunkBg      color.Color
}{
	addNumFg:    lipgloss.Color("#629657"),
	addNumBg:    lipgloss.Color("#2b322a"),
	addSymbolFg: lipgloss.Color("#629657"),
	addCodeBg:   lipgloss.Color("#323931"),
	delNumFg:    lipgloss.Color("#a45c59"),
	delNumBg:    lipgloss.Color("#312929"),
	delSymbolFg: lipgloss.Color("#a45c59"),
	delCodeBg:   lipgloss.Color("#383030"),
	ctxNumFg:    lipgloss.Color("#9ba0bf"),
	ctxCodeFg:   lipgloss.Color("#9ba0bf"),
	ctxBg:       lipgloss.Color("#1a1a2e"),
	hunkFg:      lipgloss.Color("#6b7394"),
	hunkBg:      lipgloss.Color("#252535"),
}

// --- Diff Rendering ---

// RenderDiffView renders a unified diff as a styled string with line numbers,
// +/- symbols, and colored backgrounds matching crush's dark theme.
// Width constrains the total output width; lines are truncated if needed.
func RenderDiffView(lines []DiffLine, width int) string {
	if len(lines) == 0 || width <= 0 {
		return ""
	}

	// Compute gutter widths from max line numbers.
	maxBefore, maxAfter := 0, 0
	for _, l := range lines {
		if l.Before > maxBefore {
			maxBefore = l.Before
		}
		if l.After > maxAfter {
			maxAfter = l.After
		}
	}
	beforeW := digitWidth(maxBefore)
	afterW := digitWidth(maxAfter)
	if beforeW < 2 {
		beforeW = 2
	}
	if afterW < 2 {
		afterW = 2
	}

	// Gutter = beforeNum + space + afterNum + space + symbol + space.
	gutterW := beforeW + 1 + afterW + 1 + 2
	codeW := width - gutterW
	if codeW < 10 {
		codeW = 10
	}

	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(renderDiffLine(line, beforeW, afterW, codeW))
	}
	return b.String()
}

// renderDiffLine renders a single diff line with styled gutter and content.
func renderDiffLine(line DiffLine, beforeW, afterW, codeW int) string {
	switch line.Kind {
	case DiffHunk:
		return renderHunkLine(line.Text, beforeW+1+afterW+1+2+codeW)
	case DiffAdd:
		return renderAddLine(line, beforeW, afterW, codeW)
	case DiffDelete:
		return renderDeleteLine(line, beforeW, afterW, codeW)
	default:
		return renderContextLine(line, beforeW, afterW, codeW)
	}
}

// renderHunkLine renders a @@ hunk header across the full width.
func renderHunkLine(text string, totalW int) string {
	style := lipgloss.NewStyle().
		Foreground(diffColors.hunkFg).
		Background(diffColors.hunkBg)
	padded := padRight(text, totalW)
	return style.Render(padded)
}

// renderAddLine renders an added (+) line.
func renderAddLine(line DiffLine, beforeW, afterW, codeW int) string {
	numStyle := lipgloss.NewStyle().
		Foreground(diffColors.addNumFg).
		Background(diffColors.addNumBg)
	symStyle := lipgloss.NewStyle().
		Foreground(diffColors.addSymbolFg).
		Background(diffColors.addCodeBg)
	codeStyle := lipgloss.NewStyle().
		Background(diffColors.addCodeBg)

	beforeNum := strings.Repeat(" ", beforeW)
	afterNum := fmt.Sprintf("%*d", afterW, line.After)

	code := padRight(truncateStr(line.Text, codeW), codeW)
	return numStyle.Render(beforeNum+" ") +
		numStyle.Render(afterNum+" ") +
		symStyle.Render("+ ") +
		codeStyle.Render(code)
}

// renderDeleteLine renders a removed (-) line.
func renderDeleteLine(line DiffLine, beforeW, afterW, codeW int) string {
	numStyle := lipgloss.NewStyle().
		Foreground(diffColors.delNumFg).
		Background(diffColors.delNumBg)
	symStyle := lipgloss.NewStyle().
		Foreground(diffColors.delSymbolFg).
		Background(diffColors.delCodeBg)
	codeStyle := lipgloss.NewStyle().
		Background(diffColors.delCodeBg)

	beforeNum := fmt.Sprintf("%*d", beforeW, line.Before)
	afterNum := strings.Repeat(" ", afterW)

	code := padRight(truncateStr(line.Text, codeW), codeW)
	return numStyle.Render(beforeNum+" ") +
		numStyle.Render(afterNum+" ") +
		symStyle.Render("- ") +
		codeStyle.Render(code)
}

// renderContextLine renders an unchanged context line.
func renderContextLine(line DiffLine, beforeW, afterW, codeW int) string {
	numStyle := lipgloss.NewStyle().
		Foreground(diffColors.ctxNumFg).
		Background(diffColors.ctxBg)
	codeStyle := lipgloss.NewStyle().
		Foreground(diffColors.ctxCodeFg).
		Background(diffColors.ctxBg)

	beforeNum := fmt.Sprintf("%*d", beforeW, line.Before)
	afterNum := fmt.Sprintf("%*d", afterW, line.After)

	code := padRight(truncateStr(line.Text, codeW), codeW)
	return numStyle.Render(beforeNum+" ") +
		numStyle.Render(afterNum+" ") +
		codeStyle.Render("  ") +
		codeStyle.Render(code)
}

// --- Diff Parsing ---

// ParseUnifiedDiff parses a slice of raw diff text lines (with leading +/-/space)
// into typed DiffLine values. Expects standard unified diff format.
func ParseUnifiedDiff(rawLines []string) []DiffLine {
	var result []DiffLine
	beforeLine, afterLine := 0, 0

	for _, raw := range rawLines {
		if strings.HasPrefix(raw, "@@") {
			before, after := parseHunkHeader(raw)
			beforeLine = before
			afterLine = after
			result = append(result, DiffLine{
				Kind: DiffHunk,
				Text: raw,
			})
			continue
		}

		if len(raw) == 0 {
			result = append(result, DiffLine{
				Kind:   DiffContext,
				Before: beforeLine,
				After:  afterLine,
				Text:   "",
			})
			beforeLine++
			afterLine++
			continue
		}

		switch raw[0] {
		case '+':
			result = append(result, DiffLine{
				Kind:  DiffAdd,
				After: afterLine,
				Text:  raw[1:],
			})
			afterLine++
		case '-':
			result = append(result, DiffLine{
				Kind:   DiffDelete,
				Before: beforeLine,
				Text:   raw[1:],
			})
			beforeLine++
		default:
			text := raw
			if len(text) > 0 && text[0] == ' ' {
				text = text[1:]
			}
			result = append(result, DiffLine{
				Kind:   DiffContext,
				Before: beforeLine,
				After:  afterLine,
				Text:   text,
			})
			beforeLine++
			afterLine++
		}
	}
	return result
}

// DiffRowsToLines converts []DiffRow (label/from/to) into []DiffLine for RenderDiffView.
// Changed fields produce a DiffDelete + DiffAdd pair; identical fields produce DiffContext.
// Labels and values are sanitized to single lines so markdown/multi-line content renders cleanly.
func DiffRowsToLines(rows []DiffRow) []DiffLine {
	fields := make([]string, len(rows))
	before := make(map[string]string, len(rows))
	after := make(map[string]string, len(rows))
	for i, r := range rows {
		label := SanitizeOneLine(r.Label)
		fields[i] = label
		before[label] = SanitizeOneLine(r.From)
		after[label] = SanitizeOneLine(r.To)
	}
	return BuildFieldDiff(fields, before, after)
}

// BuildFieldDiff creates diff lines from before/after field value maps.
// This is for nebula's audit/approval diffs where we compare field values.
func BuildFieldDiff(fields []string, before, after map[string]string) []DiffLine {
	var lines []DiffLine
	lineNum := 1

	for _, field := range fields {
		oldVal := before[field]
		newVal := after[field]

		if oldVal == newVal {
			lines = append(lines, DiffLine{
				Kind:   DiffContext,
				Before: lineNum,
				After:  lineNum,
				Text:   field + ": " + oldVal,
			})
			lineNum++
			continue
		}

		if oldVal != "" {
			lines = append(lines, DiffLine{
				Kind:   DiffDelete,
				Before: lineNum,
				Text:   field + ": " + oldVal,
			})
		}
		if newVal != "" {
			lines = append(lines, DiffLine{
				Kind:  DiffAdd,
				After: lineNum,
				Text:  field + ": " + newVal,
			})
		}
		lineNum++
	}
	return lines
}

// --- Helpers ---

// parseHunkHeader extracts start line numbers from a @@ -a,b +c,d @@ header.
func parseHunkHeader(header string) (before, after int) {
	// Minimal parser for @@ -X,Y +A,B @@
	before, after = 1, 1
	parts := strings.SplitN(header, " ", 5)
	for _, part := range parts {
		if strings.HasPrefix(part, "-") && strings.Contains(part, ",") {
			_, _ = fmt.Sscanf(part, "-%d", &before)
		}
		if strings.HasPrefix(part, "+") && strings.Contains(part, ",") {
			_, _ = fmt.Sscanf(part, "+%d", &after)
		}
	}
	return before, after
}

// digitWidth returns the number of decimal digits needed to display n.
func digitWidth(n int) int {
	if n <= 0 {
		return 1
	}
	w := 0
	for n > 0 {
		n /= 10
		w++
	}
	return w
}

// truncateStr truncates a string to fit within maxWidth visual columns.
func truncateStr(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Rune-safe truncation.
	w := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > maxWidth {
			return s[:i]
		}
		w += rw
	}
	return s
}

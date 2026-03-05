package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gravitrone/nebula-core/cli/internal/ui"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// commandWidth handles command width.
func commandWidth(out io.Writer) int {
	const fallback = 120
	if raw := strings.TrimSpace(os.Getenv("COLUMNS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// centerBlockLines handles center block lines.
func centerBlockLines(block string, width int) string {
	lines := strings.Split(block, "\n")
	for i, line := range lines {
		clean := components.SanitizeText(line)
		clean = components.ClampTextWidthEllipsis(clean, width)
		lines[i] = components.CenterLine(clean, width)
	}
	return strings.Join(lines, "\n")
}

// shouldRenderCommandBanner handles whether command banner should render.
func shouldRenderCommandBanner(out io.Writer) bool {
	enabled := strings.ToLower(strings.TrimSpace(os.Getenv("NEBULA_COMMAND_ASCII")))
	if enabled != "1" && enabled != "true" && enabled != "yes" {
		return false
	}
	file, ok := out.(*os.File)
	if !ok || file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// renderCommandPanel renders render command panel.
func renderCommandPanel(out io.Writer, title string, rows []components.TableRow) {
	mode := resolveOutputMode(OutputModeTable)
	switch mode {
	case OutputModeJSON:
		_ = writeCleanJSON(out, map[string]any{
			"title": title,
			"rows":  rows,
		})
		return
	case OutputModePlain:
		renderPlainPanel(out, title, rows)
		return
	}

	width := commandWidth(out)
	panel := components.Table(title, rows, width)
	if !shouldRenderCommandBanner(out) {
		_, _ = fmt.Fprintf(out, "%s\n", panel)
		return
	}
	banner := centerBlockLines(ui.RenderBanner(), width)
	_, _ = fmt.Fprintf(out, "%s\n\n%s\n", banner, panel)
}

// renderCommandMessage renders render command message.
func renderCommandMessage(out io.Writer, title, message string) {
	mode := resolveOutputMode(OutputModeTable)
	switch mode {
	case OutputModeJSON:
		_ = writeCleanJSON(out, map[string]any{
			"title":   title,
			"message": message,
		})
		return
	case OutputModePlain:
		renderPlainMessage(out, title, message)
		return
	}

	width := commandWidth(out)
	body := components.TitledBox(title, message, width)
	if !shouldRenderCommandBanner(out) {
		_, _ = fmt.Fprintf(out, "%s\n", body)
		return
	}
	banner := centerBlockLines(ui.RenderBanner(), width)
	_, _ = fmt.Fprintf(out, "%s\n\n%s\n", banner, body)
}

// renderPlainPanel renders machine-clean text output (no box drawing or ANSI styles).
func renderPlainPanel(out io.Writer, title string, rows []components.TableRow) {
	title = strings.TrimSpace(components.SanitizeOneLine(title))
	if title != "" {
		_, _ = fmt.Fprintln(out, title)
	}
	for _, row := range rows {
		label := strings.TrimSpace(components.SanitizeOneLine(row.Label))
		value := strings.TrimSpace(components.SanitizeText(row.Value))
		if label == "" {
			if value != "" {
				_, _ = fmt.Fprintln(out, value)
			}
			continue
		}
		_, _ = fmt.Fprintf(out, "%s: %s\n", label, value)
	}
}

// renderPlainMessage renders plain text output without decorative UI framing.
func renderPlainMessage(out io.Writer, title, message string) {
	title = strings.TrimSpace(components.SanitizeOneLine(title))
	message = strings.TrimSpace(components.SanitizeText(message))
	switch {
	case title == "":
		_, _ = fmt.Fprintln(out, message)
	case message == "":
		_, _ = fmt.Fprintln(out, title)
	default:
		_, _ = fmt.Fprintf(out, "%s: %s\n", title, message)
	}
}

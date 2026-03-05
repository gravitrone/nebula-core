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
	width := commandWidth(out)
	body := components.TitledBox(title, message, width)
	if !shouldRenderCommandBanner(out) {
		_, _ = fmt.Fprintf(out, "%s\n", body)
		return
	}
	banner := centerBlockLines(ui.RenderBanner(), width)
	_, _ = fmt.Fprintf(out, "%s\n\n%s\n", banner, body)
}

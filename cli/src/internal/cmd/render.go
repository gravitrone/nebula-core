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
		lines[i] = components.CenterLine(line, width)
	}
	return strings.Join(lines, "\n")
}

// renderCommandPanel renders render command panel.
func renderCommandPanel(out io.Writer, title string, rows []components.TableRow) {
	width := commandWidth(out)
	banner := centerBlockLines(ui.RenderBanner(), width)
	panel := components.Table(title, rows, width)
	_, _ = fmt.Fprintf(out, "%s\n\n%s\n", banner, panel)
}

// renderCommandMessage renders render command message.
func renderCommandMessage(out io.Writer, title, message string) {
	width := commandWidth(out)
	banner := centerBlockLines(ui.RenderBanner(), width)
	body := components.TitledBox(title, message, width)
	_, _ = fmt.Fprintf(out, "%s\n\n%s\n", banner, body)
}

package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// TestCommandWidthUsesColumnsEnv verifies terminal width override behavior.
func TestCommandWidthUsesColumnsEnv(t *testing.T) {
	t.Setenv("COLUMNS", "77")
	assert.Equal(t, 77, commandWidth(nil))
}

// TestCommandWidthFallsBackOnInvalidColumns verifies invalid values use fallback width.
func TestCommandWidthFallsBackOnInvalidColumns(t *testing.T) {
	t.Setenv("COLUMNS", "invalid")
	assert.Equal(t, 120, commandWidth(nil))

	t.Setenv("COLUMNS", "0")
	assert.Equal(t, 120, commandWidth(nil))
}

// TestRenderCommandPanelClampsNarrowWidth verifies command panels never overflow very narrow terminals.
func TestRenderCommandPanelClampsNarrowWidth(t *testing.T) {
	t.Setenv("COLUMNS", "20")

	var out bytes.Buffer
	renderCommandPanel(
		&out,
		"Help",
		[]components.TableRow{
			{Label: "command", Value: "nebula"},
			{Label: "usage", Value: "nebula --help"},
		},
	)

	clean := components.SanitizeText(out.String())
	for _, line := range strings.Split(clean, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 20)
	}
}

// TestRenderCommandMessageClampsNarrowWidth verifies command messages also clamp at narrow widths.
func TestRenderCommandMessageClampsNarrowWidth(t *testing.T) {
	t.Setenv("COLUMNS", "20")

	var out bytes.Buffer
	renderCommandMessage(&out, "Nebula API", "API is not running.")

	clean := components.SanitizeText(out.String())
	for _, line := range strings.Split(clean, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), 20)
	}
}

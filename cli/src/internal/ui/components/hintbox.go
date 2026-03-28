package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// --- Theme Styles ---

var (
	hintKeyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7f57b4")).
			Bold(true)

	hintActionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9ba0bf"))

	hintSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#273540"))

	hintBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#273540")).
			Foreground(lipgloss.Color("#9ba0bf"))
)

// --- HintBox ---

// HintBox renders a styled box with context-specific keyboard hints.
// Each hint should be formatted as "key action" (e.g. "esc back").
type HintBox struct {
	hints []string
	width int
}

// NewHintBox creates a HintBox with the given hints.
// Each hint is formatted as "key action" where the first space separates
// the key binding from the action description.
func NewHintBox(hints []string) HintBox {
	return HintBox{hints: hints}
}

// SetWidth sets the available width for the hint box.
func (h *HintBox) SetWidth(w int) {
	h.width = w
}

// View renders the hint box as a styled string.
// Returns an empty string if there are no hints.
func (h HintBox) View() string {
	if len(h.hints) == 0 {
		return ""
	}

	separator := hintSeparatorStyle.Render(" \u00b7 ")
	parts := make([]string, 0, len(h.hints))

	for _, hint := range h.hints {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		key, action := splitHint(hint)
		styled := hintKeyStyle.Render(key)
		if action != "" {
			styled += " " + hintActionStyle.Render(action)
		}
		parts = append(parts, styled)
	}

	if len(parts) == 0 {
		return ""
	}

	content := strings.Join(parts, separator)

	style := hintBoxStyle
	if h.width > 0 {
		borderW := style.GetBorderLeftSize() + style.GetBorderRightSize()
		padW := style.GetPaddingLeft() + style.GetPaddingRight()
		inner := h.width - borderW - padW
		if inner > 0 {
			style = style.Width(inner)
		}
	}

	return style.Render(content)
}

// splitHint splits a hint string at the first space into key and action.
func splitHint(hint string) (string, string) {
	idx := strings.IndexByte(hint, ' ')
	if idx < 0 {
		return hint, ""
	}
	return hint[:idx], strings.TrimSpace(hint[idx+1:])
}

package ui

import (
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
)

// scopeNameList handles scope name list.
func scopeNameList(names map[string]string) []string {
	if len(names) == 0 {
		return nil
	}
	opts := make([]string, 0, len(names))
	for _, name := range names {
		if name != "" {
			opts = append(opts, name)
		}
	}
	sort.Strings(opts)
	return opts
}

// scopeSelected handles scope selected.
func scopeSelected(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// toggleScope handles toggle scope.
func toggleScope(scopes []string, scope string) []string {
	out := make([]string, 0, len(scopes))
	removed := false
	for _, s := range scopes {
		if s == scope {
			removed = true
			continue
		}
		out = append(out, s)
	}
	if !removed {
		out = append(out, scope)
	}
	return out
}

// renderScopePills renders render scope pills.
func renderScopePills(scopes []string, focused bool) string {
	if len(scopes) == 0 && !focused {
		return "-"
	}
	var b strings.Builder
	for i, s := range scopes {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(AccentStyle.Render("[" + s + "]"))
	}
	if focused {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(AccentStyle.Render("█"))
	}
	return b.String()
}

// scopeBadgeStyle returns a pill-style badge for a scope name.
func scopeBadgeStyle(scope string) lipgloss.Style {
	base := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true).
		Padding(0, 1)

	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "public":
		return base.Background(ColorSuccess)
	case "private":
		return base.Background(ColorWarning)
	case "sensitive":
		return base.Background(ColorError)
	case "admin":
		return base.Background(ColorPrimary)
	default:
		return base.Background(ColorSecondary)
	}
}

// renderScopeBadge renders a scope as a styled pill badge.
func renderScopeBadge(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return ""
	}
	return scopeBadgeStyle(scope).Render(scope)
}

// renderScopeOptions renders render scope options.
func renderScopeOptions(selected []string, options []string, idx int) string {
	if len(options) == 0 {
		options = append([]string{}, selected...)
	}
	if len(options) == 0 {
		return MutedStyle.Render("no scopes available")
	}
	var b strings.Builder
	for i, opt := range options {
		label := opt
		if scopeSelected(selected, opt) {
			label = "[" + opt + "]"
		}
		switch {
		case i == idx:
			b.WriteString(AccentStyle.Render(label))
		case scopeSelected(selected, opt):
			b.WriteString(SelectedStyle.Render(label))
		default:
			b.WriteString(MutedStyle.Render(label))
		}
		if i < len(options)-1 {
			b.WriteString(" ")
		}
	}
	return b.String()
}

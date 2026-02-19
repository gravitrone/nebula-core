package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type MetadataEditor struct {
	Active bool
	Buffer string
	Scopes []string

	scopeOptions   []string
	scopeIdx       int
	scopeSelecting bool
}

func (m *MetadataEditor) Open(initial map[string]any) {
	m.Active = true
	m.Load(initial)
}

func (m *MetadataEditor) Reset() {
	m.Active = false
	m.Buffer = ""
	m.Scopes = nil
	m.scopeIdx = 0
	m.scopeSelecting = false
}

func (m *MetadataEditor) Load(initial map[string]any) {
	m.Scopes = extractMetadataScopes(initial)
	m.Buffer = metadataToInput(stripMetadataScopes(initial))
}

func (m *MetadataEditor) HandleKey(msg tea.KeyMsg) bool {
	if m.scopeSelecting {
		options := m.scopeOptions
		if len(options) == 0 {
			options = append([]string{}, m.Scopes...)
		}
		switch {
		case isKey(msg, "left"):
			if len(options) > 0 {
				m.scopeIdx = (m.scopeIdx - 1 + len(options)) % len(options)
			}
			return false
		case isKey(msg, "right"):
			if len(options) > 0 {
				m.scopeIdx = (m.scopeIdx + 1) % len(options)
			}
			return false
		case isSpace(msg):
			if len(options) > 0 {
				scope := options[m.scopeIdx]
				m.Scopes = toggleScope(m.Scopes, scope)
			}
			return false
		case isEnter(msg), isBack(msg):
			m.scopeSelecting = false
			return false
		}
	}
	switch {
	case isBack(msg):
		m.Active = false
		return true
	case isKey(msg, "s"):
		m.scopeSelecting = true
		return false
	case isSpace(msg):
		if strings.TrimSpace(m.Buffer) == "" {
			m.scopeSelecting = true
			return false
		}
		m.Buffer += " "
		return false
	case isKey(msg, "backspace", "delete"):
		m.Buffer = dropLastRune(m.Buffer)
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		m.Buffer = ""
	case isEnter(msg):
		m.Buffer += "\n"
	case isKey(msg, "tab"):
		m.Buffer += "  "
	default:
		ch := msg.String()
		if len(ch) == 1 {
			m.Buffer += ch
		}
	}
	return false
}

func (m MetadataEditor) Render(width int) string {
	var content strings.Builder
	content.WriteString(MutedStyle.Render("Scopes:"))
	content.WriteString("\n  ")
	if m.scopeSelecting {
		content.WriteString(renderScopeOptions(m.Scopes, m.scopeOptions, m.scopeIdx))
	} else {
		content.WriteString(renderScopePills(m.Scopes, true))
	}
	content.WriteString("\n\n")
	content.WriteString(renderMetadataInput(m.Buffer))
	contentStr := content.String()
	if strings.TrimSpace(contentStr) == "" {
		content.Reset()
		content.WriteString("-")
		contentStr = content.String()
	}
	contentStr += AccentStyle.Render("█")
	hint := MutedStyle.Render("example: profile | timezone | europe/warsaw\nspace scopes  |  tab indent  |  enter newline  |  esc back")
	if _, err := parseMetadataInput(m.Buffer); err != nil {
		hint = hint + "\n" + ErrorStyle.Render(err.Error())
	}
	return components.Indent(components.TitledBox("Metadata", contentStr+"\n\n"+hint, width), 1)
}

func (m *MetadataEditor) SetScopeOptions(options []string) {
	m.scopeOptions = options
	if len(m.scopeOptions) == 0 {
		m.scopeIdx = 0
		return
	}
	if m.scopeIdx >= len(m.scopeOptions) {
		m.scopeIdx = 0
	}
}

func dropLastRune(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(runes[:len(runes)-1])
}

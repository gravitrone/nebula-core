package components

import (
	"charm.land/glamour/v2"
)

// RenderMarkdown renders markdown content for terminal display.
func RenderMarkdown(content string, width int) string {
	if content == "" {
		return ""
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}
	out, err := r.Render(content)
	if err != nil {
		return content
	}
	return out
}

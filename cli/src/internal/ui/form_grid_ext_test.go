package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRenderFormGridClampsWidthAndNormalizesRows(t *testing.T) {
	rows := [][2]string{
		{"Name\nLabel", "  "},
		{"Scopes", "public\nprivate"},
		{"\x1b[31mTag\x1b[0m", "\x1b[31malpha\x1b[0m"},
	}

	out := renderFormGrid("Entity Form", rows, 0, 120)
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "Entity Form")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, clean, "Name Label")
	assert.Contains(t, clean, "│-")
	assert.Contains(t, clean, "public · private")
	assert.Contains(t, clean, "Tag")
	assert.Contains(t, clean, "alpha")

	// Very narrow width should still render a stable table payload.
	narrow := components.SanitizeText(renderFormGrid("Entity Form", rows, 0, 12))
	assert.Contains(t, narrow, "Field")
	assert.Contains(t, narrow, "Value")
}

func TestRenderFormGridHandlesEmptyRowsAndOutOfRangeActiveRow(t *testing.T) {
	out := renderFormGrid("Empty Grid", nil, 999, 100)
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "Empty Grid")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.False(t, strings.Contains(clean, "panic"))
}

package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodyViewportNoClipShortContent(t *testing.T) {
	vp := components.NewNebulaViewport(80, 10)
	body := "line 1\nline 2"
	vp.SetContent(body)
	assert.Equal(t, 2, vp.TotalLineCount())
	assert.True(t, vp.AtTop())
}

func TestBodyViewportScrollsLongContent(t *testing.T) {
	lines := make([]string, 0, 20)
	for i := 1; i <= 20; i++ {
		lines = append(lines, "row "+string('A'+rune(i-1)))
	}
	body := strings.Join(lines, "\n")

	vp := components.NewNebulaViewport(80, 8)
	vp.SetContent(body)
	require.Equal(t, 20, vp.TotalLineCount())
	assert.True(t, vp.AtTop())

	vp.SetYOffset(5)
	assert.False(t, vp.AtTop())
	assert.False(t, vp.AtBottom())

	vp.GotoBottom()
	assert.True(t, vp.AtBottom())
}

func TestBodyViewportClampsNegativeOffset(t *testing.T) {
	vp := components.NewNebulaViewport(80, 6)
	lines := []string{
		"row 1", "row 2", "row 3", "row 4",
		"row 5", "row 6", "row 7", "row 8",
	}
	body := strings.Join(lines, "\n")
	vp.SetContent(body)

	vp.SetYOffset(-42)
	assert.True(t, vp.AtTop())
}

func TestBodyViewportClampsExcessiveOffset(t *testing.T) {
	vp := components.NewNebulaViewport(80, 6)
	lines := make([]string, 0, 10)
	for i := 1; i <= 10; i++ {
		lines = append(lines, "row "+string('A'+rune(i-1)))
	}
	body := strings.Join(lines, "\n")
	vp.SetContent(body)

	vp.SetYOffset(999)
	assert.True(t, vp.AtBottom())
}

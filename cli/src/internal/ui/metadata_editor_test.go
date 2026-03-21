package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMetadataEditorOpenLoadsInitialAndActivates handles test metadata editor open loads initial and activates.
func TestMetadataEditorOpenLoadsInitialAndActivates(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{
		"scopes": []any{"public"},
		"name":   "alex",
	})

	require.True(t, ed.Active)
	assert.Equal(t, []string{"public"}, ed.Scopes)
	assert.Contains(t, ed.Buffer, "name: alex")
}

// TestMetadataEditorHandleKeyTypingScopesAndExit handles test metadata editor handle key typing scopes and exit.
func TestMetadataEditorHandleKeyTypingScopesAndExit(t *testing.T) {
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()

	var copied string
	copyMetadataEditorClipboard = func(text string) error {
		copied = text
		return nil
	}

	var ed MetadataEditor
	ed.Open(map[string]any{})
	ed.SetScopeOptions([]string{"public", "private"})

	// Scope selector opens with s.
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.True(t, ed.scopeSelecting)

	// Move to "private" and toggle it on.
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyRight})
	ed.HandleKey(tea.KeyMsg{Type: tea.KeySpace})
	assert.Contains(t, ed.Scopes, "private")

	// Exit scope selection.
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, ed.scopeSelecting)

	// Add row via entry mode.
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	require.True(t, ed.entryMode)
	for _, ch := range "profile | timezone | europe/warsaw" {
		ed.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, ed.entryMode)
	assert.Contains(t, ed.Buffer, "profile:")
	assert.Contains(t, ed.Buffer, "timezone: europe/warsaw")
	require.Len(t, ed.rows, 1)

	// Enter on selected row opens inspect and Enter copies only value.
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, ed.inspectMode)
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "europe/warsaw", copied)

	// Esc exits inspect, then Esc closes editor and returns done.
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, ed.inspectMode)
	done := ed.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.True(t, done)
	assert.False(t, ed.Active)
}

// TestMetadataEditorRenderIsStable handles test metadata editor render is stable.
func TestMetadataEditorRenderIsStable(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{"name": "alex"})
	out := ed.Render(80)
	clean := components.SanitizeText(out)
	assert.NotContains(t, clean, "Sel")
	assert.Contains(t, clean, "Group")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, clean, "name")
	assert.Contains(t, clean, "alex")
	assert.NotContains(t, clean, ">[")

	ed.inspectMode = true
	ed.inspectRowIdx = 0
	out = ed.Render(80)
	clean = components.SanitizeText(out)
	assert.Contains(t, clean, "enter copy value")
}

// TestMetadataEditorRenderShowsSelectionColumnOnlyAfterSelectingRows handles test metadata editor render shows selection column only after selecting rows.
func TestMetadataEditorRenderShowsSelectionColumnOnlyAfterSelectingRows(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{
		"name": "alex",
		"role": "cto",
	})

	clean := components.SanitizeText(ed.Render(80))
	assert.NotContains(t, clean, "Sel")
	assert.NotContains(t, clean, "[ ]")

	ed.HandleKey(tea.KeyMsg{Type: tea.KeySpace})
	clean = components.SanitizeText(ed.Render(80))
	assert.Contains(t, clean, "Sel")
	assert.Contains(t, clean, "[X]")
}

// TestDropLastRuneHandlesMultibyteRunes handles test drop last rune handles multibyte runes.
func TestDropLastRuneHandlesMultibyteRunes(t *testing.T) {
	assert.Equal(t, "", dropLastRune(""))
	assert.Equal(t, "a", dropLastRune("ab"))
	assert.Equal(t, "a", dropLastRune("a😊"))
}

// TestMetadataEditorToggleSelectAllAndCopySelectedValues ensures bulk select
// and copy flows stay stable across repeated toggles.
func TestMetadataEditorToggleSelectAllAndCopySelectedValues(t *testing.T) {
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()

	var copied string
	copyMetadataEditorClipboard = func(text string) error {
		copied = text
		return nil
	}

	var ed MetadataEditor
	ed.Open(map[string]any{
		"name":  "alex",
		"owner": "alxx",
		"role":  "cto",
	})

	ed.toggleSelectAll()
	assert.Equal(t, len(ed.rows), len(ed.selected))

	count, err := ed.copySelectedValues()
	require.NoError(t, err)
	assert.Equal(t, len(ed.rows), count)
	assert.NotEmpty(t, copied)
	assert.GreaterOrEqual(t, len(splitTrimmedLines(copied)), 3)

	ed.toggleSelectAll()
	assert.Empty(t, ed.selected)
}

// TestMetadataEditorMoveInspectClampsOffset ensures inspect scrolling is always
// bounded to valid visible ranges.
func TestMetadataEditorMoveInspectClampsOffset(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{
		"note": "line 01\nline 02\nline 03\nline 04\nline 05\nline 06\nline 07\nline 08\nline 09\nline 10\nline 11\nline 12\nline 13\nline 14",
	})
	ed.inspectMode = true
	ed.inspectRowIdx = 0

	ed.moveInspect(999)
	lines := ed.inspectLines()
	maxOffset := len(lines) - ed.inspectPageSize()
	if maxOffset < 0 {
		maxOffset = 0
	}
	assert.Equal(t, maxOffset, ed.inspectOffset)

	ed.moveInspect(-999)
	assert.Zero(t, ed.inspectOffset)
}

// TestMetadataEditorRenderEntryModeIncludesFormatHints ensures add/edit entry
// mode keeps structured input hints visible.
func TestMetadataEditorRenderEntryModeIncludesFormatHints(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{})
	ed.entryMode = true
	ed.entryBuf = "profile | timezone | europe/warsaw"
	ed.notice = "invalid value"

	out := components.SanitizeText(ed.Render(90))
	assert.Contains(t, out, "Add Metadata Row")
	assert.Contains(t, out, "format: group | field | value")
	assert.Contains(t, out, "enter save")
	assert.Contains(t, out, "invalid value")
}

// splitTrimmedLines handles split trimmed lines.
func splitTrimmedLines(input string) []string {
	raw := strings.Split(strings.TrimSpace(input), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

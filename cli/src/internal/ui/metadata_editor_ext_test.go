package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataEditorHandleKeyScopeSelectingFallbackOptions(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{"scopes": []any{"public", "private"}})
	ed.scopeSelecting = true
	ed.scopeIdx = 0

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 1, ed.scopeIdx)

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	assert.Equal(t, []string{"public"}, ed.Scopes)

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, ed.scopeSelecting)
}

func TestMetadataEditorHandleKeyEntryModeEditingPaths(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{})
	ed.entryMode = true
	ed.entryBuf = "profile | timezone | europe/warsaw"
	ed.entryEditIdx = -1

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	assert.Contains(t, ed.entryBuf, "!")

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.NotContains(t, ed.entryBuf, "!")

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Equal(t, "", ed.entryBuf)

	ed.entryBuf = "bad line without delimiter"
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, ed.entryMode)
	assert.NotEmpty(t, ed.notice)

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, ed.entryMode)
	assert.Equal(t, "", ed.entryBuf)
	assert.Equal(t, -1, ed.entryEditIdx)
}

func TestMetadataEditorHandleKeyInspectModeCopyErrorPath(t *testing.T) {
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()
	copyMetadataEditorClipboard = func(string) error { return errors.New("clipboard fail") }

	var ed MetadataEditor
	ed.Open(map[string]any{"owner": "alxx"})
	ed.inspectMode = true
	ed.inspectRowIdx = 0

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyDown})
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyUp})
	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Contains(t, ed.notice, "clipboard fail")

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, ed.inspectMode)
	assert.Zero(t, ed.inspectOffset)
}

func TestMetadataEditorHandleKeyDeleteAndCopyNoticePaths(t *testing.T) {
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()
	copyMetadataEditorClipboard = func(string) error { return errors.New("copy exploded") }

	var ed MetadataEditor
	ed.Open(map[string]any{"owner": "alxx", "role": "cto"})

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	assert.Contains(t, ed.notice, "copy exploded")

	ed.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	assert.Equal(t, 1, len(ed.rows))
	assert.Contains(t, ed.notice, "row removed")
}

func TestMetadataEditorSetScopeOptionsResetsOutOfRangeIndex(t *testing.T) {
	var ed MetadataEditor
	ed.scopeIdx = 9
	ed.SetScopeOptions([]string{"public"})
	assert.Equal(t, 0, ed.scopeIdx)

	ed.scopeIdx = 2
	ed.SetScopeOptions(nil)
	assert.Equal(t, 0, ed.scopeIdx)
}

func TestMetadataEditorSyncListCleansInvalidSelectedIndexes(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{{path: "owner", value: "alxx"}},
		selected: map[int]bool{
			0: true,
			9: true,
		},
	}
	ed.syncList()
	assert.True(t, ed.selected[0])
	_, ok := ed.selected[9]
	assert.False(t, ok)
}

func TestMetadataEditorSelectedRowIndexHandlesMissingAndOutOfRange(t *testing.T) {
	ed := MetadataEditor{}
	assert.Equal(t, -1, ed.selectedRowIndex())

	ed.rows = []metadataEditorRow{{path: "owner", value: "alxx"}}
	ed.list = components.NewList(metadataPanelPageSize(false))
	syncMetadataList(ed.list, ed.toDisplayRows(), metadataPanelPageSize(false))
	ed.list.Cursor = 10
	assert.Equal(t, -1, ed.selectedRowIndex())
}

func TestMetadataEditorToggleSelectionInvalidIndexNoop(t *testing.T) {
	ed := MetadataEditor{
		rows:     []metadataEditorRow{{path: "owner", value: "alxx"}},
		selected: map[int]bool{},
	}
	ed.toggleSelection(-1)
	ed.toggleSelection(5)
	assert.Empty(t, ed.selected)

	ed.toggleSelection(0)
	assert.True(t, ed.selected[0])
	ed.toggleSelection(0)
	assert.Empty(t, ed.selected)
}

func TestMetadataEditorCommitEntryDedupesAndEditPath(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: "alxx"},
			{path: "owner", value: "old"},
		},
		entryBuf:     "owner | updated",
		entryEditIdx: -1,
	}
	require.NoError(t, ed.commitEntry())
	require.Len(t, ed.rows, 1)
	assert.Equal(t, "updated", ed.rows[0].value)
	assert.Contains(t, ed.notice, "row saved")

	ed.entryMode = true
	ed.entryEditIdx = 0
	ed.entryBuf = "owner | edited"
	require.NoError(t, ed.commitEntry())
	require.Len(t, ed.rows, 1)
	assert.Equal(t, "edited", ed.rows[0].value)
}

func TestMetadataEditorCommitEntryRejectsInvalidPipeLine(t *testing.T) {
	ed := MetadataEditor{entryBuf: " | value"}
	err := ed.commitEntry()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty key segment")
}

func TestMetadataEditorRebuildBufferHandlesValueParseErrors(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "profile.note", value: "{broken"},
		},
	}
	ed.rebuildBuffer()
	assert.Contains(t, ed.Buffer, "profile:")
	assert.Contains(t, ed.Buffer, "note: {broken")

	ed.rows = nil
	ed.rebuildBuffer()
	assert.Equal(t, "", ed.Buffer)
}

func TestMetadataEditorCopySelectedValuesFallbackAndEmptyHandling(t *testing.T) {
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()

	copied := ""
	copyMetadataEditorClipboard = func(text string) error {
		copied = text
		return nil
	}

	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "owner", value: ""},
		},
		list: components.NewList(metadataPanelPageSize(false)),
	}
	syncMetadataList(ed.list, ed.toDisplayRows(), metadataPanelPageSize(false))

	count, err := ed.copySelectedValues()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, "None", copied)

	ed.selected = map[int]bool{9: true}
	count, err = ed.copySelectedValues()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	ed.selected = map[int]bool{}
	count, err = ed.copySelectedValues()
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestMetadataEditorInspectValueAndLinesGuardBranches(t *testing.T) {
	ed := MetadataEditor{}
	assert.Equal(t, "", ed.inspectValue())
	assert.Nil(t, ed.inspectLines())

	ed.rows = []metadataEditorRow{{path: "profile.note", value: ""}}
	ed.inspectRowIdx = 0
	lines := ed.inspectLines()
	assert.NotEmpty(t, lines)
	assert.Contains(t, lines[len(lines)-1], "None")
}

func TestMetadataEditorInspectLinesPreservesBlankRowsFromMultilineValues(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{{
			path:  "profile.notes",
			value: "line one\n\nline two",
		}},
		inspectRowIdx: 0,
	}

	lines := ed.inspectLines()
	require.NotEmpty(t, lines)

	blankRows := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankRows++
		}
	}
	assert.GreaterOrEqual(t, blankRows, 2)
}

func TestMetadataEditorRenderInspectModeBranchMatrix(t *testing.T) {
	var ed MetadataEditor
	ed.Open(map[string]any{
		"note": "line 01\nline 02\nline 03\nline 04\nline 05\nline 06\nline 07\nline 08\nline 09\nline 10\nline 11\nline 12\nline 13\nline 14\nline 15",
	})
	ed.inspectMode = true
	ed.inspectRowIdx = 0
	ed.inspectOffset = 2
	ed.notice = "copied"

	out := components.SanitizeText(ed.Render(90))
	assert.Contains(t, out, "... ↑ more")
	assert.Contains(t, out, "... ↓ more")
	assert.Contains(t, out, "Lines")
	assert.Contains(t, out, "copied")
	assert.Contains(t, out, "enter copy value")

	ed.inspectRowIdx = 99
	out = components.SanitizeText(ed.Render(90))
	assert.Contains(t, out, "No value")
}

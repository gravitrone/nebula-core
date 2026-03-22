package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetadataEditorHandleKeyBulkCopyAndEditBranches(t *testing.T) {
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()

	copied := ""
	copyMetadataEditorClipboard = func(text string) error {
		copied = text
		return nil
	}

	var ed MetadataEditor
	ed.Open(map[string]any{
		"owner": "alxx",
		"role":  "cto",
	})

	ed.HandleKey(tea.KeyPressMsg{Code: tea.KeyUp})
	ed.HandleKey(tea.KeyPressMsg{Code: tea.KeyDown})

	ed.HandleKey(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.Equal(t, len(ed.rows), len(ed.selected))

	ed.HandleKey(tea.KeyPressMsg{Code: 'c', Text: "c"})
	assert.Contains(t, ed.notice, "copied")
	assert.NotEmpty(t, copied)

	ed.HandleKey(tea.KeyPressMsg{Code: 'e', Text: "e"})
	assert.True(t, ed.entryMode)
	assert.GreaterOrEqual(t, ed.entryEditIdx, 0)
	assert.Contains(t, ed.entryBuf, "|")
}

func TestMetadataEditorInspectEnterCopiesNoneForBlankValues(t *testing.T) {
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()

	copied := ""
	copyMetadataEditorClipboard = func(text string) error {
		copied = text
		return nil
	}

	ed := MetadataEditor{
		inspectMode:   true,
		inspectRowIdx: 0,
		rows: []metadataEditorRow{
			{path: "profile.note", value: "   "},
		},
	}

	done := ed.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, done)
	assert.Equal(t, "None", copied)
	assert.Equal(t, "copied value.", ed.notice)
}

func TestMetadataEditorRenderTableAndEntryBranchMatrix(t *testing.T) {
	ed := MetadataEditor{
		Scopes: []string{"public"},
		notice: "heads up",
	}

	table := components.SanitizeText(ed.renderTableMode(20))
	assert.Contains(t, table, "No metadata")
	assert.Contains(t, table, "rows. Press")
	assert.Contains(t, table, "heads up")
	assert.Contains(t, table, "Scopes")

	ed.entryEditIdx = 0
	ed.entryBuf = "profile | timezone | europe/warsaw"
	entry := components.SanitizeText(ed.renderEntryMode(80))
	assert.Contains(t, entry, "Edit Metadata Row")
	assert.Contains(t, entry, "heads up")
}

func TestMetadataEditorRebuildBufferHandlesInlineObjectErrorBranch(t *testing.T) {
	ed := MetadataEditor{
		rows: []metadataEditorRow{
			{path: "profile.bad", value: "{a:1}"},
		},
	}

	ed.rebuildBuffer()
	assert.Contains(t, ed.Buffer, "profile:")
	assert.Contains(t, ed.Buffer, "bad: {a:1}")
}

func TestMetadataEditorCopySelectedValuesEmptyBranches(t *testing.T) {
	ed := MetadataEditor{}
	count, err := ed.copySelectedValues()
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// With rows set but out-of-range explicit selection and no clipboard, cursor fallback
	// returns 0 when selected map contains only OOB index.
	prevCopy := copyMetadataEditorClipboard
	defer func() { copyMetadataEditorClipboard = prevCopy }()
	copied := ""
	copyMetadataEditorClipboard = func(text string) error { copied = text; return nil }

	// With rows but OOB explicit selection: all indices filtered → 0.
	ed.rows = []metadataEditorRow{{path: "owner", value: "alxx"}}
	ed.syncList()
	ed.selected = map[int]bool{9: true} // OOB only
	count, err = ed.copySelectedValues()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Equal(t, "", copied)
}

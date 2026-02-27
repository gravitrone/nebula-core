package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportExportHandleResourceAndFormatKeysMatrix(t *testing.T) {
	m := NewImportExportModel(nil)
	m.Start(importMode)

	updated, cmd := m.handleResourceKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.resourceIndex)

	updated.resourceIndex = len(updated.resources) - 1
	updated, _ = updated.handleResourceKeys(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, len(updated.resources)-1, updated.resourceIndex)

	updated, _ = updated.handleResourceKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, len(updated.resources)-2, updated.resourceIndex)

	updated, _ = updated.handleResourceKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stepFormat, updated.step)

	updated, _ = updated.handleFormatKeys(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, updated.formatIndex)
	updated, _ = updated.handleFormatKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, updated.formatIndex)
	updated, _ = updated.handleFormatKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stepPath, updated.step)
	updated, _ = updated.handleFormatKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stepResource, updated.step)

	m, _ = m.handleResourceKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.True(t, m.closed)
}

func TestImportExportHandlePathKeysMatrix(t *testing.T) {
	m := NewImportExportModel(nil)
	m.Start(importMode)
	m.step = stepPath

	updated, cmd := m.handlePathKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, stepPath, updated.step)

	updated, _ = updated.handlePathKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	updated, _ = updated.handlePathKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	assert.Equal(t, "ab", updated.path)

	updated, _ = updated.handlePathKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", updated.path)

	updated, _ = updated.handlePathKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, stepFormat, updated.step)
}

func TestRunImportUnknownResourceAndMissingPath(t *testing.T) {
	msg := runImport(nil, "entities", "json", filepath.Join(t.TempDir(), "missing.json"))
	_, ok := msg.(importExportErrorMsg)
	assert.True(t, ok)

	tmp := t.TempDir()
	path := filepath.Join(tmp, "in.json")
	require.NoError(t, os.WriteFile(path, []byte("[]"), 0o644))
	msg = runImport(nil, "unknown", "json", path)
	errMsg, ok := msg.(importExportErrorMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errMsg.err, "unknown import resource")
}

func TestRunImportTruncatesErrorDetails(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/import/entities" {
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"created": 2,
					"failed":  7,
					"errors": []map[string]any{
						{"row": 1, "error": "e1"},
						{"row": 2, "error": "e2"},
						{"row": 3, "error": "e3"},
						{"row": 4, "error": "e4"},
						{"row": 5, "error": "e5"},
						{"row": 6, "error": "e6"},
						{"row": 7, "error": "e7"},
					},
				},
			})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	tmp := t.TempDir()
	path := filepath.Join(tmp, "entities.json")
	require.NoError(t, os.WriteFile(path, []byte("[]"), 0o644))

	msg := runImport(client, "entities", "json", path)
	done, ok := msg.(importExportDoneMsg)
	require.True(t, ok)
	assert.Equal(t, "Created 2, Failed 7", done.summary)
	require.Len(t, done.details, 6)
	assert.Contains(t, done.details[0], "Row 1: e1")
	assert.Contains(t, done.details[5], "...and 2 more errors")
}

func TestRunExportUnknownResourceAndWriteError(t *testing.T) {
	msg := runExport(nil, "unknown", "json", filepath.Join(t.TempDir(), "out.json"))
	errMsg, ok := msg.(importExportErrorMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errMsg.err, "unknown export resource")

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/export/entities" {
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"format":  "csv",
					"content": "id,name\nent-1,Alpha\n",
					"count":   1,
				},
			})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	msg = runExport(client, "entities", "csv", t.TempDir())
	_, ok = msg.(importExportErrorMsg)
	assert.True(t, ok)
}

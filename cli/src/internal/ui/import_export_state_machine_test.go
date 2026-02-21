package ui

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportExportImportFlowReadsFileCallsAPIAndShowsResult handles test import export import flow reads file calls apiand shows result.
func TestImportExportImportFlowReadsFileCallsAPIAndShowsResult(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path == "/api/import/entities" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"created": 1,
					"failed":  0,
					"errors":  []map[string]any{},
					"items":   []map[string]any{{"id": "ent-1"}},
				},
			})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "entities.json")
	require.NoError(t, os.WriteFile(inPath, []byte(`[]`), 0o644))

	m := NewImportExportModel(client)
	m.width = 80
	m.Start(importMode)
	assert.Contains(t, components.SanitizeText(m.View()), "Choose resource")

	// Resource -> Format -> Path.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stepFormat, m.step)
	assert.Contains(t, components.SanitizeText(m.View()), "Choose format")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stepPath, m.step)
	assert.Contains(t, components.SanitizeText(m.View()), "Enter file path")

	// Type path and run.
	for _, r := range inPath {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, stepRunning, m.step)

	msg := cmd()
	m, _ = m.Update(msg)
	assert.Equal(t, stepResult, m.step)

	assert.Equal(t, "/api/import/entities", gotPath)
	assert.Equal(t, "json", gotBody["format"])
	assert.Equal(t, "[]", gotBody["data"])

	out := m.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Created 1, Failed 0")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.True(t, m.closed)
}

// TestImportExportEmptyPathDoesNotRun handles test import export empty path does not run.
func TestImportExportEmptyPathDoesNotRun(t *testing.T) {
	m := NewImportExportModel(nil)
	m.Start(importMode)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stepPath, m.step)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated
	assert.Nil(t, cmd)
	assert.Equal(t, stepPath, m.step)
}

// TestImportExportExportJSONWritesFile handles test import export export jsonwrites file.
func TestImportExportExportJSONWritesFile(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/export/entities" {
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"format": "json",
					"items":  []map[string]any{{"id": "ent-1", "name": "Alpha"}},
					"count":  1,
				},
			})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "entities.json")

	m := NewImportExportModel(client)
	m.width = 80
	m.Start(exportMode)
	assert.Contains(t, components.SanitizeText(m.View()), "Choose resource")

	// Resource -> Format -> Path.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Contains(t, components.SanitizeText(m.View()), "Choose format")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stepPath, m.step)
	assert.Contains(t, components.SanitizeText(m.View()), "Export file path")

	for _, r := range outPath {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	m, _ = m.Update(msg)
	assert.Equal(t, stepResult, m.step)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\"id\": \"ent-1\"")

	clean := components.SanitizeText(m.View())
	assert.Contains(t, clean, "Exported 1 entities")
}

// TestImportExportExportCSVWritesFile handles test import export export csvwrites file.
func TestImportExportExportCSVWritesFile(t *testing.T) {
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

	tmp := t.TempDir()
	outPath := filepath.Join(tmp, "entities.csv")

	m := NewImportExportModel(client)
	m.width = 80
	m.Start(exportMode)
	assert.Contains(t, components.SanitizeText(m.View()), "Choose resource")

	// Resource -> Format (move to csv) -> Path.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Contains(t, components.SanitizeText(m.View()), "Choose format")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, stepPath, m.step)
	assert.Contains(t, components.SanitizeText(m.View()), "Export file path")

	for _, r := range outPath {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	m, _ = m.Update(msg)
	assert.Equal(t, stepResult, m.step)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	assert.Equal(t, "id,name\nent-1,Alpha\n", string(data))
}

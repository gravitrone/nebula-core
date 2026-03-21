package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportExportUpdateHandlesDoneErrorAndResultCloseKeys(t *testing.T) {
	m := NewImportExportModel(nil)
	m.Start(importMode)
	m.step = stepRunning

	updated, cmd := m.Update(importExportDoneMsg{
		summary: "done",
		details: []string{"row 1"},
	})
	require.Nil(t, cmd)
	assert.Equal(t, stepResult, updated.step)
	assert.Equal(t, "done", updated.summary)
	assert.Equal(t, []string{"row 1"}, updated.details)

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.closed)

	updated.closed = false
	updated.step = stepRunning
	updated, cmd = updated.Update(importExportErrorMsg{err: errors.New("boom")})
	require.Nil(t, cmd)
	assert.Equal(t, stepResult, updated.step)
	assert.Equal(t, "boom", updated.errText)

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.True(t, updated.closed)
}

func TestImportExportViewCoversStepBranches(t *testing.T) {
	m := NewImportExportModel(nil)
	m.width = 70
	m.Start(importMode)

	m.step = stepResource

	m.step = stepFormat

	m.step = stepPath
	m.mode = importMode
	assert.Contains(t, components.SanitizeText(m.View()), "Enter file path")

	m.step = stepPath
	m.mode = exportMode
	assert.Contains(t, components.SanitizeText(m.View()), "Export file path")

	m.step = stepRunning
	m.mode = importMode
	assert.Contains(t, components.SanitizeText(m.View()), "Importing...")

	m.step = stepRunning
	m.mode = exportMode
	assert.Contains(t, components.SanitizeText(m.View()), "Exporting...")

	m.step = stepResult
	m.errText = "permission denied"
	assert.Contains(t, components.SanitizeText(m.View()), "Import/Export Failed")
	assert.Contains(t, components.SanitizeText(m.View()), "permission denied")

	m.errText = ""
	m.summary = "Export finished"
	m.details = []string{"line 1", "line 2"}
	rendered := components.SanitizeText(m.View())
	assert.Contains(t, rendered, "Export finished")
	assert.Contains(t, rendered, "line 1")
	assert.Contains(t, rendered, "line 2")

	m.step = importExportStep(99)
	assert.Equal(t, "", m.View())
}

func TestImportExportRenderOptionsAndFormatsWithTinyWidth(t *testing.T) {
	m := NewImportExportModel(nil)
	m.width = 0

	options := []importExportResource{
		{label: "   \n\t", value: "entities"},
	}
	optionsView := m.renderOptions(options, 0)
	assert.Contains(t, components.SanitizeText(optionsView), "enter: select | esc: cancel")

	m.formats = []string{""}
	formatView := m.renderFormatOptions()
	assert.Contains(t, components.SanitizeText(formatView), "enter: select | esc: back")
}

func TestRunImportSupportsAllKnownResources(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		path     string
	}{
		{name: "context", resource: "context", path: "/api/import/context"},
		{name: "relationships", resource: "relationships", path: "/api/import/relationships"},
		{name: "jobs", resource: "jobs", path: "/api/import/jobs"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				err := json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"created": 4,
						"failed":  1,
						"errors": []map[string]any{
							{"row": 7, "error": "bad row"},
						},
					},
				})
				require.NoError(t, err)
			})

			inPath := filepath.Join(t.TempDir(), tc.resource+".json")
			require.NoError(t, os.WriteFile(inPath, []byte("[]"), 0o644))

			msg := runImport(client, tc.resource, "json", inPath)
			done, ok := msg.(importExportDoneMsg)
			require.True(t, ok)
			assert.Equal(t, "Created 4, Failed 1", done.summary)
			assert.Equal(t, []string{"Row 7: bad row"}, done.details)
		})
	}
}

func TestRunImportReturnsErrorOnAPIFailure(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/import/context" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})

	inPath := filepath.Join(t.TempDir(), "context.json")
	require.NoError(t, os.WriteFile(inPath, []byte("[]"), 0o644))

	msg := runImport(client, "context", "json", inPath)
	errMsg, ok := msg.(importExportErrorMsg)
	require.True(t, ok)
	assert.Error(t, errMsg.err)
}

func TestRunExportSupportsAllKnownResources(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		path     string
	}{
		{name: "context", resource: "context", path: "/api/export/context"},
		{name: "relationships", resource: "relationships", path: "/api/export/relationships"},
		{name: "jobs", resource: "jobs", path: "/api/export/jobs"},
		{name: "snapshot", resource: "snapshot", path: "/api/export/snapshot"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				err := json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"format":  "csv",
						"content": "id,name\nx,Name\n",
						"count":   1,
					},
				})
				require.NoError(t, err)
			})

			outPath := filepath.Join(t.TempDir(), tc.resource+".csv")
			msg := runExport(client, tc.resource, "csv", outPath)
			done, ok := msg.(importExportDoneMsg)
			require.True(t, ok)
			assert.Contains(t, done.summary, "Exported 1 "+tc.resource)

			data, err := os.ReadFile(outPath)
			require.NoError(t, err)
			assert.Equal(t, "id,name\nx,Name\n", string(data))
		})
	}
}

func TestRunExportReturnsErrorOnAPIFailure(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/export/context" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})

	msg := runExport(client, "context", "json", filepath.Join(t.TempDir(), "context.json"))
	errMsg, ok := msg.(importExportErrorMsg)
	require.True(t, ok)
	assert.Error(t, errMsg.err)
}

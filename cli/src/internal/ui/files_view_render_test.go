package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesListViewRendersItemsAndSearchSuggestTabCompletes(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/files":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "file-1",
						"filename":   "Alpha.txt",
						"file_path":  "/tmp/alpha.txt",
						"status":     "active",
						"tags":       []string{},
						"metadata":   map[string]any{},
						"created_at": time.Now(),
						"updated_at": time.Now(),
					},
				},
			})
		case "/api/audit/scopes":
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-1", "name": "public", "agent_count": 1},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model.width = 80

	cmd := model.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	model, cmd = model.Update(msg)
	require.NotNil(t, cmd) // loadScopeOptions
	msg = cmd()
	model, _ = model.Update(msg)

	out := model.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Files")
	assert.Contains(t, clean, "1 total")
	assert.Contains(t, clean, "Alpha.txt")
	assert.Contains(t, clean, "Add")
	assert.Contains(t, clean, "Library")

	// Type "a" to trigger search suggest.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.Equal(t, "Alpha.txt", model.searchSuggest)

	// Tab should complete to suggestion.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "Alpha.txt", model.searchBuf)
}

func TestFilesDetailViewRendersMetadataWhenExpanded(t *testing.T) {
	mime := "text/plain"
	size := int64(2048)
	model := NewFilesModel(nil)
	model.width = 80
	model.view = filesViewDetail
	model.detail = &api.File{
		ID:        "file-1",
		Filename:  "Alpha.txt",
		FilePath:  "/tmp/alpha.txt",
		MimeType:  &mime,
		SizeBytes: &size,
		Status:    "active",
		Tags:      []string{"docs"},
		Metadata:  api.JSONMap{"note": "hello"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	model.metaExpanded = true

	out := model.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Alpha.txt")
	assert.Contains(t, clean, "Metadata")
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, clean, "note")
	assert.Contains(t, clean, "hello")
}

func TestFilesAddFlowRendersAndResetsOnEscAfterSave(t *testing.T) {
	createCalled := false
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/files" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
		case r.URL.Path == "/api/files" && r.Method == http.MethodPost:
			createCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":        "file-1",
					"filename":  "Alpha.txt",
					"file_path": "/tmp/alpha.txt",
				},
			})
		case r.URL.Path == "/api/audit/scopes":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model.width = 80
	model, _ = model.Update(filesLoadedMsg{items: []api.File{}})

	// Enter add view.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, filesViewAdd, model.view)

	// Render add form (coverage for renderAdd + helpers).
	assert.Contains(t, components.SanitizeText(model.View()), "Filename")

	// Filename.
	for _, r := range []rune("Alpha.txt") {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Path.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	for _, r := range []rune("/tmp/alpha.txt") {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	// Tags.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	for _, r := range []rune("docs") {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter}) // commit tag
	assert.Contains(t, model.addTags, "docs")

	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.True(t, createCalled)
	assert.True(t, model.addSaved)

	// Esc should reset add state.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, model.addSaved)
	assert.Empty(t, model.addName)
	assert.Empty(t, model.addPath)
}

func TestFilesEditViewCommitsTagsAndRenders(t *testing.T) {
	model := NewFilesModel(nil)
	model.width = 80
	model.view = filesViewEdit
	model.detail = &api.File{
		ID:        "file-1",
		Filename:  "Alpha.txt",
		FilePath:  "/tmp/alpha.txt",
		Status:    "active",
		Tags:      []string{},
		Metadata:  api.JSONMap{},
		CreatedAt: time.Now(),
	}
	model.startEdit()
	model.view = filesViewEdit

	// Move focus to tags field.
	model.editFocus = fileFieldTags
	for _, r := range []rune("alpha") {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Contains(t, model.editTags, "alpha")

	out := model.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "alpha")
}

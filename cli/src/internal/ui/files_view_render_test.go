package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFilesListViewRendersItemsAndSearchSuggestTabCompletes handles test files list view renders items and search suggest tab completes.
func TestFilesListViewRendersItemsAndSearchSuggestTabCompletes(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/files":
			err := json.NewEncoder(w).Encode(map[string]any{
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
			require.NoError(t, err)
		case "/api/audit/scopes":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-1", "name": "public", "agent_count": 1},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model.width = 80

	model, cmd := model.Update(runCmdFirst(model.Init()))
	require.NotNil(t, cmd) // loadScopeOptions
	model, _ = model.Update(cmd())

	out := model.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "1 total")
	assert.Contains(t, clean, "Alpha.txt")
	assert.Contains(t, clean, "Add")
	assert.Contains(t, clean, "Library")

	// Type "a" to trigger search suggest.
	model, _ = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.Equal(t, "Alpha.txt", model.searchSuggest)

	// Tab should complete to suggestion.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, "Alpha.txt", model.searchBuf)
}

// TestFilesDetailViewRendersMetadataWhenExpanded handles test files detail view renders metadata when expanded.
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
	assert.Contains(t, clean, "Field")
	assert.Contains(t, clean, "Value")
	assert.Contains(t, clean, "note")
	assert.Contains(t, clean, "hello")
}

// TestFilesDetailViewRendersRelationshipsSection handles test files detail view renders relationships section.
func TestFilesDetailViewRendersRelationshipsSection(t *testing.T) {
	model := NewFilesModel(nil)
	model.width = 90
	model.view = filesViewDetail
	model.detail = &api.File{
		ID:        "file-1",
		Filename:  "Alpha.txt",
		FilePath:  "/tmp/alpha.txt",
		Status:    "active",
		Metadata:  api.JSONMap{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	model.detailRels = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "entity",
			SourceID:   "ent-1",
			SourceName: "Bro",
			TargetType: "file",
			TargetID:   "file-1",
			TargetName: "Alpha.txt",
			Type:       "has-file",
			Status:     "active",
			CreatedAt:  time.Now(),
		},
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "has-file")
	assert.Contains(t, out, "Bro")
}

// TestFilesAddFlowRendersAndResetsOnEscAfterSave handles test files add flow renders and resets on esc after save.
func TestFilesAddFlowRendersAndResetsOnEscAfterSave(t *testing.T) {
	createCalled := false
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/files" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		case r.URL.Path == "/api/files" && r.Method == http.MethodPost:
			createCalled = true
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":        "file-1",
					"filename":  "Alpha.txt",
					"file_path": "/tmp/alpha.txt",
				},
			})
			require.NoError(t, err)
		case r.URL.Path == "/api/audit/scopes":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model.width = 80
	model, _ = model.Update(filesLoadedMsg{items: []api.File{}})

	// Enter add view.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, filesViewAdd, model.view)

	// Render add form (coverage for renderAdd + helpers).
	assert.Contains(t, components.SanitizeText(model.View()), "Filename")

	// Filename.
	for _, r := range "Alpha.txt" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	// Path.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	for _, r := range "/tmp/alpha.txt" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	// Tags.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	for _, r := range "docs" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // commit tag
	assert.Contains(t, model.addTags, "docs")

	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.True(t, createCalled)
	assert.True(t, model.addSaved)

	// Esc should reset add state.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, model.addSaved)
	assert.Empty(t, model.addName)
	assert.Empty(t, model.addPath)
}

// TestFilesEditViewCommitsTagsAndRenders handles test files edit view commits tags and renders.
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
	for _, r := range "alpha" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Contains(t, model.editTags, "alpha")

	out := model.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "alpha")
}

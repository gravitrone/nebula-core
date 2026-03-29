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
	assert.Equal(t, "Alpha.txt", model.searchInput.Value())
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
		Notes: "hello",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	model.metaExpanded = true

	out := model.View()
	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Alpha.txt")
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
		Notes: "",
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
	out := components.SanitizeText(model.View())
	assert.NotEmpty(t, out)

	// Directly set fields and call saveAdd to exercise the save path.
	model.addName = "Alpha.txt"
	model.addPath = "/tmp/alpha.txt"
	model.addTagStr = "docs"

	updated, cmd := model.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)

	msg := cmd()
	updated, _ = updated.Update(msg)

	assert.True(t, createCalled)
	assert.True(t, updated.addSaved)

	// Esc should reset add state.
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.addSaved)
	assert.Empty(t, updated.addName)
	assert.Empty(t, updated.addPath)
}

// TestFilesEditViewRendersWithTagStr handles test files edit view renders with tag string.
func TestFilesEditViewRendersWithTagStr(t *testing.T) {
	model := NewFilesModel(nil)
	model.width = 80
	model.view = filesViewEdit
	model.detail = &api.File{
		ID:        "file-1",
		Filename:  "Alpha.txt",
		FilePath:  "/tmp/alpha.txt",
		Status:    "active",
		Tags:      []string{"alpha"},
		Notes: "",
		CreatedAt: time.Now(),
	}
	model.startEdit()
	model.view = filesViewEdit

	// startEdit sets editTagStr from Tags.
	assert.Equal(t, "alpha", model.editTagStr)

	out := model.View()
	clean := components.SanitizeText(out)
	assert.NotEmpty(t, clean)
}

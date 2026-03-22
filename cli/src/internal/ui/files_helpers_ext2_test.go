package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesTagStrParsingMatrix(t *testing.T) {
	model := NewFilesModel(nil)

	// Empty string returns nil.
	model.addTagStr = ""
	assert.Nil(t, parseCommaSeparated(model.addTagStr))

	// Single tag.
	model.addTagStr = "alpha"
	tags := parseCommaSeparated(model.addTagStr)
	assert.Equal(t, []string{"alpha"}, tags)

	// Multiple tags.
	model.addTagStr = "alpha, beta"
	tags = parseCommaSeparated(model.addTagStr)
	assert.Equal(t, []string{"alpha", "beta"}, tags)

	// Edit tags same.
	model.editTagStr = "docs, infra"
	tags = parseCommaSeparated(model.editTagStr)
	assert.Equal(t, []string{"docs", "infra"}, tags)
}

func TestFilesSaveAddValidationAndCreateCommand(t *testing.T) {
	now := time.Now()
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/files" && r.Method == http.MethodPost:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":         "file-1",
					"filename":   "Alpha.txt",
					"file_path":  "/tmp/alpha.txt",
					"uri":        "file:///tmp/alpha.txt",
					"status":     "active",
					"tags":       []string{"docs"},
					"metadata":   map[string]any{},
					"created_at": now,
					"updated_at": now,
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)

	updated, cmd := model.saveAdd()
	assert.Nil(t, cmd)
	assert.Equal(t, "Filename is required", updated.addErr)

	updated.addName = "Alpha.txt"
	updated, cmd = updated.saveAdd()
	assert.Nil(t, cmd)
	assert.Equal(t, "File path is required", updated.addErr)

	updated.addPath = "/tmp/alpha.txt"
	updated.addSize = "abc"
	updated, cmd = updated.saveAdd()
	assert.Nil(t, cmd)
	assert.Contains(t, updated.addErr, "non-negative")

	updated.addSize = ""
	updated.addMeta.Buffer = "invalid"
	updated, cmd = updated.saveAdd()
	assert.Nil(t, cmd)
	assert.Contains(t, updated.addErr, "expected 'key: value'")

	updated.addMeta.Buffer = ""
	updated.addTagStr = "docs"
	updated, cmd = updated.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)
	assert.Equal(t, "", updated.addErr)
	_, ok := cmd().(fileCreatedMsg)
	assert.True(t, ok)
}

func TestFilesSaveEditValidationAndUpdateCommandPaths(t *testing.T) {
	now := time.Now()
	_, okClient := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/files/file-1" && r.Method == http.MethodPatch:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":         "file-1",
					"filename":   "Alpha.txt",
					"file_path":  "/tmp/alpha.txt",
					"uri":        "file:///tmp/alpha.txt",
					"status":     "active",
					"tags":       []string{"docs"},
					"metadata":   map[string]any{},
					"created_at": now,
					"updated_at": now,
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(okClient)
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/alpha.txt", Status: "active"}
	model.startEdit()

	model.editSize = "oops"
	updated, cmd := model.saveEdit()
	assert.Nil(t, cmd)
	assert.Contains(t, updated.errText, "non-negative")

	updated.editSize = ""
	updated.editMeta.Buffer = "invalid"
	updated, cmd = updated.saveEdit()
	assert.Nil(t, cmd)
	assert.Contains(t, updated.errText, "expected 'key: value'")

	updated.editMeta.Buffer = ""
	updated.editTagStr = "docs"
	updated, cmd = updated.saveEdit()
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)
	assert.Equal(t, "", updated.errText)
	_, ok := cmd().(fileUpdatedMsg)
	assert.True(t, ok)

	_, failingClient := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	failing := NewFilesModel(failingClient)
	failing.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/alpha.txt", Status: "active"}
	failing.startEdit()
	failing, cmd = failing.saveEdit()
	require.NotNil(t, cmd)
	_, isErr := cmd().(errMsg)
	assert.True(t, isErr)
}

func TestFilesSaveEditIncludesOptionalMimeSizeChecksumFields(t *testing.T) {
	now := time.Now()
	var patchBody map[string]any

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/files/file-1" && r.Method == http.MethodPatch:
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patchBody))
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":         "file-1",
					"filename":   "Alpha.txt",
					"file_path":  "/tmp/alpha.txt",
					"uri":        "file:///tmp/alpha.txt",
					"status":     "active",
					"metadata":   map[string]any{},
					"created_at": now,
					"updated_at": now,
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model.detail = &api.File{
		ID:       "file-1",
		Filename: "Alpha.txt",
		FilePath: "/tmp/alpha.txt",
		Status:   "active",
	}
	model.startEdit()
	model.editName = "   "
	model.editPath = " "
	model.editMime = "application/pdf"
	model.editSize = "7"
	model.editChecksum = "abc123"

	updated, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)
	_, ok := cmd().(fileUpdatedMsg)
	assert.True(t, ok)

	require.NotNil(t, patchBody)
	assert.NotContains(t, patchBody, "filename")
	assert.NotContains(t, patchBody, "file_path")
	assert.Equal(t, "application/pdf", patchBody["mime_type"])
	assert.Equal(t, "abc123", patchBody["checksum"])
	assert.Equal(t, float64(7), patchBody["size_bytes"])
}

func TestFilesLoadScopeOptionsAndEditSavingBranch(t *testing.T) {
	model := NewFilesModel(nil)
	assert.Nil(t, model.loadScopeOptions())

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/audit/scopes":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-private", "name": "private"},
					{"id": "scope-public", "name": "public"},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	model = NewFilesModel(client)
	cmd := model.loadScopeOptions()
	require.NotNil(t, cmd)
	scopesMsg, ok := cmd().(filesScopesLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, []string{"private", "public"}, scopesMsg.options)

	_, failingClient := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	failing := NewFilesModel(failingClient)
	cmd = failing.loadScopeOptions()
	require.NotNil(t, cmd)
	_, isErr := cmd().(errMsg)
	assert.True(t, isErr)

	// editSaving blocks key input.
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/alpha.txt", Status: "active"}
	model.startEdit()
	model.editSaving = true
	updated, cmdMsg := model.handleEditKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Nil(t, cmdMsg)
	assert.True(t, updated.editSaving)
}

package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesStartEditBranchMatrix(t *testing.T) {
	model := NewFilesModel(nil)

	// Nil detail should no-op.
	model.startEdit()
	assert.Equal(t, "", model.editName)
	assert.Equal(t, "", model.editMime)

	// Detail with nil optional pointers should hit fallback branches.
	model.detail = &api.File{
		ID:       "file-1",
		Filename: "alpha.txt",
		FilePath: "/tmp/alpha.txt",
		Status:   "inactive",
		Tags:     []string{"docs"},
		Metadata: api.JSONMap{"k": "v"},
	}
	model.startEdit()
	assert.Equal(t, 0, model.editFocus)
	assert.Equal(t, "alpha.txt", model.editName)
	assert.Equal(t, "/tmp/alpha.txt", model.editPath)
	assert.Equal(t, "", model.editMime)
	assert.Equal(t, "", model.editSize)
	assert.Equal(t, "", model.editChecksum)
	assert.Equal(t, []string{"docs"}, model.editTags)

	// Ensure tags were copied, not aliased.
	model.editTags[0] = "changed"
	assert.Equal(t, []string{"docs"}, model.detail.Tags)

	// Detail with optional pointers should fill values.
	mime := "text/plain"
	size := int64(4096)
	checksum := "deadbeef"
	model.detail = &api.File{
		ID:        "file-2",
		Filename:  "beta.txt",
		FilePath:  "/tmp/beta.txt",
		Status:    "active",
		MimeType:  &mime,
		SizeBytes: &size,
		Checksum:  &checksum,
		Metadata:  api.JSONMap{"a": 1},
	}
	model.startEdit()
	assert.Equal(t, "text/plain", model.editMime)
	assert.Equal(t, "4096", model.editSize)
	assert.Equal(t, "deadbeef", model.editChecksum)
}

func TestFilesRenderEditTagsFocusedAndBufferBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.editTags = []string{"one"}

	focused := components.SanitizeText(model.renderEditTags(true))
	assert.Contains(t, focused, "one")
	assert.Contains(t, focused, "█")

	model.editTagInput.SetValue("tmp")
	focusedWithBuf := components.SanitizeText(model.renderEditTags(true))
	assert.Contains(t, focusedWithBuf, "tmp")
	assert.Contains(t, focusedWithBuf, "█")

	model.editTags = nil
	blurredWithBuf := components.SanitizeText(model.renderEditTags(false))
	assert.Contains(t, blurredWithBuf, "tmp")
}

func TestFilesLoadFilesSuccessAndErrorBranches(t *testing.T) {
	now := time.Now()
	_, okClient := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/files" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assert.Equal(t, "active", r.URL.Query().Get("status_category"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":         "file-1",
					"filename":   "alpha.txt",
					"file_path":  "/tmp/alpha.txt",
					"status":     "active",
					"metadata":   map[string]any{},
					"created_at": now,
					"updated_at": now,
				},
			},
		}))
	})

	successModel := NewFilesModel(okClient)
	cmd := successModel.loadFiles()
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(filesLoadedMsg)
	require.True(t, ok)
	require.Len(t, loaded.items, 1)
	assert.Equal(t, "file-1", loaded.items[0].ID)

	_, failClient := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/files" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	failModel := NewFilesModel(failClient)
	cmd = failModel.loadFiles()
	require.NotNil(t, cmd)
	_, ok = cmd().(errMsg)
	assert.True(t, ok)
}

package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetFile handles test get file.
func TestGetFile(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/files/")

		_, err := w.Write(jsonResponse(map[string]any{
			"id":        "file-1",
			"filename":  "demo.txt",
			"file_path": "/tmp/demo.txt",
		}))
		require.NoError(t, err)
	})

	file, err := client.GetFile("file-1")
	require.NoError(t, err)
	assert.Equal(t, "file-1", file.ID)
	assert.Equal(t, "demo.txt", file.Filename)
}

// TestCreateFile handles test create file.
func TestCreateFile(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/files", r.URL.Path)

		var body CreateFileInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "demo.txt", body.Filename)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":        "file-2",
			"filename":  body.Filename,
			"file_path": body.FilePath,
		}))
		require.NoError(t, err)
	})

	file, err := client.CreateFile(CreateFileInput{
		Filename: "demo.txt",
		FilePath: "/tmp/demo.txt",
	})
	require.NoError(t, err)
	assert.Equal(t, "file-2", file.ID)
}

// TestQueryFiles handles test query files.
func TestQueryFiles(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "application/pdf", r.URL.Query().Get("mime_type"))
		assert.Equal(t, "archived", r.URL.Query().Get("status_category"))
		assert.Equal(t, "tag-1", r.URL.Query().Get("tags"))

		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "file-1", "filename": "demo.pdf"},
			{"id": "file-2", "filename": "spec.pdf"},
		}))
		require.NoError(t, err)
	})

	files, err := client.QueryFiles(QueryParams{
		"mime_type":       "application/pdf",
		"status_category": "archived",
		"tags":            "tag-1",
	})
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

// TestUpdateFile handles test update file.
func TestUpdateFile(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Contains(t, r.URL.Path, "/api/files/")

		var body UpdateFileInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NotNil(t, body.Status)
		assert.Equal(t, "archived", *body.Status)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":       "file-3",
			"filename": "demo.txt",
			"status":   "archived",
		}))
		require.NoError(t, err)
	})

	status := "archived"
	file, err := client.UpdateFile("file-3", UpdateFileInput{Status: &status})
	require.NoError(t, err)
	assert.Equal(t, "archived", file.Status)
}

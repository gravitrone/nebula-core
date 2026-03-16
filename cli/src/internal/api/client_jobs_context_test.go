package api

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateJobEncodesBodyAndDecodesResponse handles test update job encodes body and decodes response.
func TestUpdateJobEncodesBodyAndDecodesResponse(t *testing.T) {
	now := time.Now()
	title := "Updated Title"

	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/jobs/job-1", r.URL.Path)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, title, body["title"])

		_, err := w.Write(jsonResponse(map[string]any{
			"id":          "job-1",
			"title":       title,
			"description": nil,
			"status":      "active",
			"priority":    nil,
			"created_at":  now,
			"updated_at":  now,
		}))
		require.NoError(t, err)
	})

	out, err := client.UpdateJob("job-1", UpdateJobInput{Title: &title})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "job-1", out.ID)
	assert.Equal(t, title, out.Title)
}

// TestGetContextDecodesResponse handles test get context decodes response.
func TestGetContextDecodesResponse(t *testing.T) {
	now := time.Now()

	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/context/kn-1", r.URL.Path)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":          "kn-1",
			"name":        "Doc",
			"source_type": "note",
			"status":      "active",
			"tags":        []string{"docs"},
			"created_at":  now,
			"updated_at":  now,
		}))
		require.NoError(t, err)
	})

	out, err := client.GetContext("kn-1")
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "kn-1", out.ID)
	assert.Equal(t, "Doc", out.Name)
}

// TestUpdateContextEncodesBodyAndDecodesResponse handles test update context encodes body and decodes response.
func TestUpdateContextEncodesBodyAndDecodesResponse(t *testing.T) {
	now := time.Now()
	title := "New Title"

	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/context/kn-1", r.URL.Path)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, title, body["title"])

		_, err := w.Write(jsonResponse(map[string]any{
			"id":          "kn-1",
			"name":        "New Title",
			"source_type": "note",
			"status":      "active",
			"tags":        []string{},
			"created_at":  now,
			"updated_at":  now,
		}))
		require.NoError(t, err)
	})

	out, err := client.UpdateContext("kn-1", UpdateContextInput{
		Title: &title,
	})
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, "kn-1", out.ID)
	assert.Equal(t, "New Title", out.Name)
}

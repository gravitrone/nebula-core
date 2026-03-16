package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateContext handles test create context.
func TestCreateContext(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/context", r.URL.Path)

		var body CreateContextInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "video", body.SourceType)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":          "know-1",
			"title":       body.Title,
			"source_type": body.SourceType,
			"url":         body.URL,
		}))
		require.NoError(t, err)
	})

	context, err := client.CreateContext(CreateContextInput{
		Title:      "Test Video",
		SourceType: "video",
		URL:        "https://youtube.com/watch?v=test",
		Scopes:     []string{"public"},
		Tags:       []string{},
	})
	require.NoError(t, err)
	assert.Equal(t, "know-1", context.ID)
	assert.Equal(t, "video", context.SourceType)
}

// TestQueryContext handles test query context.
func TestQueryContext(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "video", r.URL.Query().Get("source_type"))

		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "k1", "title": "Video 1", "source_type": "video", "url": "url1"},
			{"id": "k2", "title": "Video 2", "source_type": "video", "url": "url2"},
		}))
		require.NoError(t, err)
	})

	items, err := client.QueryContext(QueryParams{"source_type": "video"})
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "video", items[0].SourceType)
}

// TestLinkContext handles test link context.
func TestLinkContext(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/link")

		var body LinkContextInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "entity", body.OwnerType)
		assert.Equal(t, "ent-1", body.OwnerID)

		_, err := w.Write(jsonResponse(map[string]any{}))
		require.NoError(t, err)
	})

	err := client.LinkContext("know-1", LinkContextInput{OwnerType: "entity", OwnerID: "ent-1"})
	require.NoError(t, err)
}

// TestCreateContextMissingURL handles test create context missing url.
func TestCreateContextMissingURL(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "VALIDATION_ERROR",
				"message": "url required for source type video",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.CreateContext(CreateContextInput{
		Title:      "Test",
		SourceType: "video",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "VALIDATION_ERROR")
}

// TestQueryContextEmpty handles test query context empty.
func TestQueryContextEmpty(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(jsonResponse([]map[string]any{}))
		require.NoError(t, err)
	})

	items, err := client.QueryContext(QueryParams{})
	require.NoError(t, err)
	assert.Len(t, items, 0)
}

// TestLinkContextInvalidEntity handles test link context invalid entity.
func TestLinkContextInvalidEntity(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "NOT_FOUND",
				"message": "entity not found",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	err := client.LinkContext("know-1", LinkContextInput{OwnerType: "entity", OwnerID: "invalid-ent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_FOUND")
}

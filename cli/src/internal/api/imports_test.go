package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportEntities handles test import entities.
func TestImportEntities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/import/entities", r.URL.Path)

		var body BulkImportRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "json", body.Format)
		assert.Equal(t, "[]", body.Data)

		_, err := w.Write(jsonResponse(map[string]any{
			"created": 1,
			"failed":  0,
			"errors":  []map[string]any{},
			"items":   []map[string]any{{"id": "ent-1"}},
		}))
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.URL, "nbl_testkey")
	resp, err := client.ImportEntities(BulkImportRequest{
		Format: "json",
		Data:   "[]",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Created)
}

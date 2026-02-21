package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateProtocol handles test create protocol.
func TestCreateProtocol(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/protocols", r.URL.Path)

		var body CreateProtocolInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "proto-1", body.Name)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":      "proto-id",
			"name":    body.Name,
			"title":   body.Title,
			"content": body.Content,
		}))
		require.NoError(t, err)
	})

	proto, err := client.CreateProtocol(CreateProtocolInput{
		Name:    "proto-1",
		Title:   "Protocol One",
		Content: "Body",
		Status:  "active",
	})
	require.NoError(t, err)
	assert.Equal(t, "proto-id", proto.ID)
	assert.Equal(t, "proto-1", proto.Name)
}

// TestQueryProtocols handles test query protocols.
func TestQueryProtocols(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "active", r.URL.Query().Get("status_category"))
		assert.Equal(t, "ops", r.URL.Query().Get("protocol_type"))

		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "p1", "name": "proto-1", "title": "Protocol One"},
			{"id": "p2", "name": "proto-2", "title": "Protocol Two"},
		}))
		require.NoError(t, err)
	})

	items, err := client.QueryProtocols(QueryParams{
		"status_category": "active",
		"protocol_type":   "ops",
	})
	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "proto-1", items[0].Name)
}

// TestGetProtocol handles test get protocol.
func TestGetProtocol(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/protocols/proto-1", r.URL.Path)
		_, err := w.Write(jsonResponse(map[string]any{
			"id":    "p1",
			"name":  "proto-1",
			"title": "Protocol One",
		}))
		require.NoError(t, err)
	})

	proto, err := client.GetProtocol("proto-1")
	require.NoError(t, err)
	assert.Equal(t, "proto-1", proto.Name)
}

// TestUpdateProtocol handles test update protocol.
func TestUpdateProtocol(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/protocols/proto-1", r.URL.Path)

		var body UpdateProtocolInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "Protocol Updated", *body.Title)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":    "p1",
			"name":  "proto-1",
			"title": "Protocol Updated",
		}))
		require.NoError(t, err)
	})

	title := "Protocol Updated"
	proto, err := client.UpdateProtocol("proto-1", UpdateProtocolInput{Title: &title})
	require.NoError(t, err)
	assert.Equal(t, "Protocol Updated", proto.Title)
}

package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip handles round trip.
func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// TestExportMethodsBuildPathQueryAndDecode handles test export methods build path query and decode.
func TestExportMethodsBuildPathQueryAndDecode(t *testing.T) {
	cases := []struct {
		name string
		path string
		call func(c *Client) (*ExportResult, error)
	}{
		{
			name: "entities",
			path: "/api/export/entities",
			call: func(c *Client) (*ExportResult, error) {
				return c.ExportEntities(QueryParams{"format": "json", "limit": "10"})
			},
		},
		{
			name: "context-items",
			path: "/api/export/context",
			call: func(c *Client) (*ExportResult, error) {
				return c.ExportContextItems(QueryParams{"format": "json", "limit": "10"})
			},
		},
		{
			name: "relationships",
			path: "/api/export/relationships",
			call: func(c *Client) (*ExportResult, error) {
				return c.ExportRelationships(QueryParams{"format": "json", "limit": "10"})
			},
		},
		{
			name: "jobs",
			path: "/api/export/jobs",
			call: func(c *Client) (*ExportResult, error) {
				return c.ExportJobs(QueryParams{"format": "json", "limit": "10"})
			},
		},
		{
			name: "context-snapshot",
			path: "/api/export/snapshot",
			call: func(c *Client) (*ExportResult, error) {
				return c.ExportContext(QueryParams{"format": "json", "limit": "10"})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, tc.path, r.URL.Path)
				assert.Equal(t, "json", r.URL.Query().Get("format"))
				assert.Equal(t, "10", r.URL.Query().Get("limit"))

				_, err := w.Write(jsonResponse(map[string]any{
					"format": "json",
					"items":  []map[string]any{{"id": "x"}},
					"count":  1,
				}))
				require.NoError(t, err)
			})

			out, err := tc.call(client)
			require.NoError(t, err)
			require.NotNil(t, out)
			assert.Equal(t, "json", out.Format)
			assert.Equal(t, 1, out.Count)
		})
	}
}

// TestImportMethodsEncodeBodyAndDecode handles test import methods encode body and decode.
func TestImportMethodsEncodeBodyAndDecode(t *testing.T) {
	cases := []struct {
		name string
		path string
		call func(c *Client) (*BulkImportResult, error)
	}{
		{
			name: "context",
			path: "/api/import/context",
			call: func(c *Client) (*BulkImportResult, error) {
				return c.ImportContext(BulkImportRequest{Format: "json", Data: "[]"})
			},
		},
		{
			name: "relationships",
			path: "/api/import/relationships",
			call: func(c *Client) (*BulkImportResult, error) {
				return c.ImportRelationships(BulkImportRequest{Format: "json", Data: "[]"})
			},
		},
		{
			name: "jobs",
			path: "/api/import/jobs",
			call: func(c *Client) (*BulkImportResult, error) {
				return c.ImportJobs(BulkImportRequest{Format: "json", Data: "[]"})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, tc.path, r.URL.Path)

				var body BulkImportRequest
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				assert.Equal(t, "json", body.Format)
				assert.Equal(t, "[]", body.Data)

				_, err := w.Write(jsonResponse(map[string]any{
					"created": 1,
					"failed":  0,
					"errors":  []map[string]any{},
					"items":   []map[string]any{{"id": "ok"}},
				}))
				require.NoError(t, err)
			})

			out, err := tc.call(client)
			require.NoError(t, err)
			require.NotNil(t, out)
			assert.Equal(t, 1, out.Created)
		})
	}
}

// TestBulkUpdateCallsEncodeBodyAndDecode handles test bulk update calls encode body and decode.
func TestBulkUpdateCallsEncodeBodyAndDecode(t *testing.T) {
	cases := []struct {
		name string
		path string
		call func(c *Client) (*BulkUpdateResult, error)
	}{
		{
			name: "tags",
			path: "/api/entities/bulk/tags",
			call: func(c *Client) (*BulkUpdateResult, error) {
				return c.BulkUpdateEntityTags(BulkUpdateEntityTagsInput{
					EntityIDs: []string{"e1", "e2"},
					Tags:      []string{"t"},
					Op:        "add",
				})
			},
		},
		{
			name: "scopes",
			path: "/api/entities/bulk/scopes",
			call: func(c *Client) (*BulkUpdateResult, error) {
				return c.BulkUpdateEntityScopes(BulkUpdateEntityScopesInput{
					EntityIDs: []string{"e1"},
					Scopes:    []string{"public"},
					Op:        "add",
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, tc.path, r.URL.Path)

				// Keep this minimal: just confirm the payload contains the expected operation.
				var raw map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&raw))
				assert.Equal(t, "add", raw["op"])

				_, err := w.Write(jsonResponse(map[string]any{
					"updated":    2,
					"entity_ids": []string{"e1", "e2"},
				}))
				require.NoError(t, err)
			})

			out, err := tc.call(client)
			require.NoError(t, err)
			require.NotNil(t, out)
			assert.Equal(t, 2, out.Updated)
		})
	}
}

// TestClientErrorEnvelopeReturnsCodeMessage handles test client error envelope returns code message.
func TestClientErrorEnvelopeReturnsCodeMessage(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte(`{"error":{"code":"BAD_REQUEST","message":"nope"}}`))
		require.NoError(t, err)
	})

	_, err := client.ExportEntities(QueryParams{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "BAD_REQUEST: nope")
}

// TestClientTransportFailureSurfacesDeterministicError handles test client transport failure surfaces deterministic error.
func TestClientTransportFailureSurfacesDeterministicError(t *testing.T) {
	client := NewClient("http://example.com", "nbl_testkey")
	client.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})

	_, err := client.ExportEntities(QueryParams{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "request failed:")
	assert.ErrorContains(t, err, "boom")
}

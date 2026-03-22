package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFilesClient handles test files client.
func testFilesClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestFilesInitLoadsFilesAndScopes handles test files init loads files and scopes.
func TestFilesInitLoadsFilesAndScopes(t *testing.T) {
	now := time.Now()
	_, client := testFilesClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/files") && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "file-1",
						"filename":   "spec.md",
						"file_path":  "/vault/spec.md",
						"status":     "active",
						"tags":       []string{},
						"metadata":   map[string]any{},
						"created_at": now,
						"updated_at": now,
					},
				},
			}))
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":            "scope-1",
						"name":          "public",
						"description":   nil,
						"agent_count":   0,
						"entity_count":  0,
						"context_count": 0,
					},
				},
			}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model, cmd := model.Update(runCmdFirst(model.Init()))

	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.False(t, model.loading)
	assert.Len(t, model.items, 1)
	assert.Equal(t, "file-1", model.items[0].ID)
	assert.Contains(t, model.scopeOptions, "public")
}

// TestFilesAddValidationErrorOnEmpty handles test files add validation error on empty.
func TestFilesAddValidationErrorOnEmpty(t *testing.T) {
	_, client := testFilesClient(t, func(w http.ResponseWriter, r *http.Request) {})
	model := NewFilesModel(client)
	model.view = filesViewAdd

	model, _ = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.Equal(t, "Filename is required", model.addErr)
}

// TestFilesUpdateHandlesAPIError handles test files update handles apierror.
func TestFilesUpdateHandlesAPIError(t *testing.T) {
	now := time.Now()
	_, client := testFilesClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/files") && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "file-1",
						"filename":   "spec.md",
						"file_path":  "/vault/spec.md",
						"status":     "active",
						"tags":       []string{},
						"metadata":   map[string]any{},
						"created_at": now,
						"updated_at": now,
					},
				},
			}))
			return
		case strings.HasPrefix(r.URL.Path, "/api/files/") && r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusBadRequest)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "FAIL", "message": "nope"},
			}))
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model, cmd := model.Update(runCmdFirst(model.Init()))
	msg := cmd()
	model, _ = model.Update(msg)

	// Enter detail
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, model.detail)

	// Enter edit
	model, _ = model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	assert.Equal(t, filesViewEdit, model.view)

	// Attempt save; server returns FAIL.
	model, cmd = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.Contains(t, model.errText, "FAIL")
}

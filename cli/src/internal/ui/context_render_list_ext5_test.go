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

func TestContextRenderListLoadingAndEmptyBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 80
	model.loadingList = true

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "Loading context")

	model.loadingList = false
	out = components.SanitizeText(model.renderList())
	assert.Contains(t, out, "No context found")
}

func TestContextRenderListPreviewAndFilterBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 160
	model.filterInput.SetValue("alpha")
	model.modeFocus = true

	now := time.Now().UTC()
	url := "https://example.com/docs/context"
	item := api.Context{
		ID:        "ctx-1",
		Title:     "Alpha Context",
		URL:       &url,
		CreatedAt: now,
	}
	model.items = []api.Context{item}
	model.list.SetItems([]string{formatContextLine(item)})

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "filter: alpha")
	assert.Contains(t, out, "Alpha Context")
	assert.Contains(t, out, "note")
	assert.Contains(t, out, "Selected")
}

func TestContextLoadContextListAndDetailBranches(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)

	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/context":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id":          "ctx-1",
					"title":       "Alpha Context",
					"source_type": "note",
					"status":      "active",
					"tags":        []string{"demo"},
					"created_at":  now,
					"updated_at":  now,
				}},
			}))
		case "/api/context/ctx-1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          "ctx-1",
					"title":       "Alpha Context",
					"source_type": "note",
					"status":      "active",
					"tags":        []string{"demo"},
					"created_at":  now,
					"updated_at":  now,
				},
			}))
		case "/api/relationships/context/ctx-1":
			http.Error(w, `{"error":{"code":"REL_FAIL","message":"boom"}}`, http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewContextModel(client)

	cmd := model.loadContextList()
	require.NotNil(t, cmd)
	msg := cmd()
	listLoaded, ok := msg.(contextListLoadedMsg)
	require.True(t, ok)
	require.Len(t, listLoaded.items, 1)
	assert.Equal(t, "ctx-1", listLoaded.items[0].ID)

	cmd = model.loadContextDetail("")
	require.NotNil(t, cmd)
	msg = cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "context id is required")

	cmd = model.loadContextDetail("ctx-1")
	require.NotNil(t, cmd)
	msg = cmd()
	detailLoaded, ok := msg.(contextDetailLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "ctx-1", detailLoaded.item.ID)
	assert.Nil(t, detailLoaded.relationships)
}

func TestContextLoadContextListErrorBranch(t *testing.T) {
	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/context" {
			http.Error(w, `{"error":{"code":"CTX_FAIL","message":"context query failed"}}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewContextModel(client)
	cmd := model.loadContextList()
	require.NotNil(t, cmd)

	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "CTX_FAIL")
}

func TestContextRenderContextPreviewWidthAndURLFallbackBranches(t *testing.T) {
	model := NewContextModel(nil)
	assert.Equal(t, "", model.renderContextPreview(api.Context{}, 0))

	now := time.Now().UTC()
	url := "https://example.com/really/long/context/url"
	item := api.Context{
		ID:        "ctx-1",
		Title:     "Alpha Context",
		URL:       &url,
		CreatedAt: now,
	}
	model.detail = &item
	model.detailRelationships = []api.Relationship{{ID: "rel-1"}}

	preview := components.SanitizeText(model.renderContextPreview(item, 44))
	assert.Contains(t, preview, "Links")
	assert.Contains(t, preview, "Preview")
	assert.Contains(t, preview, "https://example.com")
}

func TestFormatContextLineFallbackBranches(t *testing.T) {
	url := "https://example.com/path"
	line := components.SanitizeText(formatContextLine(api.Context{URL: &url}))

	assert.Contains(t, line, "(untitled)")
	assert.Contains(t, line, "note")
	assert.Contains(t, line, "https://example.com")
}

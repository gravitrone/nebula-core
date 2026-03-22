package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextAddSaveAndReset verifies that the add flow saves a context and resets on Esc.
func TestContextAddSaveAndReset(t *testing.T) {
	now := time.Now()
	createCalled := false
	linkCalled := false

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/audit/scopes":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-1", "name": "public", "agent_count": 1},
				},
			}))
		case r.URL.Path == "/api/context" && r.Method == http.MethodPost:
			createCalled = true
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                "k-1",
					"name":              "Alpha",
					"source_type":       "note",
					"status":            "active",
					"tags":              []string{"demo"},
					"privacy_scope_ids": []string{"scope-1"},
					"created_at":        now,
					"updated_at":        now,
				},
			}))
		case r.URL.Path == "/api/context/k-1/link" && r.Method == http.MethodPost:
			linkCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewContextModel(client)
	model.width = 90

	// Init + load scopes.
	cmd := model.Init()
	require.NotNil(t, cmd)
	model, _ = model.Update(runCmdFirst(cmd))
	assert.Contains(t, model.scopeOptions, "public")

	// Set add form fields directly (huh forms don't support programmatic key navigation).
	model.addTitle = "Alpha"
	model.addTagStr = "demo"
	model.addScopeStr = "public"
	model.addType = "note"

	// Add a linked entity.
	model.linkEntities = []api.Entity{{ID: "ent-1", Name: "OpenAI"}}
	assert.Contains(t, components.SanitizeText(model.View()), "OpenAI")

	// Trigger save directly.
	var saveCmd tea.Cmd
	model, saveCmd = model.save()
	require.NotNil(t, saveCmd)
	saveMsg := saveCmd()
	model, _ = model.Update(saveMsg)

	assert.True(t, createCalled)
	assert.True(t, linkCalled)
	assert.True(t, model.saved)
	assert.Contains(t, components.SanitizeText(model.View()), "Context saved!")

	// Esc should reset add state.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, model.saved)
	assert.Equal(t, "", model.addTitle)
}

// TestContextLibraryDetailEditAndSave handles test context library detail edit and save.
func TestContextLibraryDetailEditAndSave(t *testing.T) {
	now := time.Now()
	updateCalled := false
	vaultPath := "/vault/context/alpha.md"
	content := "notes"
	url := "https://example.com"

	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/audit/scopes":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-1", "name": "public", "agent_count": 1},
				},
			}))
		case r.URL.Path == "/api/context" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":                "k-1",
						"name":              "Alpha",
						"url":               url,
						"source_type":       "note",
						"content":           content,
						"privacy_scope_ids": []string{"scope-1"},
						"status":            "active",
						"tags":              []string{"demo"},
						"source_path":       vaultPath,
						"created_at":        now,
						"updated_at":        now,
					},
				},
			}))
		case r.URL.Path == "/api/context/k-1" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                "k-1",
					"name":              "Alpha",
					"url":               url,
					"source_type":       "note",
					"content":           content,
					"privacy_scope_ids": []string{"scope-1"},
					"status":            "active",
					"tags":              []string{"demo"},
					"source_path":       vaultPath,
					"created_at":        now,
					"updated_at":        now,
				},
			}))
		case r.URL.Path == "/api/context/k-1" && r.Method == http.MethodPatch:
			updateCalled = true
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                "k-1",
					"name":              "Alpha",
					"url":               url,
					"source_type":       "note",
					"content":           content,
					"privacy_scope_ids": []string{"scope-1"},
					"status":            "active",
					"tags":              []string{"demo", "new"},
					"source_path":       vaultPath,
					"created_at":        now,
					"updated_at":        now,
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewContextModel(client)
	model.width = 90

	// Init + load scopes.
	cmd := model.Init()
	require.NotNil(t, cmd)
	model, _ = model.Update(runCmdFirst(cmd))

	// Toggle to Library view directly (huh form captures KeyUp in add view).
	model, cmd = model.toggleMode()
	require.NotNil(t, cmd)
	model, _ = model.Update(runCmdFirst(cmd))
	assert.Equal(t, contextViewList, model.view)

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "Alpha")

	// Open detail and load it.
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	model, _ = model.Update(runCmdFirst(cmd))
	assert.Equal(t, contextViewDetail, model.view)

	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Title")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "Source Path")

	// Enter edit mode.
	model, _ = model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	assert.Equal(t, contextViewEdit, model.view)

	// Set edit tag directly (huh forms don't support programmatic field navigation).
	model.editTagStr = "demo, new"

	// Trigger save directly.
	var saveCmd tea.Cmd
	model, saveCmd = model.saveEdit()
	require.NotNil(t, saveCmd)
	saveMsg := saveCmd()
	model, _ = model.Update(saveMsg)

	assert.True(t, updateCalled)
	assert.Equal(t, contextViewDetail, model.view)
}

// TestContextDetailAndPreviewShowRelationshipSummary handles test context detail and preview show relationship summary.
func TestContextDetailAndPreviewShowRelationshipSummary(t *testing.T) {
	now := time.Now()
	model := NewContextModel(nil)
	model.width = 100
	model.scopeNames = map[string]string{"scope-1": "public"}

	item := api.Context{
		ID:              "ctx-1",
		Name:            "Context Alpha",
		SourceType:      "note",
		Status:          "active",
		PrivacyScopeIDs: []string{"scope-1"},
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	model.detail = &item
	model.detailRelationships = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "context",
			SourceID:   "ctx-1",
			SourceName: "Context Alpha",
			TargetType: "entity",
			TargetID:   "ent-1",
			TargetName: "Bro",
			Type:       "references",
			Status:     "active",
		},
	}

	detail := components.SanitizeText(model.renderDetail())
	assert.Contains(t, detail, "references")
	assert.Contains(t, detail, "Bro")

	preview := components.SanitizeText(model.renderContextPreview(item, 42))
	assert.Contains(t, preview, "Links")
	assert.Contains(t, preview, "1")
}

// TestContextRenderLinkEntityPreviewShowsCoreFields handles test context render link entity preview shows core fields.
func TestContextRenderLinkEntityPreviewShowsCoreFields(t *testing.T) {
	model := NewContextModel(nil)
	preview := components.SanitizeText(
		model.renderLinkEntityPreview(
			api.Entity{
				ID:     "ent-1",
				Name:   "Alpha",
				Type:   "person",
				Status: "active",
				Tags:   []string{"core"},
			},
			48,
		),
	)

	assert.Contains(t, preview, "Selected")
	assert.Contains(t, preview, "Alpha")
	assert.Contains(t, preview, "Type")
	assert.Contains(t, preview, "Status")
}

// TestContextAddLinkSearchIntegration verifies link search selects entities and renders them.
func TestContextAddLinkSearchIntegration(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/entities") && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "OpenAI", "type": "organization", "status": "active", "tags": []string{}},
				},
			}))
		case r.URL.Path == "/api/audit/scopes":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewContextModel(client)
	model.width = 90
	model.startLinkSearch()
	assert.True(t, model.linkSearching)

	// Type a query and run the search command.
	var searchCmd tea.Cmd
	for _, r := range "Open" {
		model, searchCmd = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	require.NotNil(t, searchCmd)
	msg := searchCmd()
	model, _ = model.Update(msg)
	assert.False(t, model.linkLoading)
	assert.Len(t, model.linkResults, 1)

	// Select first result.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, model.linkSearching)
	assert.Len(t, model.linkEntities, 1)
	assert.Contains(t, components.SanitizeText(model.renderLinkedEntities()), "OpenAI")
}

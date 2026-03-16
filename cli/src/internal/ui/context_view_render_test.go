package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextAddLinkSearchSaveAndReset handles test context add link search save and reset.
func TestContextAddLinkSearchSaveAndReset(t *testing.T) {
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
		case strings.HasPrefix(r.URL.Path, "/api/entities") && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "OpenAI", "type": "organization", "status": "active", "tags": []string{}},
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
	msg := cmd()
	model, _ = model.Update(msg)
	assert.Contains(t, model.scopeOptions, "public")

	// Move focus to Entities field and start link search.
	for i := 0; i < fieldEntities; i++ {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, fieldEntities, model.focus)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, model.linkSearching)

	// Type a query and run the search command.
	var searchCmd tea.Cmd
	for _, r := range "Open" {
		model, searchCmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	require.NotNil(t, searchCmd)
	msg = searchCmd()
	model, _ = model.Update(msg)
	assert.False(t, model.linkLoading)
	assert.Len(t, model.linkResults, 1)

	// Select first result.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, model.linkSearching)
	assert.Len(t, model.linkEntities, 1)
	assert.Contains(t, components.SanitizeText(model.View()), "OpenAI")

	// Fill title.
	model.focus = fieldTitle
	for _, r := range "Alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Commit a tag.
	model.focus = fieldTags
	for _, r := range "demo" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Contains(t, model.tags, "demo")

	// Select a scope via selector.
	model.focus = fieldScopes
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace}) // enter selector
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace}) // toggle current scope
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter}) // exit selector
	assert.Contains(t, model.scopes, "public")

	// Save context (Create + Link).
	var saveCmd tea.Cmd
	model, saveCmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, saveCmd)
	msg = saveCmd()
	model, _ = model.Update(msg)

	assert.True(t, createCalled)
	assert.True(t, linkCalled)
	assert.True(t, model.saved)
	assert.Contains(t, components.SanitizeText(model.View()), "Context saved!")

	// Esc should reset add state.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, model.saved)
	assert.Equal(t, "", model.fields[fieldTitle].value)
	assert.Len(t, model.tags, 0)
	assert.Len(t, model.scopes, 0)
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
	msg := cmd()
	model, _ = model.Update(msg)

	// Toggle to Library view via modeFocus.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, model.modeFocus)
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, contextViewList, model.view)

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "1 total")
	assert.Contains(t, out, "Alpha")

	// Open detail and load it.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, contextViewDetail, model.view)

	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Title")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "Source Path")

	// Enter edit mode.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	assert.Equal(t, contextViewEdit, model.view)

	// Add a tag and save.
	model.editFocus = contextEditFieldTags
	for _, r := range "new" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Contains(t, model.editTags, "new")

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

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

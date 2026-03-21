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
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProtocolsClient handles test protocols client.
func testProtocolsClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestProtocolsListToDetailToEditFlow handles test protocols list to detail to edit flow.
func TestProtocolsListToDetailToEditFlow(t *testing.T) {
	now := time.Now()
	_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/protocols") && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "proto-1",
						"name":       "p1",
						"title":      "Protocol 1",
						"content":    "hello",
						"status":     "active",
						"tags":       []string{},
						"metadata":   map[string]any{},
						"created_at": now,
						"updated_at": now,
					},
				},
			})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewProtocolsModel(client)
	cmd := model.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.False(t, model.loading)
	assert.Len(t, model.items, 1)

	// Enter detail
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, model.detail)
	assert.Equal(t, protocolsViewDetail, model.view)

	// Enter edit
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	assert.Equal(t, protocolsViewEdit, model.view)
}

// TestProtocolsAddValidationErrorOnEmpty handles test protocols add validation error on empty.
func TestProtocolsAddValidationErrorOnEmpty(t *testing.T) {
	_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {})
	model := NewProtocolsModel(client)
	model.view = protocolsViewAdd

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	assert.Equal(t, "Name is required", model.addErr)
}

// TestProtocolsDetailRendersRelationshipsSection handles test protocols detail renders relationships section.
func TestProtocolsDetailRendersRelationshipsSection(t *testing.T) {
	now := time.Now()
	content := "rules"
	model := NewProtocolsModel(nil)
	model.width = 100
	model.view = protocolsViewDetail
	model.detail = &api.Protocol{
		ID:        "proto-1",
		Name:      "policy",
		Title:     "Policy",
		Content:   &content,
		Status:    "active",
		Metadata:  api.JSONMap{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	model.detailRels = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "protocol",
			SourceID:   "proto-1",
			SourceName: "Policy",
			TargetType: "job",
			TargetID:   "2026Q1-ABCD",
			TargetName: "Sprint Job",
			Type:       "references",
			Status:     "active",
			CreatedAt:  now,
		},
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "references")
	assert.Contains(t, out, "Sprint Job")
}

// TestProtocolsAddFlowSubmitsCreateProtocol handles test protocols add flow submits create protocol.
func TestProtocolsAddFlowSubmitsCreateProtocol(t *testing.T) {
	now := time.Now()
	var created api.CreateProtocolInput
	var posted bool

	_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/protocols") && r.Method == http.MethodGet:
			// Used both for initial load and post-create reload.
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":         "proto-1",
				"name":       "p1",
				"title":      "Protocol 1",
				"content":    "hello",
				"status":     "active",
				"tags":       []string{"t1"},
				"metadata":   map[string]any{},
				"created_at": now,
				"updated_at": now,
			}}})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/protocols" && r.Method == http.MethodPost:
			posted = true
			require.NoError(t, json.NewDecoder(r.Body).Decode(&created))
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "proto-1"}})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewProtocolsModel(client)
	cmd := model.Init()
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	// Enter Add.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, protocolsViewAdd, model.view)

	// Name.
	for _, r := range "p1" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Title.
	for _, r := range "Protocol 1" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Jump to Applies and Tags, leaving buffers uncommitted so saveAdd() commits them.
	for i := 0; i < 3; i++ { // Version, Type, Applies To
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, protoFieldApplies, model.addFocus)
	for _, r := range "entity" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown}) // Status
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown}) // Tags
	assert.Equal(t, protoFieldTags, model.addFocus)
	for _, r := range "t1" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Content (required).
	for i := 0; i < 1; i++ { // Content
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, protoFieldContent, model.addFocus)
	for _, r := range "hello" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Save.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	require.True(t, posted)
	assert.Equal(t, "p1", created.Name)
	assert.Equal(t, "Protocol 1", created.Title)
	assert.Equal(t, "hello", created.Content)
	assert.Equal(t, []string{"entity"}, created.AppliesTo)
	assert.Equal(t, []string{"t1"}, created.Tags)
}

// TestProtocolsEditFlowCommitsTagAndApplyAndSaves handles test protocols edit flow commits tag and apply and saves.
func TestProtocolsEditFlowCommitsTagAndApplyAndSaves(t *testing.T) {
	now := time.Now()
	var patched api.UpdateProtocolInput
	var patchedName string

	_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/protocols") && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":         "proto-1",
				"name":       "p1",
				"title":      "Protocol 1",
				"content":    "hello",
				"status":     "active",
				"tags":       []string{},
				"metadata":   map[string]any{},
				"created_at": now,
				"updated_at": now,
			}}})
			require.NoError(t, err)
			return
		case strings.HasPrefix(r.URL.Path, "/api/protocols/") && r.Method == http.MethodPatch:
			patchedName = strings.TrimPrefix(r.URL.Path, "/api/protocols/")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "proto-1"}})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewProtocolsModel(client)
	cmd := model.Init()
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	// List -> detail -> edit.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, model.detail)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	assert.Equal(t, protocolsViewEdit, model.view)

	// Focus Applies To and type buf (commit happens on save).
	for i := 0; i < protoEditFieldApplies; i++ {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, protoEditFieldApplies, model.editFocus)
	for _, r := range "entity" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Focus Tags and type buf.
	for i := model.editFocus; i < protoEditFieldTags; i++ {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, protoEditFieldTags, model.editFocus)
	for _, r := range "t2" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.Equal(t, "p1", patchedName)
	require.NotNil(t, patched.AppliesTo)
	assert.Equal(t, []string{"entity"}, *patched.AppliesTo)
	require.NotNil(t, patched.Tags)
	assert.Equal(t, []string{"t2"}, *patched.Tags)
	assert.Equal(t, protocolsViewList, model.view)
}

// TestProtocolPtrHelpers handles test protocol ptr helpers.
func TestProtocolPtrHelpers(t *testing.T) {
	assert.Nil(t, stringPtr(""))
	assert.Nil(t, stringPtr("  "))
	require.NotNil(t, stringPtr("x"))
	assert.Equal(t, "x", *stringPtr("x"))

	assert.Nil(t, slicePtr(nil))
	assert.Nil(t, slicePtr([]string{}))
	require.NotNil(t, slicePtr([]string{"a"}))
	assert.Equal(t, []string{"a"}, *slicePtr([]string{"a"}))
}

// TestProtocolsRenderHelpersCoverListAddAndEdit handles test protocols render helpers cover list add and edit.
func TestProtocolsRenderHelpersCoverListAddAndEdit(t *testing.T) {
	now := time.Now()
	content := "protocol content"
	version := "v1"
	typ := "policy"

	model := NewProtocolsModel(nil)
	model.width = 96
	model.view = protocolsViewList
	model.items = []api.Protocol{
		{
			ID:           "proto-1",
			Name:         "alpha",
			Title:        "Alpha Protocol",
			Content:      &content,
			Status:       "active",
			Version:      &version,
			ProtocolType: &typ,
			AppliesTo:    []string{"entity"},
			Tags:         []string{"core"},
			Metadata:     api.JSONMap{"scope": "public"},
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	model.list.SetItems([]string{"alpha"})

	modeLine := components.SanitizeText(model.renderModeLine())
	assert.Contains(t, modeLine, "Library")

	listView := components.SanitizeText(model.renderList())
	assert.Contains(t, listView, "Alpha Protocol")

	preview := components.SanitizeText(model.renderProtocolPreview(model.items[0], 48))
	assert.Contains(t, preview, "Selected")
	assert.Contains(t, preview, "Name")
	assert.Contains(t, preview, "alpha")

	model.view = protocolsViewAdd
	model.addFocus = protoFieldStatus
	addView := components.SanitizeText(model.renderAdd())
	assert.Contains(t, addView, "Status")

	model.detail = &model.items[0]
	model.startEdit()
	editView := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editView, "Tags")

	assert.Equal(t, "a, b", model.renderTags([]string{"a"}, "b"))
	assert.Equal(t, "entity, job", model.renderApplies([]string{"entity"}, "job"))
}

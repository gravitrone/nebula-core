package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// relTestClient handles rel test client.
func relTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestRelationshipsInitLoadsNames handles test relationships init loads names.
func TestRelationshipsInitLoadsNames(t *testing.T) {
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/relationships":
			resp := map[string]any{
				"data": []map[string]any{
					{"id": "rel-1", "source_id": "ent-1", "target_id": "ent-2", "relationship_type": "uses", "properties": map[string]any{}},
				},
			}
			require.NoError(t, json.NewEncoder(w).Encode(resp))
		case "/api/entities/ent-1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ent-1", "name": "Nebula", "tags": []string{}}}))
		case "/api/entities/ent-2":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ent-2", "name": "Postgres", "tags": []string{}}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewRelationshipsModel(client)
	cmds := []tea.Cmd{
		model.loadRelationships(),
		model.loadScopeOptions(),
		model.loadEntityCache(),
	}
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		model = applyMsg(model, cmd())
	}

	require.Len(t, model.list.Items, 1)
	assert.Contains(t, model.list.Items[0], "uses · Nebula -> Postgres")
}

// applyMsg handles apply msg.
func applyMsg(model RelationshipsModel, msg tea.Msg) RelationshipsModel {
	var cmd tea.Cmd
	model, cmd = model.Update(msg)
	if cmd == nil {
		return model
	}
	next := cmd()
	if next == nil {
		return model
	}
	return applyMsg(model, next)
}

// TestRelationshipsCreateSubmitCallsAPI handles test relationships create submit calls api.
func TestRelationshipsCreateSubmitCallsAPI(t *testing.T) {
	var captured api.CreateRelationshipInput
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/relationships" && r.Method == http.MethodPost {
			var body api.CreateRelationshipInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			captured = body
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "rel-1"}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewRelationshipsModel(client)
	model.view = relsViewCreateType
	model.createSource = &relationshipCreateCandidate{ID: "ent-1", NodeType: "entity", Kind: "entity/tool"}
	model.createTarget = &relationshipCreateCandidate{ID: "job-2", NodeType: "job", Kind: "job/high"}
	model.createType = "uses"

	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "ent-1", captured.SourceID)
	assert.Equal(t, "job-2", captured.TargetID)
	assert.Equal(t, "uses", captured.Type)
	assert.Equal(t, "entity", captured.SourceType)
	assert.Equal(t, "job", captured.TargetType)
}

// TestRelationshipsCreateLiveSearch handles test relationships create live search.
func TestRelationshipsCreateLiveSearch(t *testing.T) {
	capturedQueries := map[string]string{}
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			capturedQueries["entities"] = r.URL.Query().Get("search_text")
			resp := map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "alxx", "type": "person"},
				},
			}
			require.NoError(t, json.NewEncoder(w).Encode(resp))
			return
		case "/api/context":
			capturedQueries["context"] = r.URL.Query().Get("search_text")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "ctx-1", "title": "alpha note", "source_type": "note", "status": "active"}},
			}))
			return
		case "/api/jobs":
			capturedQueries["jobs"] = r.URL.Query().Get("search_text")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "job-1", "title": "alpha job", "status": "planning", "priority": "high"}},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewRelationshipsModel(client)
	model.view = relsViewCreateSourceSearch

	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "a", capturedQueries["entities"])
	assert.Equal(t, "a", capturedQueries["context"])
	assert.Equal(t, "a", capturedQueries["jobs"])
	require.Len(t, model.createResults, 3)
	assert.Equal(t, "ent-1", model.createResults[0].ID)
}

// TestRelationshipTypeSuggestions handles test relationship type suggestions.
func TestRelationshipTypeSuggestions(t *testing.T) {
	model := NewRelationshipsModel(api.NewClient("http://example.com", "key"))
	model.typeOptions = []string{"works-on", "created-by"}
	model.view = relsViewCreateType

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	require.NotEmpty(t, model.createTypeResults)
	assert.Equal(t, "works-on", model.createTypeResults[0])
}

// TestRelationshipsInitViewDetailEditAndConfirmFlow handles test relationships init view detail edit and confirm flow.
func TestRelationshipsInitViewDetailEditAndConfirmFlow(t *testing.T) {
	now := time.Now()
	var patched bool
	var cmd tea.Cmd

	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/relationships" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":                "rel-1",
					"source_type":       "entity",
					"source_id":         "ent-1",
					"source_name":       "Alpha",
					"target_type":       "entity",
					"target_id":         "ent-2",
					"target_name":       "Beta",
					"relationship_type": "uses",
					"status":            "active",
					"properties":        map[string]any{},
					"created_at":        now,
				},
			}}))
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "s1", "name": "public", "agent_count": 0, "entity_count": 0, "context_count": 0},
			}}))
			return
		case r.URL.Path == "/api/entities" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "ent-1", "name": "Alpha", "type": "entity", "tags": []string{}},
				{"id": "ent-2", "name": "Beta", "type": "entity", "tags": []string{}},
			}}))
			return
		case strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodPatch:
			patched = true
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                "rel-1",
				"source_id":         "ent-1",
				"target_id":         "ent-2",
				"relationship_type": "uses",
				"status":            "archived",
				"properties":        map[string]any{},
				"created_at":        now,
			}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewRelationshipsModel(client)
	model.width = 80
	_ = model.Init() // covers Init() branch; run the cmds explicitly to avoid tea.BatchMsg handling here.
	for _, cmd := range []tea.Cmd{model.loadRelationships(), model.loadScopeOptions(), model.loadEntityCache()} {
		if cmd == nil {
			continue
		}
		model = applyMsg(model, cmd())
	}
	assert.False(t, model.loading)
	require.Len(t, model.items, 1)

	// Enter detail.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, relsViewDetail, model.view)

	// Open archive confirm and accept.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	assert.Equal(t, relsViewConfirm, model.view)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.NotNil(t, cmd)
	model = applyMsg(model, cmd())
	require.True(t, patched)
}

// TestRelationshipsModeFocusTogglesToAddFlow handles test relationships mode focus toggles to add flow.
func TestRelationshipsModeFocusTogglesToAddFlow(t *testing.T) {
	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/relationships" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		case r.URL.Path == "/api/entities" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewRelationshipsModel(client)
	model.width = 80
	_ = model.Init()
	for _, cmd := range []tea.Cmd{model.loadRelationships(), model.loadScopeOptions(), model.loadEntityCache()} {
		if cmd == nil {
			continue
		}
		model = applyMsg(model, cmd())
	}

	// Focus mode line from list selection 0, then toggle into add flow.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, model.modeFocus)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, model.modeFocus)
	assert.True(t, model.isAddView())
	assert.Equal(t, relsViewCreateSourceSearch, model.view)
}

// TestRelationshipsEditPropertiesUsesMetadataPreviewTable handles test relationships edit properties uses metadata preview table.
func TestRelationshipsEditPropertiesUsesMetadataPreviewTable(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 100
	model.view = relsViewEdit
	model.detail = &api.Relationship{
		ID:         "rel-1",
		SourceType: "entity",
		SourceID:   "ent-1",
		TargetType: "entity",
		TargetID:   "ent-2",
		Type:       "owns",
		Status:     "active",
	}
	model.editFocus = relsEditFieldProperties
	model.editMeta.Buffer = "profile | timezone | Europe/Warsaw"

	out := components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "profile | timezone | Europe/Warsaw")
}

// TestRelationshipsCreateFlowSubmitsAndReturnsToList handles test relationships create flow submits and returns to list.
func TestRelationshipsCreateFlowSubmitsAndReturnsToList(t *testing.T) {
	now := time.Now()
	var createdType string

	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/relationships" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		case r.URL.Path == "/api/entities" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "ent-1", "name": "Alpha", "type": "entity", "tags": []string{}},
				{"id": "ent-2", "name": "Beta", "type": "entity", "tags": []string{}},
			}}))
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		case r.URL.Path == "/api/relationships" && r.Method == http.MethodPost:
			var body api.CreateRelationshipInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			createdType = body.Type
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                "rel-1",
				"relationship_type": body.Type,
				"created_at":        now,
			}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewRelationshipsModel(client)
	model.width = 80
	_ = model.Init()
	for _, cmd := range []tea.Cmd{model.loadRelationships(), model.loadScopeOptions(), model.loadEntityCache()} {
		if cmd == nil {
			continue
		}
		model = applyMsg(model, cmd())
	}

	// Start create flow.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, relsViewCreateSourceSearch, model.view)

	// Type query to filter from cache.
	for _, r := range "al" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	require.NotEmpty(t, model.createResults)

	// Select source.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, relsViewCreateTargetSearch, model.view)

	// Type query and select target.
	for _, r := range "be" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	require.NotEmpty(t, model.createResults)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, relsViewCreateType, model.view)

	// Type relationship type and submit.
	for _, r := range "knows" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	model = applyMsg(model, cmd())

	assert.Equal(t, "knows", createdType)
	assert.Equal(t, relsViewList, model.view)
}

// TestRelationshipsListClampsLongEdgeAndPreviewRows handles test relationships list clamps long edge and preview rows.
func TestRelationshipsListClampsLongEdgeAndPreviewRows(t *testing.T) {
	now := time.Now()
	model := NewRelationshipsModel(nil)
	model.width = 70
	model.loading = false

	longName := strings.Repeat("very-long-relationship-endpoint-name-", 6)
	model, _ = model.Update(relTabLoadedMsg{items: []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "entity",
			SourceID:   "ent-1",
			SourceName: longName,
			TargetType: "entity",
			TargetID:   "ent-2",
			TargetName: longName,
			Type:       "depends-on",
			Status:     "active",
			CreatedAt:  now,
		},
	}})

	view := model.renderList()
	maxWidth := lipgloss.Width(strings.Split(components.Box("x", model.width), "\n")[0])
	for _, line := range strings.Split(view, "\n") {
		assert.LessOrEqual(t, lipgloss.Width(line), maxWidth)
	}
}

// TestRelationshipsEditAndCreatePreviewHelpers handles test relationships edit and create preview helpers.
func TestRelationshipsEditAndCreatePreviewHelpers(t *testing.T) {
	now := time.Now()
	var patched api.UpdateRelationshipInput

	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodPatch {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                "rel-1",
				"source_type":       "entity",
				"source_id":         "ent-1",
				"target_type":       "entity",
				"target_id":         "ent-2",
				"relationship_type": "related-to",
				"status":            "active",
				"properties":        map[string]any{"note": "ok"},
				"created_at":        now,
			}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewRelationshipsModel(client)
	model.width = 96
	model.detail = &api.Relationship{
		ID:         "rel-1",
		SourceType: "entity",
		SourceID:   "ent-1",
		SourceName: "Alpha",
		TargetType: "entity",
		TargetID:   "ent-2",
		TargetName: "Beta",
		Type:       "related-to",
		Status:     "active",
		Properties: api.JSONMap{"note": "ok"},
		CreatedAt:  now,
	}

	model.startEdit()
	assert.Equal(t, relsEditFieldStatus, model.editFocus)
	editOut := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editOut, "Status")

	model, _ = model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
	assert.NotEqual(t, -1, model.editStatusIdx)

	model.editMeta.Load(map[string]any{"note": "updated"})
	model, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)
	require.NotNil(t, patched.Status)

	entityPreview := components.SanitizeText(
		model.renderCreateNodePreview(
			relationshipCreateCandidate{
				ID:       "ent-1",
				NodeType: "entity",
				Name:     "Alpha",
				Kind:     "entity/entity",
				Status:   "active",
				Tags:     []string{"core"},
			},
			48,
		),
	)
	assert.Contains(t, entityPreview, "Selected")
	assert.Contains(t, entityPreview, "Alpha")

	typePreview := components.SanitizeText(model.renderCreateTypePreview("depends-on", 48))
	assert.Contains(t, typePreview, "depends-on")
	assert.Contains(t, typePreview, "Source")
	assert.Contains(t, typePreview, "Target")
}

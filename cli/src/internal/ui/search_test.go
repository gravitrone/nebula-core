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

// searchTestClient handles search test client.
func searchTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestSearchModelQueryCallsEndpoints handles test search model query calls endpoints.
func TestSearchModelQueryCallsEndpoints(t *testing.T) {
	var entityQuery, contextQuery, jobQuery string
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			entityQuery = r.URL.Query().Get("search_text")
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "alxx", "type": "person"},
				},
			})
			require.NoError(t, err)
		case "/api/context":
			contextQuery = r.URL.Query().Get("search_text")
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "kn-1", "name": "Nebula Notes", "source_type": "note"},
				},
			})
			require.NoError(t, err)
		case "/api/jobs":
			jobQuery = r.URL.Query().Get("search_text")
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "job-1", "title": "Alpha Job", "status": "active"},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "a", entityQuery)
	assert.Equal(t, "a", contextQuery)
	assert.Equal(t, "a", jobQuery)
	assert.Len(t, model.items, 3)
}

// TestSearchModelSelectionEmitsMsg handles test search model selection emits msg.
func TestSearchModelSelectionEmitsMsg(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "alpha", "type": "tool"},
				},
			})
			require.NoError(t, err)
		case "/api/context":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		case "/api/jobs":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	msg := cmd()
	model, _ = model.Update(msg)

	_, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	selection := cmd().(searchSelectionMsg)

	assert.Equal(t, "entity", selection.kind)
	require.NotNil(t, selection.entity)
	assert.Equal(t, "ent-1", selection.entity.ID)
}

// TestSearchModelSemanticModeCallsSemanticEndpoint handles test search model semantic mode calls semantic endpoint.
func TestSearchModelSemanticModeCallsSemanticEndpoint(t *testing.T) {
	var semanticQuery string
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/semantic":
			var payload map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			semanticQuery = payload["query"].(string)
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"kind":     "entity",
						"id":       "ent-1",
						"title":    "Agent Memory Mesh",
						"subtitle": "project",
						"snippet":  "project · memory",
						"score":    0.93,
					},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, searchModeSemantic, model.mode)

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "m", semanticQuery)
	require.Len(t, model.items, 1)
	assert.Equal(t, "entity", model.items[0].kind)
}

// TestSearchModelSemanticSelectionFetchesDetail handles test search model semantic selection fetches detail.
func TestSearchModelSemanticSelectionFetchesDetail(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/search/semantic":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"kind":     "entity",
						"id":       "ent-1",
						"title":    "Agent Memory Mesh",
						"subtitle": "project",
						"snippet":  "project · memory",
						"score":    0.93,
					},
				},
			})
			require.NoError(t, err)
		case "/api/entities/ent-1":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-1",
					"name": "Agent Memory Mesh",
					"type": "project",
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'm', Text: "m"})
	msg := cmd()
	model, _ = model.Update(msg)

	_, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	selection := cmd().(searchSelectionMsg)
	require.NotNil(t, selection.entity)
	assert.Equal(t, "ent-1", selection.entity.ID)
}

// TestBuildPaletteSearchEntriesIncludesRelationshipFileProtocolVariants handles palette entry branches.
func TestBuildPaletteSearchEntriesIncludesRelationshipFileProtocolVariants(t *testing.T) {
	mime := " text/plain "
	protocolKind := "  checklist "

	entries := buildPaletteSearchEntries(
		"",
		nil,
		nil,
		nil,
		[]api.Relationship{{
			ID:       "rel-1",
			Type:     "",
			SourceID: "entity-1234",
			TargetID: "entity-5678",
			Status:   "",
		}},
		nil,
		[]api.File{
			{ID: "file-1", Filename: "notes.txt", MimeType: &mime},
			{ID: "file-2", Filename: "blob.bin"},
		},
		[]api.Protocol{
			{ID: "proto-1", Title: "Ops Playbook", ProtocolType: &protocolKind},
			{ID: "proto-2", Name: "fallback-proto"},
		},
	)
	require.Len(t, entries, 5)

	var relationshipEntry, fileMimeEntry, fileFallbackEntry, protocolTypedEntry, protocolFallbackEntry *searchEntry
	for i := range entries {
		entry := &entries[i]
		switch {
		case entry.kind == "relationship":
			relationshipEntry = entry
		case entry.kind == "file" && entry.id == "file-1":
			fileMimeEntry = entry
		case entry.kind == "file" && entry.id == "file-2":
			fileFallbackEntry = entry
		case entry.kind == "protocol" && entry.id == "proto-1":
			protocolTypedEntry = entry
		case entry.kind == "protocol" && entry.id == "proto-2":
			protocolFallbackEntry = entry
		}
	}

	require.NotNil(t, relationshipEntry)
	assert.Contains(t, strings.ToLower(relationshipEntry.label), "entity-")
	assert.Contains(t, relationshipEntry.desc, "relationship")

	require.NotNil(t, fileMimeEntry)
	assert.Contains(t, fileMimeEntry.desc, "text/plain")
	require.NotNil(t, fileFallbackEntry)
	assert.Contains(t, fileFallbackEntry.desc, "file")

	require.NotNil(t, protocolTypedEntry)
	assert.Contains(t, protocolTypedEntry.desc, "checklist")
	require.NotNil(t, protocolFallbackEntry)
	assert.Contains(t, protocolFallbackEntry.desc, "protocol")
}

// TestSearchModelEmitSelectionRelationshipPassThrough handles relationship selections without fetch.
func TestSearchModelEmitSelectionRelationshipPassThrough(t *testing.T) {
	model := NewSearchModel(nil)
	rel := api.Relationship{ID: "rel-1", Type: "owns"}
	cmd := model.emitSelection(searchEntry{
		kind: "relationship",
		id:   "rel-1",
		rel:  &rel,
	})
	require.NotNil(t, cmd)
	msg := cmd().(searchSelectionMsg)
	assert.Equal(t, "relationship", msg.kind)
	require.NotNil(t, msg.rel)
	assert.Equal(t, "rel-1", msg.rel.ID)
}

// TestSearchModelEmitSelectionFetchesFileProtocolAndLog handles detail fetch branches.
func TestSearchModelEmitSelectionFetchesFileProtocolAndLog(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/files/file-1":
			_, _ = w.Write([]byte(`{"data":{"id":"file-1","filename":"notes.txt","uri":"path:notes.txt","file_path":"notes.txt","status":"active","metadata":{},"created_at":"` + now + `","updated_at":"` + now + `"}}`))
		case "/api/protocols/proto-1":
			_, _ = w.Write([]byte(`{"data":{"id":"proto-1","name":"ops","title":"Ops","content":"runbook","status":"active","metadata":{},"created_at":"` + now + `","updated_at":"` + now + `"}}`))
		case "/api/logs/log-1":
			_, _ = w.Write([]byte(`{"data":{"id":"log-1","timestamp":"` + now + `","log_type":"event","status":"active","value":{},"metadata":{},"created_at":"` + now + `","updated_at":"` + now + `"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)

	fileMsg := model.emitSelection(searchEntry{kind: "file", id: "file-1"})().(searchSelectionMsg)
	require.NotNil(t, fileMsg.file)
	assert.Equal(t, "file-1", fileMsg.file.ID)

	protoMsg := model.emitSelection(searchEntry{kind: "protocol", id: "proto-1"})().(searchSelectionMsg)
	require.NotNil(t, protoMsg.proto)
	assert.Equal(t, "proto-1", protoMsg.proto.ID)

	logMsg := model.emitSelection(searchEntry{kind: "log", id: "log-1"})().(searchSelectionMsg)
	require.NotNil(t, logMsg.log)
	assert.Equal(t, "log-1", logMsg.log.ID)
}

// TestSearchModelUpdateClearAndSpaceHandling handles clear and empty-space branches.
func TestSearchModelUpdateClearAndSpaceHandling(t *testing.T) {
	model := NewSearchModel(nil)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.queryInput.Value())

	updated.queryInput.SetValue("abc")
	updated.items = []searchEntry{{id: "x"}}
	updated.list.SetItems([]string{"x"})

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.queryInput.Value())
	assert.Empty(t, updated.items)
}

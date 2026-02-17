package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatEntityLineTruncatesLongNames(t *testing.T) {
	longName := strings.Repeat("a", maxEntityNameLen+10)

	line := formatEntityLine(api.Entity{
		Name: longName,
		Type: "person",
	})

	stripped := stripANSI(line)
	assert.LessOrEqual(t, len([]rune(stripped)), maxEntityLineLen)
	assert.Contains(t, stripped, "...")
	assert.Contains(t, stripped, "person")
}

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func testEntitiesClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

func TestEntitiesSaveEditCallsUpdate(t *testing.T) {
	var captured api.UpdateEntityInput
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/entities/") && r.Method == http.MethodPatch {
			json.NewDecoder(r.Body).Decode(&captured)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ent-1", "name": "Test", "tags": []string{"alpha"}}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.detail = &api.Entity{ID: "ent-1", Status: "active"}
	model.editTags = []string{"alpha"}
	model.editStatusIdx = 1

	_, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	cmd()

	if assert.NotNil(t, captured.Status) {
		assert.Equal(t, "inactive", *captured.Status)
	}
	if assert.NotNil(t, captured.Tags) {
		assert.Equal(t, []string{"alpha"}, *captured.Tags)
	}
}

func TestEntitiesEditHeaderSanitized(t *testing.T) {
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	model := NewEntitiesModel(client)
	model.detail = &api.Entity{
		ID:     "ent-1",
		Name:   "safe\u202Eevil",
		Status: "active",
	}

	rendered := model.renderEdit()
	assert.NotContains(t, rendered, "\u202E")
}

func TestEntitiesCreateRelationshipCommand(t *testing.T) {
	var captured api.CreateRelationshipInput
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/relationships" && r.Method == http.MethodPost {
			json.NewDecoder(r.Body).Decode(&captured)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "rel-1"}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	source := api.Entity{ID: "ent-1", Type: "person"}
	target := api.Entity{ID: "ent-2"}
	cmd := model.createRelationship(source, target, "knows")
	msg := cmd()
	_, _ = model.Update(msg)

	assert.Equal(t, "ent-1", captured.SourceID)
	assert.Equal(t, "ent-2", captured.TargetID)
	assert.Equal(t, "entity", captured.SourceType)
	assert.Equal(t, "entity", captured.TargetType)
	assert.Equal(t, "knows", captured.Type)
}

func TestTruncateStringEdges(t *testing.T) {
	assert.Equal(t, "", truncateString("hello", 0))
	assert.Equal(t, "", truncateString("hello", -1))
	assert.Equal(t, "hello", truncateString("hello", 5))
	assert.Equal(t, "hell...", truncateString("hello", 4))
	assert.Equal(t, "你...", truncateString("你好世界", 1))
}

func TestEntitiesLiveSearchTriggersQuery(t *testing.T) {
	var searchText string
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			searchText = r.URL.Query().Get("search_text")
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "ent-1", "name": "alpha", "tags": []string{}}}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msg = cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "a", searchText)
	assert.Equal(t, "a", model.searchBuf)
}

func TestNormalizeEntityNameType(t *testing.T) {
	name, typ := normalizeEntityNameType("[organization] OpenAI", "")
	assert.Equal(t, "OpenAI", name)
	assert.Equal(t, "organization", typ)

	name, typ = normalizeEntityNameType("[Organization] OpenAI", "organization")
	assert.Equal(t, "OpenAI", name)
	assert.Equal(t, "organization", typ)

	name, typ = normalizeEntityNameType("[organization] OpenAI", "person")
	assert.Equal(t, "[organization] OpenAI", name)
	assert.Equal(t, "person", typ)
}

func TestEntitiesSearchSuggest(t *testing.T) {
	var searchText string
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			searchText = r.URL.Query().Get("search_text")
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "ent-1", "name": "alxx", "type": "person", "tags": []string{}},
			}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	msg = cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "a", searchText)
	assert.Equal(t, "alxx", model.searchSuggest)
}

func TestEntitiesSearchSuggestTabAccepts(t *testing.T) {
	var searchText string
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			searchText = r.URL.Query().Get("search_text")
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.searchBuf = "al"
	model.searchSuggest = "alxx"

	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "alxx", model.searchBuf)
	assert.Equal(t, "alxx", searchText)
}

func TestEntityHistoryRevertFlow(t *testing.T) {
	historyCalled := false
	revertCalled := false
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/entities/ent-1/history"):
			historyCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":             "audit-1",
						"table_name":     "entities",
						"record_id":      "ent-1",
						"action":         "update",
						"changed_fields": []string{"tags"},
						"changed_at":     "2026-02-09T00:00:00Z",
					},
				},
			})
			return
		case strings.HasPrefix(r.URL.Path, "/api/entities/ent-1/revert"):
			revertCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-1",
					"name": "Restored",
					"tags": []string{},
				},
			})
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewEntitiesModel(client)
	model.detail = &api.Entity{ID: "ent-1", Name: "Original"}
	model.view = entitiesViewDetail

	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.True(t, historyCalled)
	assert.Equal(t, entitiesViewHistory, model.view)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, entitiesViewConfirm, model.view)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.True(t, revertCalled)
	assert.Equal(t, entitiesViewDetail, model.view)
	require.NotNil(t, model.detail)
	assert.Equal(t, "Restored", model.detail.Name)
}

func TestParseBulkInput(t *testing.T) {
	spec, err := parseBulkInput("add:alpha, beta")
	require.NoError(t, err)
	assert.Equal(t, "add", spec.op)
	assert.Equal(t, []string{"alpha", "beta"}, spec.values)

	spec, err = parseBulkInput("-one two")
	require.NoError(t, err)
	assert.Equal(t, "remove", spec.op)
	assert.Equal(t, []string{"one", "two"}, spec.values)

	spec, err = parseBulkInput("set:")
	require.NoError(t, err)
	assert.Equal(t, "set", spec.op)
	assert.Empty(t, spec.values)
}

func TestNormalizeBulkTags(t *testing.T) {
	out := normalizeBulkTags([]string{" Foo", "foo", "Bar-Baz", "bar_baz"})
	assert.Equal(t, []string{"foo", "bar-baz"}, out)
}

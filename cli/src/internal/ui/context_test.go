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

// contextTestClient handles context test client.
func contextTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestContextSaveLinksEntities handles test context save links entities.
func TestContextSaveLinksEntities(t *testing.T) {
	var linked []string
	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/context" && r.Method == http.MethodPost:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "k-1", "name": "note"}}))
		case strings.HasPrefix(r.URL.Path, "/api/context/") && strings.HasSuffix(r.URL.Path, "/link"):
			var body api.LinkContextInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			linked = append(linked, body.OwnerID)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewContextModel(client)
	model.fields[fieldTitle].value = "Test"
	model.fields[fieldNotes].value = "Notes"
	model.linkEntities = []api.Entity{{ID: "ent-1", Name: "Alpha"}, {ID: "ent-2", Name: "Beta"}}

	model, cmd := model.save()
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.ElementsMatch(t, []string{"ent-1", "ent-2"}, linked)
	assert.True(t, model.saved)
}

// TestContextLinkSearchAddsEntity handles test context link search adds entity.
func TestContextLinkSearchAddsEntity(t *testing.T) {
	model := NewContextModel(nil)
	model.linkSearching = true
	model.linkResults = []api.Entity{{ID: "ent-1", Name: "Alpha"}}
	model.linkList.SetItems([]string{"Alpha"})

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	assert.Len(t, model.linkEntities, 1)
	assert.False(t, model.linkSearching)
}

// TestContextLinkSearchCommand handles test context link search command.
func TestContextLinkSearchCommand(t *testing.T) {
	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "ent-1", "name": "Alpha", "tags": []string{}}}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewContextModel(client)
	model.linkSearching = true
	model.linkQuery = ""

	model, cmd := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Len(t, model.linkResults, 1)
	assert.Equal(t, "ent-1", model.linkResults[0].ID)
}

// TestNormalizeTag handles test normalize tag.
func TestNormalizeTag(t *testing.T) {
	assert.Equal(t, "hello-world", normalizeTag(" Hello_World "))
	assert.Equal(t, "foo-bar-baz", normalizeTag("#Foo  Bar   Baz"))
	assert.Equal(t, "", normalizeTag(""))
}

// TestNormalizeScope handles test normalize scope.
func TestNormalizeScope(t *testing.T) {
	assert.Equal(t, "private", normalizeScope(" Private "))
	assert.Equal(t, "team-scope", normalizeScope("#Team Scope"))
}

// TestCommitTagDedupes handles test commit tag dedupes.
func TestCommitTagDedupes(t *testing.T) {
	model := NewContextModel(nil)
	model.tags = []string{"alpha"}
	model.tagBuf = "ALPHA"
	model.commitTag()
	assert.Equal(t, []string{"alpha"}, model.tags)
}

// TestCommitScopeDedupes handles test commit scope dedupes.
func TestCommitScopeDedupes(t *testing.T) {
	model := NewContextModel(nil)
	model.scopes = []string{"public"}
	model.scopeBuf = " Public "
	model.commitScope()
	assert.Equal(t, []string{"public"}, model.scopes)
}

// TestCommitEditScopeDedupes handles test commit edit scope dedupes.
func TestCommitEditScopeDedupes(t *testing.T) {
	model := NewContextModel(nil)
	model.editScopes = []string{"public"}
	model.editScopeBuf = " PUBLIC "
	model.commitEditScope()
	assert.Equal(t, []string{"public"}, model.editScopes)
}

// TestContextToggleModeLoadsList handles test context toggle mode loads list.
func TestContextToggleModeLoadsList(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/context" {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "k-1", "name": "Alpha", "source_type": "note", "tags": []string{}, "created_at": now},
			}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewContextModel(client)
	model.view = contextViewAdd

	model, cmd := model.toggleMode()
	require.NotNil(t, cmd)
	model, _ = model.Update(runCmdFirst(cmd))

	assert.Equal(t, contextViewList, model.view)
	assert.Len(t, model.items, 1)
}

// TestContextListEnterShowsDetail handles test context list enter shows detail.
func TestContextListEnterShowsDetail(t *testing.T) {
	model := NewContextModel(nil)
	model.view = contextViewList
	model.items = []api.Context{
		{ID: "k-1", Name: "Alpha", SourceType: "note"},
	}
	model.list.SetItems([]string{formatContextLine(model.items[0])})

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.NotNil(t, model.detail)
	assert.Equal(t, "k-1", model.detail.ID)
	assert.Equal(t, contextViewDetail, model.view)
}

// TestContextListEnterRejectsMissingID handles test context list enter rejects missing id.
func TestContextListEnterRejectsMissingID(t *testing.T) {
	model := NewContextModel(nil)
	model.view = contextViewList
	model.items = []api.Context{
		{Name: "Alpha", SourceType: "note"},
	}
	model.list.SetItems([]string{formatContextLine(model.items[0])})

	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.NotNil(t, cmd)
	msg := cmd()
	err, ok := msg.(errMsg)
	require.True(t, ok)
	require.Error(t, err.err)
	assert.Contains(t, err.err.Error(), "missing id")
	assert.Nil(t, model.detail)
	assert.Equal(t, contextViewList, model.view)
}

// TestContextRenderEditShowsTagsAndScopes handles test context render edit shows tags and scopes.
func TestContextRenderEditShowsTagsAndScopes(t *testing.T) {
	model := NewContextModel(nil)
	model.width = 100
	model.view = contextViewEdit
	model.editTags = []string{"alpha"}
	model.editScopes = []string{"public"}

	out := model.renderEdit()
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "public")
}

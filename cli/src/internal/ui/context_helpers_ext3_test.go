package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextRenderModeLineMatrix(t *testing.T) {
	model := NewContextModel(nil)
	model.view = contextViewAdd
	out := model.renderModeLine()
	assert.Contains(t, out, "Add")
	assert.Contains(t, out, "Library")

	model.view = contextViewList
	out = model.renderModeLine()
	assert.Contains(t, out, "Add")
	assert.Contains(t, out, "Library")

	model.view = contextViewAdd
	model.modeFocus = true
	out = model.renderModeLine()
	assert.Contains(t, out, "Add")

	model.view = contextViewEdit
	model.modeFocus = true
	out = model.renderModeLine()
	assert.Contains(t, out, "Library")
}

func TestContextHandleListKeysAdditionalBranches(t *testing.T) {
	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/context/ctx-2":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":      "ctx-2",
					"title":   "With ID",
					"status":  "active",
					"content": "hello",
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewContextModel(client)
	model.items = []api.Context{
		{ID: "", Title: "Missing ID"},
		{ID: "ctx-2", Title: "With ID"},
	}
	model.list.SetItems([]string{"missing", "with-id"})

	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.list.Selected())

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.list.Selected())

	// Enter on row with missing ID returns err command.
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "missing id")

	// Enter on valid row opens detail and emits load command.
	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, contextViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	msg = cmd()
	_, ok = msg.(contextDetailLoadedMsg)
	require.True(t, ok)

	// f branch.
	updated.view = contextViewList
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 'f', Text: "f"})
	require.Nil(t, cmd)
	assert.True(t, updated.filtering)

	// back branch.
	updated.filtering = false
	updated.view = contextViewList
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, contextViewAdd, updated.view)

	// Enter with stale selected index (idx >= len(items)) is no-op.
	updated.view = contextViewList
	updated.items = []api.Context{{ID: "ctx-1", Title: "only one"}}
	updated.list.SetItems([]string{"one", "two"})
	updated.list.Down() // selected = 1 while len(items)=1
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
}

func TestContextScopeFormattingAndNameHelpers(t *testing.T) {
	model := NewContextModel(nil)
	model.scopeNames = map[string]string{
		"s-public":  "public",
		"s-private": "private",
	}

	assert.Equal(t, "-", model.formatContextScopes(nil))
	assert.Equal(t, "-", model.formatContextScopes([]string{}))

	formatted := model.formatContextScopes([]string{"s-public", "unknown-id"})
	assert.True(t, strings.Contains(formatted, "public"))
	assert.True(t, strings.Contains(formatted, "unknown-id"))

	assert.Nil(t, model.scopeNamesFromIDs(nil))
	assert.Nil(t, model.scopeNamesFromIDs([]string{}))
	assert.Equal(
		t,
		[]string{"private", "fallback"},
		model.scopeNamesFromIDs([]string{"s-private", "fallback"}),
	)

	assert.Equal(t, "", truncateContextName("alpha", 0))
	assert.Equal(t, "alpha", truncateContextName("alpha", 10))
	assert.Equal(t, "alp...", truncateContextName("alpha", 3))
}

func TestContextCommitEditTagBranches(t *testing.T) {
	model := NewContextModel(nil)
	model.editTags = []string{"alpha-tag"}

	model.editTagInput.SetValue("   ")
	model.commitEditTag()
	assert.Equal(t, "", model.editTagInput.Value())
	assert.Equal(t, []string{"alpha-tag"}, model.editTags)

	model.editTagInput.SetValue("#Alpha Tag")
	model.commitEditTag()
	assert.Equal(t, "", model.editTagInput.Value())
	assert.Equal(t, []string{"alpha-tag"}, model.editTags)

	model.editTagInput.SetValue("Beta_Tag")
	model.commitEditTag()
	assert.Equal(t, "", model.editTagInput.Value())
	assert.Equal(t, []string{"alpha-tag", "beta-tag"}, model.editTags)
}

func TestContextLoadScopeNamesSuccessErrorAndNilClient(t *testing.T) {
	model := NewContextModel(nil)
	assert.Nil(t, model.loadScopeNames())

	_, client := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/audit/scopes":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-1", "name": "public", "agent_count": 1},
					{"id": "scope-2", "name": "private", "agent_count": 1},
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	model = NewContextModel(client)
	cmd := model.loadScopeNames()
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(contextScopesLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "public", loaded.names["scope-1"])
	assert.Equal(t, "private", loaded.names["scope-2"])

	_, badClient := contextTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":"SCOPES_FAILED","message":"scope load failed"}}`, http.StatusInternalServerError)
	})
	model = NewContextModel(badClient)
	cmd = model.loadScopeNames()
	require.NotNil(t, cmd)
	msg = cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "SCOPES_FAILED")
}

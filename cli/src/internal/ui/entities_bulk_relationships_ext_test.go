package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBulkInputExtendedMatrix(t *testing.T) {
	_, err := parseBulkInput("   ")
	require.Error(t, err)

	_, err = parseBulkInput("add:")
	require.Error(t, err)

	_, err = parseBulkInput("remove:  ")
	require.Error(t, err)

	_, err = parseBulkInput("add:\t\t")
	require.Error(t, err)

	spec, err := parseBulkInput("= public, private")
	require.NoError(t, err)
	assert.Equal(t, "set", spec.op)
	assert.Equal(t, []string{"public", "private"}, spec.values)

	spec, err = parseBulkInput("+foo bar")
	require.NoError(t, err)
	assert.Equal(t, "add", spec.op)
	assert.Equal(t, []string{"foo", "bar"}, spec.values)

	spec, err = parseBulkInput("add:foo,\t,bar")
	require.NoError(t, err)
	assert.Equal(t, "add", spec.op)
	assert.Equal(t, []string{"foo", "bar"}, spec.values)

	spec, err = parseBulkInput("alpha, beta")
	require.NoError(t, err)
	assert.Equal(t, "add", spec.op)
	assert.Equal(t, []string{"alpha", "beta"}, spec.values)

	spec, err = parseBulkInput("remove: foo,bar")
	require.NoError(t, err)
	assert.Equal(t, "remove", spec.op)
	assert.Equal(t, []string{"foo", "bar"}, spec.values)

	spec, err = parseBulkInput("SET:")
	require.NoError(t, err)
	assert.Equal(t, "set", spec.op)
	assert.Empty(t, spec.values)

	spec, err = parseBulkInput("=   ")
	require.NoError(t, err)
	assert.Equal(t, "set", spec.op)
	assert.Empty(t, spec.values)
}

func TestNormalizeBulkScopesExtended(t *testing.T) {
	out := normalizeBulkScopes([]string{" Public", "#public", "PRIVATE", " private ", "", "#"})
	assert.Equal(t, []string{"public", "private"}, out)
}

func TestEntitiesHandleBulkPromptKeysBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.bulkPrompt = "bulk tags"
	model.bulkInput.SetValue("abc")

	updated, cmd := model.handleBulkPromptKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.bulkPrompt)
	assert.Equal(t, "", updated.bulkInput.Value())

	updated.bulkInput.SetValue("ab")
	updated, cmd = updated.handleBulkPromptKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.bulkInput.Value())

	updated.bulkInput.SetValue("abc")
	updated, cmd = updated.handleBulkPromptKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.bulkInput.Value())

	updated, cmd = updated.handleBulkPromptKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.Equal(t, "x", updated.bulkInput.Value())
	updated, _ = updated.handleBulkPromptKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "x ", updated.bulkInput.Value())

	updated.bulkInput.SetValue("   ")
	_, cmd = updated.handleBulkPromptKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	em, ok := msg.(errMsg)
	require.True(t, ok)
	require.Error(t, em.err)

	updated.bulkInput.SetValue("add:")
	_, cmd = updated.handleBulkPromptKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	em, ok = msg.(errMsg)
	require.True(t, ok)
	assert.Contains(t, strings.ToLower(em.err.Error()), "no values")
}

func TestEntitiesHandleBulkPromptKeysRoutesToTagsAndScopes(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.items = []api.Entity{{ID: "ent-1"}}
	model.bulkSelected = map[string]bool{"ent-1": true}
	model.bulkPrompt = "bulk tags"
	model.bulkInput.SetValue("set:alpha,beta")
	model.bulkTarget = bulkTargetTags

	updated, cmd := model.handleBulkPromptKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, "", updated.bulkPrompt)
	assert.Equal(t, "", updated.bulkInput.Value())
	assert.True(t, updated.bulkRunning)

	model.bulkPrompt = "bulk scopes"
	model.bulkInput.SetValue("set:public,private")
	model.bulkTarget = bulkTargetScopes
	updated, cmd = model.handleBulkPromptKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.bulkRunning)
}

func TestEntitiesBulkUpdateTagsAndScopesCommandBranches(t *testing.T) {
	now := time.Now().UTC()

	t.Run("tags no ids and invalid tags", func(t *testing.T) {
		model := NewEntitiesModel(nil)

		assert.Nil(t, model.bulkUpdateTags(bulkInput{op: "add", values: []string{"x"}}))

		model.bulkSelected = map[string]bool{"ent-1": true}
		cmd := model.bulkUpdateTags(bulkInput{op: "add", values: []string{"#", "   "}})
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		assert.Contains(t, strings.ToLower(em.err.Error()), "no valid tags")
	})

	t.Run("scopes no ids and invalid scopes", func(t *testing.T) {
		model := NewEntitiesModel(nil)

		assert.Nil(t, model.bulkUpdateScopes(bulkInput{op: "add", values: []string{"public"}}))

		model.bulkSelected = map[string]bool{"ent-1": true}
		cmd := model.bulkUpdateScopes(bulkInput{op: "add", values: []string{"#", "   "}})
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		assert.Contains(t, strings.ToLower(em.err.Error()), "no valid scopes")
	})

	t.Run("tags/scopes success and server error paths", func(t *testing.T) {
		var tagsCalls int
		var scopesCalls int
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/entities/bulk/tags":
				tagsCalls++
				if tagsCalls == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"error": map[string]any{"code": "INTERNAL", "message": "tags failed"},
					}))
					return
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"updated_count": 1, "op": "set"},
				}))
				return
			case "/api/entities/bulk/scopes":
				scopesCalls++
				if scopesCalls == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"error": map[string]any{"code": "INTERNAL", "message": "scopes failed"},
					}))
					return
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{"updated_count": 1, "op": "set"},
				}))
				return
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})

		model := NewEntitiesModel(client)
		model.bulkSelected = map[string]bool{"ent-1": true}

		cmd := model.bulkUpdateTags(bulkInput{op: "set", values: []string{"alpha"}})
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		assert.Contains(t, strings.ToLower(em.err.Error()), "tags failed")

		cmd = model.bulkUpdateTags(bulkInput{op: "set", values: []string{"alpha"}})
		require.NotNil(t, cmd)
		msg = cmd()
		_, ok = msg.(entityBulkUpdatedMsg)
		require.True(t, ok)

		cmd = model.bulkUpdateScopes(bulkInput{op: "set", values: []string{"public"}})
		require.NotNil(t, cmd)
		msg = cmd()
		em, ok = msg.(errMsg)
		require.True(t, ok)
		assert.Contains(t, strings.ToLower(em.err.Error()), "scopes failed")

		cmd = model.bulkUpdateScopes(bulkInput{op: "set", values: []string{"public"}})
		require.NotNil(t, cmd)
		msg = cmd()
		_, ok = msg.(entityBulkUpdatedMsg)
		require.True(t, ok)

		assert.Equal(t, 2, tagsCalls)
		assert.Equal(t, 2, scopesCalls)
		_ = now
	})
}

func TestEntitiesRenderRelationshipsAndSelectionHelpersBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 96
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}

	model.relLoading = true
	out := components.SanitizeText(model.renderRelationships())
	assert.Contains(t, out, "Loading relationships")

	model.relLoading = false
	model.rels = nil
	out = components.SanitizeText(model.renderRelationships())
	assert.Contains(t, out, "No relationships yet")

	model.rels = []api.Relationship{
		{ID: "rel-1", SourceID: "ent-1", TargetID: "ent-2", TargetName: "Beta", Type: "uses", Status: "active"},
	}
	model.relTable.SetRows([]table.Row{{"uses"}})
	out = components.SanitizeText(model.renderRelationships())
	assert.Contains(t, out, "Direction")

	assert.Nil(t, model.selectedRelationshipByID(""))
	assert.Nil(t, model.selectedRelationshipByID("missing"))
	rel := model.selectedRelationshipByID("rel-1")
	require.NotNil(t, rel)
	assert.Equal(t, "rel-1", rel.ID)
}

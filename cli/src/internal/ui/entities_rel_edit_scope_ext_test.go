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

func TestEntitiesLoadScopeNamesSuccessAndError(t *testing.T) {
	t.Run("success maps scope ids to names", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet {
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{
						{"id": "scope-1", "name": "public", "agent_count": 1},
						{"id": "scope-2", "name": "private", "agent_count": 1},
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		cmd := model.loadScopeNames()
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(entityScopesLoadedMsg)
		require.True(t, ok)
		assert.Equal(t, "public", loaded.names["scope-1"])
		assert.Equal(t, "private", loaded.names["scope-2"])
	})

	t.Run("error returns err message", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{
						"code":    "INTERNAL",
						"message": "boom",
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		cmd := model.loadScopeNames()
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		require.Error(t, em.err)
		assert.Contains(t, strings.ToLower(em.err.Error()), "boom")
	})
}

func TestEntitiesRelationshipLoadHelpersBranchMatrix(t *testing.T) {
	now := time.Now().UTC()

	t.Run("loadRelationships nil detail returns empty message", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		cmd := model.loadRelationships()
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(relationshipsLoadedMsg)
		require.True(t, ok)
		assert.Nil(t, loaded.items)
	})

	t.Run("loadRelationships error path returns errMsg", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/relationships/") {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": "INTERNAL", "message": "rel failed"},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.detail = &api.Entity{ID: "ent-1"}
		cmd := model.loadRelationships()
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		require.Error(t, em.err)
		assert.Contains(t, strings.ToLower(em.err.Error()), "rel failed")
	})

	t.Run("loadRelationships success path returns items", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/relationships/") {
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{
						{
							"id":                "rel-1",
							"source_type":       "entity",
							"source_id":         "ent-1",
							"target_type":       "entity",
							"target_id":         "ent-2",
							"relationship_type": "depends-on",
							"status":            "active",
							"created_at":        now,
						},
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.detail = &api.Entity{ID: "ent-1"}
		cmd := model.loadRelationships()
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(relationshipsLoadedMsg)
		require.True(t, ok)
		require.Len(t, loaded.items, 1)
		assert.Equal(t, "rel-1", loaded.items[0].ID)
	})

	t.Run("loadEntityDetailRelationships swallows errors into empty items", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/relationships/") {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": "INTERNAL", "message": "cannot load"},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		cmd := model.loadEntityDetailRelationships("ent-1")
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(entityDetailRelationshipsLoadedMsg)
		require.True(t, ok)
		assert.Equal(t, "ent-1", loaded.id)
		assert.Nil(t, loaded.items)
	})
}

func TestEntitiesStartRelEditAndHandleRelEditKeysBranchMatrix(t *testing.T) {
	now := time.Now().UTC()
	var patchedStatus string
	var patchedProperties map[string]any

	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodPatch {
			var input api.UpdateRelationshipInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			if input.Status != nil {
				patchedStatus = *input.Status
			}
			patchedProperties = input.Properties
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                "rel-1",
					"source_type":       "entity",
					"source_id":         "ent-1",
					"target_type":       "entity",
					"target_id":         "ent-2",
					"relationship_type": "depends-on",
					"status":            patchedStatus,
					"properties":        patchedProperties,
					"created_at":        now,
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)

	// Guard path: no selected relationship.
	model.startRelEdit()
	assert.Equal(t, "", model.relEditID)

	// Happy path.
	model.rels = []api.Relationship{
		{
			ID:         "rel-1",
			SourceID:   "ent-1",
			TargetID:   "ent-2",
			Type:       "depends-on",
			Status:     "active",
			Properties: api.JSONMap{},
			CreatedAt:  now,
		},
	}
	model.relList.SetItems([]string{"depends-on"})
	model.startRelEdit()
	assert.Equal(t, "rel-1", model.relEditID)
	assert.Equal(t, relEditFieldStatus, model.relEditFocus)
	assert.Equal(t, 0, model.relEditStatusIdx)
	assert.Equal(t, "", strings.TrimSpace(model.relEditInput.Value()))

	// Status selector branches.
	updated, cmd := model.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.relEditStatusIdx)

	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, 0, updated.relEditStatusIdx)

	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, 1, updated.relEditStatusIdx)

	// Focus movement and properties input.
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, relEditFieldProperties, updated.relEditFocus)
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, relEditFieldStatus, updated.relEditFocus)
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, relEditFieldStatus, updated.relEditFocus)
	updated.relEditFocus = relEditFieldProperties
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, relEditFieldStatus, updated.relEditFocus)

	updated.relEditFocus = relEditFieldProperties
	updated.relEditInput.SetValue("a")
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.relEditInput.Value())
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: '{', Text: "{"})
	assert.Equal(t, "{", updated.relEditInput.Value())
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "{ ", updated.relEditInput.Value())

	// Save invalid JSON branch.
	updated.relEditInput.SetValue("{")
	updated, cmd = updated.handleRelEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.Nil(t, cmd)
	assert.NotEmpty(t, updated.errText)

	// Save valid branch.
	updated.errText = ""
	updated.relEditStatusIdx = 0
	updated.relEditInput.SetValue(`{"note":"ok"}`)
	updated, cmd = updated.handleRelEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)
	msg := cmd()
	relMsg, ok := msg.(relationshipUpdatedMsg)
	require.True(t, ok)
	assert.Equal(t, "rel-1", relMsg.rel.ID)
	assert.Equal(t, "active", patchedStatus)
	assert.Equal(t, "ok", patchedProperties["note"])

	// Back exits to relationships view.
	updated.view = entitiesViewRelEdit
	updated, _ = updated.handleRelEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, entitiesViewRelationships, updated.view)
}

func TestEntitiesCommitEditTagScopeAndRenderEditTagsBranches(t *testing.T) {
	model := NewEntitiesModel(nil)

	model.editTagInput.SetValue("  ")
	model.commitEditTag()
	assert.Equal(t, "", model.editTagInput.Value())

	model.editTags = []string{"alpha"}
	model.editTagInput.SetValue("ALPHA")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())

	model.editTagInput.SetValue("beta")
	model.commitEditTag()
	assert.Equal(t, []string{"alpha", "beta"}, model.editTags)

	model.editScopeInput.SetValue("  ")
	model.commitEditScope()
	assert.Equal(t, "", model.editScopeInput.Value())
	assert.False(t, model.editScopesDirty)

	model.editScopes = []string{"public"}
	model.editScopeInput.SetValue(" PUBLIC ")
	model.commitEditScope()
	assert.Equal(t, []string{"public"}, model.editScopes)
	assert.False(t, model.editScopesDirty)

	model.editScopeInput.SetValue("private")
	model.commitEditScope()
	assert.Equal(t, []string{"public", "private"}, model.editScopes)
	assert.True(t, model.editScopesDirty)

	empty := model.renderEditTags(false)
	assert.NotEqual(t, "-", empty)
	model.editTags = nil
	model.editTagInput.SetValue("")
	assert.Equal(t, "-", model.renderEditTags(false))
	assert.Contains(t, components.SanitizeText(model.renderEditTags(true)), "█")
}

func TestEntitiesHandleRelationshipsKeysBranchMatrix(t *testing.T) {
	now := time.Now().UTC()
	model := NewEntitiesModel(nil)
	model.view = entitiesViewRelationships

	// Back key exits relationship detail.
	next, cmd := model.handleRelationshipsKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewDetail, next.view)

	// Enter relate flow and ensure startRelate reset is applied.
	model.view = entitiesViewRelationships
	model.relateQueryInput.SetValue("stale")
	model.relateResults = []api.Entity{{ID: "ent-x"}}
	model.relateTarget = &api.Entity{ID: "ent-y"}
	model.relateTypeInput.SetValue("knows")
	model.relateLoading = true
	next, cmd = model.handleRelationshipsKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewRelateSearch, next.view)
	assert.Equal(t, "", next.relateQueryInput.Value())
	assert.Nil(t, next.relateResults)
	assert.Nil(t, next.relateTarget)
	assert.Equal(t, "", next.relateTypeInput.Value())
	assert.False(t, next.relateLoading)

	// Down/up branches on relationship list.
	model.view = entitiesViewRelationships
	model.rels = []api.Relationship{
		{ID: "rel-1", Type: "uses", SourceID: "ent-1", TargetID: "ent-2", CreatedAt: now},
		{ID: "rel-2", Type: "depends-on", SourceID: "ent-1", TargetID: "ent-3", CreatedAt: now},
	}
	model.relList.SetItems([]string{"uses", "depends-on"})
	model.relList.Cursor = 0
	next, _ = model.handleRelationshipsKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, next.relList.Selected())
	next, _ = next.handleRelationshipsKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, next.relList.Selected())

	// Edit with invalid selection remains in relationships view.
	model.view = entitiesViewRelationships
	model.relList.Cursor = 99
	next, cmd = model.handleRelationshipsKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewRelationships, next.view)

	// Delete with invalid selection remains in relationships view.
	next, cmd = model.handleRelationshipsKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewRelationships, next.view)
	assert.Equal(t, "", next.confirmKind)

	// Edit with valid relationship enters edit view.
	model.relList.Cursor = 0
	next, cmd = model.handleRelationshipsKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewRelEdit, next.view)
	assert.Equal(t, "rel-1", next.relEditID)

	// Delete with valid relationship opens confirm flow.
	model.view = entitiesViewRelationships
	next, cmd = model.handleRelationshipsKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewConfirm, next.view)
	assert.Equal(t, "rel-archive", next.confirmKind)
	assert.Equal(t, "rel-1", next.confirmRelID)
	assert.Equal(t, entitiesViewRelationships, next.confirmReturn)
}

func TestEntitiesLoadRelateResultsAndDetailRelationshipsMatrix(t *testing.T) {
	now := time.Now().UTC()

	t.Run("loadEntityDetailRelationships success", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodGet {
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{
						{
							"id":                "rel-1",
							"source_type":       "entity",
							"source_id":         "ent-1",
							"target_type":       "entity",
							"target_id":         "ent-2",
							"relationship_type": "uses",
							"status":            "active",
							"created_at":        now,
						},
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		cmd := model.loadEntityDetailRelationships("ent-1")
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(entityDetailRelationshipsLoadedMsg)
		require.True(t, ok)
		assert.Equal(t, "ent-1", loaded.id)
		require.Len(t, loaded.items, 1)
		assert.Equal(t, "rel-1", loaded.items[0].ID)
	})

	t.Run("loadRelateResults success forwards query and rows", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/entities" && r.Method == http.MethodGet {
				assert.Equal(t, "beta", r.URL.Query().Get("search_text"))
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{
						{"id": "ent-2", "name": "Beta", "type": "tool", "status": "active"},
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		cmd := model.loadRelateResults("beta")
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(relateResultsMsg)
		require.True(t, ok)
		require.Len(t, loaded.items, 1)
		assert.Equal(t, "ent-2", loaded.items[0].ID)
		assert.Equal(t, "Beta", loaded.items[0].Name)
	})

	t.Run("loadRelateResults error path returns errMsg", func(t *testing.T) {
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/entities" && r.Method == http.MethodGet {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": "INTERNAL", "message": "query failed"},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		cmd := model.loadRelateResults("beta")
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		require.Error(t, em.err)
		assert.Contains(t, strings.ToLower(em.err.Error()), "query failed")
	})
}

func TestEntitiesSaveRelEditReturnsErrMsgOnUpdateFailure(t *testing.T) {
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusInternalServerError)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "INTERNAL_ERROR",
					"message": "write failed",
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.relEditID = "rel-1"
	model.relEditStatusIdx = 0
	model.relEditInput.SetValue(`{"note":"ok"}`)

	updated, cmd := model.saveRelEdit()
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewRelationships, updated.view)

	msg := cmd()
	_, ok := msg.(errMsg)
	assert.True(t, ok)
}

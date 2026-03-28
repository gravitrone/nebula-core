package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationshipsHandleModeKeysBranchMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.modeFocus = true
	model.view = relsViewList

	updated, cmd := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated.view = relsViewList
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)

	updated.modeFocus = true
	updated.view = relsViewCreateType
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, relsViewList, updated.view)
}

func TestRelationshipsHandleDetailKeysBranchMatrix(t *testing.T) {
	now := time.Now().UTC()

	t.Run("up and toggle metadata", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewDetail
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}

		updated, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyUp})
		require.Nil(t, cmd)
		assert.True(t, updated.modeFocus)

		updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: 'm', Text: "m"})
		require.Nil(t, cmd)
		assert.True(t, updated.metaExpanded)

		updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: 'm', Text: "m"})
		require.Nil(t, cmd)
		assert.False(t, updated.metaExpanded)
	})

	t.Run("edit and confirm branches", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewDetail
		model.detail = &api.Relationship{ID: "rel-1", Status: "active", Notes: "", CreatedAt: now}

		updated, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
		require.Nil(t, cmd)
		assert.Equal(t, relsViewEdit, updated.view)
		assert.Equal(t, relsEditFieldStatus, updated.editFocus)

		updated.view = relsViewDetail
		updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: 'd', Text: "d"})
		require.Nil(t, cmd)
		assert.Equal(t, relsViewConfirm, updated.view)
		assert.Equal(t, "archive", updated.confirmKind)
	})

	t.Run("back clears detail state", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewDetail
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}
		model.metaExpanded = true

		updated, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
		require.Nil(t, cmd)
		assert.Equal(t, relsViewList, updated.view)
		assert.Nil(t, updated.detail)
		assert.False(t, updated.metaExpanded)
	})
}

func TestRelationshipsRenderDetailBranchMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 88

	model.loading = true
	out := model.renderDetail()
	assert.Contains(t, out, "Loading relationships")

	now := time.Now().UTC()
	model.loading = false
	model.detail = &api.Relationship{
		ID:         "rel-1",
		Type:       "depends-on",
		Status:     "active",
		SourceType: "entity",
		SourceID:   "ent-1",
		SourceName: "alpha",
		TargetType: "entity",
		TargetID:   "ent-2",
		TargetName: "beta",
		CreatedAt:  now,
	}
	out = model.renderDetail()
	assert.Contains(t, out, "depends-on")

	model.detail.Notes = "hello world"
	out = model.renderDetail()
	assert.Contains(t, out, "hello world")
}

func TestRelationshipsHandleEditKeysBranchMatrix(t *testing.T) {
	now := time.Now().UTC()

	t.Run("editSaving short-circuits", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewEdit
		model.editSaving = true
		model.editFocus = relsEditFieldStatus
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}

		updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
		require.Nil(t, cmd)
		assert.Equal(t, relsEditFieldStatus, updated.editFocus)
	})

	t.Run("status and properties input branches", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewEdit
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}
		model.editStatusIdx = 0
		model.editFocus = relsEditFieldStatus

		updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyRight})
		require.Nil(t, cmd)
		assert.Equal(t, 1, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
		require.Nil(t, cmd)
		assert.Equal(t, 0, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeySpace})
		require.Nil(t, cmd)
		assert.Equal(t, 1, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
		require.Nil(t, cmd)
		assert.Equal(t, relsEditFieldNotes, updated.editFocus)

		updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.Nil(t, cmd)
		assert.True(t, updated.editMeta.Active)

		updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
		require.Nil(t, cmd)
		assert.Equal(t, relsEditFieldStatus, updated.editFocus)

		before := updated.editStatusIdx
		updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
		require.Nil(t, cmd)
		assert.Equal(t, before, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
		require.Nil(t, cmd)
		assert.Equal(t, relsViewDetail, updated.view)
	})

	t.Run("save branch with notes text", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewEdit
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}
		model.editFocus = relsEditFieldNotes
		model.editMeta.Buffer = "some notes text"

		updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
		require.NotNil(t, cmd)
		assert.True(t, updated.editSaving)
	})
}

func TestFormatCreateCandidateLineFallbacks(t *testing.T) {
	line := formatCreateCandidateLine(relationshipCreateCandidate{})
	assert.Equal(t, "node · node · -", line)

	line = formatCreateCandidateLine(relationshipCreateCandidate{
		Name:   "alpha",
		Kind:   "entity/person",
		Status: "active",
	})
	assert.Equal(t, "alpha · entity/person · active", line)
}

func TestRelationshipsHandleFilterInputBranchMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.filtering = true
	model.names["ent-1"] = "alpha"
	model.names["ent-2"] = "beta"
	model.allItems = []api.Relationship{
		{ID: "rel-1", Type: "depends-on", Status: "active", SourceID: "ent-1", TargetID: "ent-2"},
	}
	model.applyListFilter()

	updated, cmd := model.handleFilterInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.filterBuf)

	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "a ", updated.filterBuf)

	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.filterBuf)

	updated.filterBuf = ""
	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)

	updated.filterBuf = ""
	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)

	updated.filterBuf = "dep"
	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.filterBuf)
	require.Len(t, updated.items, 1)

	updated.filtering = true
	updated.filterBuf = "x"
	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "x", updated.filterBuf)
}

func TestRelationshipsRenderModeLineStateVariants(t *testing.T) {
	model := NewRelationshipsModel(nil)

	model.view = relsViewList
	listLine := model.renderModeLine()
	assert.Contains(t, listLine, "Add")
	assert.Contains(t, listLine, "Library")

	model.view = relsViewCreateType
	addLine := model.renderModeLine()
	assert.Contains(t, addLine, "Add")
	assert.Contains(t, addLine, "Library")

	model.modeFocus = true
	addFocusLine := model.renderModeLine()
	assert.Contains(t, addFocusLine, "Add")
	assert.Contains(t, addFocusLine, "Library")

	model.view = relsViewList
	listFocusLine := model.renderModeLine()
	assert.Contains(t, listFocusLine, "Add")
	assert.Contains(t, listFocusLine, "Library")
}

func TestRelationshipsHandleConfirmKeysBranchMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewConfirm
	model.detail = &api.Relationship{ID: "rel-1"}

	updated, cmd := model.handleConfirmKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewDetail, updated.view)

	updated.view = relsViewConfirm
	updated, cmd = updated.handleConfirmKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewDetail, updated.view)

	_, okClient := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/api/relationships/rel-1" {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                "rel-1",
					"source_id":         "ent-1",
					"target_id":         "ent-2",
					"relationship_type": "uses",
					"status":            "inactive",
					"created_at":        time.Now(),
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	model = NewRelationshipsModel(okClient)
	model.view = relsViewConfirm
	model.detail = &api.Relationship{ID: "rel-1"}

	updated, cmd = model.handleConfirmKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(relTabSavedMsg)
	require.True(t, ok)
	assert.Equal(t, relsViewDetail, updated.view)

	_, errClient := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":"PATCH_FAILED","message":"patch failed"}}`, http.StatusInternalServerError)
	})
	errModel := NewRelationshipsModel(errClient)
	errModel.view = relsViewConfirm
	errModel.detail = &api.Relationship{ID: "rel-1"}

	_, cmd = errModel.handleConfirmKeys(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	errOut, ok := cmd().(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "PATCH_FAILED")
}

func TestRelationshipsRenderConfirmBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	out := components.SanitizeText(model.renderConfirm())
	assert.Contains(t, out, "Archive this relationship")

	model.width = 90
	model.detail = &api.Relationship{
		ID:         "rel-1",
		Type:       "depends-on",
		Status:     "",
		SourceType: "entity",
		SourceID:   "ent-1",
		TargetType: "context",
		TargetID:   "ctx-1",
	}

	out = components.SanitizeText(model.renderConfirm())
	assert.Contains(t, out, "depends-on")
	assert.Contains(t, out, "unknown entity")
	assert.Contains(t, out, "unknown context")
	assert.Contains(t, out, "inactive")
}

func TestRelationshipsLoadRelationshipNamesMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)
	assert.Nil(t, model.loadRelationshipNames(nil))

	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities/ent-2":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   "ent-2",
					"name": "Entity Two",
				},
			}))
			return
		case "/api/context/ctx-1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":    "ctx-1",
					"title": "Context One",
				},
			}))
			return
		case "/api/jobs/job-1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":    "job-1",
					"title": "Job One",
				},
			}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model = NewRelationshipsModel(client)
	cmd := model.loadRelationshipNames([]api.Relationship{
		{
			SourceID:   "ent-1",
			SourceType: "entity",
			SourceName: "Entity One",
			TargetID:   "ctx-1",
			TargetType: "context",
		},
		{
			SourceID:   "job-1",
			SourceType: "job",
			TargetID:   "ent-2",
			TargetType: "entity",
		},
		{
			SourceID:   "file-1",
			SourceType: "file",
			TargetID:   "file-2",
			TargetType: "file",
		},
	})
	require.NotNil(t, cmd)
	msg := cmd()
	namesMsg, ok := msg.(relTabNamesLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "Entity One", namesMsg.names["ent-1"])
	assert.Equal(t, "Context One", namesMsg.names["ctx-1"])
	assert.Equal(t, "Job One", namesMsg.names["job-1"])
	assert.Equal(t, "Entity Two", namesMsg.names["ent-2"])
	assert.NotContains(t, namesMsg.names, "file-1")
	assert.NotContains(t, namesMsg.names, "file-2")
}

func TestRelationshipsLoadRelationshipsSuccessAndError(t *testing.T) {
	_, okClient := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/relationships", r.URL.Path)
		assert.Equal(t, "active", r.URL.Query().Get("status_category"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
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
					"created_at":        time.Now(),
				},
			},
		}))
	})

	model := NewRelationshipsModel(okClient)
	cmd := model.loadRelationships()
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(relTabLoadedMsg)
	require.True(t, ok)
	require.Len(t, loaded.items, 1)
	assert.Equal(t, "rel-1", loaded.items[0].ID)

	_, errClient := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":"REL_FAILED","message":"relationships failed"}}`, http.StatusInternalServerError)
	})
	errModel := NewRelationshipsModel(errClient)
	cmd = errModel.loadRelationships()
	require.NotNil(t, cmd)
	errOut, ok := cmd().(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "REL_FAILED")
}

func TestRelationshipsHandleListKeysAdditionalBranches(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.view = relsViewList
	model.dataTable.SetRows([]table.Row{{"a"}, {"b"}})
	model.items = []api.Relationship{
		{ID: "rel-1"},
		{ID: "rel-2"},
	}

	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.dataTable.Cursor())

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.dataTable.Cursor())

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated.modeFocus = false
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewDetail, updated.view)
	require.NotNil(t, updated.detail)

	updated.view = relsViewList
	updated.detail = nil
	updated.items = nil
	updated.dataTable.SetRows(nil)
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Nil(t, updated.detail)

	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 'f', Text: "f"})
	require.Nil(t, cmd)
	assert.True(t, updated.filtering)

	updated.filtering = false
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)
	assert.Equal(t, "", updated.createQuery)
}

func TestRelationshipsViewBranchMatrix(t *testing.T) {
	now := time.Now().UTC()
	model := NewRelationshipsModel(nil)
	model.width = 90
	model.loading = false
	model.items = []api.Relationship{
		{
			ID:         "rel-1",
			Type:       "depends-on",
			Status:     "active",
			SourceType: "entity",
			SourceID:   "ent-1",
			TargetType: "entity",
			TargetID:   "ent-2",
			CreatedAt:  now,
		},
	}
	model.dataTable.SetRows([]table.Row{{"depends-on · unknown entity -> unknown entity"}})

	model.filtering = true
	model.view = relsViewList
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Filter Relationships")

	model.filtering = false
	model.view = relsViewList
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Library")
}

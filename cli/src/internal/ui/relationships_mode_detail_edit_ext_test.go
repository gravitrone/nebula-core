package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationshipsHandleModeKeysBranchMatrix(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.modeFocus = true
	model.view = relsViewList

	updated, cmd := model.handleModeKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated.view = relsViewList
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
	assert.Equal(t, relsViewCreateSourceSearch, updated.view)

	updated.modeFocus = true
	updated.view = relsViewCreateType
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyRight})
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

		updated, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyUp})
		require.Nil(t, cmd)
		assert.True(t, updated.modeFocus)

		updated, cmd = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		require.Nil(t, cmd)
		assert.True(t, updated.metaExpanded)

		updated, cmd = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		require.Nil(t, cmd)
		assert.False(t, updated.metaExpanded)
	})

	t.Run("edit and confirm branches", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewDetail
		model.detail = &api.Relationship{ID: "rel-1", Status: "active", Properties: api.JSONMap{}, CreatedAt: now}

		updated, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
		require.Nil(t, cmd)
		assert.Equal(t, relsViewEdit, updated.view)
		assert.Equal(t, relsEditFieldStatus, updated.editFocus)

		updated.view = relsViewDetail
		updated, cmd = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		require.Nil(t, cmd)
		assert.Equal(t, relsViewConfirm, updated.view)
		assert.Equal(t, "archive", updated.confirmKind)
	})

	t.Run("back clears detail state", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewDetail
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}
		model.metaExpanded = true

		updated, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
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
	assert.Contains(t, out, "Relationship")
	assert.Contains(t, out, "depends-on")

	model.detail.Properties = api.JSONMap{"note": "hello world"}
	out = model.renderDetail()
	assert.Contains(t, out, "note")
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

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		require.Nil(t, cmd)
		assert.Equal(t, relsEditFieldStatus, updated.editFocus)
	})

	t.Run("status and properties input branches", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewEdit
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}
		model.editStatusIdx = 0
		model.editFocus = relsEditFieldStatus

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		require.Nil(t, cmd)
		assert.Equal(t, 1, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		require.Nil(t, cmd)
		assert.Equal(t, 0, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		require.Nil(t, cmd)
		assert.Equal(t, 1, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		require.Nil(t, cmd)
		assert.Equal(t, relsEditFieldProperties, updated.editFocus)

		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		require.Nil(t, cmd)
		assert.True(t, updated.editMeta.Active)

		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
		require.Nil(t, cmd)
		assert.Equal(t, relsEditFieldStatus, updated.editFocus)

		before := updated.editStatusIdx
		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		require.Nil(t, cmd)
		assert.Equal(t, before, updated.editStatusIdx)

		updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
		require.Nil(t, cmd)
		assert.Equal(t, relsViewDetail, updated.view)
	})

	t.Run("save branch with parse error", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewEdit
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}
		model.editFocus = relsEditFieldProperties
		model.editMeta.Buffer = "bad metadata line"

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
		require.Nil(t, cmd)
		assert.False(t, updated.editSaving)
	})

	t.Run("save branch with valid metadata", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewEdit
		model.detail = &api.Relationship{ID: "rel-1", CreatedAt: now}
		model.editFocus = relsEditFieldProperties
		model.editMeta.Buffer = "note: ok"

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
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

	updated, cmd := model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.filterBuf)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "a ", updated.filterBuf)

	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.filterBuf)

	updated.filterBuf = ""
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)

	updated.filterBuf = ""
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyTab})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)

	updated.filterBuf = "dep"
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.filterBuf)
	require.Len(t, updated.items, 1)

	updated.filtering = true
	updated.filterBuf = "x"
	updated, cmd = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEnter})
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

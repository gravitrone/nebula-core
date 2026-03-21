package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesUpdateDispatchAdditionalBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewSearch

	updated, cmd := model.Update(entitiesLoadedMsg{
		items: []api.Entity{{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active"}},
	})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewList, updated.view)
	require.Len(t, updated.items, 1)

	updated.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	updated.detailRels = []api.Relationship{{ID: "rel-existing"}}
	next, cmd := updated.Update(entityDetailRelationshipsLoadedMsg{
		id:    "other-id",
		items: []api.Relationship{{ID: "rel-new"}},
	})
	require.Nil(t, cmd)
	require.Len(t, next.detailRels, 1)
	assert.Equal(t, "rel-existing", next.detailRels[0].ID)

	next.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	next, cmd = next.Update(entityDetailRelationshipsLoadedMsg{
		id:    "ent-1",
		items: []api.Relationship{{ID: "rel-new"}},
	})
	require.Nil(t, cmd)
	require.Len(t, next.detailRels, 1)
	assert.Equal(t, "rel-new", next.detailRels[0].ID)

	next.scopeNames = nil
	withScopes, cmd := next.Update(entityScopesLoadedMsg{
		names: map[string]string{"scope-1": "public", "scope-2": "private"},
	})
	require.Nil(t, cmd)
	require.NotNil(t, withScopes.scopeNames)
	assert.Equal(t, "public", withScopes.scopeNames["scope-1"])
	assert.Equal(t, []string{"private", "public"}, withScopes.scopeOptions)

	withScopes.loading = true
	withScopes.relLoading = true
	withScopes.relateLoading = true
	withScopes.historyLoading = true
	withScopes.editSaving = true
	withScopes.addSaving = true
	withScopes.bulkRunning = true
	withErr, cmd := withScopes.Update(errMsg{err: errors.New("boom")})
	require.Nil(t, cmd)
	assert.False(t, withErr.loading)
	assert.False(t, withErr.relLoading)
	assert.False(t, withErr.relateLoading)
	assert.False(t, withErr.historyLoading)
	assert.False(t, withErr.editSaving)
	assert.False(t, withErr.addSaving)
	assert.False(t, withErr.bulkRunning)
	assert.Contains(t, withErr.errText, "boom")
}

func TestEntitiesUpdateKeyDispatchWithEditView(t *testing.T) {
	model := NewEntitiesModel(nil)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)

	updated.view = entitiesViewEdit
	updated.detail = &api.Entity{ID: "ent-1", Name: "Alpha", Status: "active", Type: "person"}
	updated.startEdit()
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, entitiesViewDetail, updated.view)
}

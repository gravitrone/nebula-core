package ui

import (
	"encoding/json"
	"net/http"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolsLoadProtocolsSuccessAndError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/protocols" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "proto-1", "name": "Checklist", "title": "Ops checklist", "status": "active"},
				},
			}))
		})
		model := NewProtocolsModel(client)
		msg := model.loadProtocols()
		loaded, ok := msg.(protocolsLoadedMsg)
		require.True(t, ok)
		require.Len(t, loaded.items, 1)
		assert.Equal(t, "proto-1", loaded.items[0].ID)
	})

	t.Run("error", func(t *testing.T) {
		_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		model := NewProtocolsModel(client)
		msg := model.loadProtocols()
		_, ok := msg.(errMsg)
		assert.True(t, ok)
	})
}

func TestProtocolsUpdateMessageMatrix(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.width = 88
	model.dataTable.SetRows([]table.Row{{"stale"}})
	model.dataTable.SetCursor(0)
	model.loading = true
	model.searchBuf = "ops"

	model, cmd := model.Update(protocolsLoadedMsg{items: []api.Protocol{
		{ID: "proto-1", Name: "ops-checklist", Title: "daily ops", Status: "active"},
		{ID: "proto-2", Name: "deploy", Title: "release", Status: "active"},
	}})
	require.Nil(t, cmd)
	assert.False(t, model.loading)
	require.Len(t, model.allItems, 2)
	require.Len(t, model.items, 1)
	assert.Equal(t, "proto-1", model.items[0].ID)

	model.addSaving = true
	model.addErr = "old"
	model, cmd = model.Update(protocolCreatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, model.addSaving)
	assert.Equal(t, "", model.addErr)
	assert.True(t, model.loading)

	model.editSaving = true
	model.detail = &api.Protocol{ID: "proto-1"}
	model.detailRels = []api.Relationship{{ID: "rel-1"}}
	model.view = protocolsViewEdit
	model, cmd = model.Update(protocolUpdatedMsg{})
	require.NotNil(t, cmd)
	assert.False(t, model.editSaving)
	assert.Nil(t, model.detail)
	assert.Nil(t, model.detailRels)
	assert.Equal(t, protocolsViewList, model.view)
	assert.True(t, model.loading)

	model.detail = &api.Protocol{ID: "proto-1"}
	model, cmd = model.Update(protocolRelationshipsLoadedMsg{
		id:            "proto-1",
		relationships: []api.Relationship{{ID: "rel-2"}},
	})
	require.Nil(t, cmd)
	require.Len(t, model.detailRels, 1)
	assert.Equal(t, "rel-2", model.detailRels[0].ID)

	model.detailRels = nil
	model, cmd = model.Update(protocolRelationshipsLoadedMsg{
		id:            "proto-999",
		relationships: []api.Relationship{{ID: "rel-x"}},
	})
	require.Nil(t, cmd)
	assert.Nil(t, model.detailRels)

	model.loading = true
	model.addSaving = true
	model.editSaving = true
	model.addErr = ""
	model, cmd = model.Update(errMsg{assert.AnError})
	require.Nil(t, cmd)
	assert.False(t, model.loading)
	assert.False(t, model.addSaving)
	assert.False(t, model.editSaving)
	assert.Contains(t, model.addErr, "assert.AnError")
}

func TestProtocolsUpdateKeyRoutingByView(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.modeFocus = true
	model.view = protocolsViewList

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.view = protocolsViewList
	updated.modeFocus = false
	updated.filtering = false
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.Equal(t, protocolsViewAdd, updated.view)

	updated.view = protocolsViewAdd
	updated.modeFocus = false
	updated.addFocus = protoFieldStatus
	updated.addStatusIdx = 0
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.addStatusIdx)

	updated.view = protocolsViewDetail
	updated.detail = &api.Protocol{ID: "proto-1", Name: "checklist", Title: "ops", Status: "active"}
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, protocolsViewEdit, updated.view)

	updated.view = protocolsViewEdit
	updated.editFocus = protoEditFieldStatus
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, protocolsViewDetail, updated.view)
}

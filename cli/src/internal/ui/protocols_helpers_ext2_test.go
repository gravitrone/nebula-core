package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolsLoadDetailRelationshipsSuccessAndError(t *testing.T) {
	now := time.Now()
	_, client := testProtocolsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/relationships/protocol/proto-1":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id":          "rel-1",
					"source_type": "protocol",
					"source_id":   "proto-1",
					"target_type": "entity",
					"target_id":   "ent-1",
					"type":        "applies-to",
					"status":      "active",
					"created_at":  now,
				}},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	model := NewProtocolsModel(client)
	msg := model.loadDetailRelationships("proto-1")().(protocolRelationshipsLoadedMsg) //nolint:forcetypeassert
	require.Len(t, msg.relationships, 1)
	assert.Equal(t, "rel-1", msg.relationships[0].ID)

	msg = model.loadDetailRelationships("proto-2")().(protocolRelationshipsLoadedMsg) //nolint:forcetypeassert
	assert.Equal(t, "proto-2", msg.id)
	assert.Nil(t, msg.relationships)
}

func TestProtocolsHandleListKeysBranches(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.items = []api.Protocol{{ID: "proto-1", Name: "alpha", Title: "Alpha"}, {ID: "proto-2", Name: "beta", Title: "Beta"}}
	model.list.SetItems([]string{"alpha", "beta"})

	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.list.Selected())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, updated.list.Selected())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.True(t, updated.modeFocus)

	updated.modeFocus = false
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, protocolsViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "proto-1", updated.detail.ID)

	updated.view = protocolsViewList
	updated.searchInput.SetValue("a")
	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.searchInput.Value())

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: 'f', Text: "f"})
	assert.True(t, updated.filtering)

	updated.filtering = false
	updated.view = protocolsViewList
	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, protocolsViewAdd, updated.view)

	updated.view = protocolsViewList
	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	assert.Equal(t, protocolsViewAdd, updated.view)
}

func TestProtocolsHandleFilterInputBranches(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.filtering = true

	updated, cmd := model.handleFilterInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchInput.Value())

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.searchInput.Value())

	updated.searchInput.SetValue("abc")
	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.searchInput.Value())

	updated.filtering = true
	updated.searchInput.SetValue("xy")
	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, updated.filtering)
	assert.Equal(t, "xy", updated.searchInput.Value())
}

func TestProtocolsHandleAddKeysStatusTagsApplyMetadataAndBack(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.view = protocolsViewAdd
	model.addFocus = protoFieldStatus

	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.addStatusIdx)

	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = protoFieldTags
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'A', Text: "A"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, []string{"a"}, updated.addTags)

	updated.addFocus = protoFieldApplies
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 't', Text: "t"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, []string{"ent"}, updated.addApplies)

	updated.addFocus = protoFieldMetadata
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.addMeta.Active)
	updated.addMeta.Active = false

	updated.addFocus = protoFieldName
	updated.addFields[protoFieldName].value = "ab"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "a", updated.addFields[protoFieldName].value)

	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, protocolsViewList, updated.view)
}

func TestProtocolsHandleEditKeysStatusTagsApplyMetadataAndBack(t *testing.T) {
	content := "hello"
	model := NewProtocolsModel(nil)
	model.detail = &api.Protocol{ID: "proto-1", Name: "p1", Title: "Protocol", Content: &content, Status: "active"}
	model.startEdit()
	model.view = protocolsViewEdit

	model.editFocus = protoEditFieldStatus
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.editStatusIdx)

	updated.editFocus = protoEditFieldTags
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'B', Text: "B"})
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, []string{"b"}, updated.editTags)

	updated.editFocus = protoEditFieldApplies
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'j', Text: "j"})
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, []string{"j"}, updated.editApplies)

	updated.editFocus = protoEditFieldMetadata
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.editMeta.Active)
	updated.editMeta.Active = false

	updated.editFocus = protoEditFieldTitle
	updated.editFields[protoEditFieldTitle].value = "ab"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "a", updated.editFields[protoEditFieldTitle].value)

	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, protocolsViewDetail, updated.view)
}

func TestProtocolsTagAndApplyInputHelpers(t *testing.T) {
	model := NewProtocolsModel(nil)

	updated, cmd := model.handleTagInput(tea.KeyPressMsg{Code: 'A', Text: "A"}, true)
	require.Nil(t, cmd)
	assert.Equal(t, "A", updated.addTagInput.Value())

	updated, _ = updated.handleTagInput(tea.KeyPressMsg{Code: tea.KeyBackspace}, true)
	assert.Equal(t, "", updated.addTagInput.Value())

	updated.addTagInput.SetValue("#Tag")
	updated, _ = updated.handleTagInput(tea.KeyPressMsg{Code: tea.KeyEnter}, true)
	assert.Equal(t, []string{"tag"}, updated.addTags)

	updated.editTagInput.SetValue("edit-tag")
	updated, _ = updated.handleTagInput(tea.KeyPressMsg{Code: tea.KeyEnter}, false)
	assert.Equal(t, []string{"edit-tag"}, updated.editTags)

	updated, _ = updated.handleApplyInput(tea.KeyPressMsg{Code: 'e', Text: "e"}, true)
	assert.Equal(t, "e", updated.addApplyInput.Value())
	updated, _ = updated.handleApplyInput(tea.KeyPressMsg{Code: tea.KeyEnter}, true)
	assert.Equal(t, []string{"e"}, updated.addApplies)

	updated.editApplyInput.SetValue("job")
	updated, _ = updated.handleApplyInput(tea.KeyPressMsg{Code: tea.KeyEnter}, false)
	assert.Equal(t, []string{"job"}, updated.editApplies)
}

func TestProtocolsCommitTagEmptyAndNormalizeBranches(t *testing.T) {
	model := NewProtocolsModel(nil)

	model.addTagInput.SetValue("   ")
	model.commitTag(true)
	assert.Empty(t, model.addTags)
	assert.Equal(t, "   ", model.addTagInput.Value())

	model.addTagInput.SetValue("#")
	model.commitTag(true)
	assert.Empty(t, model.addTags)
	assert.Equal(t, "", model.addTagInput.Value())

	model.editTagInput.SetValue("  Mixed Case Tag  ")
	model.commitTag(false)
	assert.Equal(t, []string{"mixed-case-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())

	model.editTagInput.SetValue("#")
	model.commitTag(false)
	assert.Equal(t, []string{"mixed-case-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagInput.Value())
}

func TestProtocolsHandleEditKeysAdditionalBranches(t *testing.T) {
	content := "hello"
	model := NewProtocolsModel(nil)
	model.detail = &api.Protocol{ID: "proto-1", Name: "p1", Title: "Protocol", Content: &content, Status: "active"}
	model.startEdit()
	model.view = protocolsViewEdit

	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	model.editSaving = false
	model.editMeta.Active = true
	updated, cmd = model.handleEditKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.True(t, updated.editMeta.Active)

	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.editMeta.Active)

	updated.editFocus = protoEditFieldStatus
	updated.editStatusIdx = 0
	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, len(protocolStatusOptions)-1, updated.editStatusIdx)

	updated.editFocus = protoEditFieldTitle
	updated.editFields[protoEditFieldTitle].value = ""
	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.editFields[protoEditFieldTitle].value)

	updated.editFields[protoEditFieldTitle].value = ""
	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.editFields[protoEditFieldTitle].value)

	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)
}

func TestProtocolsSaveAddValidationAndMetadataError(t *testing.T) {
	model := NewProtocolsModel(nil)
	updated, cmd := model.saveAdd()
	assert.Nil(t, cmd)
	assert.Equal(t, "Name is required", updated.addErr)

	updated.addFields[protoFieldName].value = "p1"
	updated, cmd = updated.saveAdd()
	assert.Nil(t, cmd)
	assert.Equal(t, "Title is required", updated.addErr)

	updated.addFields[protoFieldTitle].value = "Protocol"
	updated, cmd = updated.saveAdd()
	assert.Nil(t, cmd)
	assert.Equal(t, "Content is required", updated.addErr)

	updated.addFields[protoFieldContent].value = "rules"
	updated.addMeta.Buffer = "invalid"
	updated, cmd = updated.saveAdd()
	assert.Nil(t, cmd)
	assert.NotEmpty(t, updated.addErr)
}

func TestProtocolsSaveEditNilDetailAndMetadataError(t *testing.T) {
	model := NewProtocolsModel(nil)
	updated, cmd := model.saveEdit()
	assert.Nil(t, cmd)
	assert.Equal(t, model, updated)

	content := "rules"
	model.detail = &api.Protocol{ID: "proto-1", Name: "p1", Title: "Protocol", Content: &content, Status: "active"}
	model.startEdit()
	model.editMeta.Buffer = "invalid"
	updated, cmd = model.saveEdit()
	assert.Nil(t, cmd)
	assert.NotEmpty(t, updated.addErr)
}

func TestProtocolsViewOverlayBranches(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.width = 90
	model.view = protocolsViewList

	model.filtering = true
	out := model.View()
	assert.Contains(t, out, "Filter Protocols")

	model.filtering = false
	model.addMeta.Active = true
	assert.NotEmpty(t, model.View())
	model.addMeta.Active = false

	model.editMeta.Active = true
	assert.NotEmpty(t, model.View())
}

func TestProtocolsHandleApplyInputAdditionalBranches(t *testing.T) {
	model := NewProtocolsModel(nil)

	model.addApplyInput.SetValue("entity")
	updated, cmd := model.handleApplyInput(tea.KeyPressMsg{Code: tea.KeyDelete}, true)
	require.Nil(t, cmd)
	assert.Equal(t, "entit", updated.addApplyInput.Value())

	updated.editApplyInput.SetValue("job")
	updated, cmd = updated.handleApplyInput(tea.KeyPressMsg{Code: tea.KeyBackspace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "jo", updated.editApplyInput.Value())

	updated.addApplyInput.SetValue("context")
	updated, cmd = updated.handleApplyInput(tea.KeyPressMsg{Code: ',', Text: ","}, true)
	require.Nil(t, cmd)
	assert.Contains(t, updated.addApplies, "context")
	assert.Equal(t, "", updated.addApplyInput.Value())

	updated.editApplyInput.SetValue("file")
	updated, cmd = updated.handleApplyInput(tea.KeyPressMsg{Code: tea.KeySpace}, false)
	require.Nil(t, cmd)
	assert.Contains(t, updated.editApplies, "file")
	assert.Equal(t, "", updated.editApplyInput.Value())

	updated, cmd = updated.handleApplyInput(tea.KeyPressMsg{Code: 'x', Text: "x"}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "x", updated.editApplyInput.Value())

	// Non-char keys should leave buffers unchanged.
	beforeAdd := updated.addApplyInput.Value()
	beforeEdit := updated.editApplyInput.Value()
	updated, cmd = updated.handleApplyInput(tea.KeyPressMsg{Code: tea.KeyUp}, true)
	require.Nil(t, cmd)
	assert.Equal(t, beforeAdd, updated.addApplyInput.Value())
	assert.Equal(t, beforeEdit, updated.editApplyInput.Value())
}

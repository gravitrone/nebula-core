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

	updated, cmd := model.handleListKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.list.Selected())

	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, updated.list.Selected())

	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, updated.modeFocus)

	updated.modeFocus = false
	updated, cmd = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, protocolsViewDetail, updated.view)
	require.NotNil(t, updated.detail)
	assert.Equal(t, "proto-1", updated.detail.ID)

	updated.view = protocolsViewList
	updated.searchBuf = "a"
	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "", updated.searchBuf)

	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	assert.True(t, updated.filtering)

	updated.filtering = false
	updated.view = protocolsViewList
	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, protocolsViewAdd, updated.view)

	updated.view = protocolsViewList
	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.Equal(t, protocolsViewAdd, updated.view)
}

func TestProtocolsHandleFilterInputBranches(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.filtering = true

	updated, cmd := model.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.searchBuf)

	updated, _ = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "", updated.searchBuf)

	updated.searchBuf = "abc"
	updated, _ = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.searchBuf)

	updated.filtering = true
	updated.searchBuf = "xy"
	updated, _ = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, updated.filtering)
	assert.Equal(t, "xy", updated.searchBuf)
}

func TestProtocolsHandleAddKeysStatusTagsApplyMetadataAndBack(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.view = protocolsViewAdd
	model.addFocus = protoFieldStatus

	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.addStatusIdx)

	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = protoFieldTags
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"a"}, updated.addTags)

	updated.addFocus = protoFieldApplies
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"ent"}, updated.addApplies)

	updated.addFocus = protoFieldMetadata
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.addMeta.Active)
	updated.addMeta.Active = false

	updated.addFocus = protoFieldName
	updated.addFields[protoFieldName].value = "ab"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", updated.addFields[protoFieldName].value)

	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, protocolsViewList, updated.view)
}

func TestProtocolsHandleEditKeysStatusTagsApplyMetadataAndBack(t *testing.T) {
	content := "hello"
	model := NewProtocolsModel(nil)
	model.detail = &api.Protocol{ID: "proto-1", Name: "p1", Title: "Protocol", Content: &content, Status: "active"}
	model.startEdit()
	model.view = protocolsViewEdit

	model.editFocus = protoEditFieldStatus
	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.editStatusIdx)

	updated.editFocus = protoEditFieldTags
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'B'}})
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"b"}, updated.editTags)

	updated.editFocus = protoEditFieldApplies
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"j"}, updated.editApplies)

	updated.editFocus = protoEditFieldMetadata
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.editMeta.Active)
	updated.editMeta.Active = false

	updated.editFocus = protoEditFieldTitle
	updated.editFields[protoEditFieldTitle].value = "ab"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "a", updated.editFields[protoEditFieldTitle].value)

	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, protocolsViewDetail, updated.view)
}

func TestProtocolsTagAndApplyInputHelpers(t *testing.T) {
	model := NewProtocolsModel(nil)

	updated, cmd := model.handleTagInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}, true)
	require.Nil(t, cmd)
	assert.Equal(t, "A", updated.addTagBuf)

	updated, _ = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyBackspace}, true)
	assert.Equal(t, "", updated.addTagBuf)

	updated.addTagBuf = "#Tag"
	updated, _ = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyEnter}, true)
	assert.Equal(t, []string{"tag"}, updated.addTags)

	updated.editTagBuf = "edit-tag"
	updated, _ = updated.handleTagInput(tea.KeyMsg{Type: tea.KeyEnter}, false)
	assert.Equal(t, []string{"edit-tag"}, updated.editTags)

	updated, _ = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}, true)
	assert.Equal(t, "e", updated.addApplyBuf)
	updated, _ = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeyEnter}, true)
	assert.Equal(t, []string{"e"}, updated.addApplies)

	updated.editApplyBuf = "job"
	updated, _ = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeyEnter}, false)
	assert.Equal(t, []string{"job"}, updated.editApplies)
}

func TestProtocolsCommitTagEmptyAndNormalizeBranches(t *testing.T) {
	model := NewProtocolsModel(nil)

	model.addTagBuf = "   "
	model.commitTag(true)
	assert.Empty(t, model.addTags)
	assert.Equal(t, "   ", model.addTagBuf)

	model.addTagBuf = "#"
	model.commitTag(true)
	assert.Empty(t, model.addTags)
	assert.Equal(t, "", model.addTagBuf)

	model.editTagBuf = "  Mixed Case Tag  "
	model.commitTag(false)
	assert.Equal(t, []string{"mixed-case-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)

	model.editTagBuf = "#"
	model.commitTag(false)
	assert.Equal(t, []string{"mixed-case-tag"}, model.editTags)
	assert.Equal(t, "", model.editTagBuf)
}

func TestProtocolsHandleEditKeysAdditionalBranches(t *testing.T) {
	content := "hello"
	model := NewProtocolsModel(nil)
	model.detail = &api.Protocol{ID: "proto-1", Name: "p1", Title: "Protocol", Content: &content, Status: "active"}
	model.startEdit()
	model.view = protocolsViewEdit

	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	model.editSaving = false
	model.editMeta.Active = true
	updated, cmd = model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	require.Nil(t, cmd)
	assert.True(t, updated.editMeta.Active)

	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.False(t, updated.editMeta.Active)

	updated.editFocus = protoEditFieldStatus
	updated.editStatusIdx = 0
	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, len(protocolStatusOptions)-1, updated.editStatusIdx)

	updated.editFocus = protoEditFieldTitle
	updated.editFields[protoEditFieldTitle].value = ""
	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.editFields[protoEditFieldTitle].value)

	updated.editFields[protoEditFieldTitle].value = ""
	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.editFields[protoEditFieldTitle].value)

	updated, cmd = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
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

	model.addApplyBuf = "entity"
	updated, cmd := model.handleApplyInput(tea.KeyMsg{Type: tea.KeyDelete}, true)
	require.Nil(t, cmd)
	assert.Equal(t, "entit", updated.addApplyBuf)

	updated.editApplyBuf = "job"
	updated, cmd = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeyBackspace}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "jo", updated.editApplyBuf)

	updated.addApplyBuf = "context"
	updated, cmd = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}}, true)
	require.Nil(t, cmd)
	assert.Contains(t, updated.addApplies, "context")
	assert.Equal(t, "", updated.addApplyBuf)

	updated.editApplyBuf = "file"
	updated, cmd = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeySpace}, false)
	require.Nil(t, cmd)
	assert.Contains(t, updated.editApplies, "file")
	assert.Equal(t, "", updated.editApplyBuf)

	updated, cmd = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, false)
	require.Nil(t, cmd)
	assert.Equal(t, "x", updated.editApplyBuf)

	// Non-char keys should leave buffers unchanged.
	beforeAdd := updated.addApplyBuf
	beforeEdit := updated.editApplyBuf
	updated, cmd = updated.handleApplyInput(tea.KeyMsg{Type: tea.KeyUp}, true)
	require.Nil(t, cmd)
	assert.Equal(t, beforeAdd, updated.addApplyBuf)
	assert.Equal(t, beforeEdit, updated.editApplyBuf)
}

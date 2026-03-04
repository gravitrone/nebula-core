package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesHandleEditKeysBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)

	t.Run("editSaving short-circuits", func(t *testing.T) {
		model.editSaving = true
		model.editFocus = editFieldStatus
		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		require.Nil(t, cmd)
		assert.Equal(t, editFieldStatus, updated.editFocus)
	})

	t.Run("scope selector branches", func(t *testing.T) {
		model.editSaving = false
		model.editFocus = editFieldScopes
		model.editScopeSelecting = true
		model.scopeOptions = []string{"public", "private"}
		model.editScopes = []string{"public"}
		model.editScopeIdx = 0

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		require.Nil(t, cmd)
		assert.Equal(t, 1, updated.editScopeIdx)

		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		assert.Equal(t, []string{"public", "private"}, updated.editScopes)
		assert.True(t, updated.editScopesDirty)

		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, 0, updated.editScopeIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.False(t, updated.editScopeSelecting)

		updated.editScopeSelecting = true
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.False(t, updated.editScopeSelecting)
	})

	t.Run("navigation, status, tags, metadata branches", func(t *testing.T) {
		model.editFocus = editFieldTags
		model.editTagBuf = "ab"

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		require.Nil(t, cmd)
		assert.Equal(t, "a", updated.editTagBuf)

		updated.editTagBuf = ""
		updated.editTags = []string{"alpha", "beta"}
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, []string{"alpha"}, updated.editTags)

		updated.editFocus = editFieldScopes
		updated.editScopes = []string{"public"}
		updated.editScopesDirty = false
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Empty(t, updated.editScopes)
		assert.True(t, updated.editScopesDirty)

		updated.editFocus = editFieldTags
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
		assert.Equal(t, "x", updated.editTagBuf)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Equal(t, []string{"alpha", "x"}, updated.editTags)
		assert.Equal(t, "", updated.editTagBuf)

		updated.editFocus = editFieldScopes
		updated.editScopeSelecting = false
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.True(t, updated.editScopeSelecting)

		updated.editFocus = editFieldStatus
		startStatus := updated.editStatusIdx
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
		assert.Equal(t, (startStatus+1)%len(entityStatusOptions), updated.editStatusIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyLeft})
		assert.Equal(t, startStatus, updated.editStatusIdx)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		assert.Equal(t, (startStatus+1)%len(entityStatusOptions), updated.editStatusIdx)

		updated.editFocus = editFieldMetadata
		updated.editMeta.Active = false
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.True(t, updated.editMeta.Active)

		updated.editFocus = editFieldStatus
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyDown})
		assert.Equal(t, editFieldScopes, updated.editFocus)
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Equal(t, editFieldStatus, updated.editFocus)

		updated.editScopeSelecting = true
		updated.view = entitiesViewEdit
		updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Equal(t, entitiesViewDetail, updated.view)
		assert.False(t, updated.editScopeSelecting)
	})
}

func TestEntitiesHandleEditKeysSaveBranches(t *testing.T) {
	now := time.Now().UTC()

	t.Run("invalid metadata blocks save", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.detail = &api.Entity{ID: "ent-1"}
		model.editFocus = editFieldMetadata
		model.editMeta.Buffer = "bad metadata line"

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
		require.Nil(t, cmd)
		assert.False(t, updated.editSaving)
		assert.NotEmpty(t, updated.errText)
	})

	t.Run("valid metadata returns save command", func(t *testing.T) {
		var captured api.UpdateEntityInput
		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/entities/") && r.Method == http.MethodPatch {
				require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": map[string]any{
						"id":                "ent-1",
						"name":              "Alpha",
						"type":              "person",
						"status":            "active",
						"privacy_scope_ids": []string{"scope-1"},
						"tags":              []string{"demo"},
						"metadata":          map[string]any{"profile": map[string]any{"timezone": "utc"}},
						"created_at":        now,
						"updated_at":        now,
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewEntitiesModel(client)
		model.detail = &api.Entity{ID: "ent-1", Status: "active"}
		model.editFocus = editFieldMetadata
		model.editMeta.Buffer = "profile | timezone | utc"

		updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyCtrlS})
		require.NotNil(t, cmd)
		assert.True(t, updated.editSaving)

		msg := cmd()
		umsg, ok := msg.(entityUpdatedMsg)
		require.True(t, ok)
		assert.Equal(t, "ent-1", umsg.entity.ID)
		require.NotNil(t, captured.Status)
		assert.Equal(t, "active", *captured.Status)
		assert.Equal(t, "utc", ((captured.Metadata["profile"]).(map[string]any))["timezone"])
	})
}

func TestEntitiesMetadataCopyHelpersBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)

	assert.Nil(t, model.copyCurrentMetadataRow())
	assert.Nil(t, model.copySelectedMetadataRows())
	assert.Nil(t, model.copyMetadataRows(nil))

	model.metaRows = []metadataDisplayRow{
		{field: "profile.timezone", value: "UTC"},
		{field: "profile.lang", value: ""},
	}
	model.metaList = components.NewList(5)
	model.metaList.SetItems([]string{"timezone", "lang"})

	origClipboard := copyEntityMetadataClipboard
	t.Cleanup(func() { copyEntityMetadataClipboard = origClipboard })

	var captured string
	copyEntityMetadataClipboard = func(text string) error {
		captured = text
		return nil
	}

	cmd := model.copyCurrentMetadataRow()
	require.NotNil(t, cmd)
	msg := cmd()
	copied, ok := msg.(entityMetadataCopiedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, copied.count)
	assert.Equal(t, "UTC", captured)

	model.metaSelected = map[int]bool{1: true, 9: true}
	cmd = model.copySelectedMetadataRows()
	require.NotNil(t, cmd)
	msg = cmd()
	copied, ok = msg.(entityMetadataCopiedMsg)
	require.True(t, ok)
	assert.Equal(t, 1, copied.count)
	assert.Equal(t, "None", captured)

	assert.Nil(t, model.copyMetadataRows([]int{99, -1}))

	copyEntityMetadataClipboard = func(text string) error {
		return fmt.Errorf("clipboard offline")
	}
	cmd = model.copyMetadataRows([]int{0})
	require.NotNil(t, cmd)
	msg = cmd()
	em, ok := msg.(errMsg)
	require.True(t, ok)
	require.Error(t, em.err)
	assert.Contains(t, em.err.Error(), "clipboard offline")
}

func TestEntitiesCompactJSONAndRelationshipDirectionBranches(t *testing.T) {
	assert.Equal(t, "", compactJSON(map[string]any{}))
	assert.Equal(t, "", compactJSON(nil))
	assert.Equal(t, `{"a":1}`, compactJSON(map[string]any{"a": 1}))

	model := NewEntitiesModel(nil)
	rel := api.Relationship{
		SourceID:   "ent-1",
		SourceName: "Alpha",
		TargetID:   "ent-2",
		TargetName: "Beta",
	}

	direction, other := model.relationshipDirection(rel)
	assert.Equal(t, "", direction)
	assert.Equal(t, "Beta", other)

	model.detail = &api.Entity{ID: "ent-1"}
	direction, other = model.relationshipDirection(rel)
	assert.Equal(t, "outgoing", direction)
	assert.Equal(t, "Beta", other)

	model.detail = &api.Entity{ID: "ent-3"}
	direction, other = model.relationshipDirection(rel)
	assert.Equal(t, "incoming", direction)
	assert.Equal(t, "Alpha", other)
}

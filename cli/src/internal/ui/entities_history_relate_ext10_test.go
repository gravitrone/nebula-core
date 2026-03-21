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

func TestEntitiesLoadHistoryAndHandleHistoryKeysBranches(t *testing.T) {
	t.Run("loadHistory nil-detail and error path", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		assert.Nil(t, model.loadHistory())

		_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "INTERNAL", "message": "history failed"},
			})
		})
		model = NewEntitiesModel(client)
		model.detail = &api.Entity{ID: "ent-1"}
		cmd := model.loadHistory()
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		assert.Contains(t, strings.ToLower(em.err.Error()), "history failed")
	})

	t.Run("handleHistoryKeys navigation and confirm branches", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.view = entitiesViewHistory
		model.history = []api.AuditEntry{
			{ID: "a1", Action: "update", ChangedAt: time.Now()},
			{ID: "a2", Action: "update", ChangedAt: time.Now()},
		}
		model.historyList = components.NewList(8)
		model.historyList.SetItems([]string{"a1", "a2"})

		next, cmd := model.handleHistoryKeys(tea.KeyMsg{Type: tea.KeyDown})
		assert.Nil(t, cmd)
		assert.Equal(t, 1, next.historyList.Selected())

		next, cmd = next.handleHistoryKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Nil(t, cmd)
		assert.Equal(t, 0, next.historyList.Selected())

		next.historyList.Cursor = 9
		next, cmd = next.handleHistoryKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewHistory, next.view)
		assert.Equal(t, "", next.confirmKind)

		next.historyList.Cursor = 0
		next, cmd = next.handleHistoryKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewConfirm, next.view)
		assert.Equal(t, "entity-revert", next.confirmKind)
		assert.Equal(t, "a1", next.confirmAuditID)
		assert.Equal(t, entitiesViewDetail, next.confirmReturn)

		next.view = entitiesViewHistory
		next, cmd = next.handleHistoryKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewDetail, next.view)
	})
}

func TestEntitiesRenderHistoryBranchMatrix(t *testing.T) {
	now := time.Now()
	model := NewEntitiesModel(nil)
	model.width = 42
	model.historyLoading = true
	out := components.SanitizeText(model.renderHistory())
	assert.Contains(t, out, "Loading history")

	model.historyLoading = false
	model.history = nil
	out = components.SanitizeText(model.renderHistory())
	assert.Contains(t, out, "No history entries yet")

	model.width = 120
	model.detail = &api.Entity{Name: "Alpha"}
	model.history = []api.AuditEntry{
		{ID: "a1", Action: "", ChangedAt: now, ChangedFields: nil},
		{ID: "a2", Action: "update", ChangedAt: now, ChangedFields: []string{"name", "status"}},
	}
	model.historyList = components.NewList(8)
	model.historyList.SetItems([]string{formatHistoryLine(model.history[0]), formatHistoryLine(model.history[1])})
	model.historyList.Cursor = 0

	out = components.SanitizeText(model.renderHistory())
	assert.Contains(t, out, "2 entries")
	assert.Contains(t, out, "UPDATE")
	assert.Contains(t, out, "Selected")

	// Force narrow mode path (preview below table).
	model.width = 52
	out = components.SanitizeText(model.renderHistory())
	assert.Contains(t, out, "2 entries")
}

func TestEntitiesRelateAndRelEditBranchMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 100
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha", Type: "person"}

	t.Run("search state keys", func(t *testing.T) {
		m := model
		m.view = entitiesViewRelateSearch

		next, cmd := m.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewRelateSearch, next.view)

		next.relateQuery = "be"
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd)
		assert.Equal(t, entitiesViewRelateSelect, next.view)
		assert.True(t, next.relateLoading)

		next.view = entitiesViewRelateSearch
		next.relateQuery = "be"
		next, _ = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, "b", next.relateQuery)
		next, _ = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeySpace})
		assert.Equal(t, "b ", next.relateQuery)
		next, _ = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Equal(t, entitiesViewRelationships, next.view)
	})

	t.Run("select and type states", func(t *testing.T) {
		m := model
		m.relateResults = []api.Entity{{ID: "ent-2", Name: "Beta", Type: "tool", Status: "active"}}
		m.relateList = components.NewList(8)
		m.relateList.SetItems([]string{"Beta"})
		m.view = entitiesViewRelateSelect

		next, cmd := m.handleRelateKeys(tea.KeyMsg{Type: tea.KeyDown})
		assert.Nil(t, cmd)
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyUp})
		assert.Nil(t, cmd)

		next.relateList.Cursor = 9
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewRelateSelect, next.view)
		assert.Nil(t, next.relateTarget)

		next.relateList.Cursor = 0
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewRelateType, next.view)
		require.NotNil(t, next.relateTarget)
		assert.Equal(t, "ent-2", next.relateTarget.ID)

		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewRelateType, next.view)

		next, _ = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		next, _ = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		next, _ = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyBackspace})
		assert.Equal(t, "k", next.relateType)
		next, _ = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd)
		assert.Equal(t, entitiesViewRelationships, next.view)
		assert.True(t, next.relLoading)

		next.view = entitiesViewRelateType
		next.relateTarget = nil
		next.relateType = "knows"
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewRelateType, next.view)

		next.view = entitiesViewRelateType
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewRelateSelect, next.view)

		next.view = entitiesViewRelateSelect
		next, cmd = next.handleRelateKeys(tea.KeyMsg{Type: tea.KeyEsc})
		assert.Nil(t, cmd)
		assert.Equal(t, entitiesViewRelateSearch, next.view)
	})

	t.Run("render relate and rel edit branches", func(t *testing.T) {
		m := model
		m.view = entitiesViewRelateSearch
		out := components.SanitizeText(m.renderRelate())
		assert.Contains(t, out, "Search Entity")

		m.view = entitiesViewRelateSelect
		m.relateLoading = true
		out = components.SanitizeText(m.renderRelate())
		assert.Contains(t, out, "Searching")

		m.relateLoading = false
		m.relateResults = nil
		out = components.SanitizeText(m.renderRelate())
		assert.Contains(t, out, "No matches")

		m.relateResults = []api.Entity{{ID: "ent-2", Name: "Beta", Type: "", Status: "", Tags: []string{"x"}}}
		m.relateList = components.NewList(8)
		m.relateList.SetItems([]string{"Beta"})
		out = components.SanitizeText(m.renderRelate())
		assert.Contains(t, out, "Beta")

		m.view = entitiesViewRelateType
		m.relateType = "knows"
		out = components.SanitizeText(m.renderRelate())
		assert.Contains(t, out, "Relationship Type")

		m.view = entitiesViewList
		assert.Equal(t, "", m.renderRelate())

		assert.Equal(t, "", m.renderRelateEntityPreview(api.Entity{}, 0))

		m.rels = []api.Relationship{{ID: "rel-1", SourceID: "ent-1", TargetID: "ent-2", Type: "", Status: "", Properties: map[string]any{"note": "x"}}}
		m.relList = components.NewList(8)
		m.relList.SetItems([]string{"rel-1"})
		m.startRelEdit()
		m.relEditFocus = relEditFieldStatus
		out = components.SanitizeText(m.renderRelEdit())
		assert.Contains(t, out, "Status:")

		m.relEditFocus = relEditFieldProperties
		out = components.SanitizeText(m.renderRelEdit())
		assert.Contains(t, out, "Properties (JSON)")
	})
}

func TestEntitiesRelationshipLabelAndLineBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	rel := api.Relationship{
		ID:         "rel-1",
		SourceID:   "ent-1",
		SourceName: "",
		TargetID:   "ent-2-long-id",
		TargetName: "",
		Type:       "",
	}

	// detail nil path -> no direction and short-id fallback label.
	line := model.formatRelationshipLine(rel)
	assert.Contains(t, line, "relationship")
	assert.Contains(t, line, shortID(rel.TargetID))

	model.detail = &api.Entity{ID: "ent-1"}
	line = model.formatRelationshipLine(rel)
	assert.Contains(t, line, "outgoing")

	rel.SourceID = "ent-9"
	line = model.formatRelationshipLine(rel)
	assert.Contains(t, line, "incoming")

	assert.Equal(t, "Beta", relationshipLabel("id-1", "Beta"))
	assert.Equal(t, shortID("abcdef123456"), relationshipLabel("abcdef123456", " "))
}

func TestEntitiesRenderRelationshipsPropertiesAndCursorFallback(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 96
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha", Type: "person"}
	model.rels = []api.Relationship{
		{
			ID:         "rel-1",
			SourceID:   "ent-9",
			SourceName: "Gamma",
			TargetID:   "ent-1",
			TargetName: "Alpha",
			Type:       "uses",
			Status:     "active",
			CreatedAt:  time.Now(),
			Properties: map[string]any{"note": "hi"},
		},
	}
	model.relList = components.NewList(6)
	model.relList.SetItems([]string{"uses"})
	model.relList.Cursor = 99 // hit out-of-range fallback -> idx=0

	out := components.SanitizeText(model.renderRelationships())
	assert.Contains(t, out, "incoming")
	assert.Contains(t, out, "note")
	assert.Contains(t, out, "hi")
}

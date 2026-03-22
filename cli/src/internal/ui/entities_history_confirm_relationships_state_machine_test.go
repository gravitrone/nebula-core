package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEntitiesHistoryRevertConfirmCallsAPI handles test entities history revert confirm calls api.
func TestEntitiesHistoryRevertConfirmCallsAPI(t *testing.T) {
	now := time.Now()
	var gotAuditID string

	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/entities/ent-1/history" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":             "audit-1",
						"table_name":     "entities",
						"record_id":      "ent-1",
						"action":         "update",
						"changed_fields": []string{"name"},
						"old_data":       map[string]any{},
						"new_data":       map[string]any{},
						"changed_at":     now,
					},
				},
			}))
			return
		case r.URL.Path == "/api/entities/ent-1/revert" && r.Method == http.MethodPost:
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			gotAuditID = body["audit_id"]
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":     "ent-1",
					"name":   "Reverted",
					"type":   "entity",
					"status": "active",
					"tags":   []string{},
				},
			}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewEntitiesModel(client)
	model.width = 60
	model, _ = model.Update(entitiesLoadedMsg{items: []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "entity", Status: "active", Tags: []string{}},
	}})

	// Enter detail.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, model.detail)

	// Enter history.
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	require.False(t, model.historyLoading)
	require.Len(t, model.history, 1)

	// Select the first entry to open confirm.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, entitiesViewConfirm, model.view)
	assert.Equal(t, "entity-revert", model.confirmKind)
	assert.Equal(t, "audit-1", model.confirmAuditID)

	// Confirm revert.
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "audit-1", gotAuditID)
	assert.Equal(t, entitiesViewDetail, model.view)
	require.NotNil(t, model.detail)
	assert.Equal(t, "Reverted", model.detail.Name)
}

// TestEntitiesArchiveConfirmCallsUpdateEntity handles test entities archive confirm calls update entity.
func TestEntitiesArchiveConfirmCallsUpdateEntity(t *testing.T) {
	var gotStatus string
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities/ent-1" && r.Method == http.MethodPatch {
			var input api.UpdateEntityInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			if input.Status != nil {
				gotStatus = *input.Status
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":     "ent-1",
				"name":   "Alpha",
				"type":   "entity",
				"status": gotStatus,
				"tags":   []string{},
			}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.width = 60
	model, _ = model.Update(entitiesLoadedMsg{items: []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "entity", Status: "active", Tags: []string{}},
	}})

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, model.detail)

	// Open archive confirm.
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	assert.Equal(t, entitiesViewConfirm, model.view)

	// Confirm.
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "inactive", gotStatus)
	assert.Equal(t, entitiesViewDetail, model.view)
	require.NotNil(t, model.detail)
	assert.Equal(t, "inactive", model.detail.Status)
}

// TestEntitiesEditScopesDirtyTriggersBulkScopesAndRefresh handles test entities edit scopes dirty triggers bulk scopes and refresh.
func TestEntitiesEditScopesDirtyTriggersBulkScopesAndRefresh(t *testing.T) {
	now := time.Now()
	var bulkCalled bool

	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/entities/ent-1" && r.Method == http.MethodPatch:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                "ent-1",
				"name":              "Alpha",
				"type":              "entity",
				"status":            "active",
				"privacy_scope_ids": []string{"s1"},
				"tags":              []string{},
				"created_at":        now,
				"updated_at":        now,
			}}))
			return
		case r.URL.Path == "/api/entities/bulk/scopes" && r.Method == http.MethodPost:
			bulkCalled = true
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"updated": 1, "entity_ids": []string{"ent-1"}},
			}))
			return
		case r.URL.Path == "/api/entities/ent-1" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                "ent-1",
				"name":              "Alpha",
				"type":              "entity",
				"status":            "active",
				"privacy_scope_ids": []string{"s1"},
				"tags":              []string{},
				"created_at":        now,
				"updated_at":        now,
			}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewEntitiesModel(client)
	model.width = 60
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha", Type: "entity", Status: "active", Tags: []string{}}
	model.scopeNames = map[string]string{"s1": "public"}
	model.scopeOptions = []string{"public", "private"}

	model.startEdit()
	model.editScopeInput.SetValue("private")
	model.commitEditScope()

	_, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)

	assert.True(t, bulkCalled)
	assert.Equal(t, entitiesViewDetail, model.view)
}

// TestEntitiesRelationshipsRelateArchiveAndRelEditValidation handles test entities relationships relate archive and rel edit validation.
func TestEntitiesRelationshipsRelateArchiveAndRelEditValidation(t *testing.T) {
	now := time.Now()
	var gotCreateType string
	var gotPatchStatus string

	relationshipList := []map[string]any{
		{
			"id":                "rel-1",
			"source_type":       "entity",
			"source_id":         "ent-1",
			"source_name":       "Alpha",
			"target_type":       "entity",
			"target_id":         "ent-2",
			"target_name":       "Beta",
			"relationship_type": "uses",
			"status":            "active",
			"properties":        map[string]any{},
			"created_at":        now,
		},
	}

	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": relationshipList}))
			return
		case r.URL.Path == "/api/entities" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{"id": "ent-2", "name": "Beta", "type": "entity", "tags": []string{}},
			}}))
			return
		case r.URL.Path == "/api/relationships" && r.Method == http.MethodPost:
			var body api.CreateRelationshipInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			gotCreateType = body.Type
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "rel-2"}}))
			return
		case strings.HasPrefix(r.URL.Path, "/api/relationships/") && r.Method == http.MethodPatch:
			var body api.UpdateRelationshipInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if body.Status != nil {
				gotPatchStatus = *body.Status
			}
			relID := strings.TrimPrefix(r.URL.Path, "/api/relationships/")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":                relID,
				"source_id":         "ent-1",
				"target_id":         "ent-2",
				"relationship_type": "uses",
				"status":            gotPatchStatus,
				"properties":        map[string]any{"note": "ok"},
				"created_at":        now,
			}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewEntitiesModel(client)
	model.width = 60
	model, _ = model.Update(entitiesLoadedMsg{items: []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "entity", Status: "active", Tags: []string{}},
	}})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, model.detail)

	// Load relationships view.
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, entitiesViewRelationships, model.view)
	require.Len(t, model.rels, 1)

	// Relate flow: open search, query, select first, type rel kind, submit.
	model, _ = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	assert.Equal(t, entitiesViewRelateSearch, model.view)

	for _, r := range "be" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)
	assert.Equal(t, entitiesViewRelateSelect, model.view)

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, entitiesViewRelateType, model.view)

	for _, r := range "knows" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	model, cmd = model.Update(msg)
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "knows", gotCreateType)

	// Archive relationship confirm renders and triggers update.
	model.view = entitiesViewRelationships
	model, _ = model.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	assert.Equal(t, entitiesViewConfirm, model.view)

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	msg = cmd()
	model, cmd = model.Update(msg)
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.Equal(t, "inactive", gotPatchStatus)

	// Relationship edit validation: invalid JSON blocks save.
	model.view = entitiesViewRelationships
	model, _ = model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	assert.Equal(t, entitiesViewRelEdit, model.view)
	model.relEditInput.SetValue("{")
	model, _ = model.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	assert.NotEmpty(t, model.errText)
}

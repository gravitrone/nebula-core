package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelationshipsStartEditNoDetailNoop(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.editFocus = relsEditFieldNotes
	model.editStatusIdx = 2
	model.editMeta.Buffer = "note: keep"

	model.startEdit()

	assert.Equal(t, relsEditFieldNotes, model.editFocus)
	assert.Equal(t, 2, model.editStatusIdx)
	assert.Equal(t, "note: keep", model.editMeta.Buffer)
}

func TestRelationshipsStartEditUnknownStatusFallsBackAndLoadsMetadata(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.detail = &api.Relationship{
		ID:         "rel-1",
		Status:     "mystery-status",
		Notes: "note: edge",
	}

	model.startEdit()

	assert.Equal(t, relsEditFieldStatus, model.editFocus)
	assert.Equal(t, 0, model.editStatusIdx)
	assert.Contains(t, model.editMeta.Buffer, "note: edge")
	assert.False(t, model.editSaving)
}

func TestRelationshipsSaveEditNoDetailReturnsNoop(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.editSaving = true

	updated, cmd := model.saveEdit()

	assert.Nil(t, cmd)
	assert.True(t, updated.editSaving)
}

func TestRelationshipsSaveEditOmitsNotesWhenBufferIsEmpty(t *testing.T) {
	now := time.Now().UTC()
	var patched map[string]any

	_, client := relTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && r.URL.Path == "/api/relationships/rel-1" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":                "rel-1",
					"source_type":       "entity",
					"source_id":         "ent-1",
					"target_type":       "entity",
					"target_id":         "ent-2",
					"relationship_type": "related-to",
					"status":            "active",
					"notes":             "",
					"created_at":        now,
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewRelationshipsModel(client)
	model.detail = &api.Relationship{
		ID:        "rel-1",
		Status:    "active",
		Notes:     "before",
		CreatedAt: now,
	}
	model.editStatusIdx = 1
	model.editMeta.Buffer = ""

	updated, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(relTabSavedMsg)
	require.True(t, ok, "expected relTabSavedMsg, got %T", msg)

	assert.True(t, updated.editSaving)
	assert.Equal(t, relsStatusOptions[1], patched["status"])
	notesVal, hasNotes := patched["notes"]
	// Empty buffer means notes is omitted or empty.
	if hasNotes {
		assert.Equal(t, "", notesVal)
	}
}

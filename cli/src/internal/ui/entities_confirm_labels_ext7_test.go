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

func TestEntitiesHandleConfirmKeysFallbackAndCancelBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.view = entitiesViewConfirm

	// entity-archive without detail should just close confirm.
	model.confirmKind = "entity-archive"
	model.confirmReturn = entitiesViewDetail
	next, cmd := model.handleConfirmKeys(tea.KeyPressMsg{Code: 'y', Text: "y"})
	assert.Nil(t, cmd)
	assert.Equal(t, entitiesViewDetail, next.view)
	assert.Equal(t, "", next.confirmKind)

	// entity-revert without audit id should just close confirm.
	next.view = entitiesViewConfirm
	next.confirmKind = "entity-revert"
	next.confirmReturn = entitiesViewHistory
	next.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	next.confirmAuditID = ""
	next, cmd = next.handleConfirmKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.Equal(t, entitiesViewHistory, next.view)
	assert.Equal(t, "", next.confirmKind)

	// rel-archive without relationship id should just close confirm.
	next.view = entitiesViewConfirm
	next.confirmKind = "rel-archive"
	next.confirmReturn = entitiesViewRelationships
	next.confirmRelID = ""
	next, cmd = next.handleConfirmKeys(tea.KeyPressMsg{Code: 'y', Text: "y"})
	assert.Nil(t, cmd)
	assert.Equal(t, entitiesViewRelationships, next.view)
	assert.Equal(t, "", next.confirmKind)

	// explicit cancel path.
	next.view = entitiesViewConfirm
	next.confirmKind = "entity-archive"
	next.confirmReturn = entitiesViewDetail
	next, cmd = next.handleConfirmKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	assert.Nil(t, cmd)
	assert.Equal(t, entitiesViewDetail, next.view)
	assert.Equal(t, "", next.confirmKind)
}

func TestEntitiesRenderConfirmAndRelationshipLabelBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 90
	model.confirmKind = "entity-revert"
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	model.confirmAuditID = "audit-1"
	model.confirmAudit = &api.AuditEntry{
		ID:        "audit-1",
		TableName: "entities",
		RecordID:  "ent-1",
		Action:    "update",
		OldValues: `{"status": "active"}`,
		NewValues: `{"status": "inactive"}`,
		ChangedAt: time.Now(),
	}

	rendered := model.renderConfirm()
	assert.Contains(t, rendered, "Audit ID")

	model.confirmKind = "rel-archive"
	model.confirmRelID = "rel-1"
	model.rels = []api.Relationship{
		{
			ID:         "rel-1",
			Type:       "uses",
			Status:     "",
			SourceName: "",
			SourceID:   "source-id-12345678",
			SourceType: "",
			TargetName: "Target",
			TargetID:   "target-id-12345678",
			TargetType: "entity",
		},
	}

	rendered = model.renderConfirm()
	assert.Contains(t, rendered, "inactive")

	require.NotNil(t, model.selectedRelationshipByID("rel-1"))
	assert.Nil(t, model.selectedRelationshipByID("missing"))
	assert.Nil(t, model.selectedRelationshipByID(""))

	// relationshipNodeLabel and firstNonEmpty fallback paths.
	lbl := model.relationshipNodeLabel("", "abc123456789", "")
	assert.NotEqual(t, "", strings.TrimSpace(lbl))
	assert.NotContains(t, lbl, "(")

	lbl = model.relationshipNodeLabel("Alpha", "abc123456789", "person")
	assert.Equal(t, "Alpha (person)", lbl)

	assert.Equal(t, "active", firstNonEmpty(" ", "", "active"))
	assert.Equal(t, "-", firstNonEmpty(" ", ""))
}

func TestEntitiesHandleConfirmKeysReturnsErrMsgOnMutationFailures(t *testing.T) {
	_, client := testEntitiesClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/entities/") {
			w.WriteHeader(http.StatusInternalServerError)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "INTERNAL_ERROR", "message": "entity update failed"},
			}))
			return
		}
		if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/entities/") && strings.HasSuffix(r.URL.Path, "/revert") {
			w.WriteHeader(http.StatusInternalServerError)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "INTERNAL_ERROR", "message": "revert failed"},
			}))
			return
		}
		if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/relationships/") {
			w.WriteHeader(http.StatusInternalServerError)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{"code": "INTERNAL_ERROR", "message": "relationship update failed"},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewEntitiesModel(client)
	model.view = entitiesViewConfirm
	model.confirmReturn = entitiesViewDetail

	// entity archive error closure
	model.confirmKind = "entity-archive"
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	next, cmd := model.handleConfirmKeys(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewDetail, next.view)
	msg := cmd()
	_, ok := msg.(errMsg)
	assert.True(t, ok)

	// entity revert error closure
	next.view = entitiesViewConfirm
	next.confirmReturn = entitiesViewHistory
	next.confirmKind = "entity-revert"
	next.detail = &api.Entity{ID: "ent-1", Name: "Alpha"}
	next.confirmAuditID = "audit-1"
	nextAfterRevert, cmd := next.handleConfirmKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewHistory, nextAfterRevert.view)
	msg = cmd()
	_, ok = msg.(errMsg)
	assert.True(t, ok)

	// relationship archive error closure
	nextAfterRevert.view = entitiesViewConfirm
	nextAfterRevert.confirmReturn = entitiesViewRelationships
	nextAfterRevert.confirmKind = "rel-archive"
	nextAfterRevert.confirmRelID = "rel-1"
	nextAfterRel, cmd := nextAfterRevert.handleConfirmKeys(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	assert.Equal(t, entitiesViewRelationships, nextAfterRel.view)
	msg = cmd()
	_, ok = msg.(errMsg)
	assert.True(t, ok)
}

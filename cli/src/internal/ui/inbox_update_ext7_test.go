package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInboxUpdateDiffInitConfirmNoopAndListEscClear(t *testing.T) {
	model := NewInboxModel(nil)
	model.detail = &api.Approval{ID: "ap-1"}

	updated, cmd := model.Update(approvalDiffLoadedMsg{
		id:      "ap-1",
		changes: map[string]any{"status": "approved"},
	})
	require.Nil(t, cmd)
	require.NotNil(t, updated.detail)
	require.NotNil(t, updated.detailChangeMap)
	assert.Equal(t, "approved", updated.detailChangeMap["changes"].(map[string]any)["status"])

	updated.confirming = true
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Nil(t, cmd)
	assert.True(t, updated.confirming)

	updated.confirming = false
	updated.detail = nil
	updated.items = []api.Approval{{ID: "ap-1"}}
	updated.filtered = []int{0}
	updated.dataTable.SetRows([]table.Row{{"ap-1"}})
	updated.dataTable.SetCursor(0)
	updated.selected = map[string]bool{"ap-1": true}
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Nil(t, cmd)
	assert.Empty(t, updated.selected)
}

func TestInboxApproveSelectedUsesDetailFallback(t *testing.T) {
	approved := ""
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/approvals/ap-detail/approve" {
			approved = "ap-detail"
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "ap-detail"}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewInboxModel(client)
	model.detail = &api.Approval{ID: "ap-detail"}

	updated, cmd := model.approveSelected()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(approvalDoneMsg)
	require.True(t, ok)
	assert.Nil(t, updated.detail)
	assert.Equal(t, "ap-detail", approved)
}

func TestInboxRenderDetailEntityNamesAndFormatAnyEmptySlice(t *testing.T) {
	assert.Equal(t, "None", formatAny([]any{}))

	model := NewInboxModel(nil)
	model.width = 100
	model.detail = &api.Approval{
		ID:          "ap-1",
		RequestType: "bulk_update_entity_scopes",
		Status:      "pending",
		AgentName:   "agent",
		RequestedBy: "entity-id",
		CreatedAt:   time.Now(),
		ChangeDetails: `{"entity_ids":["ent-1","ent-2"],"entity_names":["Alpha","Beta"]}`,
	}

	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "Entity Ids")
	assert.Contains(t, out, "Alpha, Beta")
}

func TestInboxPreviewScopeFallbackAndStartRejectNoSelection(t *testing.T) {
	approval := api.Approval{
		RequestType: "update_entity",
		Status:      "pending",
		AgentName:   "agent",
		CreatedAt:   time.Now(),
		ChangeDetails: `{"scope":"public"}`,
	}
	preview := components.SanitizeText(renderApprovalPreview(approval, false, 44))
	assert.Contains(t, preview, "Scope")
	assert.Contains(t, preview, "public")

	model := NewInboxModel(nil)
	updated, cmd := model.startReject()
	assert.Nil(t, cmd)
	assert.False(t, updated.rejecting)
	assert.Nil(t, updated.detail)
}

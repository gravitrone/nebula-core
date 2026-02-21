package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetApproval handles test get approval.
func TestGetApproval(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/approvals/")
		_, err := w.Write(jsonResponse(map[string]any{
			"id":          "ap-1",
			"agent_id":    "ag-1",
			"action_type": "create_entity",
			"status":      "pending",
			"details":     map[string]any{},
		}))
		require.NoError(t, err)
	})

	approval, err := client.GetApproval("ap-1")
	require.NoError(t, err)
	assert.Equal(t, "ap-1", approval.ID)
	assert.Equal(t, "pending", approval.Status)
}

// TestRejectRequest handles test reject request.
func TestRejectRequest(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/reject")

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "not authorized", body["review_notes"])

		_, err := w.Write(jsonResponse(map[string]any{
			"id":          "ap-1",
			"status":      "rejected",
			"agent_id":    "ag-1",
			"action_type": "create_entity",
			"details":     map[string]any{},
		}))
		require.NoError(t, err)
	})

	approval, err := client.RejectRequest("ap-1", "not authorized")
	require.NoError(t, err)
	assert.Equal(t, "rejected", approval.Status)
}

// TestRejectRequestEmptyNotes handles test reject request empty notes.
func TestRejectRequestEmptyNotes(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "", body["review_notes"])

		_, err := w.Write(jsonResponse(map[string]any{
			"id":          "ap-1",
			"status":      "rejected",
			"agent_id":    "ag-1",
			"action_type": "create_entity",
			"details":     map[string]any{},
		}))
		require.NoError(t, err)
	})

	approval, err := client.RejectRequest("ap-1", "")
	require.NoError(t, err)
	assert.Equal(t, "rejected", approval.Status)
}

// TestApproveAlreadyApproved handles test approve already approved.
func TestApproveAlreadyApproved(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "ALREADY_PROCESSED",
				"message": "approval already processed",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.ApproveRequest("ap-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ALREADY_PROCESSED")
}

// TestGetPendingApprovalsEmpty handles test get pending approvals empty.
func TestGetPendingApprovalsEmpty(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(jsonResponse([]map[string]any{}))
		require.NoError(t, err)
	})

	approvals, err := client.GetPendingApprovals()
	require.NoError(t, err)
	assert.Len(t, approvals, 0)
}

// TestGetApprovalDiff handles test get approval diff.
func TestGetApprovalDiff(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/approvals/ap-1/diff")
		_, err := w.Write(jsonResponse(map[string]any{
			"approval_id":  "ap-1",
			"request_type": "update_entity",
			"changes": map[string]any{
				"status": map[string]any{"from": "active", "to": "archived"},
			},
		}))
		require.NoError(t, err)
	})

	diff, err := client.GetApprovalDiff("ap-1")
	require.NoError(t, err)
	assert.Equal(t, "ap-1", diff.ApprovalID)
	assert.Equal(t, "update_entity", diff.RequestType)
	if assert.Contains(t, diff.Changes, "status") {
		changes, ok := diff.Changes["status"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "active", changes["from"])
		assert.Equal(t, "archived", changes["to"])
	}
}

// TestApproveRequestWithInput handles test approve request with input.
func TestApproveRequestWithInput(t *testing.T) {
	var body map[string]any
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/approve")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, err := w.Write(jsonResponse(map[string]any{
			"id":           "ap-1",
			"status":       "approved",
			"request_type": "register_agent",
		}))
		require.NoError(t, err)
	})

	trust := false
	input := &ApproveRequestInput{
		GrantScopes:           []string{"public", "private"},
		GrantRequiresApproval: &trust,
	}
	approval, err := client.ApproveRequestWithInput("ap-1", input)
	require.NoError(t, err)
	assert.Equal(t, "approved", approval.Status)
	assert.Equal(t, []any{"public", "private"}, body["grant_scopes"])
	assert.Equal(t, false, body["grant_requires_approval"])
}

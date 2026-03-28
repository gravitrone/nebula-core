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

func TestInboxLoadApprovalsLimitFallbackAndError(t *testing.T) {
	var limits []string
	_, okClient := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		limits = append(limits, r.URL.Query().Get("limit"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "agent", "requested_by": "entity-id", "change_details": "{}", "created_at": time.Now()},
			},
		}))
	})

	model := NewInboxModel(okClient)
	model.pendingLimit = -10
	msg := model.loadApprovals()
	loaded, ok := msg.(approvalsLoadedMsg)
	require.True(t, ok)
	require.Len(t, loaded.items, 1)
	assert.Equal(t, []string{"500"}, limits)

	model.SetPendingLimit(9999)
	assert.Equal(t, 5000, model.pendingLimit)
	msg = model.loadApprovals()
	loaded, ok = msg.(approvalsLoadedMsg)
	require.True(t, ok)
	require.Len(t, loaded.items, 1)
	assert.Equal(t, []string{"500", "5000"}, limits)

	_, errClient := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":"FAILED","message":"pending broke"}}`, http.StatusInternalServerError)
	})
	errModel := NewInboxModel(errClient)
	errMsgOut, ok := errModel.loadApprovals().(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errMsgOut.err, "FAILED")
}

func TestInboxLoadApprovalDiffSuccessAndError(t *testing.T) {
	_, okClient := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/approvals/ap-1/diff" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"approval_id":  "ap-1",
				"request_type": "update_entity",
				"changes": map[string]any{
					"status": map[string]any{"from": "pending", "to": "approved"},
				},
			},
		}))
	})

	model := NewInboxModel(okClient)
	cmd := model.loadApprovalDiff("ap-1")
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(approvalDiffLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "ap-1", loaded.id)
	require.NotNil(t, loaded.changes["status"])

	_, errClient := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":"DIFF_FAILED","message":"diff failed"}}`, http.StatusInternalServerError)
	})
	errModel := NewInboxModel(errClient)
	cmd = errModel.loadApprovalDiff("ap-1")
	require.NotNil(t, cmd)
	errOut, ok := cmd().(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "DIFF_FAILED")
}

func TestInboxHandleRejectPreviewBranches(t *testing.T) {
	var rejected []string
	var notesSeen []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/approvals/ap-err/reject" {
			http.Error(w, `{"error":{"code":"FAILED","message":"reject failed"}}`, http.StatusInternalServerError)
			return
		}
		if r.URL.Path == "/api/approvals/ap-1/reject" || r.URL.Path == "/api/approvals/ap-2/reject" {
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			rejected = append(rejected, r.URL.Path)
			notesSeen = append(notesSeen, body["review_notes"])
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": "ok"},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewInboxModel(client)
	model.rejectPreview = true
	model.bulkRejectIDs = []string{"ap-1", "ap-2"}
	model.rejectBuf = "nope"

	updated, cmd := model.handleRejectPreview(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.True(t, updated.rejectPreview)
	assert.Nil(t, cmd)

	updated, cmd = updated.handleRejectPreview(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.rejectPreview)
	assert.True(t, updated.rejecting)
	assert.Nil(t, cmd)

	updated.rejecting = false
	updated.rejectPreview = true
	updated.bulkRejectIDs = []string{"ap-1", "ap-2"}
	updated.rejectBuf = "nope"

	updated, cmd = updated.handleRejectPreview(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(approvalDoneMsg)
	require.True(t, ok)
	assert.False(t, updated.rejectPreview)
	assert.Equal(t, "", updated.rejectBuf)
	assert.Nil(t, updated.detail)
	assert.Nil(t, updated.bulkRejectIDs)
	assert.Equal(t, []string{"/api/approvals/ap-1/reject", "/api/approvals/ap-2/reject"}, rejected)
	assert.Equal(t, []string{"nope", "nope"}, notesSeen)

	errModel := NewInboxModel(client)
	errModel.rejectPreview = true
	errModel.bulkRejectIDs = []string{"ap-err"}
	errModel.rejectBuf = "bad"
	_, cmd = errModel.handleRejectPreview(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	errOut, ok := cmd().(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "FAILED")
}

func TestInboxApproveSummaryRowsFallbackMatrix(t *testing.T) {
	base := NewInboxModel(nil)
	base.items = []api.Approval{{ID: "ap-1"}, {ID: "ap-2"}}
	base.filtered = []int{1}
	base.dataTable.SetRows([]table.Row{{"ap-2"}})
	base.dataTable.SetCursor(0)

	model := base
	model.selected = map[string]bool{"ap-1": true, "ap-2": true}
	rows := model.approveSummaryRows()
	require.Len(t, rows, 2)
	assert.Equal(t, "2", rows[1].Value)

	model = base
	model.selected = map[string]bool{"ap-1": true}
	rows = model.approveSummaryRows()
	require.Len(t, rows, 3)
	assert.Equal(t, "ap-1", rows[2].Value)

	model = base
	model.detail = &api.Approval{ID: "ap-detail"}
	rows = model.approveSummaryRows()
	require.Len(t, rows, 3)
	assert.Equal(t, "ap-detail", rows[2].Value)

	model = base
	rows = model.approveSummaryRows()
	require.Len(t, rows, 3)
	assert.Equal(t, "ap-2", rows[2].Value)
}

func TestInboxHandleFilterInputMatrix(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{
		{ID: "ap-1", RequestType: "create_entity", AgentName: "alpha", Status: "pending", CreatedAt: time.Now()},
		{ID: "ap-2", RequestType: "update_entity", AgentName: "beta", Status: "pending", CreatedAt: time.Now()},
	}
	model.applyFilter(true)
	model.filtering = true

	updated, cmd := model.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Nil(t, cmd)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.filtered, 2)

	updated, cmd = updated.handleFilterInput(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.Nil(t, cmd)
	assert.Equal(t, "a", updated.filterBuf)

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.filterBuf)

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, "", updated.filterBuf)

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.Equal(t, "b", updated.filterBuf)
	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, updated.filtering)
	assert.Equal(t, "b", updated.filterBuf)

	updated.filtering = true
	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.filterBuf)
	assert.Len(t, updated.filtered, 2)
}

func TestInboxRenderDetailAdditionalBranches(t *testing.T) {
	sourceID := "11111111-1111-1111-1111-111111111111"
	targetID := "22222222-2222-2222-2222-222222222222"
	entityOne := "33333333-3333-3333-3333-333333333333"
	entityTwo := "44444444-4444-4444-4444-444444444444"
	jobID := "job-2"
	notes := "looks good"

	model := NewInboxModel(nil)
	model.width = 110
	model.detail = &api.Approval{
		ID:          "ap-1",
		RequestType: "update_relationship",
		Status:      "pending",
		AgentName:   "agent",
		RequestedBy: "entity-id",
		CreatedAt:   time.Now(),
		JobID:       &jobID,
		Notes: &notes,
		ChangeDetails: `{"source_id":"` + sourceID + `","source_name":"Source A","target_id":"` + targetID + `","target_name":"Target B","entity_ids":["` + entityOne + `","` + entityTwo + `"],"Settings":{"Mode":"strict"},"MeTaDaTa":{"role":"builder"},"changes":{"status":{"from":"pending","to":"approved"},"same":{"from":"x","to":"x"}}}`,
	}

	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "Source A")
	assert.Contains(t, out, "Target B")
	assert.Contains(t, out, shortID(entityOne))
	assert.Contains(t, out, shortID(entityTwo))
	assert.Contains(t, out, "strict")
	assert.Contains(t, out, "builder")
	assert.Contains(t, out, "approved")
}

func TestInboxRenderApprovalPreviewBranches(t *testing.T) {
	assert.Equal(t, "", renderApprovalPreview(api.Approval{}, false, 0))

	sourceID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	targetID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	entityID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	a := api.Approval{
		ID:              "ap-1",
		RequestType:     "create_relationship",
		Status:          "pending",
		AgentName:       "agent-alpha",
		RequestedByName: "agent-alpha",
		CreatedAt:       time.Now(),
		ChangeDetails: `{"name":"Rel One","scopes":["public","admin"],"tags":["x","y"],"type":"relation","relationship_type":"owns","source_id":"` + sourceID + `","target_id":"` + targetID + `","entity_ids":["` + entityID + `"],"log_type":"event"}`,
	}

	out := components.SanitizeText(renderApprovalPreview(a, true, 52))
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "In batch")
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "Rel")
	assert.Contains(t, out, "From")
	assert.Contains(t, out, shortID(sourceID))
	assert.Contains(t, out, "To")
	assert.Contains(t, out, shortID(targetID))
	assert.Contains(t, out, "Entities")
	assert.Contains(t, out, shortID(entityID))
	assert.Contains(t, out, "Log")
}

func TestInboxViewAdditionalBranches(t *testing.T) {
	now := time.Now()
	model := NewInboxModel(nil)
	model.width = 120
	model.items = []api.Approval{
		{
			ID:              "ap-1",
			Status:          "pending",
			RequestType:     "create_entity",
			AgentName:       "agent",
			RequestedBy:     "entity-id",
			RequestedByName: "agent",
			CreatedAt:       now,
			ChangeDetails: `{"name":"Alpha","scopes":["public"]}`,
		},
	}
	model.applyFilter(true)

	model.filtering = true
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Filter Approvals")

	model.filtering = false
	model.rejecting = true
	model.detail = &api.Approval{ID: "ap-1"}
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Reject: Enter Review Notes")

	model.rejecting = false
	model.detail = nil
	model.selected = map[string]bool{"ap-1": true}
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "selected: 1")
	assert.Contains(t, out, "In batch")

	model.filterBuf = "agent:nope"
	model.applyFilter(true)
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "No approvals match the filter")
}

func TestInboxFilterTimeAndMatchExtraBranches(t *testing.T) {
	yesterday := parseFilterTime("yesterday")
	require.NotNil(t, yesterday)
	assert.Equal(t, 0, yesterday.Hour())

	isoDay := parseFilterTime("2026-03-01")
	require.NotNil(t, isoDay)
	assert.Equal(t, 2026, isoDay.Year())

	filter := approvalFilter{
		agent: "alpha",
		req:   "create",
		since: func() *time.Time {
			tm := time.Now().Add(-2 * time.Hour)
			return &tm
		}(),
		terms: []string{"pending"},
	}
	match := api.Approval{
		AgentName:   "alpha-agent",
		RequestType: "create_entity",
		Status:      "pending",
		CreatedAt:   time.Now().Add(-1 * time.Hour),
	}
	assert.True(t, matchesApprovalFilter(match, filter))

	old := match
	old.CreatedAt = time.Now().Add(-24 * time.Hour)
	assert.False(t, matchesApprovalFilter(old, filter))

	agentMismatch := match
	agentMismatch.AgentName = "beta-agent"
	assert.False(t, matchesApprovalFilter(agentMismatch, filter))

	reqMismatch := match
	reqMismatch.RequestType = "update_job_status"
	assert.False(t, matchesApprovalFilter(reqMismatch, filter))

	termMismatch := match
	termMismatch.Status = "approved"
	assert.False(t, matchesApprovalFilter(termMismatch, filter))
}

func TestInboxHandleRejectInputDetailNilAndEnterBranches(t *testing.T) {
	model := NewInboxModel(nil)
	model.rejecting = true
	model.rejectBuf = "stale"
	model.bulkRejectIDs = []string{"ap-1"}

	updated, cmd := model.handleRejectInput(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Nil(t, cmd)
	assert.False(t, updated.rejecting)
	assert.Equal(t, "", updated.rejectBuf)
	assert.Nil(t, updated.bulkRejectIDs)

	model = NewInboxModel(nil)
	model.detail = &api.Approval{ID: "ap-2"}
	model.rejecting = true
	model.rejectBuf = "reason"

	updated, cmd = model.handleRejectInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.False(t, updated.rejecting)
	assert.True(t, updated.rejectPreview)
	assert.Equal(t, []string{"ap-2"}, updated.bulkRejectIDs)

	updated.rejecting = true
	updated.rejectPreview = false
	updated.bulkRejectIDs = []string{"ap-2", "ap-3"}
	updated.detail = &api.Approval{ID: "ap-2"}
	updated, _ = updated.handleRejectInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Nil(t, updated.detail)
	assert.False(t, updated.rejecting)
	assert.Equal(t, "", updated.rejectBuf)
	assert.Nil(t, updated.bulkRejectIDs)
}

func TestInboxSelectAllFilteredSkipsOutOfRangeEntries(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{{ID: "ap-1"}, {ID: "ap-2"}}
	model.filtered = []int{-1, 0, 4, 1}
	model.selectAllFiltered()
	assert.Equal(t, map[string]bool{"ap-1": true, "ap-2": true}, model.selected)
}

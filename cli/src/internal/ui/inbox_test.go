package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClient handles test client.
func testClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestInboxModelInit handles test inbox model init.
func TestInboxModelInit(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"data": []map[string]any{}}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)
	cmd := model.Init()
	assert.NotNil(t, cmd)
}

// TestInboxModelLoadsApprovals handles test inbox model loads approvals.
func TestInboxModelLoadsApprovals(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				{"id": "ap-2", "status": "pending", "request_type": "update_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)

	// Init and load
	model, _ = model.Update(runCmdFirst(model.Init()))

	assert.Len(t, model.items, 2)
	assert.Equal(t, "ap-1", model.items[0].ID)
}

// TestInboxModelNavigationKeys handles test inbox model navigation keys.
func TestInboxModelNavigationKeys(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				{"id": "ap-2", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)

	// Load items
	model, _ = model.Update(runCmdFirst(model.Init()))

	// Initially at index 0
	assert.Equal(t, 0, model.dataTable.Cursor())

	// Press down (down arrow)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, model.dataTable.Cursor())

	// Press up (up arrow)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, model.dataTable.Cursor())
}

// TestInboxModelEnterShowsDetail handles test inbox model enter shows detail.
func TestInboxModelEnterShowsDetail(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)

	// Load items
	model, _ = model.Update(runCmdFirst(model.Init()))

	// Press enter to view detail
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	// Should show detail view
	assert.NotNil(t, model.detail)
	assert.Equal(t, "ap-1", model.detail.ID)
}

// TestInboxDetailLoadsDiff handles test inbox detail loads diff.
func TestInboxDetailLoadsDiff(t *testing.T) {
	diffCalled := false
	var paths []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch {
		case r.URL.Path == "/api/approvals/pending":
			resp := map[string]any{
				"data": []map[string]any{
					{"id": "ap-1", "status": "pending", "request_type": "update_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				},
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		case strings.Contains(r.URL.Path, "/diff"):
			diffCalled = true
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"approval_id":  "ap-1",
					"request_type": "update_entity",
					"changes": map[string]any{
						"status": map[string]any{"from": "active", "to": "archived"},
					},
				},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	model, _ = model.Update(runCmdFirst(model.Init()))

	var detailCmd tea.Cmd
	model, detailCmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, detailCmd)
	msg := detailCmd()
	model, _ = model.Update(msg)

	if !diffCalled {
		t.Fatalf("diff endpoint not called, paths=%v", paths)
	}
	require.NotNil(t, model.detail)
	changes, ok := model.detail.ChangeDetails["changes"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, changes, "status")
}

// TestInboxModelEscapeBackFromDetail handles test inbox model escape back from detail.
func TestInboxModelEscapeBackFromDetail(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)

	// Load and enter detail
	model, _ = model.Update(runCmdFirst(model.Init()))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.NotNil(t, model.detail)

	// Press escape to go back
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	// Should exit detail view
	assert.Nil(t, model.detail)
}

// TestInboxModelRenderEmpty handles test inbox model render empty.
func TestInboxModelRenderEmpty(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"data": []map[string]any{}}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)

	// Load empty list
	model, _ = model.Update(runCmdFirst(model.Init()))

	view := model.View()
	assert.Contains(t, view, "No pending approvals")
}

// TestInboxBulkApproveRequiresConfirm handles test inbox bulk approve requires confirm.
func TestInboxBulkApproveRequiresConfirm(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				{"id": "ap-2", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)
	model.confirmBulk = true

	model, _ = model.Update(runCmdFirst(model.Init()))

	model, _ = model.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})

	assert.True(t, model.confirming)
	view := model.View()
	_ = view
}

// TestInboxModelRenderLoading handles test inbox model render loading.
func TestInboxModelRenderLoading(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		// should not be called
	})

	model := NewInboxModel(client)
	model.loading = true

	view := model.View()
	assert.Contains(t, view, "Loading")
}

// TestInboxModelRejectInputHandling handles test inbox model reject input handling.
func TestInboxModelRejectInputHandling(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		err := json.NewEncoder(w).Encode(resp)
		require.NoError(t, err)
	})

	model := NewInboxModel(client)

	// Load items
	model, _ = model.Update(runCmdFirst(model.Init()))

	// Press 'r' to start rejecting
	model, _ = model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	// Should enter reject mode
	assert.True(t, model.rejecting)
	assert.NotNil(t, model.detail)

	// Type some text
	model, _ = model.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	assert.Equal(t, "test", model.rejectBuf)

	// Backspace once
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "tes", model.rejectBuf)

	// Escape to cancel
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, model.rejecting)
	assert.Equal(t, "", model.rejectBuf)
}

// TestInboxToggleSelectAll handles test inbox toggle select all.
func TestInboxToggleSelectAll(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending"},
		{ID: "ap-2", Status: "pending"},
	}
	model.applyFilter(true)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.Equal(t, 2, model.selectedCount())

	model, _ = model.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.Equal(t, 0, model.selectedCount())
}

// TestInboxApproveAllFiltered handles test inbox approve all filtered.
func TestInboxApproveAllFiltered(t *testing.T) {
	var approved []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/approvals/") && strings.HasSuffix(r.URL.Path, "/approve") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/approvals/"), "/approve")
			approved = append(approved, id)
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id}})
			require.NoError(t, err)
			return
		}
		if r.URL.Path == "/api/approvals/pending" {
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewInboxModel(client)
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending"},
		{ID: "ap-2", Status: "pending"},
	}
	model.applyFilter(true)

	var cmd tea.Cmd
	model, cmd = model.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	cmd()

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, approved)
}

// TestInboxApproveAllConfirmAcceptsEnter handles test inbox approve all confirm accepts enter.
func TestInboxApproveAllConfirmAcceptsEnter(t *testing.T) {
	var approved []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/approvals/") && strings.HasSuffix(r.URL.Path, "/approve"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/approvals/"), "/approve")
			approved = append(approved, id)
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id}})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/approvals/pending":
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewInboxModel(client)
	model.confirmBulk = true
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending"},
		{ID: "ap-2", Status: "pending"},
	}
	model.applyFilter(true)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	require.True(t, model.confirming)

	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, approved)
}

// TestInboxApproveAllConfirmCancelsOnEsc handles test inbox approve all confirm cancels on esc.
func TestInboxApproveAllConfirmCancelsOnEsc(t *testing.T) {
	model := NewInboxModel(nil)
	model.confirmBulk = true
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending"},
		{ID: "ap-2", Status: "pending"},
	}
	model.applyFilter(true)

	model, _ = model.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	require.True(t, model.confirming)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.False(t, updated.confirming)
	assert.Nil(t, cmd)
}

// TestInboxFilterByAgentAndType handles test inbox filter by agent and type.
func TestInboxFilterByAgentAndType(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending", RequestType: "create_entity", AgentName: "alpha", CreatedAt: time.Now()},
		{ID: "ap-2", Status: "pending", RequestType: "update_entity", AgentName: "beta", CreatedAt: time.Now()},
	}
	model.filterBuf = "agent:alpha type:create"
	model.applyFilter(true)

	assert.Len(t, model.dataTable.Rows(), 1)
	assert.Equal(t, "ap-1", model.items[model.filtered[0]].ID)
}

// TestInboxFilterByTerm handles test inbox filter by term.
func TestInboxFilterByTerm(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending", RequestType: "create_entity", AgentName: "alpha", ChangeDetails: api.JSONMap{"name": "Foo"}, CreatedAt: time.Now()},
		{ID: "ap-2", Status: "pending", RequestType: "create_entity", AgentName: "beta", ChangeDetails: api.JSONMap{"name": "Bar"}, CreatedAt: time.Now()},
	}
	model.filterBuf = "foo"
	model.applyFilter(true)

	assert.Len(t, model.dataTable.Rows(), 1)
	assert.Equal(t, "ap-1", model.items[model.filtered[0]].ID)
}

// TestInboxBatchApproveSelected handles test inbox batch approve selected.
func TestInboxBatchApproveSelected(t *testing.T) {
	var approved []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/approvals/pending":
			resp := map[string]any{
				"data": []map[string]any{
					{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
					{"id": "ap-2", "status": "pending", "request_type": "update_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				},
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		case strings.HasSuffix(r.URL.Path, "/approve"):
			parts := strings.Split(r.URL.Path, "/")
			approved = append(approved, parts[len(parts)-2])
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": parts[len(parts)-2]}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	model, _ = model.Update(runCmdFirst(model.Init()))

	model, _ = model.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = model.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})

	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	msg := cmd()
	_, _ = model.Update(msg)

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, approved)
}

// TestInboxApproveAllMixedRequestsStaysDeterministic handles test inbox approve all mixed requests stays deterministic.
func TestInboxApproveAllMixedRequestsStaysDeterministic(t *testing.T) {
	var approved []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/approvals/pending":
			resp := map[string]any{
				"data": []map[string]any{
					{
						"id":           "ap-bulk",
						"status":       "pending",
						"request_type": "bulk_update_entity_scopes",
						"agent_name":   "test",
						"requested_by": "user",
						"change_details": map[string]any{
							"entity_ids": []string{"ent-1"},
							"scopes":     []string{"admin"},
							"op":         "add",
						},
						"created_at": time.Now(),
					},
					{
						"id":           "ap-update",
						"status":       "pending",
						"request_type": "update_entity",
						"agent_name":   "test",
						"requested_by": "user",
						"change_details": map[string]any{
							"entity_id": "ent-1",
							"status":    "active",
						},
						"created_at": time.Now(),
					},
				},
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		case strings.HasSuffix(r.URL.Path, "/approve"):
			parts := strings.Split(r.URL.Path, "/")
			approved = append(approved, parts[len(parts)-2])
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": parts[len(parts)-2]}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	model, _ = model.Update(runCmdFirst(model.Init()))

	// Multi-select two mixed request types.
	model, _ = model.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = model.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	model, cmd := model.Update(tea.KeyPressMsg{Code: 'A', Text: "A"})
	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	msg := cmd()
	_, _ = model.Update(msg)

	assert.ElementsMatch(t, []string{"ap-bulk", "ap-update"}, approved)
}

// TestInboxApproveRegisterAgentOpensGrantEditor handles test inbox approve register agent opens grant editor.
func TestInboxApproveRegisterAgentOpensGrantEditor(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{
					"id":           "ap-register",
					"status":       "pending",
					"request_type": "register_agent",
					"agent_name":   "bootstrap-agent",
					"requested_by": "agent-id",
					"change_details": map[string]any{
						"requested_scopes":            []string{"public", "private"},
						"requested_requires_approval": false,
					},
					"created_at": time.Now(),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)
	model, _ = model.Update(runCmdFirst(model.Init()))

	model, _ = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.True(t, model.grantEditing)
	assert.Equal(t, "ap-register", model.grantApproval)
	assert.Equal(t, "public,private", model.grantScopes)
	assert.False(t, model.grantTrusted)
}

// TestInboxApproveRegisterAgentSendsGrantPayload handles test inbox approve register agent sends grant payload.
func TestInboxApproveRegisterAgentSendsGrantPayload(t *testing.T) {
	approvedBody := map[string]any{}
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/approvals/pending":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":           "ap-register",
						"status":       "pending",
						"request_type": "register_agent",
						"agent_name":   "bootstrap-agent",
						"requested_by": "agent-id",
						"change_details": map[string]any{
							"requested_scopes":            []string{"public"},
							"requested_requires_approval": true,
						},
						"created_at": time.Now(),
					},
				},
			})
		case strings.HasSuffix(r.URL.Path, "/approve"):
			require.NoError(t, json.NewDecoder(r.Body).Decode(&approvedBody))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":     "ap-register",
					"status": "approved",
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	model, _ = model.Update(runCmdFirst(model.Init()))

	model, _ = model.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.True(t, model.grantEditing)
	model, _ = model.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.Equal(t, []any{"public"}, approvedBody["grant_scopes"])
	assert.Equal(t, false, approvedBody["grant_requires_approval"])
}

// TestInboxBatchRejectSelected handles test inbox batch reject selected.
func TestInboxBatchRejectSelected(t *testing.T) {
	var rejected []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/approvals/pending":
			resp := map[string]any{
				"data": []map[string]any{
					{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
					{"id": "ap-2", "status": "pending", "request_type": "update_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				},
			}
			err := json.NewEncoder(w).Encode(resp)
			require.NoError(t, err)
		case strings.HasSuffix(r.URL.Path, "/reject"):
			parts := strings.Split(r.URL.Path, "/")
			rejected = append(rejected, parts[len(parts)-2])
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": parts[len(parts)-2]}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	model, _ = model.Update(runCmdFirst(model.Init()))

	model, _ = model.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	model, _ = model.Update(tea.KeyPressMsg{Code: ' ', Text: " "})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	model, _ = model.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	model, _ = model.Update(tea.KeyPressMsg{Code: 'o', Text: "o"})
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	require.NotNil(t, cmd)
	msg := cmd()
	_, _ = model.Update(msg)

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, rejected)
}

// TestParseFilterTime handles test parse filter time.
func TestParseFilterTime(t *testing.T) {
	if parseFilterTime("nonsense") != nil {
		t.Fatal("expected nil for invalid time")
	}

	if t1 := parseFilterTime("5h"); t1 == nil {
		t.Fatal("expected time for 5h")
	} else {
		since := time.Since(*t1)
		if since < 4*time.Hour || since > 6*time.Hour {
			t.Fatalf("expected ~5h ago, got %v", since)
		}
	}

	if t2 := parseFilterTime("2d"); t2 == nil {
		t.Fatal("expected time for 2d")
	} else {
		since := time.Since(*t2)
		if since < 47*time.Hour || since > 49*time.Hour {
			t.Fatalf("expected ~48h ago, got %v", since)
		}
	}

	if t3 := parseFilterTime("today"); t3 == nil {
		t.Fatal("expected time for today")
	} else {
		if t3.Hour() != 0 || t3.Minute() != 0 {
			t.Fatalf("expected midnight for today, got %s", t3.Format(time.RFC3339))
		}
	}
}

// TestParseApprovalFilterTokens handles test parse approval filter tokens.
func TestParseApprovalFilterTokens(t *testing.T) {
	filter := parseApprovalFilter("agent:Alpha type:Create since:1d custom")
	assert.Equal(t, "alpha", filter.agent)
	assert.Equal(t, "create", filter.req)
	assert.NotNil(t, filter.since)
	assert.Equal(t, []string{"custom"}, filter.terms)
}

// TestInboxRenderGrantEditorShowsCurrentInputs handles test inbox render grant editor shows current inputs.
func TestInboxRenderGrantEditorShowsCurrentInputs(t *testing.T) {
	model := NewInboxModel(nil)
	model.width = 90
	model.grantScopes = "public,private"
	model.grantTrusted = false

	out := stripANSI(model.renderGrantEditor())
	assert.Contains(t, out, "public,private")
	assert.Contains(t, out, "requires_approval: false")
}

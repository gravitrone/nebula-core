package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

func TestInboxModelInit(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"data": []map[string]any{}}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)
	cmd := model.Init()
	assert.NotNil(t, cmd)
}

func TestInboxModelLoadsApprovals(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				{"id": "ap-2", "status": "pending", "request_type": "update_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)

	// Init and load
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	assert.Len(t, model.items, 2)
	assert.Equal(t, "ap-1", model.items[0].ID)
}

func TestInboxModelNavigationKeys(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				{"id": "ap-2", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)

	// Load items
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	// Initially at index 0
	assert.Equal(t, 0, model.list.Selected())

	// Press down (down arrow)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, model.list.Selected())

	// Press up (up arrow)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, model.list.Selected())
}

func TestInboxModelEnterShowsDetail(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)

	// Load items
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	// Press enter to view detail
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should show detail view
	assert.NotNil(t, model.detail)
	assert.Equal(t, "ap-1", model.detail.ID)
}

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
			json.NewEncoder(w).Encode(resp)
		case strings.Contains(r.URL.Path, "/diff"):
			diffCalled = true
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"approval_id":  "ap-1",
					"request_type": "update_entity",
					"changes": map[string]any{
						"status": map[string]any{"from": "active", "to": "archived"},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	var detailCmd tea.Cmd
	model, detailCmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, detailCmd)
	msg = detailCmd()
	model, _ = model.Update(msg)

	if !diffCalled {
		t.Fatalf("diff endpoint not called, paths=%v", paths)
	}
	require.NotNil(t, model.detail)
	changes, ok := model.detail.ChangeDetails["changes"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, changes, "status")
}

func TestInboxModelEscapeBackFromDetail(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)

	// Load and enter detail
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, model.detail)

	// Press escape to go back
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	// Should exit detail view
	assert.Nil(t, model.detail)
}

func TestInboxModelRenderEmpty(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"data": []map[string]any{}}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)

	// Load empty list
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	view := model.View()
	assert.Contains(t, view, "No pending approvals")
}

func TestInboxBulkApproveRequiresConfirm(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
				{"id": "ap-2", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)
	model.confirmBulk = true

	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})

	assert.True(t, model.confirming)
	view := model.View()
	assert.Contains(t, view, "Approve")
}

func TestInboxModelRenderLoading(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		// should not be called
	})

	model := NewInboxModel(client)
	model.loading = true

	view := model.View()
	assert.Contains(t, view, "Loading")
}

func TestInboxModelRejectInputHandling(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": []map[string]any{
				{"id": "ap-1", "status": "pending", "request_type": "create_entity", "agent_name": "test", "requested_by": "user", "change_details": map[string]any{}, "created_at": time.Now()},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)

	// Load items
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	// Press 'r' to start rejecting
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	// Should enter reject mode
	assert.True(t, model.rejecting)
	assert.NotNil(t, model.detail)

	// Type some text
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	assert.Equal(t, "test", model.rejectBuf)

	// Backspace once
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "tes", model.rejectBuf)

	// Escape to cancel
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, model.rejecting)
	assert.Equal(t, "", model.rejectBuf)
}

func TestInboxToggleSelectAll(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending"},
		{ID: "ap-2", Status: "pending"},
	}
	model.applyFilter(true)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	assert.Equal(t, 2, model.selectedCount())

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	assert.Equal(t, 0, model.selectedCount())
}

func TestInboxApproveAllFiltered(t *testing.T) {
	var approved []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/approvals/") && strings.HasSuffix(r.URL.Path, "/approve") {
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/approvals/"), "/approve")
			approved = append(approved, id)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id}})
			return
		}
		if r.URL.Path == "/api/approvals/pending" {
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
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
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.NotNil(t, cmd)
	cmd()

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, approved)
}

func TestInboxApproveAllConfirmAcceptsEnter(t *testing.T) {
	var approved []string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/approvals/") && strings.HasSuffix(r.URL.Path, "/approve"):
			id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/approvals/"), "/approve")
			approved = append(approved, id)
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id}})
			return
		case r.URL.Path == "/api/approvals/pending":
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
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

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	require.True(t, model.confirming)

	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	cmd()

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, approved)
}

func TestInboxApproveAllConfirmCancelsOnEsc(t *testing.T) {
	model := NewInboxModel(nil)
	model.confirmBulk = true
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending"},
		{ID: "ap-2", Status: "pending"},
	}
	model.applyFilter(true)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	require.True(t, model.confirming)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.False(t, updated.confirming)
	assert.Nil(t, cmd)
}

func TestInboxFilterByAgentAndType(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending", RequestType: "create_entity", AgentName: "alpha", CreatedAt: time.Now()},
		{ID: "ap-2", Status: "pending", RequestType: "update_entity", AgentName: "beta", CreatedAt: time.Now()},
	}
	model.filterBuf = "agent:alpha type:create"
	model.applyFilter(true)

	assert.Len(t, model.list.Items, 1)
	assert.Equal(t, "ap-1", model.items[model.filtered[0]].ID)
}

func TestInboxFilterByTerm(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{
		{ID: "ap-1", Status: "pending", RequestType: "create_entity", AgentName: "alpha", ChangeDetails: api.JSONMap{"name": "Foo"}, CreatedAt: time.Now()},
		{ID: "ap-2", Status: "pending", RequestType: "create_entity", AgentName: "beta", ChangeDetails: api.JSONMap{"name": "Bar"}, CreatedAt: time.Now()},
	}
	model.filterBuf = "foo"
	model.applyFilter(true)

	assert.Len(t, model.list.Items, 1)
	assert.Equal(t, "ap-1", model.items[model.filtered[0]].ID)
}

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
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/approve"):
			parts := strings.Split(r.URL.Path, "/")
			approved = append(approved, parts[len(parts)-2])
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": parts[len(parts)-2]}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, approved)
}

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
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/approve"):
			parts := strings.Split(r.URL.Path, "/")
			approved = append(approved, parts[len(parts)-2])
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": parts[len(parts)-2]}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	// Multi-select two mixed request types.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.ElementsMatch(t, []string{"ap-bulk", "ap-update"}, approved)
}

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
						"requested_scopes":            []string{"public", "code"},
						"requested_requires_approval": false,
					},
					"created_at": time.Now(),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	model := NewInboxModel(client)
	cmd := model.Init()
	model, _ = model.Update(cmd())

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	assert.True(t, model.grantEditing)
	assert.Equal(t, "ap-register", model.grantApproval)
	assert.Equal(t, "public,code", model.grantScopes)
	assert.False(t, model.grantTrusted)
}

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
	cmd := model.Init()
	model, _ = model.Update(cmd())

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	require.True(t, model.grantEditing)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.Equal(t, []any{"public"}, approvedBody["grant_scopes"])
	assert.Equal(t, false, approvedBody["grant_requires_approval"])
}

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
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/reject"):
			parts := strings.Split(r.URL.Path, "/")
			rejected = append(rejected, parts[len(parts)-2])
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": parts[len(parts)-2]}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewInboxModel(client)
	cmd := model.Init()
	msg := cmd()
	model, _ = model.Update(msg)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Nil(t, cmd)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

	assert.ElementsMatch(t, []string{"ap-1", "ap-2"}, rejected)
}

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

func TestParseApprovalFilterTokens(t *testing.T) {
	filter := parseApprovalFilter("agent:Alpha type:Create since:1d custom")
	assert.Equal(t, "alpha", filter.agent)
	assert.Equal(t, "create", filter.req)
	assert.NotNil(t, filter.since)
	assert.Equal(t, []string{"custom"}, filter.terms)
}

func TestInboxRenderGrantEditorShowsCurrentInputs(t *testing.T) {
	model := NewInboxModel(nil)
	model.width = 90
	model.grantScopes = "public,private"
	model.grantTrusted = false

	out := stripANSI(model.renderGrantEditor())
	assert.Contains(t, out, "Approve Agent Enrollment")
	assert.Contains(t, out, "public,private")
	assert.Contains(t, out, "requires_approval: false")
}

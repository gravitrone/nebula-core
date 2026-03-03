package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testServer handles test server.
func testServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClient(srv.URL, "nbl_testkey")
	return srv, client
}

// jsonResponse handles json response.
func jsonResponse(data any) []byte {
	b, _ := json.Marshal(map[string]any{"data": data})
	return b
}

// TestGetEntity handles test get entity.
func TestGetEntity(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "Bearer nbl_testkey", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/api/entities/")
		_, err := w.Write(jsonResponse(map[string]any{
			"id":   "abc-123",
			"name": "test entity",
			"tags": []string{"test"},
		}))
		require.NoError(t, err)
	})

	entity, err := client.GetEntity("abc-123")
	require.NoError(t, err)
	assert.Equal(t, "abc-123", entity.ID)
	assert.Equal(t, "test entity", entity.Name)
}

// TestQueryEntities handles test query entities.
func TestQueryEntities(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "active", r.URL.Query().Get("status"))
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "1", "name": "one", "tags": []string{}},
			{"id": "2", "name": "two", "tags": []string{}},
		}))
		require.NoError(t, err)
	})

	entities, err := client.QueryEntities(QueryParams{"status": "active"})
	require.NoError(t, err)
	assert.Len(t, entities, 2)
}

// TestQueryAuditLog handles test query audit log.
func TestQueryAuditLog(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/audit", r.URL.Path)
		assert.Equal(t, "entities", r.URL.Query().Get("table"))
		assert.Equal(t, "update", r.URL.Query().Get("action"))
		assert.Equal(t, "agent", r.URL.Query().Get("actor_type"))
		assert.Equal(t, "agent-1", r.URL.Query().Get("actor_id"))
		assert.Equal(t, "ent-1", r.URL.Query().Get("record_id"))
		assert.Equal(t, "scope-1", r.URL.Query().Get("scope_id"))
		assert.Equal(t, "25", r.URL.Query().Get("limit"))
		assert.Equal(t, "0", r.URL.Query().Get("offset"))
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "audit-1", "table_name": "entities", "record_id": "ent-1"},
		}))
		require.NoError(t, err)
	})

	items, err := client.QueryAuditLogWithPagination(
		"entities",
		"update",
		"agent",
		"agent-1",
		"ent-1",
		"scope-1",
		25,
		0,
	)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "audit-1", items[0].ID)
}

// TestListAuditScopes handles test list audit scopes.
func TestListAuditScopes(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/audit/scopes", r.URL.Path)
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "scope-1", "name": "public", "agent_count": 2},
		}))
		require.NoError(t, err)
	})

	scopes, err := client.ListAuditScopes()
	require.NoError(t, err)
	require.Len(t, scopes, 1)
	assert.Equal(t, "scope-1", scopes[0].ID)
}

// TestListAuditActors handles test list audit actors.
func TestListAuditActors(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/audit/actors", r.URL.Path)
		assert.Equal(t, "agent", r.URL.Query().Get("actor_type"))
		_, err := w.Write(jsonResponse([]map[string]any{
			{"changed_by_type": "agent", "changed_by_id": "agent-1", "action_count": 3},
		}))
		require.NoError(t, err)
	})

	actors, err := client.ListAuditActors("agent")
	require.NoError(t, err)
	require.Len(t, actors, 1)
	assert.Equal(t, "agent-1", actors[0].ActorID)
}

// TestCreateEntity handles test create entity.
func TestCreateEntity(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		var body CreateEntityInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "new entity", body.Name)
		_, err := w.Write(jsonResponse(map[string]any{
			"id":   "new-id",
			"name": "new entity",
			"tags": []string{},
		}))
		require.NoError(t, err)
	})

	entity, err := client.CreateEntity(CreateEntityInput{
		Scopes: []string{"public"},
		Name:   "new entity",
		Type:   "person",
		Status: "active",
		Tags:   []string{},
	})
	require.NoError(t, err)
	assert.Equal(t, "new-id", entity.ID)
}

// TestGetPendingApprovals handles test get pending approvals.
func TestGetPendingApprovals(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/approvals/pending", r.URL.Path)
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "ap-1", "agent_id": "ag-1", "action_type": "register", "status": "pending", "details": map[string]any{}},
		}))
		require.NoError(t, err)
	})

	approvals, err := client.GetPendingApprovals()
	require.NoError(t, err)
	assert.Len(t, approvals, 1)
	assert.Equal(t, "ap-1", approvals[0].ID)
}

// TestApproveRequest handles test approve request.
func TestApproveRequest(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/approve")
		_, err := w.Write(jsonResponse(map[string]any{
			"id": "ap-1", "status": "approved", "agent_id": "ag-1",
			"action_type": "register", "details": map[string]any{},
		}))
		require.NoError(t, err)
	})

	approval, err := client.ApproveRequest("ap-1")
	require.NoError(t, err)
	assert.Equal(t, "approved", approval.Status)
}

// TestListAgents handles test list agents.
func TestListAgents(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "ag-1", "name": "test-agent", "status": "active", "requires_approval": true, "scopes": []string{"public"}},
		}))
		require.NoError(t, err)
	})

	agents, err := client.ListAgents("")
	require.NoError(t, err)
	assert.Len(t, agents, 1)
	assert.Equal(t, "test-agent", agents[0].Name)
}

// TestLogin handles test login.
func TestLogin(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/api/keys/login")
		_, err := w.Write(jsonResponse(map[string]any{
			"api_key":   "nbl_newkey",
			"entity_id": "ent-1",
			"username":  "testuser",
		}))
		require.NoError(t, err)
	})

	resp, err := client.Login("testuser")
	require.NoError(t, err)
	assert.Equal(t, "nbl_newkey", resp.APIKey)
	assert.Equal(t, "testuser", resp.Username)
}

// TestHTTPError handles test httperror.
func TestHTTPError(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "NOT_FOUND",
				"message": "entity not found",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.GetEntity("nope")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_FOUND")
}

// TestHTTPErrorNestedDetailFormat handles test httperror nested detail format.
func TestHTTPErrorNestedDetailFormat(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": map[string]any{
				"error": map[string]any{
					"code":    "FORBIDDEN",
					"message": "Admin scope required",
				},
			},
		})
	})

	_, err := client.GetEntity("nope")
	require.Error(t, err)
	assert.Equal(t, "FORBIDDEN: Admin scope required", err.Error())
}

// TestDoNormalizesInvalidKeyRecoveryHints handles auth normalization for regular API requests.
func TestDoNormalizesInvalidKeyRecoveryHints(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": map[string]any{
				"error": map[string]any{
					"code":    "UNAUTHORIZED",
					"message": "token expired",
				},
			},
		})
	})

	_, err := client.QueryEntities(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_API_KEY")
	assert.Contains(t, err.Error(), "token expired")
}

// TestDoPreservesForbiddenScopeErrors handles non-auth 403 payloads without invalid-key coercion.
func TestDoPreservesForbiddenScopeErrors(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "FORBIDDEN",
				"message": "Admin scope required",
			},
		})
	})

	_, err := client.QueryEntities(nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "INVALID_API_KEY")
	assert.Contains(t, err.Error(), "FORBIDDEN")
	assert.Contains(t, err.Error(), "Admin scope required")
}

// TestDoDoesNotNormalizeAuthorizationScopeDenied handles authz errors that mention authorization.
func TestDoDoesNotNormalizeAuthorizationScopeDenied(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "FORBIDDEN",
				"message": "authorization scope denied",
			},
		})
	})

	_, err := client.QueryEntities(nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "INVALID_API_KEY")
	assert.Contains(t, err.Error(), "FORBIDDEN")
	assert.Contains(t, err.Error(), "authorization scope denied")
}

// TestDoNormalizesMultiAPIConflict handles startup-style 500 collisions for user-facing recovery.
func TestDoNormalizesMultiAPIConflict(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": "Address already in use",
		})
	})

	_, err := client.QueryEntities(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MULTIPLE_API_INSTANCES_DETECTED")
	assert.Contains(t, err.Error(), "multiple api instances detected")
}

// TestDoParsesValidationDetailList keeps FastAPI validation errors readable for CLI.
func TestDoParsesValidationDetailList(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detail": []map[string]any{
				{"msg": "Input should be a valid string"},
				{"msg": "Field required"},
			},
		})
	})

	_, err := client.QueryEntities(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Input should be a valid string")
	assert.Contains(t, err.Error(), "Field required")
	assert.NotContains(t, err.Error(), "HTTP 422:")
}

// TestHealth handles test health.
func TestHealth(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/health", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})

	status, err := client.Health()
	require.NoError(t, err)
	assert.Equal(t, "ok", status)
}

// TestBuildQuery handles test build query.
func TestBuildQuery(t *testing.T) {
	result := buildQuery("/api/entities", QueryParams{"status": "active", "type": "person"})
	assert.Contains(t, result, "/api/entities?")
	assert.Contains(t, result, "status=active")
	assert.Contains(t, result, "type=person")
}

// TestBuildQueryEmpty handles test build query empty.
func TestBuildQueryEmpty(t *testing.T) {
	result := buildQuery("/api/entities", nil)
	assert.Equal(t, "/api/entities", result)
}

// TestBuildQuerySkipsEmptyParams ensures no trailing question mark when params normalize to empty.
func TestBuildQuerySkipsEmptyParams(t *testing.T) {
	result := buildQuery("/api/entities", QueryParams{
		"status": "",
		"type":   "",
	})
	assert.Equal(t, "/api/entities", result)
}

// TestBuildQueryMergesWithExistingQuery keeps existing query params and appends new ones.
func TestBuildQueryMergesWithExistingQuery(t *testing.T) {
	result := buildQuery("/api/entities?scope=public", QueryParams{
		"status": "active",
	})
	parsed, err := url.Parse(result)
	require.NoError(t, err)
	assert.Equal(t, "/api/entities", parsed.Path)
	assert.Equal(t, "public", parsed.Query().Get("scope"))
	assert.Equal(t, "active", parsed.Query().Get("status"))
}

// TestBuildQueryInvalidPathFallsBackToRawInput ensures parse failures do not rewrite caller paths.
func TestBuildQueryInvalidPathFallsBackToRawInput(t *testing.T) {
	input := "http://[::1"
	result := buildQuery(input, QueryParams{"status": "active"})
	assert.Equal(t, input, result)
}

// TestNewClientCustomTimeout handles test new client custom timeout.
func TestNewClientCustomTimeout(t *testing.T) {
	client := NewClient("http://example.com", "nbl_testkey", 5*time.Second)
	assert.Equal(t, 5*time.Second, client.httpClient.Timeout)
}

// TestWithTimeoutClonesClient handles test with timeout clones client.
func TestWithTimeoutClonesClient(t *testing.T) {
	client := NewClient("http://example.com", "nbl_testkey", 30*time.Second)
	clone := client.WithTimeout(2 * time.Second)

	assert.Equal(t, 2*time.Second, clone.httpClient.Timeout)
	assert.NotSame(t, client, clone)
}

// TestClientConcurrentRequests handles test client concurrent requests.
func TestClientConcurrentRequests(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		_, err := w.Write(jsonResponse(map[string]any{
			"id":   "ent-1",
			"name": "test entity",
			"tags": []string{},
		}))
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)

	client := NewClient(srv.URL, "nbl_testkey")

	const workers = 20
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := client.GetEntity(fmt.Sprintf("ent-%d", idx))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}
	assert.Equal(t, int32(workers), count.Load())
}

// TestCreateKey handles test create key.
func TestCreateKey(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		_, err := w.Write(jsonResponse(map[string]any{
			"api_key": "nbl_abc123",
			"key_id":  "key-1",
			"prefix":  "nbl_abc",
			"name":    "my-key",
		}))
		require.NoError(t, err)
	})

	resp, err := client.CreateKey("my-key")
	require.NoError(t, err)
	assert.Equal(t, "nbl_abc123", resp.APIKey)
	assert.Equal(t, "my-key", resp.Name)
}

// TestSetAPIKeyUpdatesSubsequentRequests handles test set apikey updates subsequent requests.
func TestSetAPIKeyUpdatesSubsequentRequests(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer nbl_newkey" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, err := w.Write(jsonResponse(map[string]any{
			"id":   "ent-1",
			"name": "ok",
			"tags": []string{},
		}))
		require.NoError(t, err)
	})

	client.SetAPIKey("")
	_, err := client.GetEntity("ent-1")
	require.Error(t, err)

	client.SetAPIKey("nbl_newkey")
	entity, err := client.GetEntity("ent-1")
	require.NoError(t, err)
	assert.Equal(t, "ent-1", entity.ID)
}

// TestQueryJobs handles test query jobs.
func TestQueryJobs(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "j-1", "title": "test job", "status": "pending"},
		}))
		require.NoError(t, err)
	})

	jobs, err := client.QueryJobs(nil)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "test job", jobs[0].Title)
}

// TestRegisterAgent handles test register agent.
func TestRegisterAgent(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		_, err := w.Write(jsonResponse(map[string]any{
			"agent_id":            "ag-new",
			"approval_request_id": "ap-new",
			"status":              "pending_approval",
		}))
		require.NoError(t, err)
	})

	resp, err := client.RegisterAgent(RegisterAgentInput{
		Name:            "new-agent",
		RequestedScopes: []string{"public"},
	})
	require.NoError(t, err)
	assert.Equal(t, "ag-new", resp.AgentID)
	assert.Equal(t, "pending_approval", resp.Status)
}

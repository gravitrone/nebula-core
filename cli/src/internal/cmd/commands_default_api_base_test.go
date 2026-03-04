package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startDefaultAPIBaseServer handles start default apibase server.
func startDefaultAPIBaseServer(t *testing.T, handler http.Handler) func() {
	t.Helper()

	srv := httptest.NewServer(handler)
	parsed, err := url.Parse(srv.URL)
	require.NoError(t, err)
	restore := setDefaultClientFactoryForTest(t, func(apiKey string, timeout ...time.Duration) *api.Client {
		base := "http://" + parsed.Host
		return api.NewClient(base, apiKey, timeout...)
	})
	return func() {
		restore()
		srv.Close()
	}
}

// setDefaultClientFactoryForTest handles overriding command API client construction.
func setDefaultClientFactoryForTest(
	t *testing.T,
	factory func(apiKey string, timeout ...time.Duration) *api.Client,
) func() {
	t.Helper()
	previous := newDefaultClient
	newDefaultClient = factory
	return func() {
		newDefaultClient = previous
	}
}

// TestLoginCmdSuccessAgainstDefaultAPIBaseURL handles test login cmd success against default apibase url.
func TestLoginCmdSuccessAgainstDefaultAPIBaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost:
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "alxx", body["username"])
			_, _ = io.WriteString(w, `{"data":{"api_key":"nbl_test","entity_id":"ent-1","username":"alxx"}}`)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	_, _ = io.WriteString(w, "alxx\n")
	_ = w.Close()
	os.Stdin = r

	cmd := LoginCmd()
	cmd.SetArgs([]string{})
	err = cmd.Execute()
	require.NoError(t, err)

	loaded, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "nbl_test", loaded.APIKey)
	assert.Equal(t, "ent-1", loaded.UserEntityID)
	assert.Equal(t, "alxx", loaded.Username)
	assert.True(t, loaded.QuickstartPending)
}

// TestKeysAndAgentListHappyPathsAgainstDefaultAPIBaseURL handles test keys and agent list happy paths against default apibase url.
func TestKeysAndAgentListHappyPathsAgainstDefaultAPIBaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	now := time.Now()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/keys" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":         "k1",
				"key_prefix": "nbl_abc123",
				"name":       "demo",
				"created_at": now,
			}}}))
			return
		case r.URL.Path == "/api/agents/" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":                "agent-1",
				"name":              "Alpha",
				"status":            "active",
				"requires_approval": true,
				"scopes":            []string{"public"},
				"capabilities":      []string{"read"},
				"created_at":        now,
				"updated_at":        now,
			}}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	keys := KeysCmd()
	keys.SetArgs([]string{"list"})
	require.NoError(t, keys.Execute())

	agents := AgentCmd()
	agents.SetArgs([]string{"list"})
	require.NoError(t, agents.Execute())
}

// TestAgentListCmdCoversEmptyAndTrustDescriptionBranches handles list rendering
// for empty responses plus trusted/untrusted row formatting branches.
func TestAgentListCmdCoversEmptyAndTrustDescriptionBranches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	now := time.Now()
	requests := 0
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/agents/" && r.Method == http.MethodGet:
			require.Equal(t, "active", r.URL.Query().Get("status_category"))
			requests++
			if requests == 1 {
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":                "agent-1",
					"name":              "trusted-bot",
					"description":       "handles ops",
					"status":            "active",
					"requires_approval": false,
					"scopes":            []string{"public"},
					"capabilities":      []string{"read"},
					"created_at":        now,
					"updated_at":        now,
				},
				{
					"id":                "agent-2",
					"name":              "review-bot",
					"status":            "active",
					"requires_approval": true,
					"scopes":            []string{"public"},
					"capabilities":      []string{"read"},
					"created_at":        now,
					"updated_at":        now,
				},
			}}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	var firstOut bytes.Buffer
	first := AgentCmd()
	first.SetOut(&firstOut)
	first.SetErr(&firstOut)
	first.SetArgs([]string{"list"})
	require.NoError(t, first.Execute())
	assert.Contains(t, firstOut.String(), "No agents found.")

	var secondOut bytes.Buffer
	second := AgentCmd()
	second.SetOut(&secondOut)
	second.SetErr(&secondOut)
	second.SetArgs([]string{"list"})
	require.NoError(t, second.Execute())
	assert.Contains(t, secondOut.String(), "trusted-bot")
	assert.Contains(t, secondOut.String(), "trusted - handles ops")
	assert.Contains(t, secondOut.String(), "review-bot")
	assert.Contains(t, secondOut.String(), "untrusted")
}

func TestAgentListCmdReturnsNotLoggedInWhenConfigMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := agentListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestAgentListCmdReturnsListErrorOnAPIFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"boom"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := agentListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list agents")
}

func TestAgentRegisterCmdReturnsNotLoggedInWhenConfigMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := agentRegisterCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"new-agent"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestAgentRegisterCmdReturnsRegisterErrorOnAPIFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/register" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"register failed"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := agentRegisterCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"new-agent"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "register agent")
}

// TestKeysListAllFlagAgainstDefaultAPIBaseURL handles list-all key flows on default base URL.
func TestKeysListAllFlagAgainstDefaultAPIBaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	now := time.Now()
	agentName := "cto-agent"
	entityName := "alxx"

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/keys/all" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "k-agent",
						"key_prefix": "nbl_agent",
						"name":       "agent-key",
						"owner_type": "agent",
						"agent_name": agentName,
						"created_at": now,
					},
					{
						"id":          "k-user",
						"key_prefix":  "nbl_user",
						"name":        "user-key",
						"owner_type":  "entity",
						"entity_name": entityName,
						"created_at":  now,
					},
				},
			}))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	keys := KeysCmd()
	var out bytes.Buffer
	keys.SetOut(&out)
	keys.SetErr(&out)
	keys.SetArgs([]string{"list", "--all"})
	require.NoError(t, keys.Execute())

	text := out.String()
	assert.Contains(t, text, "agent-key")
	assert.Contains(t, text, "user-key")
	assert.Contains(t, text, "agent:cto-agent")
	assert.Contains(t, text, "user:alxx")
}

// TestKeysCreateRevokeAndAgentRegisterAgainstDefaultAPIBaseURL handles success paths
// for key creation/revocation and agent registration against the default local base URL.
func TestKeysCreateRevokeAndAgentRegisterAgainstDefaultAPIBaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	var (
		createdKeyName   string
		revokedKeyID     string
		registeredAgent  string
		registeredDesc   string
		registeredScopes []string
	)

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/keys" && r.Method == http.MethodPost:
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			createdKeyName = body["name"].(string)
			_, _ = io.WriteString(w, `{"data":{"api_key":"nbl_live_key","key_id":"key-123","prefix":"nbl_live","name":"demo-key"}}`)
			return
		case r.URL.Path == "/api/keys/key-123" && r.Method == http.MethodDelete:
			revokedKeyID = "key-123"
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":{"status":"ok"}}`)
			return
		case r.URL.Path == "/api/agents/register" && r.Method == http.MethodPost:
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			registeredAgent = body["name"].(string)
			if desc, ok := body["description"].(string); ok {
				registeredDesc = desc
			}
			for _, raw := range body["requested_scopes"].([]any) {
				registeredScopes = append(registeredScopes, raw.(string))
			}
			_, _ = io.WriteString(w, `{"data":{"agent_id":"ag-1","approval_request_id":"apr-1","status":"pending"}}`)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	keysCreate := KeysCmd()
	var createOut bytes.Buffer
	keysCreate.SetOut(&createOut)
	keysCreate.SetErr(&createOut)
	keysCreate.SetArgs([]string{"create", "demo-key"})
	require.NoError(t, keysCreate.Execute())
	assert.Equal(t, "demo-key", createdKeyName)
	assert.Contains(t, createOut.String(), "nbl_live_key")

	keysRevoke := KeysCmd()
	var revokeOut bytes.Buffer
	keysRevoke.SetOut(&revokeOut)
	keysRevoke.SetErr(&revokeOut)
	keysRevoke.SetArgs([]string{"revoke", "key-123"})
	require.NoError(t, keysRevoke.Execute())
	assert.Equal(t, "key-123", revokedKeyID)
	assert.Contains(t, revokeOut.String(), "revoked")

	agentsRegister := AgentCmd()
	var registerOut bytes.Buffer
	agentsRegister.SetOut(&registerOut)
	agentsRegister.SetErr(&registerOut)
	agentsRegister.SetArgs([]string{"register", "cto-agent", "--description", "overnight test agent"})
	require.NoError(t, agentsRegister.Execute())
	assert.Equal(t, "cto-agent", registeredAgent)
	assert.Equal(t, "overnight test agent", registeredDesc)
	assert.Equal(t, []string{"public"}, registeredScopes)
	assert.Contains(t, registerOut.String(), "ag-1")
	assert.Contains(t, registerOut.String(), "apr-1")
}

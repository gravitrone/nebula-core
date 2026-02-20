package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startDefaultAPIBaseServer(t *testing.T, handler http.Handler) func() {
	t.Helper()

	parsed, err := url.Parse(api.DefaultBaseURL)
	require.NoError(t, err)
	ln, err := net.Listen("tcp", parsed.Host)
	if err != nil {
		t.Skip("default api port busy; skipping localhost happy-path cmd coverage")
	}

	srv := &http.Server{Handler: handler}
	go func() {
		_ = srv.Serve(ln)
	}()

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

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

func TestKeysAndAgentListHappyPathsAgainstDefaultAPIBaseURL(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	now := time.Now()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/keys" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":         "k1",
				"key_prefix": "nbl_abc123",
				"name":       "demo",
				"created_at": now,
			}}})
			return
		case r.URL.Path == "/api/agents/" && r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
				"id":                "agent-1",
				"name":              "Alpha",
				"status":            "active",
				"requires_approval": true,
				"scopes":            []string{"public"},
				"capabilities":      []string{"read"},
				"created_at":        now,
				"updated_at":        now,
			}}})
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

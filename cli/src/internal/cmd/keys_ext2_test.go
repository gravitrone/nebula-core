package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeysListHandlesListAllErrorPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/all" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "FORBIDDEN",
					"message": "admin required",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := KeysCmd()
	cmd.SetArgs([]string{"list", "--all"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list keys")
}

func TestKeysListHandlesListCurrentUserErrorPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "UNAUTHORIZED",
					"message": "invalid key",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := KeysCmd()
	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list keys")
}

func TestKeysListOwnerFallbackAndLastUsedFormatting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"data":[{"id":"k-1","name":"my-key","key_prefix":"nbl_demo","owner_type":"agent","created_at":"2026-01-01T00:00:00Z","last_used_at":"2026-01-02T03:04:05Z"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := KeysCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())

	text := out.String()
	assert.Contains(t, text, "my-key (my-key)")
	assert.Contains(t, text, "last used 2026-01-02 03:04")
}

func TestKeysListAllRendersNoKeysFoundState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/all" && r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := KeysCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list", "--all"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "No keys found")
}

func TestKeysCreateHandlesCreateErrorPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "INTERNAL_ERROR",
					"message": "database exploded",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := KeysCmd()
	cmd.SetArgs([]string{"create", "demo-key"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create key")
}

func TestKeysRevokeHandlesRevokeErrorPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/key-missing" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "NOT_FOUND",
					"message": "key missing",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := KeysCmd()
	cmd.SetArgs([]string{"revoke", "key-missing"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "revoke key")
}

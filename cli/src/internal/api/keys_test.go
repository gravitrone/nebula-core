package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListKeysFiltered handles test list keys filtered.
func TestListKeysFiltered(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/keys", r.URL.Path)
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "key-1", "prefix": "nbl_abc", "name": "my-key", "active": true},
		}))
		require.NoError(t, err)
	})

	keys, err := client.ListKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Equal(t, "my-key", keys[0].Name)
}

// TestListAllKeys handles test list all keys.
func TestListAllKeys(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/keys/all", r.URL.Path)
		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "key-1", "prefix": "nbl_abc", "name": "key1", "active": true},
			{"id": "key-2", "prefix": "nbl_def", "name": "key2", "active": false},
		}))
		require.NoError(t, err)
	})

	keys, err := client.ListAllKeys()
	require.NoError(t, err)
	assert.Len(t, keys, 2)
}

// TestRevokeKey handles test revoke key.
func TestRevokeKey(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Contains(t, r.URL.Path, "/api/keys/key-1")
		_, err := w.Write(jsonResponse(map[string]any{}))
		require.NoError(t, err)
	})

	err := client.RevokeKey("key-1")
	require.NoError(t, err)
}

// TestRevokeKeyNotFound handles test revoke key not found.
func TestRevokeKeyNotFound(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "NOT_FOUND",
				"message": "key not found",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	err := client.RevokeKey("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NOT_FOUND")
}

// TestLoginUnauthenticated handles test login unauthenticated.
func TestLoginUnauthenticated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Login endpoint should not require auth
		assert.Empty(t, r.Header.Get("Authorization"))

		_, err := w.Write(jsonResponse(map[string]any{
			"api_key":   "nbl_newkey",
			"entity_id": "ent-1",
			"username":  "testuser",
		}))
		require.NoError(t, err)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "")
	resp, err := client.Login("testuser")
	require.NoError(t, err)
	assert.Equal(t, "nbl_newkey", resp.APIKey)
}

// TestLoginInvalidUsername handles test login invalid username.
func TestLoginInvalidUsername(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "INVALID_USERNAME",
				"message": "username must be alphanumeric",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.Login("invalid@user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_USERNAME")
}

// TestCreateKeyDuplicateName handles test create key duplicate name.
func TestCreateKeyDuplicateName(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "DUPLICATE",
				"message": "key name already exists",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.CreateKey("existing-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "DUPLICATE")
}

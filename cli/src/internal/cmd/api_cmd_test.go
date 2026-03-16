package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPICmdRegistersResourceGroups(t *testing.T) {
	cmd := APICmd()
	for _, name := range []string{
		"health", "entities", "context", "relationships", "jobs",
		"logs", "files", "protocols", "approvals", "agents",
		"keys", "audit", "taxonomy", "search", "import", "export",
	} {
		found, _, err := cmd.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, name, found.Name())
	}
}

func TestAPICmdEntitiesQueryPrintsCleanJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	now := time.Now().UTC()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entities" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		require.Equal(t, "Bearer nbl_test", r.Header.Get("Authorization"))
		require.Equal(t, "1", r.URL.Query().Get("limit"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":         "ent-1",
					"name":       "Luna",
					"type":       "person",
					"status":     "active",
					"tags":       []string{"cat"},
					"created_at": now,
					"updated_at": now,
				},
			},
		}))
	}))
	t.Cleanup(shutdown)

	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"entities", "query", "--param", "limit=1"})
	require.NoError(t, cmd.Execute())

	text := out.String()
	assert.Contains(t, text, "\"name\": \"Luna\"")
	assert.NotContains(t, text, "Context Infrastructure for Agents")
	assert.NotContains(t, text, "NEBULA")
}

func TestAPICmdEntitiesCreateFromInputFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	inputPath := filepath.Join(t.TempDir(), "entity.json")
	require.NoError(t, os.WriteFile(inputPath, []byte(`{
  "scopes": ["public"],
  "name": "Alex",
  "type": "person",
  "status": "active",
  "tags": ["founder"]
}`), 0o600))

	now := time.Now().UTC()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entities" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "Alex", body["name"])
		require.Equal(t, "person", body["type"])
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":         "ent-2",
				"name":       "Alex",
				"type":       "person",
				"status":     "active",
				"tags":       []string{"founder"},
				"created_at": now,
				"updated_at": now,
			},
		}))
	}))
	t.Cleanup(shutdown)

	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"entities", "create", "--input-file", inputPath})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "\"id\": \"ent-2\"")
}

func TestAPICmdApprovalsRejectRequiresNotes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"approvals", "reject", "ap-1"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing --notes")
}

func TestAPICmdKeysLoginWorksWithoutConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/keys/login" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "alxx", body["username"])
		_, _ = io.WriteString(w, `{"data":{"api_key":"nbl_test","entity_id":"ent-1","username":"alxx"}}`)
	}))
	t.Cleanup(shutdown)

	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"keys", "login", "alxx"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), `"api_key": "nbl_test"`)
}

func TestParseQueryParamsRejectsInvalidPairs(t *testing.T) {
	_, err := parseQueryParams([]string{"badpair"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected key=value")
}

func TestReadInputJSONValidation(t *testing.T) {
	_, err := readInputJSON("{}", "payload.json", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "either --input or --input-file")

	_, err = readInputJSON("not-json", "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")

	_, err = readInputJSON("", "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing input")

	path := filepath.Join(t.TempDir(), "payload.json")
	require.NoError(t, os.WriteFile(path, []byte("{\"ok\":true}"), 0o600))
	raw, err := readInputJSON("", path, true)
	require.NoError(t, err)
	assert.Equal(t, "{\"ok\":true}", strings.TrimSpace(string(raw)))
}

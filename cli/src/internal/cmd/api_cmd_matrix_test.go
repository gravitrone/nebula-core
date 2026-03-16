package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runAPISubcommand(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute(), strings.Join(args, " "))
	result := out.String()
	assert.NotContains(t, result, "Context Infrastructure for Agents")
	assert.NotContains(t, result, "NEBULA")
	var payload any
	require.NoError(t, json.Unmarshal([]byte(result), &payload), strings.Join(args, " "))
	return result
}

func setupAPICommandAuth(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())
}

func TestAPICmdReadMatrixAgainstMockServer(t *testing.T) {
	setupAPICommandAuth(t)

	now := time.Now().UTC()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/health":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"status": "ok"}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/entities":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/context":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/relationships":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/jobs":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/logs":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/files":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/protocols":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/approvals/pending":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/agents/":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/keys":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/keys/all":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/audit":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/audit/scopes":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/audit/actors":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/taxonomy/scopes":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/search/semantic":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/export/snapshot":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"format":  "json",
				"content": "",
				"items":   []map[string]any{},
				"count":   0,
			}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	runAPISubcommand(t, "health")
	runAPISubcommand(t, "entities", "query", "--param", "limit=1")
	runAPISubcommand(t, "context", "query", "--param", "limit=1")
	runAPISubcommand(t, "relationships", "query", "--param", "limit=1")
	runAPISubcommand(t, "jobs", "query", "--param", "limit=1")
	runAPISubcommand(t, "logs", "query", "--param", "limit=1")
	runAPISubcommand(t, "files", "query", "--param", "limit=1")
	runAPISubcommand(t, "protocols", "query", "--param", "limit=1")
	runAPISubcommand(t, "approvals", "pending", "--limit=1")
	runAPISubcommand(t, "agents", "list", "--status-category=active")
	runAPISubcommand(t, "keys", "list")
	runAPISubcommand(t, "keys", "list-all")
	runAPISubcommand(t, "audit", "query", "--param", "limit=1")
	runAPISubcommand(t, "audit", "scopes")
	runAPISubcommand(t, "audit", "actors")
	runAPISubcommand(t, "taxonomy", "list", "scopes", "--limit=1")
	runAPISubcommand(t, "search", "semantic", "--query", "nebula", "--limit", "2")
	runAPISubcommand(t, "export", "snapshot", "--param", "format=json")

	_ = now
}

func TestAPICmdReadDetailMatrixAgainstMockServer(t *testing.T) {
	setupAPICommandAuth(t)

	now := time.Now().UTC()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/entities/e1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "e1", "name": "Entity", "type": "person", "status": "active", "tags": []string{"smoke"}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/context/c1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "c1", "title": "Context", "name": "Context", "source_type": "note", "tags": []string{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/jobs/j1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "j1", "title": "Job", "status": "pending", "priority": "low", "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/logs/l1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "l1", "log_type": "event", "timestamp": now, "value": map[string]any{"kind": "smoke"}, "status": "active", "metadata": map[string]any{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/files/f1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "f1", "filename": "file.txt", "uri": "file:///tmp/file.txt", "file_path": "/tmp/file.txt", "status": "active", "tags": []string{}, "metadata": map[string]any{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/protocols/proto":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "p1", "name": "proto", "title": "Proto", "status": "active", "content": "# proto", "tags": []string{}, "metadata": map[string]any{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/agents/alpha":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "ag1", "name": "alpha", "description": "agent", "scopes": []string{"public"}, "capabilities": []string{"read"}, "status": "active", "requires_approval": false, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && r.URL.Path == "/api/approvals/a1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "a1", "request_type": "update_entity", "requested_by": "u1", "requested_by_name": "u1", "agent_name": "alpha", "change_details": map[string]any{}, "review_details": map[string]any{}, "status": "pending", "created_at": now,
			}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	runAPISubcommand(t, "entities", "get", "e1")
	runAPISubcommand(t, "context", "get", "c1")
	runAPISubcommand(t, "jobs", "get", "j1")
	runAPISubcommand(t, "logs", "get", "l1")
	runAPISubcommand(t, "files", "get", "f1")
	runAPISubcommand(t, "protocols", "get", "proto")
	runAPISubcommand(t, "agents", "get", "alpha")
	runAPISubcommand(t, "approvals", "get", "a1")
}

func TestAPICmdWriteMatrixAgainstMockServer(t *testing.T) {
	setupAPICommandAuth(t)

	now := time.Now().UTC()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/entities":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "e1", "name": "Entity", "type": "person", "status": "active", "tags": []string{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/entities/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "e1", "name": "Entity", "type": "person", "status": "inactive", "tags": []string{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/history"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/revert"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "e1", "name": "Entity", "type": "person", "status": "active", "tags": []string{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/entities/bulk/tags":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"updated": 1, "entity_ids": []string{"e1"}}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/entities/bulk/scopes":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"updated": 1, "entity_ids": []string{"e1"}}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/context":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "c1", "title": "Context", "name": "Context", "source_type": "note", "tags": []string{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/context/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "c1", "title": "Context Updated", "name": "Context Updated", "source_type": "note", "tags": []string{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/link"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/relationships":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "r1", "source_type": "entity", "source_id": "e1", "target_type": "entity", "target_id": "e2", "relationship_type": "related-to", "properties": map[string]any{}, "created_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/relationships/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "r1", "source_type": "entity", "source_id": "e1", "target_type": "entity", "target_id": "e2", "relationship_type": "related-to", "properties": map[string]any{"phase": "updated"}, "created_at": now,
			}}))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/relationships/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/jobs":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "j1", "title": "Job", "status": "pending", "priority": "low", "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/jobs/") && strings.HasSuffix(r.URL.Path, "/status"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "j1", "title": "Job", "status": "completed", "priority": "low", "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/jobs/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "j1", "title": "Job Updated", "status": "pending", "priority": "low", "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/subtasks"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "j2", "title": "Subtask", "status": "pending", "priority": "low", "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/logs":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "l1", "log_type": "event", "timestamp": now, "value": map[string]any{}, "status": "active", "metadata": map[string]any{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/logs/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "l1", "log_type": "event", "timestamp": now, "value": map[string]any{}, "status": "active", "metadata": map[string]any{"phase": "updated"}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/files":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "f1", "filename": "file.txt", "uri": "file:///tmp/file.txt", "file_path": "/tmp/file.txt", "status": "active", "tags": []string{}, "metadata": map[string]any{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/files/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "f1", "filename": "file.txt", "uri": "file:///tmp/file.txt", "file_path": "/tmp/file.txt", "status": "active", "tags": []string{}, "metadata": map[string]any{"phase": "updated"}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/protocols":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "p1", "name": "proto", "title": "Proto", "status": "active", "tags": []string{}, "metadata": map[string]any{}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/protocols/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "p1", "name": "proto", "title": "Proto", "status": "active", "tags": []string{}, "metadata": map[string]any{"phase": "updated"}, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/approvals/") && strings.HasSuffix(r.URL.Path, "/diff"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"approval_id": "a1", "request_type": "update_entity", "changes": map[string]any{}}}))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/approvals/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "a1", "request_type": "update_entity", "requested_by": "u1", "requested_by_name": "u1", "agent_name": "alpha", "change_details": map[string]any{}, "review_details": map[string]any{}, "status": "pending", "created_at": now}}))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/approve"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "a1", "request_type": "update_entity", "requested_by": "u1", "requested_by_name": "u1", "agent_name": "alpha", "change_details": map[string]any{}, "review_details": map[string]any{}, "status": "approved", "created_at": now}}))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reject"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "a1", "request_type": "update_entity", "requested_by": "u1", "requested_by_name": "u1", "agent_name": "alpha", "change_details": map[string]any{}, "review_details": map[string]any{}, "status": "rejected", "created_at": now}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/agents/register":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"agent_id": "ag1", "approval_request_id": "a1", "registration_id": "reg1", "enrollment_token": "tok", "status": "pending"}}))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/agents/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "ag1", "name": "alpha", "description": "agent", "scopes": []string{"public"}, "capabilities": []string{"read"}, "status": "active", "requires_approval": false, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/agents/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id": "ag1", "name": "alpha", "description": "agent", "scopes": []string{"public"}, "capabilities": []string{"read"}, "status": "active", "requires_approval": true, "created_at": now, "updated_at": now,
			}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/keys":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"api_key": "nbl_new", "key_id": "k1", "prefix": "nbl_", "name": "new-key"}}))
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/keys/"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/taxonomy/") && strings.HasSuffix(r.URL.Path, "/archive"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "t1", "name": "entry", "is_active": false, "is_builtin": false, "metadata": map[string]any{}, "created_at": now, "updated_at": now}}))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/taxonomy/") && strings.HasSuffix(r.URL.Path, "/activate"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "t1", "name": "entry", "is_active": true, "is_builtin": false, "metadata": map[string]any{}, "created_at": now, "updated_at": now}}))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/taxonomy/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "t1", "name": "entry", "is_active": true, "is_builtin": false, "metadata": map[string]any{}, "created_at": now, "updated_at": now}}))
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/taxonomy/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "t1", "name": "entry", "is_active": true, "is_builtin": false, "metadata": map[string]any{}, "created_at": now, "updated_at": now}}))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/import/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"created": 0, "failed": 0, "errors": []any{}, "items": []any{}}}))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/export/"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"format": "json", "content": "", "items": []any{}, "count": 0}}))
		case r.Method == http.MethodPost && r.URL.Path == "/api/search/semantic":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	runAPISubcommand(t, "entities", "create", "--input", `{"scopes":["public"],"name":"Entity","type":"person","status":"active","tags":[]}`)
	runAPISubcommand(t, "entities", "update", "e1", "--input", `{"status":"inactive"}`)
	runAPISubcommand(t, "entities", "history", "e1", "--limit", "10", "--offset", "0")
	runAPISubcommand(t, "entities", "revert", "e1", "--audit-id", "a1")
	runAPISubcommand(t, "entities", "bulk-tags", "--input", `{"entity_ids":["e1"],"tags":["smoke"],"op":"add"}`)
	runAPISubcommand(t, "entities", "bulk-scopes", "--input", `{"entity_ids":["e1"],"scopes":["public"],"op":"replace"}`)

	runAPISubcommand(t, "context", "create", "--input", `{"title":"Ctx","source_type":"note","content":"x","scopes":["public"],"tags":[]}`)
	runAPISubcommand(t, "context", "update", "c1", "--input", `{"title":"Ctx Updated"}`)
	runAPISubcommand(t, "context", "link", "c1", "--owner-type", "entity", "--owner-id", "e1")

	runAPISubcommand(t, "relationships", "create", "--input", `{"source_type":"entity","source_id":"e1","target_type":"entity","target_id":"e2","relationship_type":"related-to","properties":{}}`)
	runAPISubcommand(t, "relationships", "update", "r1", "--input", `{"properties":{"phase":"updated"}}`)
	runAPISubcommand(t, "relationships", "for-source", "entity", "e1")

	runAPISubcommand(t, "jobs", "create", "--input", `{"title":"Job","status":"pending","priority":"low"}`)
	runAPISubcommand(t, "jobs", "update", "j1", "--input", `{"title":"Job Updated"}`)
	runAPISubcommand(t, "jobs", "set-status", "j1", "--status", "completed")
	runAPISubcommand(t, "jobs", "subtask", "j1", "--input", `{"title":"Subtask"}`)

	runAPISubcommand(t, "logs", "create", "--input", `{"log_type":"event","value":{"text":"x"},"metadata":{}}`)
	runAPISubcommand(t, "logs", "update", "l1", "--input", `{"metadata":{"phase":"updated"}}`)

	runAPISubcommand(t, "files", "create", "--input", `{"filename":"file.txt","uri":"file:///tmp/file.txt","status":"active","tags":[],"metadata":{}}`)
	runAPISubcommand(t, "files", "update", "f1", "--input", `{"metadata":{"phase":"updated"}}`)

	runAPISubcommand(t, "protocols", "create", "--input", `{"name":"proto","title":"Proto","content":"# proto","status":"active","metadata":{}}`)
	runAPISubcommand(t, "protocols", "update", "proto", "--input", `{"metadata":{"phase":"updated"}}`)

	runAPISubcommand(t, "approvals", "get", "a1")
	runAPISubcommand(t, "approvals", "diff", "a1")
	runAPISubcommand(t, "approvals", "approve", "a1")
	runAPISubcommand(t, "approvals", "approve", "a1", "--input", `{"review_notes":"ok"}`)
	runAPISubcommand(t, "approvals", "reject", "a1", "--notes", "nope")

	runAPISubcommand(t, "agents", "register", "--input", `{"name":"alpha","description":"agent","requested_scopes":["public"]}`)
	runAPISubcommand(t, "agents", "get", "alpha")
	runAPISubcommand(t, "agents", "update", "ag1", "--input", `{"requires_approval":true}`)

	runAPISubcommand(t, "keys", "create", "new-key")
	runAPISubcommand(t, "keys", "revoke", "k1")

	runAPISubcommand(t, "taxonomy", "create", "scopes", "--input", `{"name":"demo-scope","description":"demo"}`)
	runAPISubcommand(t, "taxonomy", "update", "scopes", "t1", "--input", `{"description":"updated"}`)
	runAPISubcommand(t, "taxonomy", "archive", "scopes", "t1")
	runAPISubcommand(t, "taxonomy", "activate", "scopes", "t1")

	runAPISubcommand(t, "import", "entities", "--input", `{"format":"json","items":[]}`)
	runAPISubcommand(t, "import", "context", "--input", `{"format":"json","items":[]}`)
	runAPISubcommand(t, "import", "relationships", "--input", `{"format":"json","items":[]}`)
	runAPISubcommand(t, "import", "jobs", "--input", `{"format":"json","items":[]}`)

	runAPISubcommand(t, "export", "entities", "--param", "format=json")
	runAPISubcommand(t, "export", "context", "--param", "format=json")
	runAPISubcommand(t, "export", "relationships", "--param", "format=json")
	runAPISubcommand(t, "export", "jobs", "--param", "format=json")
	runAPISubcommand(t, "export", "snapshot", "--param", "format=json")

	runAPISubcommand(t, "search", "semantic", "--query", "nebula", "--limit", "5")
}

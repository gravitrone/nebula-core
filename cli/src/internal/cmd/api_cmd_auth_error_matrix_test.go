package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runAPISubcommandExpectError(t *testing.T, want string, args ...string) {
	t.Helper()
	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.Execute()
	require.Error(t, err, "expected error for %v", args)
	assert.Contains(t, err.Error(), want, "unexpected error for %v", args)
}

func TestAPICmdAuthRequiredMatrixErrorsWithoutConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	commands := [][]string{
		{"entities", "query", "--param", "limit=1"},
		{"entities", "get", "e1"},
		{"entities", "create", "--input", `{"scopes":["public"],"name":"Entity","type":"person","status":"active","tags":[]}`},
		{"entities", "update", "e1", "--input", `{"status":"inactive"}`},
		{"entities", "history", "e1"},
		{"entities", "revert", "e1", "--audit-id", "a1"},
		{"entities", "bulk-tags", "--input", `{"entity_ids":["e1"],"tags":["smoke"],"op":"add"}`},
		{"entities", "bulk-scopes", "--input", `{"entity_ids":["e1"],"scopes":["public"],"op":"replace"}`},
		{"context", "query", "--param", "limit=1"},
		{"context", "get", "c1"},
		{"context", "create", "--input", `{"title":"Ctx","source_type":"note","content":"x","scopes":["public"],"tags":[]}`},
		{"context", "update", "c1", "--input", `{"title":"Updated"}`},
		{"context", "link", "c1", "--owner-type", "entity", "--owner-id", "e1"},
		{"relationships", "query", "--param", "limit=1"},
		{"relationships", "for-source", "entity", "e1"},
		{"relationships", "create", "--input", `{"source_type":"entity","source_id":"e1","target_type":"entity","target_id":"e2","relationship_type":"related-to"}`},
		{"relationships", "update", "r1", "--input", `{"notes":"phase: updated"}`},
		{"jobs", "query", "--param", "limit=1"},
		{"jobs", "get", "j1"},
		{"jobs", "create", "--input", `{"title":"Job","status":"pending","priority":"low"}`},
		{"jobs", "update", "j1", "--input", `{"status":"completed"}`},
		{"jobs", "set-status", "j1", "--status", "completed"},
		{"jobs", "subtask", "j1", "--input", `{"title":"Subtask"}`},
		{"logs", "query", "--param", "limit=1"},
		{"logs", "get", "l1"},
		{"logs", "create", "--input", `{"log_type":"event","value":{"text":"x"},"metadata":{}}`},
		{"logs", "update", "l1", "--input", `{"metadata":{"phase":"updated"}}`},
		{"files", "query", "--param", "limit=1"},
		{"files", "get", "f1"},
		{"files", "create", "--input", `{"filename":"file.txt","uri":"file:///tmp/file.txt","status":"active","tags":[],"metadata":{}}`},
		{"files", "update", "f1", "--input", `{"metadata":{"phase":"updated"}}`},
		{"protocols", "query", "--param", "limit=1"},
		{"protocols", "get", "proto"},
		{"protocols", "create", "--input", `{"name":"proto","title":"Proto","content":"# proto","status":"active","metadata":{}}`},
		{"protocols", "update", "proto", "--input", `{"metadata":{"phase":"updated"}}`},
		{"approvals", "pending"},
		{"approvals", "get", "a1"},
		{"approvals", "diff", "a1"},
		{"approvals", "approve", "a1"},
		{"approvals", "reject", "a1", "--notes", "nope"},
		{"agents", "list"},
		{"agents", "get", "alpha"},
		{"agents", "register", "--input", `{"name":"alpha","description":"agent","requested_scopes":["public"]}`},
		{"agents", "update", "ag1", "--input", `{"requires_approval":true}`},
		{"keys", "list"},
		{"keys", "list-all"},
		{"keys", "create", "new-key"},
		{"keys", "revoke", "k1"},
		{"audit", "query"},
		{"audit", "scopes"},
		{"audit", "actors"},
		{"taxonomy", "list", "scopes"},
		{"taxonomy", "create", "scopes", "--input", `{"name":"demo-scope","description":"demo"}`},
		{"taxonomy", "update", "scopes", "t1", "--input", `{"description":"updated"}`},
		{"taxonomy", "archive", "scopes", "t1"},
		{"taxonomy", "activate", "scopes", "t1"},
		{"search", "semantic", "--query", "nebula"},
		{"import", "entities", "--input", `{"format":"json","items":[]}`},
		{"import", "context", "--input", `{"format":"json","items":[]}`},
		{"import", "relationships", "--input", `{"format":"json","items":[]}`},
		{"import", "jobs", "--input", `{"format":"json","items":[]}`},
		{"export", "entities"},
		{"export", "context"},
		{"export", "relationships"},
		{"export", "jobs"},
		{"export", "snapshot"},
	}

	for _, args := range commands {
		runAPISubcommandExpectError(t, "not logged in", args...)
	}
}

func TestAPICmdHealthErrorWithoutServer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	prevFactory := newDefaultClient
	t.Cleanup(func() {
		newDefaultClient = prevFactory
	})
	newDefaultClient = func(apiKey string, timeout ...time.Duration) *api.Client {
		if len(timeout) > 0 {
			return api.NewClient("http://127.0.0.1:1", apiKey, timeout[0])
		}
		return api.NewClient("http://127.0.0.1:1", apiKey)
	}
	runAPISubcommandExpectError(t, "health check", "health")
}

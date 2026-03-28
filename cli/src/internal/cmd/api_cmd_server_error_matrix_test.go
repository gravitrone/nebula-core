package cmd

import (
	"net/http"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAPICmdValidationGuardsMatrix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	runAPISubcommandExpectError(t, "missing --audit-id", "entities", "revert", "e1")
	runAPISubcommandExpectError(t, "missing --owner-id", "context", "link", "c1")
	runAPISubcommandExpectError(t, "missing --status", "jobs", "set-status", "j1")
	runAPISubcommandExpectError(t, "missing --query", "search", "semantic")
}

func TestAPICmdServerErrorMatrix(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	t.Cleanup(shutdown)

	cases := []struct {
		want string
		args []string
	}{
		{want: "health check", args: []string{"health"}},
		{want: "query entities", args: []string{"entities", "query", "--param", "limit=1"}},
		{want: "get entity", args: []string{"entities", "get", "e1"}},
		{want: "create entity", args: []string{"entities", "create", "--input", `{"scopes":["public"],"name":"Entity","type":"person","status":"active","tags":[]}`}},
		{want: "update entity", args: []string{"entities", "update", "e1", "--input", `{"status":"inactive"}`}},
		{want: "entity history", args: []string{"entities", "history", "e1", "--limit", "10"}},
		{want: "revert entity", args: []string{"entities", "revert", "e1", "--audit-id", "a1"}},
		{want: "bulk update tags", args: []string{"entities", "bulk-tags", "--input", `{"entity_ids":["e1"],"tags":["smoke"],"op":"add"}`}},
		{want: "bulk update scopes", args: []string{"entities", "bulk-scopes", "--input", `{"entity_ids":["e1"],"scopes":["public"],"op":"replace"}`}},
		{want: "query context", args: []string{"context", "query", "--param", "limit=1"}},
		{want: "get context", args: []string{"context", "get", "c1"}},
		{want: "create context", args: []string{"context", "create", "--input", `{"title":"Ctx","source_type":"note","content":"x","scopes":["public"],"tags":[]}`}},
		{want: "update context", args: []string{"context", "update", "c1", "--input", `{"title":"Updated"}`}},
		{want: "link context", args: []string{"context", "link", "c1", "--owner-type", "entity", "--owner-id", "e1"}},
		{want: "query relationships", args: []string{"relationships", "query", "--param", "limit=1"}},
		{want: "get relationships", args: []string{"relationships", "for-source", "entity", "e1"}},
		{want: "create relationship", args: []string{"relationships", "create", "--input", `{"source_type":"entity","source_id":"e1","target_type":"entity","target_id":"e2","relationship_type":"related-to"}`}},
		{want: "update relationship", args: []string{"relationships", "update", "r1", "--input", `{"notes":"phase: updated"}`}},
		{want: "query jobs", args: []string{"jobs", "query", "--param", "limit=1"}},
		{want: "get job", args: []string{"jobs", "get", "j1"}},
		{want: "create job", args: []string{"jobs", "create", "--input", `{"title":"Job","status":"pending","priority":"low"}`}},
		{want: "update job", args: []string{"jobs", "update", "j1", "--input", `{"status":"completed"}`}},
		{want: "set job status", args: []string{"jobs", "set-status", "j1", "--status", "completed"}},
		{want: "create subtask", args: []string{"jobs", "subtask", "j1", "--input", `{"title":"Subtask"}`}},
		{want: "query logs", args: []string{"logs", "query", "--param", "limit=1"}},
		{want: "get log", args: []string{"logs", "get", "l1"}},
		{want: "create log", args: []string{"logs", "create", "--input", `{"log_type":"event","value":{"text":"x"},"metadata":{}}`}},
		{want: "update log", args: []string{"logs", "update", "l1", "--input", `{"metadata":{"phase":"updated"}}`}},
		{want: "query files", args: []string{"files", "query", "--param", "limit=1"}},
		{want: "get file", args: []string{"files", "get", "f1"}},
		{want: "create file", args: []string{"files", "create", "--input", `{"filename":"file.txt","uri":"file:///tmp/file.txt","status":"active","tags":[],"metadata":{}}`}},
		{want: "update file", args: []string{"files", "update", "f1", "--input", `{"metadata":{"phase":"updated"}}`}},
		{want: "query protocols", args: []string{"protocols", "query", "--param", "limit=1"}},
		{want: "get protocol", args: []string{"protocols", "get", "proto"}},
		{want: "create protocol", args: []string{"protocols", "create", "--input", `{"name":"proto","title":"Proto","content":"# proto","status":"active","metadata":{}}`}},
		{want: "update protocol", args: []string{"protocols", "update", "proto", "--input", `{"metadata":{"phase":"updated"}}`}},
		{want: "list approvals", args: []string{"approvals", "pending"}},
		{want: "get approval", args: []string{"approvals", "get", "a1"}},
		{want: "approval diff", args: []string{"approvals", "diff", "a1"}},
		{want: "approve request", args: []string{"approvals", "approve", "a1"}},
		{want: "approve request", args: []string{"approvals", "approve", "a1", "--input", `{"review_notes":"ok"}`}},
		{want: "reject request", args: []string{"approvals", "reject", "a1", "--notes", "nope"}},
		{want: "list agents", args: []string{"agents", "list", "--status-category=active"}},
		{want: "get agent", args: []string{"agents", "get", "alpha"}},
		{want: "register agent", args: []string{"agents", "register", "--input", `{"name":"alpha","description":"agent","requested_scopes":["public"]}`}},
		{want: "update agent", args: []string{"agents", "update", "ag1", "--input", `{"requires_approval":true}`}},
		{want: "login", args: []string{"keys", "login", "alxx"}},
		{want: "list keys", args: []string{"keys", "list"}},
		{want: "list all keys", args: []string{"keys", "list-all"}},
		{want: "create key", args: []string{"keys", "create", "new-key"}},
		{want: "revoke key", args: []string{"keys", "revoke", "k1"}},
		{want: "query audit log", args: []string{"audit", "query", "--param", "limit=1"}},
		{want: "list audit scopes", args: []string{"audit", "scopes"}},
		{want: "list audit actors", args: []string{"audit", "actors"}},
		{want: "list taxonomy", args: []string{"taxonomy", "list", "scopes", "--limit=1"}},
		{want: "create taxonomy", args: []string{"taxonomy", "create", "scopes", "--input", `{"name":"demo-scope","description":"demo"}`}},
		{want: "update taxonomy", args: []string{"taxonomy", "update", "scopes", "t1", "--input", `{"description":"updated"}`}},
		{want: "archive taxonomy", args: []string{"taxonomy", "archive", "scopes", "t1"}},
		{want: "activate taxonomy", args: []string{"taxonomy", "activate", "scopes", "t1"}},
		{want: "semantic search", args: []string{"search", "semantic", "--query", "nebula", "--limit", "2"}},
		{want: "import entities", args: []string{"import", "entities", "--input", `{"format":"json","items":[]}`}},
		{want: "import context", args: []string{"import", "context", "--input", `{"format":"json","items":[]}`}},
		{want: "import relationships", args: []string{"import", "relationships", "--input", `{"format":"json","items":[]}`}},
		{want: "import jobs", args: []string{"import", "jobs", "--input", `{"format":"json","items":[]}`}},
		{want: "export entities", args: []string{"export", "entities", "--param", "format=json"}},
		{want: "export context", args: []string{"export", "context", "--param", "format=json"}},
		{want: "export relationships", args: []string{"export", "relationships", "--param", "format=json"}},
		{want: "export jobs", args: []string{"export", "jobs", "--param", "format=json"}},
		{want: "export snapshot", args: []string{"export", "snapshot", "--param", "format=json"}},
	}

	for _, tc := range cases {
		runAPISubcommandExpectError(t, tc.want, tc.args...)
	}
}

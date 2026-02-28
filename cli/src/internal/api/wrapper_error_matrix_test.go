package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientWrappersReturnErrorsOnNon2xx(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    "BAD_REQUEST",
				"message": "forced failure",
			},
		})
	})

	cases := []struct {
		name string
		call func(*Client) error
	}{
		{
			name: "agents/list",
			call: func(c *Client) error { _, err := c.ListAgents("active"); return err },
		},
		{
			name: "agents/get",
			call: func(c *Client) error { _, err := c.GetAgent("agent-1"); return err },
		},
		{
			name: "agents/update",
			call: func(c *Client) error { _, err := c.UpdateAgent("agent-1", UpdateAgentInput{}); return err },
		},
		{
			name: "approvals/pending-with-params",
			call: func(c *Client) error { _, err := c.GetPendingApprovalsWithParams(10, 0); return err },
		},
		{
			name: "approvals/get",
			call: func(c *Client) error { _, err := c.GetApproval("ap-1"); return err },
		},
		{
			name: "approvals/reject",
			call: func(c *Client) error { _, err := c.RejectRequest("ap-1", "no"); return err },
		},
		{
			name: "approvals/diff",
			call: func(c *Client) error { _, err := c.GetApprovalDiff("ap-1"); return err },
		},
		{
			name: "context/get",
			call: func(c *Client) error { _, err := c.GetContext("ctx-1"); return err },
		},
		{
			name: "context/query",
			call: func(c *Client) error { _, err := c.QueryContext(QueryParams{"limit": "1"}); return err },
		},
		{
			name: "context/update",
			call: func(c *Client) error { _, err := c.UpdateContext("ctx-1", UpdateContextInput{}); return err },
		},
		{
			name: "entities/update",
			call: func(c *Client) error { _, err := c.UpdateEntity("ent-1", UpdateEntityInput{}); return err },
		},
		{
			name: "entities/search-by-metadata",
			call: func(c *Client) error { _, err := c.SearchEntities(map[string]any{"k": "v"}); return err },
		},
		{
			name: "entities/history",
			call: func(c *Client) error { _, err := c.GetEntityHistory("ent-1", 10, 0); return err },
		},
		{
			name: "entities/revert",
			call: func(c *Client) error { _, err := c.RevertEntity("ent-1", "audit-1"); return err },
		},
		{
			name: "entities/bulk-tags",
			call: func(c *Client) error {
				_, err := c.BulkUpdateEntityTags(BulkUpdateEntityTagsInput{
					EntityIDs: []string{"ent-1"},
					Tags:      []string{"t"},
					Op:        "add",
				})
				return err
			},
		},
		{
			name: "entities/bulk-scopes",
			call: func(c *Client) error {
				_, err := c.BulkUpdateEntityScopes(BulkUpdateEntityScopesInput{
					EntityIDs: []string{"ent-1"},
					Scopes:    []string{"public"},
					Op:        "add",
				})
				return err
			},
		},
		{
			name: "exports/context-items",
			call: func(c *Client) error { _, err := c.ExportContextItems(QueryParams{"format": "json"}); return err },
		},
		{
			name: "exports/relationships",
			call: func(c *Client) error { _, err := c.ExportRelationships(QueryParams{"format": "json"}); return err },
		},
		{
			name: "exports/jobs",
			call: func(c *Client) error { _, err := c.ExportJobs(QueryParams{"format": "json"}); return err },
		},
		{
			name: "exports/context-snapshot",
			call: func(c *Client) error { _, err := c.ExportContext(QueryParams{"format": "json"}); return err },
		},
		{
			name: "files/create",
			call: func(c *Client) error { _, err := c.CreateFile(CreateFileInput{}); return err },
		},
		{
			name: "files/get",
			call: func(c *Client) error { _, err := c.GetFile("file-1"); return err },
		},
		{
			name: "files/query",
			call: func(c *Client) error { _, err := c.QueryFiles(QueryParams{"limit": "1"}); return err },
		},
		{
			name: "files/update",
			call: func(c *Client) error { _, err := c.UpdateFile("file-1", UpdateFileInput{}); return err },
		},
		{
			name: "imports/entities",
			call: func(c *Client) error { _, err := c.ImportEntities(BulkImportRequest{Format: "json", Data: "[]"}); return err },
		},
		{
			name: "imports/context",
			call: func(c *Client) error { _, err := c.ImportContext(BulkImportRequest{Format: "json", Data: "[]"}); return err },
		},
		{
			name: "imports/relationships",
			call: func(c *Client) error {
				_, err := c.ImportRelationships(BulkImportRequest{Format: "json", Data: "[]"})
				return err
			},
		},
		{
			name: "imports/jobs",
			call: func(c *Client) error { _, err := c.ImportJobs(BulkImportRequest{Format: "json", Data: "[]"}); return err },
		},
		{
			name: "jobs/create",
			call: func(c *Client) error { _, err := c.CreateJob(CreateJobInput{}); return err },
		},
		{
			name: "jobs/get",
			call: func(c *Client) error { _, err := c.GetJob("job-1"); return err },
		},
		{
			name: "jobs/query",
			call: func(c *Client) error { _, err := c.QueryJobs(QueryParams{"limit": "1"}); return err },
		},
		{
			name: "jobs/update",
			call: func(c *Client) error { _, err := c.UpdateJob("job-1", UpdateJobInput{}); return err },
		},
		{
			name: "jobs/create-subtask",
			call: func(c *Client) error { _, err := c.CreateSubtask("job-1", map[string]string{"title": "child"}); return err },
		},
		{
			name: "keys/list",
			call: func(c *Client) error { _, err := c.ListKeys(); return err },
		},
		{
			name: "keys/list-all",
			call: func(c *Client) error { _, err := c.ListAllKeys(); return err },
		},
		{
			name: "logs/create",
			call: func(c *Client) error { _, err := c.CreateLog(CreateLogInput{}); return err },
		},
		{
			name: "logs/get",
			call: func(c *Client) error { _, err := c.GetLog("log-1"); return err },
		},
		{
			name: "logs/query",
			call: func(c *Client) error { _, err := c.QueryLogs(QueryParams{"limit": "1"}); return err },
		},
		{
			name: "logs/update",
			call: func(c *Client) error { _, err := c.UpdateLog("log-1", UpdateLogInput{}); return err },
		},
		{
			name: "protocols/create",
			call: func(c *Client) error { _, err := c.CreateProtocol(CreateProtocolInput{}); return err },
		},
		{
			name: "protocols/get",
			call: func(c *Client) error { _, err := c.GetProtocol("proto-1"); return err },
		},
		{
			name: "protocols/query",
			call: func(c *Client) error { _, err := c.QueryProtocols(QueryParams{"limit": "1"}); return err },
		},
		{
			name: "protocols/update",
			call: func(c *Client) error { _, err := c.UpdateProtocol("proto-1", UpdateProtocolInput{}); return err },
		},
		{
			name: "relationships/get",
			call: func(c *Client) error { _, err := c.GetRelationships("entity", "ent-1"); return err },
		},
		{
			name: "relationships/query",
			call: func(c *Client) error { _, err := c.QueryRelationships(QueryParams{"limit": "1"}); return err },
		},
		{
			name: "relationships/update",
			call: func(c *Client) error { _, err := c.UpdateRelationship("rel-1", UpdateRelationshipInput{}); return err },
		},
		{
			name: "taxonomy/create",
			call: func(c *Client) error { _, err := c.CreateTaxonomy("status", CreateTaxonomyInput{}); return err },
		},
		{
			name: "taxonomy/list",
			call: func(c *Client) error { _, err := c.ListTaxonomy("status", false, "", 0, 0); return err },
		},
		{
			name: "taxonomy/update",
			call: func(c *Client) error { _, err := c.UpdateTaxonomy("status", "id-1", UpdateTaxonomyInput{}); return err },
		},
		{
			name: "taxonomy/archive",
			call: func(c *Client) error { _, err := c.ArchiveTaxonomy("status", "id-1"); return err },
		},
		{
			name: "taxonomy/activate",
			call: func(c *Client) error { _, err := c.ActivateTaxonomy("status", "id-1"); return err },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(client)
			require.Error(t, err)
			assert.ErrorContains(t, err, "BAD_REQUEST")
		})
	}
}

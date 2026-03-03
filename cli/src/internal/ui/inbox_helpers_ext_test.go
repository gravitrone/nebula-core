package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type inboxTestStringer string

func (s inboxTestStringer) String() string { return string(s) }

func TestFormatAnyInlineAndFormatAnyMatrix(t *testing.T) {
	assert.Equal(t, "", formatAnyInline(nil))
	assert.Equal(t, "", formatAnyInline("   "))
	assert.Contains(t, formatAnyInline(map[string]any{"k": "v"}), "\"k\":\"v\"")
	assert.Equal(t, "", formatAnyInline("<nil>"))
	assert.Equal(t, "123", formatAnyInline(123))
	assert.Equal(t, "", formatAnyInline(inboxTestStringer("--")))
	assert.Equal(t, "", formatAnyInline(inboxTestStringer("-")))
	assert.Contains(t, formatAnyInline(map[string]any{"bad": func() {}}), "map[bad:")

	assert.Equal(t, "None", formatAny(nil))
	assert.Equal(t, "None", formatAny("  "))
	assert.Equal(t, "a, b", formatAny([]string{"a", "b"}))
	assert.Equal(t, "None", formatAny([]any{nil, "", "  "}))
	assert.Equal(t, "None", formatAny("-"))
	assert.Equal(t, "None", formatAny("--"))
	assert.Equal(t, "None", formatAny("<nil>"))
	assert.Equal(t, "1, two", formatAny([]any{1, "two"}))
	assert.Equal(t, "None", formatAny([]string{}))
	assert.Equal(t, "None", formatAny(map[string]any{}))
	assert.Contains(t, formatAny(map[string]any{"k": "v"}), "k")
}

func TestDetailLabelAndHumanizeApprovalType(t *testing.T) {
	assert.Equal(t, "Actor ID", detailLabel("actor_id"))
	assert.Equal(t, "API URL", detailLabel("api_url"))
	assert.Equal(t, "MCP ID", detailLabel("mcp_id"))
	assert.Equal(t, "", detailLabel("   "))
	assert.Equal(t, "__", detailLabel("__"))

	assert.Equal(t, "Create Entity", humanizeApprovalType("create_entity"))
	assert.Equal(t, "", humanizeApprovalType(""))
}

func TestApprovalTitleFallbackMatrix(t *testing.T) {
	approval := api.Approval{ChangeDetails: api.JSONMap{"name": "Alpha"}}
	assert.Equal(t, "Alpha", approvalTitle(approval))

	approval = api.Approval{ChangeDetails: api.JSONMap{"title": "Task A"}}
	assert.Equal(t, "Task A", approvalTitle(approval))

	approval = api.Approval{ChangeDetails: api.JSONMap{"entity_name": "Entity A"}}
	assert.Equal(t, "Entity A", approvalTitle(approval))

	approval = api.Approval{
		RequestType:   "create_relationship",
		ChangeDetails: api.JSONMap{"relationship_type": "owns", "source_name": "A", "target_name": "B"},
	}
	assert.Equal(t, "owns (A -> B)", approvalTitle(approval))

	approval = api.Approval{
		RequestType:   "bulk_update_entity_tags",
		ChangeDetails: api.JSONMap{"entity_names": []any{"A", "B", "C"}},
	}
	assert.Contains(t, approvalTitle(approval), "Bulk Update Entity Tags")
	assert.Contains(t, approvalTitle(approval), "A, B +1")

	approval = api.Approval{RequestType: "create_log", ChangeDetails: api.JSONMap{"log_type": "note"}}
	assert.Equal(t, "log: note", approvalTitle(approval))

	approval = api.Approval{RequestType: "update_entity", ChangeDetails: api.JSONMap{}}
	assert.Equal(t, "Update Entity", approvalTitle(approval))
}

func TestApprovalRequestedByAndWhoLabelMatrix(t *testing.T) {
	approval := api.Approval{RequestedByName: "alxx", RequestedBy: "entity-123456789"}
	assert.Contains(t, approvalRequestedBy(approval), "alxx")
	assert.Contains(t, approvalRequestedBy(approval), shortID("entity-123456789"))
	assert.Equal(t, "alxx", approvalWhoLabel(approval))

	approval = api.Approval{AgentName: "agent-bot"}
	assert.Equal(t, "agent-bot", approvalRequestedBy(approval))
	assert.Equal(t, "agent-bot", approvalWhoLabel(approval))

	approval = api.Approval{RequestedBy: "entity-123456789"}
	assert.Equal(t, shortID("entity-123456789"), approvalRequestedBy(approval))
	assert.Equal(t, shortID("entity-123456789"), approvalWhoLabel(approval))

	approval = api.Approval{}
	assert.Equal(t, "None", approvalRequestedBy(approval))
	assert.Equal(t, "system", approvalWhoLabel(approval))
}

func TestApprovalEndpointLabelAndScopeParsers(t *testing.T) {
	details := api.JSONMap{
		"source_name": "Source A",
		"source_id":   "src-123",
		"source_type": "entity",
	}
	assert.Equal(t, "Source A", approvalEndpointLabel(details, "source"))

	details = api.JSONMap{"target_id": "tgt-123456789"}
	assert.Equal(t, shortID("tgt-123456789"), approvalEndpointLabel(details, "target"))

	details = api.JSONMap{"target_type": "file"}
	assert.Equal(t, "file", approvalEndpointLabel(details, "target"))

	assert.Equal(t, []string{"public", "private"}, parseScopesCSV("public, private"))
	assert.Equal(t, []string{"public", "private"}, parseStringList([]any{"public", "private"}))
	assert.Nil(t, parseStringList(123))
}

func TestRequestedScopesAndRequiresApprovalMatrix(t *testing.T) {
	approval := api.Approval{ChangeDetails: api.JSONMap{"requested_scopes": []any{"private", "admin"}}}
	assert.Equal(t, []string{"private", "admin"}, requestedScopesFromApproval(approval))

	approval = api.Approval{ChangeDetails: api.JSONMap{}}
	assert.Equal(t, []string{"public"}, requestedScopesFromApproval(approval))
	assert.True(t, requestedRequiresApprovalFromApproval(approval))

	approval = api.Approval{ChangeDetails: api.JSONMap{"requested_requires_approval": false}}
	assert.False(t, requestedRequiresApprovalFromApproval(approval))
}

func TestInboxSelectionAndSummaryHelpers(t *testing.T) {
	model := NewInboxModel(nil)
	model.items = []api.Approval{{ID: "ap-1"}, {ID: "ap-2"}}
	model.filtered = []int{0, 1}
	model.list.SetItems([]string{"a", "b"})
	model.selected = map[string]bool{"ap-2": true}

	ids := model.selectedIDs()
	assert.Equal(t, []string{"ap-2"}, ids)
	assert.Equal(t, 1, model.selectedCount())

	item, ok := model.findApprovalByID("ap-1")
	require.True(t, ok)
	assert.Equal(t, "ap-1", item.ID)
	_, ok = model.findApprovalByID("missing")
	assert.False(t, ok)

	rows := model.approveSummaryRows()
	require.NotEmpty(t, rows)
	assert.Equal(t, components.TableRow{Label: "Action", Value: "approve"}, rows[0])
}

func TestApproveDiffRowsAndDiffValueMatrix(t *testing.T) {
	model := NewInboxModel(nil)
	model.detail = &api.Approval{
		ChangeDetails: api.JSONMap{
			"source_name": "Source A",
			"changes": map[string]any{
				"source_id": map[string]any{"from": "src-1", "to": "src-2"},
				"status":    map[string]any{"from": "pending", "to": "approved"},
				"same":      map[string]any{"from": "x", "to": "x"},
			},
		},
	}

	rows := model.approveDiffRows()
	require.Len(t, rows, 1)
	assert.Equal(t, "status", rows[0].Label)

	details := api.JSONMap{"entity_name": "Entity A", "entity_names": []any{"A", "B"}}
	assert.Equal(t, "Entity A", approvalDiffValue(details, "entity_id", "ent-1"))
	assert.Equal(t, "A, B", approvalDiffValue(details, "entity_ids", []any{"ent-1", "ent-2"}))
	assert.Equal(t, "None", approvalDiffValue(details, "status", nil))
}

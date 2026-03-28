package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApprovalTitleAdditionalFallbackBranches(t *testing.T) {
	t.Run("relationship type without endpoints returns type", func(t *testing.T) {
		approval := api.Approval{
			RequestType:   "update_relationship",
			ChangeDetails: `{"relationship_type":"depends-on"}`,
		}
		assert.Equal(t, "depends-on", approvalTitle(approval))
	})

	t.Run("bulk update with one entity name", func(t *testing.T) {
		approval := api.Approval{
			RequestType:   "bulk_update_entity_scopes",
			ChangeDetails: `{"entity_names":["Alpha"]}`,
		}
		title := approvalTitle(approval)
		assert.Contains(t, title, "Bulk Update Entity Scopes")
		assert.Contains(t, title, "Alpha")
	})

	t.Run("bulk update falls back to entity id and entity count", func(t *testing.T) {
		approval := api.Approval{
			RequestType:   "bulk_update_entity_tags",
			ChangeDetails: `{"entity_ids":["ent-1234567890"]}`,
		}
		title := approvalTitle(approval)
		assert.Contains(t, title, "Bulk Update Entity Tags")
		assert.Contains(t, title, shortID("ent-1234567890"))

		approval = api.Approval{
			RequestType:   "bulk_update_entity_tags",
			ChangeDetails: `{"entity_ids":["ent-1","ent-2","ent-3"]}`,
		}
		title = approvalTitle(approval)
		assert.Contains(t, title, "3 entities")
	})

	t.Run("log type fallback and empty request fallback", func(t *testing.T) {
		approval := api.Approval{RequestType: "create_log", ChangeDetails: "{}"}
		assert.Equal(t, "Create Log", approvalTitle(approval))

		approval = api.Approval{RequestType: "   ", ChangeDetails: "{}"}
		assert.Equal(t, "", approvalTitle(approval))
	})
}

func TestRequestedRequiresApprovalAndParseStringListAdditionalBranches(t *testing.T) {
	approval := api.Approval{
		ChangeDetails: `{"requested_requires_approval":"false"}`,
	}
	assert.True(t, requestedRequiresApprovalFromApproval(approval))

	assert.Equal(
		t,
		[]string{"public", "private", "admin"},
		parseStringList("public, private,admin"),
	)
}

func TestInboxToggleSelectAllBranchMatrix(t *testing.T) {
	model := NewInboxModel(nil)

	model.toggleSelectAll()
	assert.Empty(t, model.selected)

	model.items = []api.Approval{
		{ID: "ap-1"},
		{ID: "ap-2"},
	}
	model.filtered = []int{0, 99, 1}
	model.selected = map[string]bool{}

	model.toggleSelectAll()
	assert.True(t, model.selected["ap-1"])
	assert.True(t, model.selected["ap-2"])

	model.toggleSelectAll()
	assert.Empty(t, model.selected)
}

func TestApproveDiffRowsGuardAndEndpointMappingBranches(t *testing.T) {
	model := NewInboxModel(nil)
	assert.Nil(t, model.approveDiffRows())

	model.detail = &api.Approval{ChangeDetails: "{}"}
	assert.Nil(t, model.approveDiffRows())

	model.detail = &api.Approval{ChangeDetails: `{"changes":"bad"}`}
	assert.Nil(t, model.approveDiffRows())

	model.detail = &api.Approval{
		ChangeDetails: `{"source_name":"Source A","target_name":"Target B","changes":{"bad":"nope","unchanged":{"from":"x","to":"x"},"status":{"from":"pending","to":"approved"}}}`,
	}

	rows := model.approveDiffRows()
	require.Len(t, rows, 1)
	assert.Equal(t, "status", strings.ToLower(rows[0].Label))

	details := parseApprovalChangeDetails(model.detail.ChangeDetails)
	assert.Equal(t, "Source A", approvalDiffValue(details, "source_id", "src-1"))
	assert.Equal(t, "Target B", approvalDiffValue(details, "target_id", "tgt-1"))
}

package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

// TestInboxDetailViewRendersSummaryDiffAndNestedObjects handles test inbox detail view renders summary diff and nested objects.
func TestInboxDetailViewRendersSummaryDiffAndNestedObjects(t *testing.T) {
	now := time.Now()
	jobID := "job-1"
	notes := "review note"

	model := NewInboxModel(nil)
	model.width = 90
	model.loading = false
	model.detail = &api.Approval{
		ID:          "ap-1",
		RequestType: "update_entity",
		Status:      "pending",
		RequestedBy: "agent:test",
		AgentName:   "test-agent",
		JobID:       &jobID,
		Notes:       &notes,
		CreatedAt:   now,
		ChangeDetails: `{"name":"Alpha","tags":["one","two"],"metadata":{"role":"founder","yr":2026},"changes":{"status":{"from":"active","to":"archived"},"metadata":{"from":{"role":"builder"},"to":{"role":"founder"}}}}`,
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "update_entity")
	assert.Contains(t, out, "pending")
	assert.Contains(t, out, "Review Notes")
	assert.Contains(t, out, "review note")

	// Summary table.
	assert.Contains(t, out, "Name")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "one, two")

	// Diff table.
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "active")
	assert.Contains(t, out, "archived")

	// Metadata renders in a dedicated table above changes.
	assert.Contains(t, out, "Metadata")
	assert.Contains(t, out, "role")
	assert.Contains(t, out, "founder")
}

// TestInboxFilterInputAppliesAndClears handles test inbox filter input applies and clears.
func TestInboxFilterInputAppliesAndClears(t *testing.T) {
	now := time.Now()
	model := NewInboxModel(nil)
	model.width = 80

	model, _ = model.Update(approvalsLoadedMsg{
		items: []api.Approval{
			{
				ID:          "ap-1",
				Status:      "pending",
				RequestType: "create_entity",
				AgentName:   "OpenAI",
				RequestedBy: "agent:openai",
				ChangeDetails: `{"name":"Alpha"}`,
				CreatedAt: now,
			},
			{
				ID:          "ap-2",
				Status:      "pending",
				RequestType: "create_entity",
				AgentName:   "Anthropic",
				RequestedBy: "agent:anthropic",
				ChangeDetails: `{"name":"Beta"}`,
				CreatedAt: now,
			},
		},
	})

	// Start filtering and type a filter.
	model.filtering = true
	for _, r := range "agent:openai" {
		model, _ = model.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	assert.Equal(t, "agent:openai", model.filterInput.Value())
	assert.Len(t, model.filtered, 1)

	// Enter applies and exits filtering.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, model.filtering)
	assert.Len(t, model.filtered, 1)

	// Esc clears filter and resets.
	model.filtering = true
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, model.filtering)
	assert.Equal(t, "", model.filterInput.Value())
	assert.Len(t, model.filtered, 2)
}

// TestInboxDetailUsesRelationshipEndpointNames handles test inbox detail uses relationship endpoint names.
func TestInboxDetailUsesRelationshipEndpointNames(t *testing.T) {
	now := time.Now()
	sourceID := "11111111-1111-1111-1111-111111111111"
	targetID := "22222222-2222-2222-2222-222222222222"

	model := NewInboxModel(nil)
	model.width = 100
	model.loading = false
	model.detail = &api.Approval{
		ID:          "ap-rel-1",
		RequestType: "update_relationship",
		Status:      "pending",
		RequestedBy: "agent:alpha",
		AgentName:   "alpha",
		CreatedAt:   now,
		ChangeDetails: `{"relationship_type":"owns","source_type":"entity","source_id":"` + sourceID + `","source_name":"Alpha Entity","target_type":"entity","target_id":"` + targetID + `","target_name":"Beta Entity","changes":{"source_id":{"from":"` + sourceID + `","to":"` + targetID + `"},"target_id":{"from":"` + targetID + `","to":"` + sourceID + `"}}}`,
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Alpha Entity")
	assert.Contains(t, out, "Beta Entity")
	assert.NotContains(t, out, shortID(sourceID))
	assert.NotContains(t, out, shortID(targetID))
}

// TestInboxBulkScopePreviewUsesEntityNames handles test inbox bulk scope preview uses entity names.
func TestInboxBulkScopePreviewUsesEntityNames(t *testing.T) {
	now := time.Now()
	entityID := "33333333-3333-3333-3333-333333333333"

	model := NewInboxModel(nil)
	model.width = 100
	model.loading = false
	model.items = []api.Approval{
		{
			ID:              "ap-bulk-1",
			RequestType:     "bulk_update_entity_scopes",
			Status:          "pending",
			RequestedBy:     "44444444-4444-4444-4444-444444444444",
			RequestedByName: "agent-alpha",
			AgentName:       "agent-alpha",
			CreatedAt:       now,
			ChangeDetails: `{"entity_ids":["` + entityID + `"],"entity_names":["Bro"],"scopes":["public","admin"],"op":"add"}`,
		},
	}
	model.applyFilter(true)

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Bulk Update Entity Scopes (Bro)")
	assert.Contains(t, out, "Bro")
	assert.NotContains(t, out, shortID(entityID))
}

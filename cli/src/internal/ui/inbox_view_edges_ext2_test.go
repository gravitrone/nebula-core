package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestInboxViewTinyWidthFilterAndUnsyncedVisibleRows(t *testing.T) {
	model := NewInboxModel(nil)
	model.width = 24
	model.items = []api.Approval{
		{
			ID:              "ap-1",
			Status:          "pending",
			RequestType:     "create_entity",
			AgentName:       "alpha-agent",
			RequestedBy:     "user-1",
			RequestedByName: "alpha-agent",
			CreatedAt:       time.Now().UTC(),
			ChangeDetails:   `{"name":"Alpha"}`,
		},
	}
	model.applyFilter(true)
	model.filterInput.SetValue("agent:alpha")

	out := components.SanitizeText(model.View())

	assert.Contains(t, out, "pending")
	assert.Contains(t, out, "filter:")
	assert.Contains(t, out, "agent:alpha")
}

func TestInboxViewWideLayoutUsesSideBySidePreview(t *testing.T) {
	model := NewInboxModel(nil)
	model.width = 220
	model.items = []api.Approval{
		{
			ID:              "ap-wide-1",
			Status:          "pending",
			RequestType:     "create_entity",
			AgentName:       "agent-wide",
			RequestedBy:     "user-wide",
			RequestedByName: "agent-wide",
			CreatedAt:       time.Now().UTC(),
			ChangeDetails: `{"name":"Wide Entity","scopes":["public"]}`,
		},
	}
	model.applyFilter(true)

	out := components.SanitizeText(model.View())

	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Wide Entity")
	assert.Contains(t, out, "public")
}

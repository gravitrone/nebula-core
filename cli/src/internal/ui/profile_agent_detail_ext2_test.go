package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestProfileRenderAgentDetailFallbackBranches(t *testing.T) {
	model := NewProfileModel(nil, nil)

	assert.Equal(t, "", model.renderAgentDetail())

	blank := "   "
	model.width = 96
	model.agentDetail = &api.Agent{
		ID:               "agent-2",
		Name:             "Bravo",
		Status:           "inactive",
		RequiresApproval: true,
		Description:      &blank,
		CreatedAt:        time.Time{},
		UpdatedAt:        time.Time{},
	}

	out := stripANSI(model.renderAgentDetail())
	assert.Contains(t, out, "Trust")
	assert.Contains(t, out, "untrusted")
	assert.Contains(t, out, "Scopes")
	assert.Contains(t, out, "Capabilities")
	assert.Contains(t, out, "None")
	assert.NotContains(t, out, "Description")
}

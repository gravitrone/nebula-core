package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileUpdateRoutesPendingLimitInputFromMainUpdate(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.editPendingLimit = true
	model.pendingLimitInput.SetValue("")

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.editPendingLimit)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestProfileUpdateNavigationAndTaxonomyAdditionalBranches(t *testing.T) {
	model := NewProfileModel(nil, nil)
	model.section = 1

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.section)
	assert.True(t, updated.sectionFocus)

	updated.sectionFocus = false
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.section)
	assert.True(t, updated.sectionFocus)

	updated.section = 2
	updated.sectionFocus = false
	updated.taxList.SetRows([]table.Row{{"public"}, {"private"}})
	updated.taxList.SetCursor(0)

	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, updated.taxList.Cursor())

	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, updated.taxList.Cursor())

	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.True(t, updated.sectionFocus)

	updated.section = 0
	updated.sectionFocus = false
	updated.keyList.SetRows([]table.Row{{"k1"}, {"k2"}})
	updated.keyList.SetCursor(1)
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.Equal(t, 0, updated.keyList.Cursor())

	updated.config = nil
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	require.Nil(t, cmd)
	assert.True(t, updated.editPendingLimit)
	assert.Equal(t, "500", updated.pendingLimitInput.Value())

	desc := "scope desc"
	updated.section = 2
	updated.sectionFocus = false
	updated.editPendingLimit = false
	updated.taxSearch = "pub"
	updated.taxItems = []api.TaxonomyEntry{
		{ID: "scope-1", Name: "public", Description: &desc},
	}
	updated.taxList.SetRows([]table.Row{{"public"}})
	updated.taxList.SetCursor(0)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, taxPromptEditName, updated.taxPromptMode)
	assert.Equal(t, "scope-1", updated.taxEditID)
	assert.Equal(t, "scope desc", updated.taxPendingDesc)

	updated.taxPromptMode = taxPromptNone
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, taxPromptEditName, updated.taxPromptMode)
	assert.Equal(t, "scope-1", updated.taxEditID)
	assert.Equal(t, "scope desc", updated.taxPendingDesc)

	updated.taxPromptMode = taxPromptNone
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	require.Nil(t, cmd)
	assert.Equal(t, taxPromptFilter, updated.taxPromptMode)
	assert.Equal(t, "pub", updated.taxPromptInput.Value())
}

func TestProfileViewAndRenderKeysAdditionalBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_demo", PendingLimit: 50})
	model.width = 180

	now := time.Now()
	model.agentDetail = &api.Agent{
		ID:        "agent-1",
		Name:      "Alpha",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	model.agentDetail = nil

	agentName := "worker"
	model.keys = []api.APIKey{
		{
			ID:        "k1",
			KeyPrefix: "",
			Name:      "",
			AgentName: &agentName,
			CreatedAt: now,
		},
	}
	model.keyList.SetRows([]table.Row{{"row-1"}, {"row-2"}})
	model.keyList.SetCursor(0)
	model.section = 0
	model.sectionFocus = true

	wide := stripANSI(model.renderKeys())
	assert.Contains(t, wide, "1 keys")
	assert.Contains(t, wide, "agent:worker")

	model.width = 96
	stacked := stripANSI(model.renderKeys())
	_ = stacked
}

func TestProfileRenderKeyAndAgentPreviewAdditionalBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})

	now := time.Now()
	lastUsed := now.Add(-time.Hour)
	expires := now.Add(24 * time.Hour)
	agentName := "bot"

	keyPreview := stripANSI(model.renderKeyPreview(api.APIKey{
		KeyPrefix:  "nbl_preview",
		Name:       "",
		AgentName:  &agentName,
		CreatedAt:  now,
		LastUsedAt: &lastUsed,
		ExpiresAt:  &expires,
	}, 32))
	assert.Contains(t, keyPreview, "Selected")
	assert.Contains(t, keyPreview, "Owner")
	assert.Contains(t, keyPreview, "agent:bot")
	assert.Contains(t, keyPreview, "Last Used")
	assert.Contains(t, keyPreview, "Expires")

	desc := "handles approvals"
	model.width = 180
	model.section = 1
	model.sectionFocus = true
	model.agents = []api.Agent{
		{
			ID:               "a1",
			Name:             "",
			Status:           "",
			RequiresApproval: false,
			Scopes:           nil,
			Capabilities:     nil,
			Description:      &desc,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
	model.agentList.SetRows([]table.Row{{"row-1"}, {"row-2"}})
	model.agentList.SetCursor(0)

	agentsWide := stripANSI(model.renderAgents())
	assert.Contains(t, agentsWide, "trusted")

	model.width = 96
	agentsStacked := stripANSI(model.renderAgents())
	_ = agentsStacked

	agentPreview := stripANSI(model.renderAgentPreview(api.Agent{
		Name:             "",
		Status:           "",
		RequiresApproval: false,
		Scopes:           []string{"public", "admin"},
		Capabilities:     []string{"read", "write"},
		Description:      &desc,
	}, 34))
	assert.Contains(t, agentPreview, "Scopes")
	assert.Contains(t, agentPreview, "Caps")
	assert.Contains(t, agentPreview, "Desc")
}

func TestProfileFormatKeyLineOwnerBranches(t *testing.T) {
	now := time.Now()
	entity := "alice"
	agent := "worker"

	entityLine := stripANSI(formatKeyLine(api.APIKey{
		KeyPrefix:  "nbl_ent",
		Name:       "EntityKey",
		EntityName: &entity,
		CreatedAt:  now,
	}))
	assert.Contains(t, entityLine, "alice")

	agentLine := stripANSI(formatKeyLine(api.APIKey{
		KeyPrefix: "nbl_agent",
		Name:      "AgentKey",
		AgentName: &agent,
		CreatedAt: now,
	}))
	assert.Contains(t, agentLine, "agent: worker")
}

package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileUpdateDownAndPendingLimitConfigBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{
		Username:     "alxx",
		APIKey:       "nbl_demo",
		PendingLimit: 777,
	})

	model.section = 1
	model.sectionFocus = false
	model.agentList.SetRows([]table.Row{{"a1"}, {"a2"}})
	model.agentList.SetCursor(0)

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.agentList.Cursor())

	updated.section = 0
	updated.sectionFocus = false
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: 'p', Text: "p"})
	require.Nil(t, cmd)
	assert.True(t, updated.editPendingLimit)
	assert.Equal(t, "777", updated.pendingLimitBuf)
}

func TestProfileViewSectionFocusTabBranches(t *testing.T) {
	now := time.Now().UTC()
	entityName := "owner-entity"

	model := NewProfileModel(nil, &config.Config{
		Username:     "alxx",
		APIKey:       "nbl_demo",
		PendingLimit: 500,
	})
	model.width = 120
	model.keys = []api.APIKey{{
		ID:         "k1",
		KeyPrefix:  "nbl",
		Name:       "alpha",
		EntityName: &entityName,
		CreatedAt:  now,
	}}
	model.keyList.SetRows([]table.Row{{"k1"}})
	model.keyList.SetCursor(0)
	model.agents = []api.Agent{{
		ID:               "a1",
		Name:             "agent-a",
		Status:           "active",
		RequiresApproval: false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	model.agentList.SetRows([]table.Row{{"a1"}})
	model.agentList.SetCursor(0)
	model.taxItems = []api.TaxonomyEntry{{ID: "scope-1", Name: "public"}}
	model.taxList.SetRows([]table.Row{{"public"}})
	model.taxList.SetCursor(0)

	model.section = 0
	model.sectionFocus = true
	assert.Contains(t, stripANSI(model.View()), "API Keys")

	model.section = 1
	model.sectionFocus = true
	assert.Contains(t, stripANSI(model.View()), "Agents")

	model.section = 2
	model.sectionFocus = true
	assert.Contains(t, stripANSI(model.View()), "Taxonomy")
}

func TestProfileHandleInputsSaveErrorBranches(t *testing.T) {
	tmp := t.TempDir()
	homeFile := filepath.Join(tmp, "homefile")
	require.NoError(t, os.WriteFile(homeFile, []byte("x"), 0o600))
	t.Setenv("HOME", homeFile)

	cfg := &config.Config{
		Username:     "alxx",
		APIKey:       "nbl_old",
		PendingLimit: 500,
	}
	model := NewProfileModel(nil, cfg)

	model.editAPIKey = true
	model.apiKeyBuf = "nbl_new"
	updated, cmd := model.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.editAPIKey)
	msg := cmd()
	errMsgVal, ok := msg.(errMsg)
	require.True(t, ok)
	assert.Error(t, errMsgVal.err)

	model.editPendingLimit = true
	model.pendingLimitBuf = "900"
	updated, cmd = model.handlePendingLimitInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.editPendingLimit)
	msg = cmd()
	errMsgVal, ok = msg.(errMsg)
	require.True(t, ok)
	assert.Error(t, errMsgVal.err)
}

func TestProfileRenderKeysAndAgentsNarrowFallbackBranches(t *testing.T) {
	now := time.Now().UTC()
	entityName := "owner-entity"

	model := NewProfileModel(nil, &config.Config{
		Username:     "alxx",
		APIKey:       "nbl_demo",
		PendingLimit: 500,
	})
	model.width = 26
	model.section = 0
	model.keys = []api.APIKey{{
		ID:         "k1",
		KeyPrefix:  "nblprefix",
		Name:       "",
		EntityName: &entityName,
		CreatedAt:  now,
	}}
	model.keyList.SetRows([]table.Row{{"k1"}})
	model.keyList.SetCursor(0)

	keysOut := stripANSI(model.renderKeys())
	_ = keysOut

	model.section = 1
	model.agents = []api.Agent{{
		ID:               "a1",
		Name:             "",
		Status:           "",
		RequiresApproval: false,
		Scopes:           []string{"public"},
		CreatedAt:        now,
		UpdatedAt:        now,
	}}
	model.agentList.SetRows([]table.Row{{"a1"}})
	model.agentList.SetCursor(0)

	agentsOut := stripANSI(model.renderAgents())
	_ = agentsOut
}

func TestProfileRenderKeyPreviewEntityOwnerBranch(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	entityName := "owner-entity"
	now := time.Now().UTC()

	out := stripANSI(model.renderKeyPreview(api.APIKey{
		KeyPrefix:  "nbl_key",
		Name:       "work key",
		EntityName: &entityName,
		CreatedAt:  now,
	}, 40))
	assert.Contains(t, out, "Owner")
	assert.Contains(t, out, "owner-entity")
}

package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileInitReturnsBatchCommand(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	cmd := model.Init()
	require.NotNil(t, cmd)
}

func TestProfileUpdateMessageMatrixBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_key"})
	model.loading = true
	model.creating = true
	model.createInput.SetValue("draft")
	model.editAPIKey = true
	model.apiKeyInput.SetValue("draft")
	model.editPendingLimit = true
	model.pendingLimitInput.SetValue("42")
	model.taxLoading = false

	updated, cmd := model.Update(keysLoadedMsg{
		items: []api.APIKey{
			{ID: "k1", KeyPrefix: "nbl_pref", Name: "main", CreatedAt: time.Now()},
		},
	})
	require.Nil(t, cmd)
	assert.Len(t, updated.keys, 1)
	assert.False(t, updated.loading)

	updated, cmd = updated.Update(agentsLoadedMsg{
		items: []api.Agent{
			{ID: "a1", Name: "Agent", Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	})
	require.Nil(t, cmd)
	assert.Len(t, updated.agents, 1)

	updated, cmd = updated.Update(keyCreatedMsg{resp: &api.CreateKeyResponse{APIKey: "nbl_created"}})
	require.NotNil(t, cmd)
	assert.False(t, updated.creating)
	assert.Equal(t, "", updated.createInput.Value())
	assert.Equal(t, "nbl_created", updated.createdKey)

	updated, cmd = updated.Update(keyRevokedMsg{})
	require.NotNil(t, cmd)

	updated, cmd = updated.Update(agentUpdatedMsg{})
	require.NotNil(t, cmd)

	updated, cmd = updated.Update(apiKeySavedMsg{})
	require.Nil(t, cmd)
	assert.False(t, updated.editAPIKey)
	assert.Equal(t, "", updated.apiKeyInput.Value())

	updated, cmd = updated.Update(pendingLimitSavedMsg{limit: 100})
	require.Nil(t, cmd)
	assert.False(t, updated.editPendingLimit)
	assert.Equal(t, "", updated.pendingLimitInput.Value())

	updated.taxItems = []api.TaxonomyEntry{{ID: "tx-1", Name: "public"}}
	updated.taxKind = 0 // scopes
	noop, cmd := updated.Update(taxonomyLoadedMsg{kind: "entity-types", items: []api.TaxonomyEntry{}})
	require.Nil(t, cmd)
	assert.Equal(t, updated.taxItems, noop.taxItems)

	updated.taxLoading = false
	updated, cmd = updated.Update(taxonomyActionDoneMsg{})
	require.NotNil(t, cmd)
	assert.True(t, updated.taxLoading)
}

func TestProfileUpdateCreatedKeyGateAndSectionFocusKeys(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.createdKey = "nbl_created"

	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.Equal(t, "nbl_created", updated.createdKey)

	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.createdKey)

	updated.sectionFocus = true
	updated.section = 0
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.False(t, updated.sectionFocus)

	updated.sectionFocus = true
	updated.section = 1
	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, updated.sectionFocus)
}

func TestProfileUpdateTaxonomyHotkeysReturnLoadCommands(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.section = 2

	updated, cmd := model.Update(tea.KeyPressMsg{Code: 'i', Text: "i"})
	require.NotNil(t, cmd)
	assert.True(t, updated.taxIncludeInactive)
	assert.True(t, updated.taxLoading)

	prevKind := updated.taxKind
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: '[', Text: "["})
	require.NotNil(t, cmd)
	assert.NotEqual(t, prevKind, updated.taxKind)
	assert.True(t, updated.taxLoading)

	prevKind = updated.taxKind
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: ']', Text: "]"})
	require.NotNil(t, cmd)
	assert.NotEqual(t, prevKind, updated.taxKind)
	assert.True(t, updated.taxLoading)

	prevKind = updated.taxKind
	updated, cmd = updated.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	require.NotNil(t, cmd)
	assert.NotEqual(t, prevKind, updated.taxKind)
	assert.True(t, updated.taxLoading)
}

func TestProfileHandleCreateInputBackAndDeleteBranches(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.creating = true
	model.createInput.SetValue("abc")

	updated, cmd := model.handleCreateInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "ab", updated.createInput.Value())

	updated, _ = updated.handleCreateInput(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "ab ", updated.createInput.Value())

	updated, _ = updated.handleCreateInput(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Equal(t, "ab x", updated.createInput.Value())

	updated, _ = updated.handleCreateInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.creating)
	assert.Equal(t, "", updated.createInput.Value())
}

func TestProfileHandleCreateInputEnterReturnsErrorMessage(t *testing.T) {
	_, client := testProfileClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"nope"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewProfileModel(client, &config.Config{Username: "alxx"})
	model.creating = true
	model.createInput.SetValue("key-name")

	updated, cmd := model.handleCreateInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.False(t, updated.creating)
	assert.Equal(t, "", updated.createInput.Value())
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestProfileRevokeSelectedAndToggleTrustEdgePaths(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})

	updated, cmd := model.revokeSelected()
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	updated, cmd = model.toggleTrust()
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)
}

func TestProfileRevokeSelectedAndToggleTrustErrorPaths(t *testing.T) {
	_, client := testProfileClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/keys/") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"cannot revoke"}`))
			return
		case strings.HasPrefix(r.URL.Path, "/api/agents/") && r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"cannot patch"}`))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewProfileModel(client, &config.Config{Username: "alxx"})
	model.keys = []api.APIKey{{ID: "k1", Name: "main"}}
	model.keyList.SetItems([]string{"k1"})
	model.agents = []api.Agent{{ID: "a1", Name: "agent", RequiresApproval: true}}
	model.agentList.SetItems([]string{"a1"})

	_, cmd := model.revokeSelected()
	require.NotNil(t, cmd)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)

	_, cmd = model.toggleTrust()
	require.NotNil(t, cmd)
	_, ok = cmd().(errMsg)
	assert.True(t, ok)
}

func TestProfileLoadKeysAndAgentsErrorPaths(t *testing.T) {
	_, client := testProfileClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"nope"}`))
	})

	model := NewProfileModel(client, &config.Config{Username: "alxx"})
	_, ok := model.loadKeys().(errMsg)
	assert.True(t, ok)
	_, ok = model.loadAgents().(errMsg)
	assert.True(t, ok)
}

func TestProfileViewBranchMatrixAndPreviewHelpers(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx", APIKey: "nbl_key", PendingLimit: 123})
	model.width = 90

	model.loading = true
	assert.Contains(t, components.SanitizeText(model.View()), "Loading profile...")

	model.loading = false
	model.editAPIKey = true
	assert.Contains(t, components.SanitizeText(model.View()), "Set API Key")

	model.editAPIKey = false
	model.editPendingLimit = true
	assert.Contains(t, components.SanitizeText(model.View()), "Pending Queue Limit")

	model.editPendingLimit = false
	model.creating = true
	assert.Contains(t, components.SanitizeText(model.View()), "New Key Name")

	model.creating = false
	model.createdKey = "nbl_created"
	assert.Contains(t, components.SanitizeText(model.View()), "Key Created")

	model.createdKey = ""
	model.section = 0
	assert.Contains(t, components.SanitizeText(model.View()), "API Keys")

	model.section = 1
	assert.Contains(t, components.SanitizeText(model.View()), "Agents")

	model.section = 2
	assert.Contains(t, components.SanitizeText(model.View()), "Taxonomy")

	assert.Equal(t, "", model.renderKeyPreview(api.APIKey{}, 0))
	assert.Equal(t, "", model.renderAgentPreview(api.Agent{}, 0))
}

func TestProfileMaskedAPIKeyExactLengthTen(t *testing.T) {
	assert.Equal(t, "**********", maskedAPIKey("1234567890"))
}

func TestProfileUpdateSectionTwoEnterOpensEditPrompt(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.section = 2
	model.taxItems = []api.TaxonomyEntry{{ID: "scope-1", Name: "public"}}
	model.taxList.SetItems([]string{"public"})

	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.Equal(t, taxPromptEditName, updated.taxPromptMode)
	assert.Equal(t, "scope-1", updated.taxEditID)
	assert.Equal(t, "public", updated.taxPromptInput.Value())
}

func TestProfileUpdateHandlesTaxonomyLoadedMatch(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.taxKind = 0 // scopes
	model.taxLoading = true
	model.loading = true

	items := []api.TaxonomyEntry{
		{ID: "scope-1", Name: "public"},
		{ID: "scope-2", Name: "private"},
	}
	updated, cmd := model.Update(taxonomyLoadedMsg{kind: "scopes", items: items})
	require.Nil(t, cmd)
	assert.False(t, updated.taxLoading)
	assert.False(t, updated.loading)
	assert.Len(t, updated.taxItems, 2)
}

func TestProfileHandleAPIKeyInputSavePathWithClientUpdate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var authHeader string
	_, client := testProfileClient(t, func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		if r.URL.Path == "/api/entities/ent-1" && r.Method == http.MethodGet {
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{"id": "ent-1", "name": "ok"},
			})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.Config{Username: "alxx", APIKey: "nbl_old"}
	require.NoError(t, cfg.Save())

	model := NewProfileModel(client, cfg)
	model.editAPIKey = true
	model.apiKeyInput.SetValue("nbl_new")
	updated, cmd := model.handleAPIKeyInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	assert.True(t, updated.editAPIKey)

	msg := cmd()
	_, ok := msg.(apiKeySavedMsg)
	assert.True(t, ok)

	_, err := client.GetEntity("ent-1")
	require.NoError(t, err)
	assert.Equal(t, "Bearer nbl_new", authHeader)
}

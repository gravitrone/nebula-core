package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProfileClient handles test profile client.
func testProfileClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestProfileAgentDetailToggle handles test profile agent detail toggle.
func TestProfileAgentDetailToggle(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.section = 1
	model.agents = []api.Agent{
		{
			ID:           "agent-1",
			Name:         "Alpha",
			Status:       "active",
			Scopes:       []string{"public"},
			Capabilities: []string{"read"},
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}
	model.agentList.SetItems([]string{formatAgentLine(model.agents[0])})

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, model.agentDetail)
	assert.Equal(t, "agent-1", model.agentDetail.ID)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, model.agentDetail)
}

// TestProfileSetAPIKeyPersistsAndUpdatesClient handles test profile set apikey persists and updates client.
func TestProfileSetAPIKeyPersistsAndUpdatesClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		if seenAuth != "Bearer nbl_newkey" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"ent-1","name":"ok","tags":[]}}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		APIKey:   "nbl_oldkey",
		Username: "alxx",
	}
	require.NoError(t, cfg.Save())

	client := api.NewClient(srv.URL, "nbl_oldkey")
	model := NewProfileModel(client, cfg)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	require.True(t, model.editAPIKey)

	model.apiKeyBuf = "nbl_newkey"
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = model.Update(msg)
	require.False(t, model.editAPIKey)

	loaded, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "nbl_newkey", loaded.APIKey)

	_, err = client.GetEntity("ent-1")
	require.NoError(t, err)
	assert.Equal(t, "Bearer nbl_newkey", seenAuth)
}

// TestProfileKeysLoadCreateAndRevokeFlows handles test profile keys load create and revoke flows.
func TestProfileKeysLoadCreateAndRevokeFlows(t *testing.T) {
	now := time.Now()
	var createName string
	var revokedID string
	keysAllCalls := 0

	_, client := testProfileClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/keys/all" && r.Method == http.MethodGet:
			keysAllCalls++
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":         "k1",
					"key_prefix": "nbl_abc123",
					"name":       "demo",
					"created_at": now,
				},
			}})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/keys" && r.Method == http.MethodPost:
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			createName = body["name"]
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"api_key": "nbl_created_secret",
				"key_id":  "k2",
				"prefix":  "nbl_created",
				"name":    createName,
			}})
			require.NoError(t, err)
			return
		case strings.HasPrefix(r.URL.Path, "/api/keys/") && r.Method == http.MethodDelete:
			revokedID = strings.TrimPrefix(r.URL.Path, "/api/keys/")
			w.WriteHeader(http.StatusOK)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.Config{Username: "alxx", APIKey: "nbl_zzzzzzzzzz"}
	model := NewProfileModel(client, cfg)
	model.width = 100

	// Load keys.
	model, _ = model.Update(model.loadKeys())
	require.Len(t, model.keys, 1)
	assert.Contains(t, model.View(), "nbl_abc")

	// Create key flow.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	require.True(t, model.creating)
	for _, r := range "my key" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())
	assert.Equal(t, "my key", createName)
	assert.Equal(t, "nbl_created_secret", model.createdKey)
	require.GreaterOrEqual(t, keysAllCalls, 2)

	// Clear created key gate and revoke selected.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())
	assert.Equal(t, "k1", revokedID)
}

// TestProfileAgentsLoadAndToggleTrustFlow handles test profile agents load and toggle trust flow.
func TestProfileAgentsLoadAndToggleTrustFlow(t *testing.T) {
	now := time.Now()
	var patchedID string
	var patched api.UpdateAgentInput
	agentsCalls := 0

	_, client := testProfileClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/agents/" && r.Method == http.MethodGet:
			agentsCalls++
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
				{
					"id":                "agent-1",
					"name":              "Alpha",
					"status":            "active",
					"requires_approval": true,
					"scopes":            []string{"public"},
					"capabilities":      []string{"read"},
					"created_at":        now,
					"updated_at":        now,
				},
			}})
			require.NoError(t, err)
			return
		case strings.HasPrefix(r.URL.Path, "/api/agents/") && r.Method == http.MethodPatch:
			patchedID = strings.TrimPrefix(r.URL.Path, "/api/agents/")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": patchedID}})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.Config{Username: "alxx", APIKey: "nbl_zzzzzzzzzz"}
	model := NewProfileModel(client, cfg)
	model.section = 1
	model.width = 100

	// Load agents.
	model, _ = model.Update(model.loadAgents())
	require.Len(t, model.agents, 1)
	assert.Contains(t, model.View(), "Agents")

	// Toggle trust.
	model, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.Equal(t, "agent-1", patchedID)
	require.NotNil(t, patched.RequiresApproval)
	assert.False(t, *patched.RequiresApproval)
	require.GreaterOrEqual(t, agentsCalls, 2)
}

// TestMaskedAPIKey handles test masked apikey.
func TestMaskedAPIKey(t *testing.T) {
	assert.Equal(t, "-", maskedAPIKey(""))
	assert.Equal(t, "*****", maskedAPIKey("abcde"))
	assert.Equal(t, "abcdef...7890", maskedAPIKey("abcdef1234567890"))
}

// TestProfileActiveListAndParsePositiveIntHelpers handles test profile active list and parse positive int helpers.
func TestProfileActiveListAndParsePositiveIntHelpers(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.section = 0
	assert.Same(t, model.keyList, model.activeList())
	model.section = 1
	assert.Same(t, model.agentList, model.activeList())
	model.section = 2
	assert.Same(t, model.agentList, model.activeList())

	n, err := parsePositiveInt("42")
	require.NoError(t, err)
	assert.Equal(t, 42, n)

	_, err = parsePositiveInt("0")
	require.Error(t, err)
	_, err = parsePositiveInt("-1")
	require.Error(t, err)
	_, err = parsePositiveInt("abc")
	require.Error(t, err)
}

// TestProfileHandlePendingLimitInputAndRenderAgentDetail handles test profile handle pending limit input and render agent detail.
func TestProfileHandlePendingLimitInputAndRenderAgentDetail(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	desc := "ops agent"
	now := time.Now()
	cfg := &config.Config{Username: "alxx", APIKey: "nbl_key", PendingLimit: 25}
	model := NewProfileModel(nil, cfg)

	model.editPendingLimit = true
	model.pendingLimitBuf = ""
	_, cmd := model.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(errMsg)
	assert.True(t, ok)

	model.pendingLimitBuf = "6001"
	_, cmd = model.handlePendingLimitInput(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	saved, ok := msg.(pendingLimitSavedMsg)
	require.True(t, ok)
	assert.Equal(t, 5000, saved.limit)

	model.agentDetail = &api.Agent{
		ID:               "agent-1",
		Name:             "Alpha",
		Status:           "active",
		RequiresApproval: false,
		Scopes:           []string{"public", "admin"},
		Capabilities:     []string{"read", "write"},
		Description:      &desc,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	detail := stripANSI(model.renderAgentDetail())
	assert.Contains(t, detail, "Agent Details")
	assert.Contains(t, detail, "Scopes")
	assert.Contains(t, detail, "public,")
	assert.Contains(t, detail, "Descriptio")
}

// TestProfileSectionFocusArrowNavigation handles test profile section focus arrow navigation.
func TestProfileSectionFocusArrowNavigation(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.sectionFocus = true

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 1, model.section)
	assert.True(t, model.sectionFocus)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRight})
	assert.Equal(t, 2, model.section)
	assert.True(t, model.sectionFocus)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 1, model.section)
	assert.True(t, model.sectionFocus)

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.False(t, model.sectionFocus)
}

// TestProfileUpFromTopListReturnsToSectionFocus handles test profile up from top list returns to section focus.
func TestProfileUpFromTopListReturnsToSectionFocus(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{Username: "alxx"})
	model.section = 0
	model.sectionFocus = false
	model.keyList.SetItems([]string{"one", "two"})

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, model.sectionFocus)
}

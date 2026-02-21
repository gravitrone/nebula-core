package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReloginCmdUnavailableWithoutClientOrConfig handles test relogin cmd unavailable without client or config.
func TestReloginCmdUnavailableWithoutClientOrConfig(t *testing.T) {
	app := NewApp(nil, nil)

	cmd := app.reloginCmd()
	require.NotNil(t, cmd)
	msg := cmd()

	em, ok := msg.(errMsg)
	require.True(t, ok)
	assert.Contains(t, em.err.Error(), "re-login unavailable")
}

// TestReloginCmdRequiresUsername handles test relogin cmd requires username.
func TestReloginCmdRequiresUsername(t *testing.T) {
	client := api.NewClient("http://example.com", "key")
	app := NewApp(client, &config.Config{APIKey: "key"})

	cmd := app.reloginCmd()
	require.NotNil(t, cmd)
	msg := cmd()

	em, ok := msg.(errMsg)
	require.True(t, ok)
	assert.Contains(t, em.err.Error(), "username missing")
}

// TestReloginCmdCallsLoginAndReturnsAPIKey handles test relogin cmd calls login and returns apikey.
func TestReloginCmdCallsLoginAndReturnsAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"api_key":   "nbl_testkey",
					"entity_id": "ent-1",
					"username":  "alxx",
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	client := api.NewClient(srv.URL, "oldkey")
	app := NewApp(client, &config.Config{APIKey: "oldkey", Username: "alxx"})

	cmd := app.reloginCmd()
	require.NotNil(t, cmd)
	msg := cmd()

	done, ok := msg.(reloginDoneMsg)
	require.True(t, ok)
	require.NoError(t, done.err)
	assert.Equal(t, "nbl_testkey", done.apiKey)
}

// TestToastSanitizesTextAndRendersBranches handles test toast sanitizes text and renders branches.
func TestToastSanitizesTextAndRendersBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 80

	_ = app.setToast("success", "\x1b[2Jok")
	require.NotNil(t, app.toast)
	assert.False(t, strings.Contains(app.toast.text, "\x1b"))
	assert.NotEmpty(t, app.renderToast())

	_ = app.setToast("warning", "warn")
	assert.NotEmpty(t, app.renderToast())

	_ = app.setToast("error", "err")
	assert.NotEmpty(t, app.renderToast())

	_ = app.setToast("info", "info")
	assert.NotEmpty(t, app.renderToast())
}

// TestQuickstartKeyFlowRoutesTabsAndCompletes handles test quickstart key flow routes tabs and completes.
func TestQuickstartKeyFlowRoutesTabsAndCompletes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := NewApp(nil, &config.Config{APIKey: "key", Username: "alxx", QuickstartPending: true})
	app.quickstartOpen = true
	app.quickstartStep = 0

	model, _ := app.handleQuickstartKeys(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(App)
	assert.Equal(t, tabEntities, updated.tab)
	assert.Equal(t, entitiesViewAdd, updated.entities.view)
	assert.Equal(t, 1, updated.quickstartStep)

	app = updated
	model, _ = app.handleQuickstartKeys(tea.KeyMsg{Type: tea.KeyEnter})
	updated = model.(App)
	assert.Equal(t, tabKnow, updated.tab)
	assert.Equal(t, contextViewAdd, updated.know.view)
	assert.Equal(t, 2, updated.quickstartStep)

	app = updated
	model, _ = app.handleQuickstartKeys(tea.KeyMsg{Type: tea.KeyEnter})
	updated = model.(App)
	assert.Equal(t, tabRelations, updated.tab)
	assert.Equal(t, relsViewCreateSourceSearch, updated.rels.view)
	assert.False(t, updated.quickstartOpen)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "success", updated.toast.level)
}

// TestQuickstartSkipsOnEscapeAndResetsState handles test quickstart skips on escape and resets state.
func TestQuickstartSkipsOnEscapeAndResetsState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := NewApp(nil, &config.Config{APIKey: "key", Username: "alxx", QuickstartPending: true})
	app.quickstartOpen = true
	app.quickstartStep = 2

	model, _ := app.handleQuickstartKeys(tea.KeyMsg{Type: tea.KeyEsc})
	updated := model.(App)

	assert.False(t, updated.quickstartOpen)
	assert.Equal(t, 0, updated.quickstartStep)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "info", updated.toast.level)
}

// TestRenderQuickstartDoesNotPanic handles test render quickstart does not panic.
func TestRenderQuickstartDoesNotPanic(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 80
	app.quickstartOpen = true
	app.quickstartStep = 1

	out := app.renderQuickstart()
	assert.Contains(t, out, "Getting Started")
}

// TestStartupParsingHelpers handles test startup parsing helpers.
func TestStartupParsingHelpers(t *testing.T) {
	code, msg := parseErrorCodeAndMessage("FORBIDDEN: missing scope")
	assert.Equal(t, "FORBIDDEN", code)
	assert.Equal(t, "missing scope", msg)

	code, msg = parseErrorCodeAndMessage("HTTP 500: bad")
	assert.Equal(t, "", code)
	assert.Contains(t, msg, "HTTP 500")

	assert.True(t, shouldShowRecoveryHints("FORBIDDEN", "scope missing"))
	assert.True(t, shouldShowRecoveryHints("FORBIDDEN", "admin required"))
	assert.False(t, shouldShowRecoveryHints("NOT_FOUND", "admin required"))

	assert.Equal(t, "ok", classifyStartupAPI(""))
	assert.Equal(t, "timeout", classifyStartupAPI("deadline exceeded"))
	assert.Equal(t, "down", classifyStartupAPI("connection refused"))

	assert.Equal(t, "missing", classifyStartupAuth("", nil))
	assert.Equal(t, "missing", classifyStartupAuth("", &config.Config{APIKey: ""}))
	assert.Equal(t, "ok", classifyStartupAuth("", &config.Config{APIKey: "key"}))
	assert.Equal(t, "invalid", classifyStartupAuth("bad", &config.Config{APIKey: "key"}))

	assert.Equal(t, "ok", classifyStartupTaxonomy(""))
	assert.Equal(t, "forbidden", classifyStartupTaxonomy("forbidden: scope"))
	assert.Equal(t, "schema_error", classifyStartupTaxonomy("relation does not exist"))
	assert.Equal(t, "failed", classifyStartupTaxonomy("unknown error"))

	level, copy := startupToastCopy(startupSummary{API: "ok", Auth: "ok", Taxonomy: "ok"})
	assert.Equal(t, "success", level)
	assert.Contains(t, copy, "Startup checks passed")

	level, copy = startupToastCopy(startupSummary{API: "down", Auth: "ok", Taxonomy: "ok"})
	assert.Equal(t, "error", level)
	assert.Contains(t, copy, "API is down")

	level, copy = startupToastCopy(startupSummary{API: "ok", Auth: "missing", Taxonomy: "forbidden"})
	assert.Equal(t, "warning", level)
	assert.Contains(t, copy, "auth=missing")
}

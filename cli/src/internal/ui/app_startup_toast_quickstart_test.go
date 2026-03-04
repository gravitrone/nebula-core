package ui

import (
	"encoding/json"
	"errors"
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

// TestReloginCmdTrimsUsernameBeforeLogin handles username normalization before re-auth.
func TestReloginCmdTrimsUsernameBeforeLogin(t *testing.T) {
	var seenUsername string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost {
			var payload map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			seenUsername = payload["username"]
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"api_key":   "nbl_trimmed_key",
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
	app := NewApp(client, &config.Config{APIKey: "oldkey", Username: "  alxx  "})

	cmd := app.reloginCmd()
	require.NotNil(t, cmd)
	msg := cmd()

	done, ok := msg.(reloginDoneMsg)
	require.True(t, ok)
	require.NoError(t, done.err)
	assert.Equal(t, "nbl_trimmed_key", done.apiKey)
	assert.Equal(t, "alxx", seenUsername)
}

// TestReloginCmdReturnsErrorMsgOnLoginFailure handles login failure mapping in re-auth command.
func TestReloginCmdReturnsErrorMsgOnLoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "INVALID_API_KEY",
					"message": "token expired",
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
	require.Error(t, done.err)
	assert.Equal(t, "", done.apiKey)
	assert.Contains(t, strings.ToLower(done.err.Error()), "invalid_api_key")
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
	_ = out
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
	assert.True(t, shouldShowRecoveryHints("INVALID_API_KEY", "bad token"))
	assert.True(t, shouldShowRecoveryHints("", "invalid api key"))
	assert.True(t, shouldShowRecoveryHints("", "invalid_api_key: token expired"))
	assert.True(t, shouldShowRecoveryHints("", "auth_required: login required"))
	assert.False(t, shouldShowRecoveryHints("NOT_FOUND", "admin required"))

	assert.Equal(t, "ok", classifyStartupAPI(""))
	assert.Equal(t, "timeout", classifyStartupAPI("deadline exceeded"))
	assert.Equal(t, "down", classifyStartupAPI("connection refused"))

	assert.Equal(t, "missing", classifyStartupAuth("", nil))
	assert.Equal(t, "missing", classifyStartupAuth("", &config.Config{APIKey: ""}))
	assert.Equal(t, "ok", classifyStartupAuth("", &config.Config{APIKey: "key"}))
	assert.Equal(t, "invalid", classifyStartupAuth("HTTP 401: Unauthorized", &config.Config{APIKey: "key"}))
	assert.Equal(t, "invalid", classifyStartupAuth("INVALID_API_KEY: bad token", &config.Config{APIKey: "key"}))
	assert.Equal(t, "invalid", classifyStartupAuth("AUTH_REQUIRED: missing auth", &config.Config{APIKey: "key"}))
	assert.Equal(t, "multi_api_conflict", classifyStartupAuth("HTTP 500: Internal Server Error", &config.Config{APIKey: "key"}))
	assert.Equal(
		t,
		"multi_api_conflict",
		classifyStartupAuth("HTTP 500: multiple api instances detected", &config.Config{APIKey: "key"}),
	)
	assert.Equal(
		t,
		"multi_api_conflict",
		classifyStartupAuth("MULTIPLE_API_INSTANCES_DETECTED: duplicate processes", &config.Config{APIKey: "key"}),
	)
	assert.Equal(
		t,
		"multi_api_conflict",
		classifyStartupAuth("listen tcp 127.0.0.1:8000: bind: address already in use", &config.Config{APIKey: "key"}),
	)
	assert.Equal(
		t,
		"multi_api_conflict",
		classifyStartupAuth("socket error: EADDRINUSE", &config.Config{APIKey: "key"}),
	)
	assert.Equal(
		t,
		"multi_api_conflict",
		classifyStartupAuth("errno 98 while binding socket", &config.Config{APIKey: "key"}),
	)
	assert.Equal(
		t,
		"multi_api_conflict",
		classifyStartupAuth("socket bind failed: errno 48", &config.Config{APIKey: "key"}),
	)
	assert.Equal(
		t,
		"multi_api_conflict",
		classifyStartupAuth("ADDRESS ALREADY IN USE", &config.Config{APIKey: "key"}),
	)

	assert.Equal(t, "ok", classifyStartupTaxonomy(""))
	assert.Equal(t, "forbidden", classifyStartupTaxonomy("forbidden: scope"))
	assert.Equal(t, "schema_error", classifyStartupTaxonomy("relation does not exist"))
	assert.Equal(t, "failed", classifyStartupTaxonomy("unknown error"))
	assert.True(t, shouldShowMultiAPIRecoveryHint("MULTIPLE_API_INSTANCES_DETECTED", "", ""))
	assert.True(t, shouldShowMultiAPIRecoveryHint("", "", "listen tcp 127.0.0.1:8000: bind: address already in use"))
	assert.True(t, shouldShowMultiAPIRecoveryHint("", "", "socket error: EADDRINUSE"))
	assert.True(t, shouldShowMultiAPIRecoveryHint("", "", "bind failure errno 98"))
	assert.True(t, shouldShowMultiAPIRecoveryHint("", "multiple_api_instances_detected", ""))
	assert.False(t, shouldShowMultiAPIRecoveryHint("INVALID_API_KEY", "bad token", "auth failure"))

	level, copy := startupToastCopy(startupSummary{API: "ok", Auth: "ok", Taxonomy: "ok"})
	assert.Equal(t, "success", level)
	assert.Contains(t, copy, "Startup checks passed")

	level, copy = startupToastCopy(startupSummary{API: "down", Auth: "ok", Taxonomy: "ok"})
	assert.Equal(t, "error", level)
	assert.Contains(t, copy, "API is down")

	level, copy = startupToastCopy(startupSummary{API: "ok", Auth: "missing", Taxonomy: "forbidden"})
	assert.Equal(t, "warning", level)
	assert.Contains(t, copy, "auth=missing")

	level, copy = startupToastCopy(startupSummary{API: "ok", Auth: "multi_api_conflict", Taxonomy: "ok"})
	assert.Equal(t, "error", level)
	assert.Contains(t, strings.ToLower(copy), "multiple api instances detected")
}

// TestStartupCheckedMsgInvalidAuthEnablesRecoveryHints handles startup invalid auth recovery hints.
func TestStartupCheckedMsgInvalidAuthEnablesRecoveryHints(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.startupChecking = true
	model, cmd := app.Update(startupCheckedMsg{authErr: "HTTP 401: Unauthorized"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "invalid", updated.startup.Auth)
	assert.Equal(t, "INVALID_API_KEY: Invalid API key", updated.err)
	assert.Equal(t, "INVALID_API_KEY", updated.lastErrCode)
	assert.Equal(t, "Invalid API key", updated.lastErrMsg)
	assert.True(t, updated.showRecoveryHints)
	require.NotNil(t, cmd)
}

// TestStartupCheckedMsgRawConflictTextSetsMultiAPIConflict handles startup conflict text variants.
func TestStartupCheckedMsgRawConflictTextSetsMultiAPIConflict(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.startupChecking = true

	model, cmd := app.Update(startupCheckedMsg{authErr: "listen tcp 127.0.0.1:8000: bind: address already in use"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "multi_api_conflict", updated.startup.Auth)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED: multiple api instances detected", updated.err)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED", updated.lastErrCode)
	assert.False(t, updated.showRecoveryHints)
	require.NotNil(t, cmd)
}

// TestStartupCheckedMsgErrno48SetsMultiAPIConflict handles macOS bind conflict errno text.
func TestStartupCheckedMsgErrno48SetsMultiAPIConflict(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.startupChecking = true

	model, cmd := app.Update(startupCheckedMsg{authErr: "socket bind failed: errno 48"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "multi_api_conflict", updated.startup.Auth)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED: multiple api instances detected", updated.err)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED", updated.lastErrCode)
	assert.False(t, updated.showRecoveryHints)
	require.NotNil(t, cmd)
}

// TestStartupCheckedMsgClearsRecoveryHintsAfterAuthRecovery handles startup auth recovery clearing stale invalid-key hints.
func TestStartupCheckedMsgClearsRecoveryHintsAfterAuthRecovery(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "good-key"})
	app.startupChecking = true
	app.lastErrCode = "INVALID_API_KEY"
	app.lastErrMsg = "Invalid API key"
	app.showRecoveryHints = true

	model, cmd := app.Update(startupCheckedMsg{})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "ok", updated.startup.Auth)
	assert.Equal(t, "", updated.lastErrCode)
	assert.Equal(t, "", updated.lastErrMsg)
	assert.False(t, updated.showRecoveryHints)
	require.NotNil(t, cmd)
}

// TestRunStartupCheckCmdCollectsAuthAndTaxonomyErrors handles startup check collecting downstream failures.
func TestRunStartupCheckCmdCollectsAuthAndTaxonomyErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/health" && r.Method == http.MethodGet:
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"status": "ok"}))
		case r.URL.Path == "/api/keys" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusUnauthorized)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "INVALID_API_KEY",
					"message": "bad token",
				},
			}))
		case strings.HasPrefix(r.URL.Path, "/api/taxonomy/scopes") && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusForbidden)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "FORBIDDEN",
					"message": "scope missing",
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.NewClient(srv.URL, "bad-key"), &config.Config{APIKey: "bad-key"})
	cmd := app.runStartupCheckCmd()
	require.NotNil(t, cmd)

	msg, ok := cmd().(startupCheckedMsg)
	require.True(t, ok)
	assert.Equal(t, "", msg.apiErr)
	assert.Contains(t, msg.authErr, "INVALID_API_KEY")
	assert.Contains(t, msg.taxonomyErr, "FORBIDDEN")
}

// TestRunStartupCheckCmdStopsOnAPIError handles startup check short-circuiting when API health fails.
func TestRunStartupCheckCmdStopsOnAPIError(t *testing.T) {
	var keysCalled bool
	var taxonomyCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/health" && r.Method == http.MethodGet:
			w.WriteHeader(http.StatusServiceUnavailable)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "API_DOWN",
					"message": "unavailable",
				},
			}))
		case r.URL.Path == "/api/keys" && r.Method == http.MethodGet:
			keysCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasPrefix(r.URL.Path, "/api/taxonomy/scopes") && r.Method == http.MethodGet:
			taxonomyCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.NewClient(srv.URL, "any"), &config.Config{APIKey: "any"})
	cmd := app.runStartupCheckCmd()
	require.NotNil(t, cmd)

	msg, ok := cmd().(startupCheckedMsg)
	require.True(t, ok)
	assert.Contains(t, msg.apiErr, "API_DOWN")
	assert.Equal(t, "", msg.authErr)
	assert.Equal(t, "", msg.taxonomyErr)
	assert.False(t, keysCalled)
	assert.False(t, taxonomyCalled)
}

// TestRecoveryHintsReloginKeyFlowHandlesEndToEnd handles pressing r to relogin and clear recovery state.
func TestRecoveryHintsReloginKeyFlowHandlesEndToEnd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"api_key":   "nbl_recovered_key",
					"entity_id": "ent-1",
					"username":  "alxx",
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{APIKey: "stale-key", Username: "alxx"}
	app := NewApp(api.NewClient(srv.URL, "stale-key"), cfg)
	app.showRecoveryHints = true
	app.lastErrCode = "INVALID_API_KEY"
	app.lastErrMsg = "Invalid API key"

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.NotNil(t, cmd)

	updated := model.(App)
	reloginMsg := cmd()
	model, _ = updated.Update(reloginMsg)
	recovered := model.(App)

	assert.Equal(t, "nbl_recovered_key", recovered.config.APIKey)
	assert.Equal(t, "", recovered.lastErrCode)
	assert.Equal(t, "", recovered.lastErrMsg)
	assert.False(t, recovered.showRecoveryHints)
	require.NotNil(t, recovered.toast)
	assert.Equal(t, "success", recovered.toast.level)
}

// TestInvalidAPIKeyRecoveryReauthFlow handles startup invalid-key -> relogin -> recovered state.
func TestInvalidAPIKeyRecoveryReauthFlow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"api_key":   "nbl_reauth_key",
					"entity_id": "ent-1",
					"username":  "alxx",
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	cfg := &config.Config{APIKey: "bad-key", Username: "alxx"}
	app := NewApp(api.NewClient(srv.URL, "bad-key"), cfg)
	app.startupChecking = true

	model, _ := app.Update(startupCheckedMsg{authErr: "HTTP 401: Unauthorized"})
	invalid := model.(App)
	assert.Equal(t, "invalid", invalid.startup.Auth)
	assert.True(t, invalid.showRecoveryHints)
	assert.Equal(t, "INVALID_API_KEY", invalid.lastErrCode)

	model, cmd := invalid.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	require.NotNil(t, cmd)

	reloginMsg := cmd()
	model, _ = model.(App).Update(reloginMsg)
	recovered := model.(App)

	assert.Equal(t, "nbl_reauth_key", recovered.config.APIKey)
	assert.False(t, recovered.showRecoveryHints)
	assert.Equal(t, "", recovered.lastErrCode)
	assert.Equal(t, "", recovered.lastErrMsg)
	require.NotNil(t, recovered.toast)
	assert.Equal(t, "success", recovered.toast.level)
}

// TestRecoveryHintsSettingsShortcutSwitchesToSettingsTab handles recovery shortcut routing to settings/profile.
func TestRecoveryHintsSettingsShortcutSwitchesToSettingsTab(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.tab = tabEntities
	app.tabNav = false
	app.showRecoveryHints = true
	app.lastErrCode = "INVALID_API_KEY"
	app.lastErrMsg = "Invalid API key"

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	updated := model.(App)

	assert.Equal(t, tabProfile, updated.tab)
	assert.True(t, updated.tabNav)
	assert.NotNil(t, cmd)
}

// TestRecoveryHintsCopyShortcutShowsRecoveryCommand handles recovery shortcut exposing the command to run.
func TestRecoveryHintsCopyShortcutShowsRecoveryCommand(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.recoveryCommand = "nebula login"
	app.showRecoveryHints = true
	app.lastErrCode = "INVALID_API_KEY"
	app.lastErrMsg = "Invalid API key"

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	updated := model.(App)

	require.NotNil(t, cmd)
	assert.True(t, updated.showRecoveryHints)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "info", updated.toast.level)
	assert.Equal(t, "nebula login", updated.toast.text)
}

// TestStartupCheckedMsgAPIDownClearsStaleRecoveryHints handles stale hint cleanup when startup API is unavailable.
func TestStartupCheckedMsgAPIDownClearsStaleRecoveryHints(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.startupChecking = true
	app.showRecoveryHints = true
	app.lastErrCode = "INVALID_API_KEY"
	app.lastErrMsg = "Invalid API key"

	model, cmd := app.Update(startupCheckedMsg{apiErr: "connection refused"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "down", updated.startup.API)
	assert.Equal(t, "missing", updated.startup.Auth)
	assert.Equal(t, "failed", updated.startup.Taxonomy)
	assert.False(t, updated.showRecoveryHints)
	assert.Equal(t, "", updated.lastErrCode)
	assert.Equal(t, "", updated.lastErrMsg)
	require.NotNil(t, cmd)
}

// TestStartupCheckedMsgAuth500SurfacesMultiAPIRecovery handles generic startup auth 500 conflicts.
func TestStartupCheckedMsgAuth500SurfacesMultiAPIRecovery(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.startupChecking = true
	app.showRecoveryHints = true
	app.lastErrCode = "INVALID_API_KEY"
	app.lastErrMsg = "Invalid API key"

	model, cmd := app.Update(startupCheckedMsg{authErr: "HTTP 500: Internal Server Error"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "ok", updated.startup.API)
	assert.Equal(t, "multi_api_conflict", updated.startup.Auth)
	assert.False(t, updated.showRecoveryHints)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED", updated.lastErrCode)
	assert.Equal(t, "multiple api instances detected", updated.lastErrMsg)
	assert.Contains(t, strings.ToLower(updated.err), "multiple api instances detected")
	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "error", updated.toast.level)
	assert.Contains(t, strings.ToLower(updated.toast.text), "multiple api instances detected")
}

// TestStartupCheckedMsgMultiAPIConflictShowsActionableToast handles explicit multi-api conflict startup messaging.
func TestStartupCheckedMsgMultiAPIConflictShowsActionableToast(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "key"})
	app.startupChecking = true

	model, cmd := app.Update(startupCheckedMsg{authErr: "HTTP 500: multiple api instances detected"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "multi_api_conflict", updated.startup.Auth)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED: multiple api instances detected", updated.err)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED", updated.lastErrCode)
	assert.False(t, updated.showRecoveryHints)
	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "error", updated.toast.level)
	assert.Contains(t, strings.ToLower(updated.toast.text), "multiple api instances detected")
	assert.Contains(t, updated.toast.text, "nebula start")
}

// TestStartupCheckedMsgMultiAPIConflictCodeShowsActionableToast handles coded multi-api conflict startup messaging.
func TestStartupCheckedMsgMultiAPIConflictCodeShowsActionableToast(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "key"})
	app.startupChecking = true

	model, cmd := app.Update(startupCheckedMsg{authErr: "MULTIPLE_API_INSTANCES_DETECTED: duplicate processes"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "multi_api_conflict", updated.startup.Auth)
	assert.Equal(t, "MULTIPLE_API_INSTANCES_DETECTED", updated.lastErrCode)
	assert.False(t, updated.showRecoveryHints)
	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "error", updated.toast.level)
	assert.Contains(t, strings.ToLower(updated.toast.text), "multiple api instances detected")
}

// TestStartupCheckedMsgAuthCodeErrorEnablesRecoveryHints handles coded auth errors from API envelopes.
func TestStartupCheckedMsgAuthCodeErrorEnablesRecoveryHints(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key"})
	app.startupChecking = true

	model, cmd := app.Update(startupCheckedMsg{authErr: "INVALID_API_KEY: bad token"})
	updated := model.(App)

	assert.False(t, updated.startupChecking)
	assert.Equal(t, "invalid", updated.startup.Auth)
	assert.Contains(t, updated.err, "INVALID_API_KEY")
	assert.True(t, updated.showRecoveryHints)
	assert.Equal(t, "INVALID_API_KEY", updated.lastErrCode)
	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "warning", updated.toast.level)
}

// TestReloginDoneMsgUnderscoreInvalidKeyEnablesRecoveryHints locks recovery hints on underscore auth errors.
func TestReloginDoneMsgUnderscoreInvalidKeyEnablesRecoveryHints(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "bad-key", Username: "alxx"})

	model, _ := app.Update(reloginDoneMsg{err: errors.New("invalid_api_key: token expired")})
	updated := model.(App)

	assert.Contains(t, strings.ToLower(updated.err), "invalid_api_key")
	assert.True(t, updated.showRecoveryHints)
}

// TestErrorBoxShowsMultiAPIRecoveryHint handles persistent UI guidance for multi-API conflicts.
func TestErrorBoxShowsMultiAPIRecoveryHint(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "key"})
	app.err = "MULTIPLE_API_INSTANCES_DETECTED: multiple api instances detected"
	app.lastErrCode = "MULTIPLE_API_INSTANCES_DETECTED"
	app.lastErrMsg = "multiple api instances detected"

	out := app.View()
	assert.Contains(t, strings.ToLower(out), "multiple api instances detected")
	assert.Contains(t, strings.ToLower(out), "stop duplicate api processes")
	assert.Contains(t, out, "nebula start")
}

package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateMessageBranchesReloginOnboardingAndLimits(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "key", Username: "alxx"})
	app.toast = &appToast{level: "info", text: "x"}

	model, _ := app.Update(errMsg{err: errors.New("INVALID_API_KEY: token expired")})
	updated := model.(App)
	assert.Equal(t, "INVALID_API_KEY", updated.lastErrCode)
	assert.True(t, updated.showRecoveryHints)

	model, _ = updated.Update(clearToastMsg{})
	updated = model.(App)
	assert.Nil(t, updated.toast)

	model, _ = updated.Update(reloginDoneMsg{err: errors.New("nope")})
	updated = model.(App)
	assert.Contains(t, updated.err, "re-login failed")

	homeFile := filepath.Join(t.TempDir(), "home-file")
	require.NoError(t, os.WriteFile(homeFile, []byte("x"), 0o600))
	t.Setenv("HOME", homeFile)
	model, _ = updated.Update(reloginDoneMsg{apiKey: "nbl_new"})
	updated = model.(App)
	assert.Contains(t, updated.err, "save config")

	t.Setenv("HOME", t.TempDir())
	client := api.NewClient("http://127.0.0.1:9", "old")
	app = NewApp(client, &config.Config{APIKey: "old", Username: "alxx"})
	app.err = "stale"
	app.lastErrCode = "INVALID_API_KEY"
	app.lastErrMsg = "stale"
	app.showRecoveryHints = true
	model, cmd := app.Update(reloginDoneMsg{apiKey: "nbl_new"})
	updated = model.(App)
	require.NotNil(t, cmd)
	assert.Equal(t, "", updated.err)
	assert.Equal(t, "", updated.lastErrCode)
	assert.Equal(t, "", updated.lastErrMsg)
	assert.False(t, updated.showRecoveryHints)
	assert.Equal(t, "nbl_new", updated.config.APIKey)
	assert.Equal(t, "success", updated.toast.level)

	app = NewApp(nil, nil)
	app.onboarding = true
	app.onboardingBusy = true
	model, _ = app.Update(onboardingLoginDoneMsg{err: errors.New("bad creds")})
	updated = model.(App)
	assert.False(t, updated.onboardingBusy)
	assert.Contains(t, updated.err, "login failed")

	model, _ = updated.Update(onboardingLoginDoneMsg{})
	updated = model.(App)
	assert.Equal(t, "login failed: empty response", updated.err)

	t.Setenv("HOME", t.TempDir())
	resp := &api.LoginResponse{APIKey: "nbl_onboarding", EntityID: "ent-1", Username: "alxx"}
	model, cmd = updated.Update(onboardingLoginDoneMsg{resp: resp})
	updated = model.(App)
	require.NotNil(t, cmd)
	assert.False(t, updated.onboarding)
	assert.True(t, updated.startupChecking)
	assert.NotNil(t, updated.client)
	assert.Equal(t, "nbl_onboarding", updated.config.APIKey)
	assert.Equal(t, "alxx", updated.config.Username)

	model, _ = updated.Update(pendingLimitSavedMsg{limit: 77})
	updated = model.(App)
	assert.Equal(t, 77, updated.inbox.pendingLimit)
}

func TestUpdateMessageBranchesImportExportAndPalette(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.importExportOpen = true
	app.impex.closed = true

	model, cmd := app.Update(importExportDoneMsg{summary: "ok"})
	updated := model.(App)
	assert.False(t, updated.importExportOpen)
	assert.Nil(t, cmd)

	app = NewApp(nil, &config.Config{})
	app.importExportOpen = true
	app.impex.closed = true
	model, cmd = app.Update(importExportErrorMsg{err: errors.New("nope")})
	updated = model.(App)
	assert.False(t, updated.importExportOpen)
	assert.Nil(t, cmd)

	app = NewApp(nil, &config.Config{})
	app.paletteSearchQuery = "alpha"
	app.paletteSearchLoading = true
	app.paletteFiltered = []paletteAction{{ID: "old"}}
	model, _ = app.Update(paletteSearchLoadedMsg{query: "beta"})
	updated = model.(App)
	assert.True(t, updated.paletteSearchLoading)
	assert.Equal(t, []paletteAction{{ID: "old"}}, updated.paletteFiltered)
}

func TestUpdateKeyFlowRecoveryAndGlobalBranches(t *testing.T) {
	app := NewApp(nil, nil)
	app.onboarding = true
	model, _ := app.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	updated := model.(App)
	assert.Equal(t, "a", updated.onboardingName)

	app = NewApp(nil, &config.Config{})
	app.importExportOpen = true
	app.impex.step = stepResult
	model, _ = app.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = model.(App)
	assert.False(t, updated.importExportOpen)

	app = NewApp(nil, &config.Config{APIKey: "x", Username: "alxx"})
	app.quickstartOpen = true
	model, _ = app.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated = model.(App)
	assert.False(t, updated.quickstartOpen)

	app = NewApp(api.NewClient("http://127.0.0.1:9", "x"), &config.Config{APIKey: "x", Username: "alxx"})
	app.showRecoveryHints = true
	model, cmd := app.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	_ = model.(App)
	require.NotNil(t, cmd)

	model, cmd = app.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	updated = model.(App)
	assert.Equal(t, tabProfile, updated.tab)
	assert.Equal(t, 0, updated.profile.section)
	_ = cmd

	app.showRecoveryHints = true
	model, cmd = app.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	updated = model.(App)
	require.NotNil(t, cmd)
	assert.NotNil(t, updated.toast)
	assert.Equal(t, "info", updated.toast.level)

	app = NewApp(nil, &config.Config{})
	app.err = "stale"
	app.lastErrCode = "X"
	app.lastErrMsg = "Y"
	app.showRecoveryHints = true
	model, _ = app.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	updated = model.(App)
	assert.Equal(t, "", updated.err)
	assert.Equal(t, "", updated.lastErrCode)
	assert.Equal(t, "", updated.lastErrMsg)
	assert.False(t, updated.showRecoveryHints)

	model, _ = updated.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	updated = model.(App)
	assert.True(t, updated.helpOpen)

	updated.helpOpen = false
	updated.quickstartOpen = false
	updated.err = ""
	updated.know.view = contextViewAdd
	updated.know.addTitle = "draft"
	model, _ = updated.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	updated = model.(App)
	assert.True(t, updated.quitConfirm)

	updated = NewApp(nil, &config.Config{})
	model, cmd = updated.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	_ = model.(App)
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok)

	updated = NewApp(nil, &config.Config{})
	model, cmd = updated.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	updated = model.(App)
	assert.True(t, updated.paletteOpen)
	assert.Nil(t, cmd)

	updated = NewApp(nil, &config.Config{})
	updated.scrollTarget = 1
	model, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	updated = model.(App)
	assert.Equal(t, float64(0), updated.scrollTarget)

	updated = NewApp(nil, &config.Config{})
	updated.tab = tabJobs
	model, _ = updated.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	updated = model.(App)
	assert.Equal(t, tabInbox, updated.tab)
}

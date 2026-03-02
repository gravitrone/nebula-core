package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateOnboardingLoginDoneSaveConfigFailureBranch(t *testing.T) {
	app := NewApp(nil, nil)
	app.onboarding = true
	app.onboardingBusy = true

	homeFile := filepath.Join(t.TempDir(), "home-file")
	require.NoError(t, os.WriteFile(homeFile, []byte("x"), 0o600))
	t.Setenv("HOME", homeFile)

	model, cmd := app.Update(onboardingLoginDoneMsg{
		resp: &api.LoginResponse{
			APIKey:   "nbl_save_fail",
			EntityID: "ent-1",
			Username: "alxx",
		},
	})
	updated := model.(App)

	assert.Nil(t, cmd)
	assert.False(t, updated.onboardingBusy)
	assert.True(t, updated.onboarding)
	assert.Contains(t, updated.err, "save config")
}

func TestUpdateOnboardingLoginDoneUsesExistingClientBranch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	client := api.NewClient("http://127.0.0.1:9", "old")
	app := NewApp(client, nil)
	app.onboarding = true
	app.onboardingBusy = true

	model, cmd := app.Update(onboardingLoginDoneMsg{
		resp: &api.LoginResponse{
			APIKey:   "nbl_new",
			EntityID: "ent-2",
			Username: "alxx",
		},
	})
	updated := model.(App)

	require.NotNil(t, cmd)
	assert.Same(t, client, updated.client)
	assert.False(t, updated.onboarding)
	assert.False(t, updated.onboardingBusy)
	assert.Equal(t, "nbl_new", updated.config.APIKey)
	assert.Equal(t, "alxx", updated.config.Username)
	assert.Equal(t, "ent-2", updated.config.UserEntityID)
}

func TestUpdateTabNavArrowLeftRightSwitchBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tabNav = true
	app.tab = tabInbox

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated := model.(App)
	require.NotNil(t, cmd)
	assert.Equal(t, tabCount-1, updated.tab)
	assert.True(t, updated.tabNav)

	model, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyRight})
	updated = model.(App)
	require.NotNil(t, cmd)
	assert.Equal(t, tabInbox, updated.tab)
	assert.True(t, updated.tabNav)
}

func TestUpdateDelegatesUnhandledMessageAcrossTabs(t *testing.T) {
	tabs := []int{
		tabInbox,
		tabEntities,
		tabRelations,
		tabKnow,
		tabJobs,
		tabLogs,
		tabFiles,
		tabProtocols,
		tabHistory,
		tabProfile,
	}

	for _, tab := range tabs {
		t.Run(tabNames[tab], func(t *testing.T) {
			app := NewApp(nil, &config.Config{})
			app.tab = tab
			app.tabNav = false

			model, cmd := app.Update(struct{ name string }{name: "noop"})
			updated := model.(App)
			assert.Equal(t, tab, updated.tab)
			assert.Nil(t, cmd)
		})
	}
}

func TestUpdateCombinesDelegateAndToastCommands(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = false

	model, cmd := app.Update(entityCreatedMsg{})
	updated := model.(App)

	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "success", updated.toast.level)
	assert.True(t, updated.entities.loading)
}

func TestUpdateReturnsToastCommandWhenDelegateReturnsNil(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = false

	model, cmd := app.Update(entityUpdatedMsg{})
	updated := model.(App)

	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "success", updated.toast.level)
	assert.Equal(t, entitiesViewDetail, updated.entities.view)
}

func TestUpdateStartupCheckedAPIFailureSetsMissingAndFailedState(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.startupChecking = true
	app.startup = startupSummary{
		API:      "checking",
		Auth:     "checking",
		Taxonomy: "checking",
	}

	model, cmd := app.Update(startupCheckedMsg{
		apiErr:      "dial tcp 127.0.0.1:8000: connect: connection refused",
		authErr:     "HTTP 401: Unauthorized",
		taxonomyErr: "timeout",
	})
	updated := model.(App)

	require.NotNil(t, cmd)
	assert.False(t, updated.startupChecking)
	assert.True(t, updated.startup.Done)
	assert.Equal(t, "down", updated.startup.API)
	assert.Equal(t, "missing", updated.startup.Auth)
	assert.Equal(t, "failed", updated.startup.Taxonomy)
	assert.Equal(t, "", updated.err)
	assert.False(t, updated.showRecoveryHints)
}

func TestUpdateImportExportMessagesNoopWhenDialogClosed(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.importExportOpen = false

	model, cmd := app.Update(importExportDoneMsg{summary: "done"})
	updated := model.(App)
	assert.False(t, updated.importExportOpen)
	assert.Nil(t, cmd)

	model, cmd = updated.Update(importExportErrorMsg{err: assert.AnError})
	updated = model.(App)
	assert.False(t, updated.importExportOpen)
	assert.Nil(t, cmd)
}

func TestUpdateHelpOpenIgnoresUnrelatedKeys(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.helpOpen = true

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	updated := model.(App)

	assert.True(t, updated.helpOpen)
	assert.Nil(t, cmd)
}

func TestUpdateHelpOpenClosesOnQuestionAndBackKeys(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.helpOpen = true

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	updated := model.(App)
	assert.False(t, updated.helpOpen)
	assert.Nil(t, cmd)

	updated.helpOpen = true
	model, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated = model.(App)
	assert.False(t, updated.helpOpen)
	assert.Nil(t, cmd)
}

func TestUpdateQuitConfirmCancelsOnNoAndBack(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.quitConfirm = true

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	updated := model.(App)
	assert.False(t, updated.quitConfirm)
	assert.Nil(t, cmd)

	updated.quitConfirm = true
	model, cmd = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated = model.(App)
	assert.False(t, updated.quitConfirm)
	assert.Nil(t, cmd)
}

func TestShouldShowMultiAPIRecoveryHintMatrix(t *testing.T) {
	assert.True(t, shouldShowMultiAPIRecoveryHint("MULTIPLE_API_INSTANCES_DETECTED", "", ""))
	assert.True(t, shouldShowMultiAPIRecoveryHint("", "address already in use", ""))
	assert.True(t, shouldShowMultiAPIRecoveryHint("", "", "ERROR: [Errno 98] Address already in use"))
	assert.True(t, shouldShowMultiAPIRecoveryHint("", "listen failed", "EADDRINUSE"))
	assert.False(t, shouldShowMultiAPIRecoveryHint("", "unauthorized", "invalid api key"))
}

func TestViewAddsMultiAPIRecoveryHintForConflictPattern(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 100
	app.height = 34
	app.err = "HTTP 500: address already in use"
	app.lastErrCode = ""
	app.lastErrMsg = ""

	view := app.View()
	assert.Contains(t, view, "stop duplicate API processes and restart with `nebula start`")
}

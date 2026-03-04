package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunPaletteActionTabCommandMatrix(t *testing.T) {
	cases := []struct {
		id      string
		wantTab int
	}{
		{id: "tab:inbox", wantTab: tabInbox},
		{id: "tab:entities", wantTab: tabEntities},
		{id: "tab:relationships", wantTab: tabRelations},
		{id: "tab:context", wantTab: tabKnow},
		{id: "tab:jobs", wantTab: tabJobs},
		{id: "tab:history", wantTab: tabHistory},
		{id: "tab:settings", wantTab: tabProfile},
		{id: "tab:profile", wantTab: tabProfile},
	}

	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			app := NewApp(nil, &config.Config{})
			model, _ := app.runPaletteAction(paletteAction{ID: tc.id})
			updated := model.(App)
			assert.Equal(t, tc.wantTab, updated.tab)
		})
	}
}

func TestRunPaletteActionRoutesContextAndJobSelections(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteSelections = map[string]paletteSelection{
		"context:ctx-1": {context: &api.Context{ID: "ctx-1"}},
		"job:job-1":     {job: &api.Job{ID: "job-1"}},
	}

	model, _ := app.runPaletteAction(paletteAction{ID: "context:ctx-1"})
	updated := model.(App)
	assert.Equal(t, tabKnow, updated.tab)
	assert.Equal(t, contextViewDetail, updated.know.view)
	require.NotNil(t, updated.know.detail)
	assert.Equal(t, "ctx-1", updated.know.detail.ID)

	model, _ = updated.runPaletteAction(paletteAction{ID: "job:job-1"})
	updated = model.(App)
	assert.Equal(t, tabJobs, updated.tab)
	require.NotNil(t, updated.jobs.detail)
	assert.Equal(t, "job-1", updated.jobs.detail.ID)
}

func TestRunPaletteActionStartsImportExportModes(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	model, _ := app.runPaletteAction(paletteAction{ID: "ops:import"})
	updated := model.(App)
	assert.True(t, updated.importExportOpen)
	assert.Equal(t, importMode, updated.impex.mode)
	assert.Equal(t, stepResource, updated.impex.step)

	model, _ = updated.runPaletteAction(paletteAction{ID: "ops:export"})
	updated = model.(App)
	assert.True(t, updated.importExportOpen)
	assert.Equal(t, exportMode, updated.impex.mode)
	assert.Equal(t, stepResource, updated.impex.step)
}

func TestRunPaletteActionQuitBehavior(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.inbox.rejecting = true

	model, cmd := app.runPaletteAction(paletteAction{ID: "quit"})
	updated := model.(App)
	assert.True(t, updated.quitConfirm)
	assert.Nil(t, cmd)

	app = NewApp(nil, &config.Config{})
	model, cmd = app.runPaletteAction(paletteAction{ID: "quit"})
	updated = model.(App)
	assert.False(t, updated.quitConfirm)
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok)
}

func TestRunPaletteActionUnknownIsNoop(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabLogs

	model, cmd := app.runPaletteAction(paletteAction{ID: "unknown:action"})
	updated := model.(App)
	assert.Equal(t, tabLogs, updated.tab)
	assert.Nil(t, cmd)
}

func TestTabWantsArrowsBranchMatrixExt(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*App)
		want  bool
	}{
		{name: "context_tab_always", setup: func(a *App) { a.tab = tabKnow }, want: true},
		{name: "inbox_default_false", setup: func(a *App) { a.tab = tabInbox }, want: false},
		{
			name: "inbox_detail_true",
			setup: func(a *App) {
				a.tab = tabInbox
				approval := a.inbox.detail
				if approval == nil {
					approval = &api.Approval{ID: "ap-1"}
				}
				a.inbox.detail = approval
			},
			want: true,
		},
		{
			name: "entities_detail_true",
			setup: func(a *App) {
				a.tab = tabEntities
				a.entities.view = entitiesViewDetail
			},
			want: true,
		},
		{
			name: "jobs_changing_status_true",
			setup: func(a *App) {
				a.tab = tabJobs
				a.jobs.changingSt = true
			},
			want: true,
		},
		{
			name: "profile_prompt_true",
			setup: func(a *App) {
				a.tab = tabProfile
				a.profile.taxPromptMode = taxPromptFilter
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp(nil, &config.Config{})
			tc.setup(&app)
			assert.Equal(t, tc.want, app.tabWantsArrows())
		})
	}
}

func TestStartupStatusColorMatrix(t *testing.T) {
	assert.Equal(t, string(ColorSuccess), startupStatusColor("ok"))
	assert.Equal(t, string(ColorMuted), startupStatusColor("checking"))
	assert.Equal(t, string(ColorWarning), startupStatusColor("missing"))
	assert.Equal(t, string(ColorWarning), startupStatusColor("forbidden"))
	assert.Equal(t, string(ColorWarning), startupStatusColor("timeout"))
	assert.Equal(t, string(ColorError), startupStatusColor("invalid"))
	assert.Equal(t, string(ColorError), startupStatusColor("down"))
	assert.Equal(t, string(ColorError), startupStatusColor("failed"))
	assert.Equal(t, string(ColorError), startupStatusColor("schema_error"))
	assert.Equal(t, string(ColorError), startupStatusColor("multi_api_conflict"))
	assert.Equal(t, string(ColorMuted), startupStatusColor("unknown-status"))
}

func TestViewStartupPanelAndErrorRecoveryHints(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 110
	app.height = 40
	app.startupChecking = true
	app.startup = startupSummary{API: "checking", Auth: "checking", Taxonomy: "checking"}
	app.err = "INVALID_API_KEY: bad token"
	app.showRecoveryHints = true

	out := components.SanitizeText(app.View())
	assert.Contains(t, out, "Recovery:")
	assert.Contains(t, out, "re-login")
}

func TestViewOverlayPriorityPrefersQuitConfirm(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 100
	app.height = 30
	app.helpOpen = true
	app.paletteOpen = true
	app.importExportOpen = true
	app.onboarding = true
	app.quickstartOpen = true
	app.quitConfirm = true

	out := components.SanitizeText(app.View())
	assert.Contains(t, out, "You have unsaved changes")
	assert.NotContains(t, out, "Command Palette")
	assert.NotContains(t, out, "Getting Started")
}

func TestViewPaletteAndImportExportOverlays(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 100
	app.height = 36
	app.openPaletteCommand()

	out := components.SanitizeText(app.View())
	_ = out

	app.paletteOpen = false
	app.importExportOpen = true
	app.impex.Start(importMode)
	_ = components.SanitizeText(app.View())
}

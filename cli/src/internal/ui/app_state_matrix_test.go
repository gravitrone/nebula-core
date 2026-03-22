package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderHelpBarContainsGlobalBindings(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 120
	bar := components.SanitizeText(app.renderHelpBar())
	assert.Contains(t, strings.ToLower(bar), "quit")
	assert.Contains(t, strings.ToLower(bar), "help")
}

func TestRenderHelpBarRendersAtWidth(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 80
	bar := app.renderHelpBar()
	require.NotEmpty(t, bar)
	assert.NotContains(t, bar, "\x1b]")
}

func TestRenderHelpBarWithZeroWidth(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 0
	bar := app.renderHelpBar()
	require.NotEmpty(t, bar)
}

func TestRowHighlightEnabledTabMatrix(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*App)
		want  bool
	}{
		{
			name:  "inbox list enabled",
			setup: func(a *App) { a.tab = tabInbox },
			want:  true,
		},
		{
			name: "relations metadata editor",
			setup: func(a *App) {
				a.tab = tabRelations
				a.rels.editMeta.Active = true
			},
			want: true,
		},
		{
			name: "logs add value",
			setup: func(a *App) {
				a.tab = tabLogs
				a.logs.addValue.Active = true
			},
			want: true,
		},
		{
			name:  "files list enabled",
			setup: func(a *App) { a.tab = tabFiles },
			want:  true,
		},
		{
			name: "history filtering disabled",
			setup: func(a *App) {
				a.tab = tabHistory
				a.history.filtering = true
			},
			want: false,
		},
		{
			name: "profile detail open disabled",
			setup: func(a *App) {
				a.tab = tabProfile
				a.profile.agentDetail = &api.Agent{ID: "ag-1"}
			},
			want: false,
		},
		{
			name: "tab nav always disabled",
			setup: func(a *App) {
				a.tab = tabEntities
				a.tabNav = true
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp(nil, &config.Config{})
			app.tabNav = false
			tc.setup(&app)
			assert.Equal(t, tc.want, app.rowHighlightEnabled())
		})
	}
}

func TestViewStateKeyCoversOverlaysAndTabStates(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	app.helpOpen = true
	assert.Equal(t, "help", app.viewStateKey())
	app.helpOpen = false

	app.quitConfirm = true
	assert.Equal(t, "quit-confirm", app.viewStateKey())
	app.quitConfirm = false

	app.paletteOpen = true
	assert.Equal(t, "palette", app.viewStateKey())
	app.paletteOpen = false

	app.importExportOpen = true
	assert.Equal(t, "import-export", app.viewStateKey())
	app.importExportOpen = false

	app.onboarding = true
	assert.Equal(t, "onboarding", app.viewStateKey())
	app.onboarding = false

	app.quickstartOpen = true
	assert.Equal(t, "quickstart", app.viewStateKey())
	app.quickstartOpen = false

	app.tab = tabInbox
	app.inbox.rejectPreview = true
	assert.Equal(t, "tab:0:inbox:reject-preview", app.viewStateKey())
	app.inbox.rejectPreview = false
	app.inbox.detail = &api.Approval{ID: "ap-1"}
	assert.Equal(t, "tab:0:inbox:detail", app.viewStateKey())

	app = NewApp(nil, &config.Config{})
	app.tab = tabJobs
	app.jobs.changingSt = true
	assert.Equal(t, "tab:4:jobs:status", app.viewStateKey())
	app.jobs.changingSt = false
	app.jobs.creatingSubtask = true
	assert.Equal(t, "tab:4:jobs:subtask", app.viewStateKey())

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.section = 2
	app.profile.sectionFocus = true
	assert.Equal(t, "tab:9:settings:2:sections", app.viewStateKey())
}

func TestTabWantsArrowsMatrix(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabKnow
	assert.True(t, app.tabWantsArrows())

	app.tab = tabInbox
	assert.False(t, app.tabWantsArrows())
	app.inbox.detail = &api.Approval{ID: "ap-1"}
	assert.True(t, app.tabWantsArrows())

	app = NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.entities.view = entitiesViewList
	assert.False(t, app.tabWantsArrows())
	app.entities.view = entitiesViewDetail
	assert.True(t, app.tabWantsArrows())

	app = NewApp(nil, &config.Config{})
	app.tab = tabJobs
	assert.False(t, app.tabWantsArrows())
	app.jobs.detail = &api.Job{ID: "job-1"}
	assert.True(t, app.tabWantsArrows())

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.taxPromptMode = taxPromptEditName
	assert.True(t, app.tabWantsArrows())
}

func TestInitTabReturnsNilForUnknownTab(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	assert.Nil(t, app.initTab(999))
}

func TestToastCmdForMsgMatrix(t *testing.T) {
	cases := []struct {
		name string
		msg  any
		want string
	}{
		{name: "approval", msg: approvalDoneMsg{}, want: "approval action completed."},
		{name: "entity created", msg: entityCreatedMsg{}, want: "entity created."},
		{name: "entity updated", msg: entityUpdatedMsg{}, want: "entity updated."},
		{name: "entity reverted", msg: entityRevertedMsg{}, want: "entity reverted."},
		{name: "relationship created", msg: relationshipCreatedMsg{}, want: "relationship created."},
		{name: "relationship updated", msg: relationshipUpdatedMsg{}, want: "relationship updated."},
		{name: "context saved", msg: contextSavedMsg{}, want: "context saved."},
		{name: "context updated", msg: contextUpdatedMsg{}, want: "context saved."},
		{name: "job created", msg: jobCreatedMsg{}, want: "job created."},
		{name: "job status", msg: jobStatusUpdatedMsg{}, want: "job status updated."},
		{name: "subtask", msg: subtaskCreatedMsg{}, want: "subtask created."},
		{name: "log created", msg: logCreatedMsg{}, want: "log saved."},
		{name: "log updated", msg: logUpdatedMsg{}, want: "log saved."},
		{name: "file created", msg: fileCreatedMsg{}, want: "file saved."},
		{name: "file updated", msg: fileUpdatedMsg{}, want: "file saved."},
		{name: "protocol created", msg: protocolCreatedMsg{}, want: "protocol saved."},
		{name: "protocol updated", msg: protocolUpdatedMsg{}, want: "protocol saved."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := NewApp(nil, &config.Config{})
			cmd := app.toastCmdForMsg(tc.msg)
			require.NotNil(t, cmd)
			require.NotNil(t, app.toast)
			assert.Equal(t, "success", app.toast.level)
			assert.Contains(t, strings.ToLower(app.toast.text), tc.want)
		})
	}

	app := NewApp(nil, &config.Config{})
	assert.Nil(t, app.toastCmdForMsg(struct{}{}))
}

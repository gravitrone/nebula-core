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

func hintsText(hints []string) string {
	return strings.ToLower(components.SanitizeText(strings.Join(hints, "\n")))
}

func assertHintsContain(t *testing.T, hints []string, parts ...string) {
	t.Helper()
	text := hintsText(hints)
	for _, part := range parts {
		assert.Contains(t, text, strings.ToLower(part))
	}
}

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
	// Should render without panic even at zero width.
	require.NotEmpty(t, bar)
}

func TestStatusHintsForInboxStates(t *testing.T) {
	base := NewApp(nil, &config.Config{})
	base.tab = tabInbox

	cases := []struct {
		name  string
		setup func(*App)
		want  []string
	}{
		{
			name: "confirming",
			setup: func(a *App) {
				a.inbox.confirming = true
			},
			want: []string{"confirm", "cancel", "aliases"},
		},
		{
			name: "filtering",
			setup: func(a *App) {
				a.inbox.filtering = true
			},
			want: []string{"apply", "clear"},
		},
		{
			name: "rejecting",
			setup: func(a *App) {
				a.inbox.rejecting = true
			},
			want: []string{"submit", "cancel"},
		},
		{
			name: "detail",
			setup: func(a *App) {
				a.inbox.detail = &api.Approval{ID: "ap-1"}
			},
			want: []string{"approve", "reject", "back"},
		},
		{
			name: "list",
			setup: func(a *App) {
				a.inbox.detail = nil
			},
			want: []string{"select all", "approve all", "details", "filter"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := base
			tc.setup(&app)
			assertHintsContain(t, app.statusHintsForTab(), tc.want...)
		})
	}
}

func TestStatusHintsForEntitiesStates(t *testing.T) {
	base := NewApp(nil, &config.Config{})
	base.tab = tabEntities

	cases := []struct {
		name  string
		setup func(*App)
		want  []string
	}{
		{
			name: "bulk prompt",
			setup: func(a *App) {
				a.entities.bulkPrompt = "tags"
			},
			want: []string{"apply", "cancel"},
		},
		{
			name: "filtering",
			setup: func(a *App) {
				a.entities.filtering = true
			},
			want: []string{"apply", "clear"},
		},
		{
			name: "detail context prompt",
			setup: func(a *App) {
				a.entities.view = entitiesViewDetail
				a.entities.contextCreating = true
			},
			want: []string{"confirm", "cancel"},
		},
		{
			name: "detail basic",
			setup: func(a *App) {
				a.entities.view = entitiesViewDetail
			},
			want: []string{"add context", "link context", "edit", "history", "relationships", "archive", "back"},
		},
		{
			name: "edit",
			setup: func(a *App) {
				a.entities.view = entitiesViewEdit
			},
			want: []string{"fields", "cycle", "save", "cancel"},
		},
		{
			name: "relationships",
			setup: func(a *App) {
				a.entities.view = entitiesViewRelationships
			},
			want: []string{"new", "edit", "archive", "back"},
		},
		{
			name: "history",
			setup: func(a *App) {
				a.entities.view = entitiesViewHistory
			},
			want: []string{"revert", "back"},
		},
		{
			name: "confirm",
			setup: func(a *App) {
				a.entities.view = entitiesViewConfirm
			},
			want: []string{"confirm", "cancel", "aliases"},
		},
		{
			name: "list with search text",
			setup: func(a *App) {
				a.entities.view = entitiesViewList
				a.entities.searchBuf = "alpha"
			},
			want: []string{"scroll", "complete", "details", "filter"},
		},
		{
			name: "list with bulk selected",
			setup: func(a *App) {
				a.entities.view = entitiesViewList
				a.entities.bulkSelected = map[string]bool{"ent-1": true}
			},
			want: []string{"tags", "scopes", "clear"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := base
			tc.setup(&app)
			assertHintsContain(t, app.statusHintsForTab(), tc.want...)
		})
	}
}

func TestStatusHintsForRelationsStates(t *testing.T) {
	base := NewApp(nil, &config.Config{})
	base.tab = tabRelations

	cases := []struct {
		name  string
		setup func(*App)
		want  []string
	}{
		{
			name: "filtering",
			setup: func(a *App) {
				a.rels.filtering = true
			},
			want: []string{"apply", "clear"},
		},
		{
			name: "detail",
			setup: func(a *App) {
				a.rels.view = relsViewDetail
			},
			want: []string{"edit", "archive", "back"},
		},
		{
			name: "edit",
			setup: func(a *App) {
				a.rels.view = relsViewEdit
			},
			want: []string{"fields", "cycle", "save", "cancel"},
		},
		{
			name: "confirm",
			setup: func(a *App) {
				a.rels.view = relsViewConfirm
			},
			want: []string{"confirm", "cancel", "aliases"},
		},
		{
			name: "create search",
			setup: func(a *App) {
				a.rels.view = relsViewCreateSourceSearch
			},
			want: []string{"scroll", "select", "back"},
		},
		{
			name: "create type",
			setup: func(a *App) {
				a.rels.view = relsViewCreateType
			},
			want: []string{"create", "back"},
		},
		{
			name: "list",
			setup: func(a *App) {
				a.rels.view = relsViewList
			},
			want: []string{"details", "new", "filter"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := base
			tc.setup(&app)
			assertHintsContain(t, app.statusHintsForTab(), tc.want...)
		})
	}
}

func TestStatusHintsForContextStates(t *testing.T) {
	base := NewApp(nil, &config.Config{})
	base.tab = tabKnow

	cases := []struct {
		name  string
		setup func(*App)
		want  []string
	}{
		{
			name: "filtering",
			setup: func(a *App) {
				a.know.filtering = true
			},
			want: []string{"apply", "clear"},
		},
		{
			name: "link searching",
			setup: func(a *App) {
				a.know.linkSearching = true
			},
			want: []string{"scroll", "select", "cancel"},
		},
		{
			name: "list",
			setup: func(a *App) {
				a.know.view = contextViewList
			},
			want: []string{"details", "filter", "back"},
		},
		{
			name: "detail",
			setup: func(a *App) {
				a.know.view = contextViewDetail
			},
			want: []string{"content", "source", "back"},
		},
		{
			name: "add",
			setup: func(a *App) {
				a.know.view = contextViewAdd
			},
			want: []string{"fields", "cycle", "save", "cancel"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := base
			tc.setup(&app)
			assertHintsContain(t, app.statusHintsForTab(), tc.want...)
		})
	}
}

func TestStatusHintsForOtherTabs(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabJobs
	app.jobs.detail = &api.Job{ID: "job-1"}
	assertHintsContain(t, app.statusHintsForTab(), "add context", "link context", "status", "subtask", "link", "unlink")

	app = NewApp(nil, &config.Config{})
	app.tab = tabLogs
	app.logs.view = logsViewDetail
	assertHintsContain(t, app.statusHintsForTab(), "edit", "value", "metadata")

	app = NewApp(nil, &config.Config{})
	app.tab = tabFiles
	app.files.view = filesViewDetail
	assertHintsContain(t, app.statusHintsForTab(), "edit", "metadata", "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProtocols
	app.protocols.view = protocolsViewDetail
	assertHintsContain(t, app.statusHintsForTab(), "edit", "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabHistory
	app.history.view = historyViewDetail
	assertHintsContain(t, app.statusHintsForTab(), "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.section = 2
	assertHintsContain(t, app.statusHintsForTab(), "kind", "new", "archive", "activate", "inactive")
}

func TestRowHighlightEnabledTabMatrix(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*App)
		want  bool
	}{
		{
			name: "inbox list enabled",
			setup: func(a *App) {
				a.tab = tabInbox
			},
			want: true,
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
			name: "files list enabled",
			setup: func(a *App) {
				a.tab = tabFiles
			},
			want: true,
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

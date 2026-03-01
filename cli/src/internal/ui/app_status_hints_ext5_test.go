package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
)

func TestStatusHintsForEntitiesAdditionalViews(t *testing.T) {
	base := NewApp(nil, &config.Config{})
	base.tab = tabEntities

	cases := []struct {
		name  string
		setup func(*App)
		want  []string
	}{
		{
			name: "relate search",
			setup: func(a *App) {
				a.entities.view = entitiesViewRelateSearch
			},
			want: []string{"search", "back"},
		},
		{
			name: "relate select",
			setup: func(a *App) {
				a.entities.view = entitiesViewRelateSelect
			},
			want: []string{"scroll", "select", "back"},
		},
		{
			name: "relate type",
			setup: func(a *App) {
				a.entities.view = entitiesViewRelateType
			},
			want: []string{"create", "back"},
		},
		{
			name: "rel edit",
			setup: func(a *App) {
				a.entities.view = entitiesViewRelEdit
			},
			want: []string{"fields", "cycle", "save", "cancel"},
		},
		{
			name: "add",
			setup: func(a *App) {
				a.entities.view = entitiesViewAdd
			},
			want: []string{"fields", "cycle", "save", "back"},
		},
		{
			name: "search",
			setup: func(a *App) {
				a.entities.view = entitiesViewSearch
			},
			want: []string{"search", "back"},
		},
		{
			name: "list empty search includes select",
			setup: func(a *App) {
				a.entities.view = entitiesViewList
				a.entities.searchBuf = ""
			},
			want: []string{"scroll", "complete", "details", "filter", "select"},
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

func TestStatusHintsForRemainingTabStates(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	app.tab = tabJobs
	app.jobs.filtering = true
	assertHintsContain(t, app.statusHintsForTab(), "apply", "clear")

	app = NewApp(nil, &config.Config{})
	app.tab = tabJobs
	app.jobs.view = jobsViewEdit
	assertHintsContain(t, app.statusHintsForTab(), "fields", "cycle", "save", "cancel")

	app = NewApp(nil, &config.Config{})
	app.tab = tabJobs
	app.jobs.view = jobsViewList
	assertHintsContain(t, app.statusHintsForTab(), "scroll", "details", "select all", "status", "filter")

	app = NewApp(nil, &config.Config{})
	app.tab = tabLogs
	app.logs.filtering = true
	assertHintsContain(t, app.statusHintsForTab(), "apply", "clear")

	app = NewApp(nil, &config.Config{})
	app.tab = tabLogs
	app.logs.view = logsViewAdd
	assertHintsContain(t, app.statusHintsForTab(), "fields", "cycle", "save", "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabLogs
	app.logs.view = logsViewList
	assertHintsContain(t, app.statusHintsForTab(), "scroll", "details", "complete", "filter")

	app = NewApp(nil, &config.Config{})
	app.tab = tabFiles
	app.files.filtering = true
	assertHintsContain(t, app.statusHintsForTab(), "apply", "clear")

	app = NewApp(nil, &config.Config{})
	app.tab = tabFiles
	app.files.view = filesViewEdit
	assertHintsContain(t, app.statusHintsForTab(), "fields", "cycle", "save", "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabFiles
	app.files.view = filesViewList
	assertHintsContain(t, app.statusHintsForTab(), "scroll", "details", "complete", "filter")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProtocols
	app.protocols.filtering = true
	assertHintsContain(t, app.statusHintsForTab(), "apply", "clear")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProtocols
	app.protocols.view = protocolsViewEdit
	assertHintsContain(t, app.statusHintsForTab(), "fields", "cycle", "save", "cancel")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProtocols
	app.protocols.view = protocolsViewList
	assertHintsContain(t, app.statusHintsForTab(), "scroll", "new", "details", "filter")

	app = NewApp(nil, &config.Config{})
	app.tab = tabHistory
	app.history.filtering = true
	assertHintsContain(t, app.statusHintsForTab(), "apply", "clear")

	app = NewApp(nil, &config.Config{})
	app.tab = tabHistory
	app.history.view = historyViewScopes
	assertHintsContain(t, app.statusHintsForTab(), "scroll", "select", "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabHistory
	app.history.view = historyViewActors
	assertHintsContain(t, app.statusHintsForTab(), "scroll", "select", "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabHistory
	app.history.view = historyViewList
	assertHintsContain(t, app.statusHintsForTab(), "scroll", "details", "filter", "scopes", "actors")
}

func TestStatusHintsForProfileSectionBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.agentDetail = &api.Agent{ID: "ag-1"}
	assertHintsContain(t, app.statusHintsForTab(), "back")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.section = 0
	assertHintsContain(t, app.statusHintsForTab(), "api key", "queue limit", "new key", "revoke")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.section = 1
	assertHintsContain(t, app.statusHintsForTab(), "details", "toggle trust")

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.section = 2
	app.profile.taxPromptMode = taxPromptEditName
	assertHintsContain(t, app.statusHintsForTab(), "apply", "cancel")
}

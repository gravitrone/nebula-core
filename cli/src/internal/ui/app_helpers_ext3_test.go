package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountViewLinesBranches(t *testing.T) {
	assert.Equal(t, 0, countViewLines(""))
	assert.Equal(t, 0, countViewLines(" \n\t "))
	assert.Equal(t, 1, countViewLines("one"))
	assert.Equal(t, 3, countViewLines("one\ntwo\nthree"))
}

func TestTabWantsArrowsProfileStateMatrix(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabProfile

	assert.False(t, app.tabWantsArrows())

	app.profile.creating = true
	assert.True(t, app.tabWantsArrows())
	app.profile.creating = false

	app.profile.createdKey = "nbl_created"
	assert.True(t, app.tabWantsArrows())
	app.profile.createdKey = ""

	app.profile.taxPromptMode = taxPromptEditName
	assert.True(t, app.tabWantsArrows())
}

func TestTabWantsArrowsFullSwitchCoverage(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	app.tab = tabInbox
	assert.False(t, app.tabWantsArrows())
	app.inbox.rejecting = true
	assert.True(t, app.tabWantsArrows())

	app.tab = tabEntities
	app.entities.view = entitiesViewList
	assert.False(t, app.tabWantsArrows())
	app.entities.view = entitiesViewDetail
	assert.True(t, app.tabWantsArrows())

	app.tab = tabRelations
	app.rels.view = relsViewList
	assert.False(t, app.tabWantsArrows())
	app.rels.view = relsViewCreateSourceSearch
	assert.True(t, app.tabWantsArrows())

	app.tab = tabJobs
	app.jobs.view = jobsViewList
	app.jobs.detail = nil
	app.jobs.changingSt = false
	assert.False(t, app.tabWantsArrows())
	app.jobs.changingSt = true
	assert.True(t, app.tabWantsArrows())

	app.tab = tabLogs
	app.logs.view = logsViewList
	assert.False(t, app.tabWantsArrows())
	app.logs.view = logsViewDetail
	assert.True(t, app.tabWantsArrows())

	app.tab = tabFiles
	app.files.view = filesViewList
	assert.False(t, app.tabWantsArrows())
	app.files.view = filesViewDetail
	assert.True(t, app.tabWantsArrows())

	app.tab = tabProfile
	app.profile.creating = false
	app.profile.createdKey = ""
	app.profile.taxPromptMode = taxPromptNone
	assert.False(t, app.tabWantsArrows())

	app.tab = 999
	assert.False(t, app.tabWantsArrows())
}

func TestFinishQuickstartSuccessAndSkippedBranches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &config.Config{APIKey: "key", Username: "alxx", QuickstartPending: true}
	app := NewApp(nil, cfg)
	app.quickstartOpen = true
	app.quickstartStep = 2

	model, cmd := app.finishQuickstart(false)
	updated := model.(App)
	assert.False(t, updated.quickstartOpen)
	assert.Equal(t, 0, updated.quickstartStep)
	assert.False(t, updated.config.QuickstartPending)
	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "success", updated.toast.level)
	assert.Equal(t, "Quickstart complete.", updated.toast.text)

	model, cmd = updated.finishQuickstart(true)
	updated = model.(App)
	require.NotNil(t, cmd)
	require.NotNil(t, updated.toast)
	assert.Equal(t, "info", updated.toast.level)
	assert.Equal(t, "Quickstart skipped.", updated.toast.text)
}

func TestFinishQuickstartSaveErrorBranch(t *testing.T) {
	tmp := t.TempDir()
	homeAsFile := filepath.Join(tmp, "home-file")
	require.NoError(t, os.WriteFile(homeAsFile, []byte("x"), 0o644))
	t.Setenv("HOME", homeAsFile)

	cfg := &config.Config{APIKey: "key", Username: "alxx", QuickstartPending: true}
	app := NewApp(nil, cfg)
	app.quickstartOpen = true
	app.quickstartStep = 1

	model, cmd := app.finishQuickstart(false)
	updated := model.(App)
	assert.False(t, updated.quickstartOpen)
	assert.Equal(t, 0, updated.quickstartStep)
	assert.NotEmpty(t, updated.err)
	assert.Contains(t, updated.err, "save config")
	assert.Nil(t, cmd)
}

func TestSetToastCommandReturnsClearToastMessage(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	cmd := app.setToast("warning", "hello")
	require.NotNil(t, cmd)
	require.NotNil(t, app.toast)
	assert.Equal(t, "warning", app.toast.level)
	assert.Equal(t, "hello", app.toast.text)

	msg := cmd()
	_, ok := msg.(clearToastMsg)
	assert.True(t, ok)
}

func TestAppInitOnboardingAndNonStartupBranches(t *testing.T) {
	onboarding := NewApp(nil, nil)
	assert.Nil(t, onboarding.Init())

	client := api.NewClient("http://127.0.0.1:9", "key")
	cfg := &config.Config{APIKey: "key", Username: "alxx"}
	app := NewApp(client, cfg)
	app.startupChecking = false

	cmd := app.Init()
	require.NotNil(t, cmd)
}

func TestHandleOnboardingKeysQuitBusyAndEnterBranches(t *testing.T) {
	app := NewApp(nil, nil)
	app.onboarding = true

	model, cmd := app.handleOnboardingKeys(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd)
	_, ok := model.(App)
	require.True(t, ok)
	_, ok = cmd().(tea.QuitMsg)
	assert.True(t, ok)

	app.onboardingBusy = true
	app.onboardingName = "alxx"
	model, cmd = app.handleOnboardingKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	updated := model.(App)
	assert.Equal(t, "alxx", updated.onboardingName)

	app.onboardingBusy = false
	model, cmd = app.handleOnboardingKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	updated = model.(App)
	assert.True(t, updated.onboardingBusy)
	assert.Equal(t, "", updated.err)
}

func TestRefreshPaletteFilteredBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteActions = []paletteAction{
		{ID: "tab:inbox", Label: "Inbox", Desc: "approvals"},
		{ID: "tab:jobs", Label: "Jobs", Desc: "tasks"},
	}

	app.paletteTextInput.SetValue("/job")
	app.paletteSearchQuery = "old"
	app.paletteSearchLoading = true
	app.paletteSelections = map[string]paletteSelection{"x": {}}
	app.paletteIndex = 9
	cmd := app.refreshPaletteFiltered()
	assert.Nil(t, cmd)
	assert.Equal(t, "", app.paletteSearchQuery)
	assert.False(t, app.paletteSearchLoading)
	assert.Nil(t, app.paletteSelections)
	require.Len(t, app.paletteFiltered, 1)
	assert.Equal(t, "tab:jobs", app.paletteFiltered[0].ID)
	assert.Equal(t, 0, app.paletteIndex)

	app.paletteTextInput.SetValue("   ")
	app.paletteFiltered = []paletteAction{{ID: "tab:inbox"}}
	app.paletteSelections = map[string]paletteSelection{"x": {}}
	app.paletteSearchQuery = "stale"
	app.paletteSearchLoading = true
	app.paletteIndex = 3
	cmd = app.refreshPaletteFiltered()
	assert.Nil(t, cmd)
	assert.Equal(t, "", app.paletteSearchQuery)
	assert.False(t, app.paletteSearchLoading)
	assert.Nil(t, app.paletteSelections)
	assert.Nil(t, app.paletteFiltered)
	assert.Equal(t, 0, app.paletteIndex)

	app.paletteTextInput.SetValue("alpha")
	app.paletteSearchQuery = "alpha"
	app.paletteFiltered = []paletteAction{{ID: "a"}}
	app.paletteIndex = 5
	cmd = app.refreshPaletteFiltered()
	assert.Nil(t, cmd)
	assert.Equal(t, 0, app.paletteIndex)
}

func TestRefreshPaletteFilteredTriggersSearchLoading(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteTextInput.SetValue("alpha")
	app.paletteSearchQuery = "beta"
	app.paletteFiltered = []paletteAction{{ID: "old"}}
	app.paletteSelections = map[string]paletteSelection{"old": {}}
	app.paletteIndex = 7

	cmd := app.refreshPaletteFiltered()
	assert.Nil(t, cmd)
	assert.Equal(t, "alpha", app.paletteSearchQuery)
	assert.True(t, app.paletteSearchLoading)
	assert.Nil(t, app.paletteFiltered)
	assert.Nil(t, app.paletteSelections)
	assert.Equal(t, 0, app.paletteIndex)
}

func TestLoadPaletteSearchSuccessAndError(t *testing.T) {
	searchTextSeen := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/entities", "/api/context", "/api/jobs":
			searchTextSeen[r.URL.Path] = r.URL.Query().Get("search_text")
		}
		if r.URL.Path == "/api/relationships" {
			http.Error(
				w,
				`{"error":{"code":"QUERY_FAILED","message":"relationship fetch failed"}}`,
				http.StatusInternalServerError,
			)
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL, "key")
	app := NewApp(client, &config.Config{APIKey: "key", Username: "alxx"})

	cmd := app.loadPaletteSearch("alpha")
	require.NotNil(t, cmd)
	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.Contains(t, errOut.err.Error(), "QUERY_FAILED")

	assert.Equal(t, "alpha", searchTextSeen["/api/entities"])
	assert.Equal(t, "alpha", searchTextSeen["/api/context"])
	assert.Equal(t, "alpha", searchTextSeen["/api/jobs"])
}

func TestLoadPaletteSearchReturnsLoadedMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/entities":
			_, _ = w.Write([]byte(`{"data":[{"id":"ent-1","name":"Alpha","type":"tool"}]}`))
		case "/api/context":
			_, _ = w.Write([]byte(`{"data":[{"id":"ctx-1","title":"Alpha Context"}]}`))
		case "/api/jobs":
			_, _ = w.Write([]byte(`{"data":[{"id":"2026Q1-0001","title":"Alpha Job","status":"active"}]}`))
		case "/api/relationships":
			_, _ = w.Write([]byte(`{"data":[{"id":"rel-1","type":"owns"}]}`))
		case "/api/logs":
			_, _ = w.Write([]byte(`{"data":[{"id":"log-1","log_type":"event"}]}`))
		case "/api/files":
			_, _ = w.Write([]byte(`{"data":[{"id":"file-1","filename":"a.txt"}]}`))
		case "/api/protocols":
			_, _ = w.Write([]byte(`{"data":[{"id":"pro-1","name":"Alpha Protocol"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := api.NewClient(srv.URL, "key")
	app := NewApp(client, &config.Config{APIKey: "key", Username: "alxx"})

	cmd := app.loadPaletteSearch("alpha")
	require.NotNil(t, cmd)
	msg := cmd()
	loaded, ok := msg.(paletteSearchLoadedMsg)
	require.True(t, ok)
	assert.Equal(t, "alpha", loaded.query)
	require.Len(t, loaded.entities, 1)
	require.Len(t, loaded.context, 1)
	require.Len(t, loaded.jobs, 1)
	require.Len(t, loaded.rels, 1)
	require.Len(t, loaded.logs, 1)
	require.Len(t, loaded.files, 1)
	require.Len(t, loaded.protos, 1)
	assert.Equal(t, "ent-1", loaded.entities[0].ID)
	assert.Equal(t, "ctx-1", loaded.context[0].ID)
	assert.True(t, strings.Contains(strings.ToLower(loaded.jobs[0].ID), "2026q1"))
}

func TestLoadPaletteSearchNilClientReturnsNil(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	assert.Nil(t, app.loadPaletteSearch("alpha"))
}

func TestLoadPaletteSearchErrorPathMatrix(t *testing.T) {
	cases := []string{
		"/api/entities",
		"/api/context",
		"/api/jobs",
		"/api/relationships",
		"/api/logs",
		"/api/files",
		"/api/protocols",
	}

	for _, failPath := range cases {
		t.Run(strings.TrimPrefix(failPath, "/api/"), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == failPath {
					http.Error(
						w,
						fmt.Sprintf(`{"error":{"code":"QUERY_FAILED","message":"fail %s"}}`, failPath),
						http.StatusInternalServerError,
					)
					return
				}
				_, _ = w.Write([]byte(`{"data":[]}`))
			}))
			defer srv.Close()

			client := api.NewClient(srv.URL, "key")
			app := NewApp(client, &config.Config{APIKey: "key", Username: "alxx"})
			cmd := app.loadPaletteSearch("alpha")
			require.NotNil(t, cmd)
			msg := cmd()
			errOut, ok := msg.(errMsg)
			require.True(t, ok)
			assert.ErrorContains(t, errOut.err, "QUERY_FAILED")
			assert.ErrorContains(t, errOut.err, failPath)
		})
	}
}

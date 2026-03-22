package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppInitAndViewRendersBannerTabsAndHints handles test app init and view renders banner tabs and hints.
func TestAppInitAndViewRendersBannerTabsAndHints(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/approvals/pending" {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	app := NewApp(client, &config.Config{})

	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)

	cmd := app.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	model, _ = app.Update(msg)
	app = model.(App)

	out := app.View().Content
	assert.NotContains(t, out, "\x1b]")

	clean := components.SanitizeText(out)
	assert.Contains(t, clean, "Context Infrastructure for Agents")
	assert.Contains(t, clean, "Command-Line Interface")
	assert.Contains(t, clean, "Inbox")
	assert.Contains(t, clean, "Entities")
	// Help bar now uses bubbles/help model - check for lowercase binding descriptions.
	assert.Contains(t, strings.ToLower(clean), "help")
	assert.Contains(t, strings.ToLower(clean), "quit")
}

// TestAppHelpAndQuitConfirmViewsRender handles test app help and quit confirm views render.
func TestAppHelpAndQuitConfirmViewsRender(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/approvals/pending" {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	app := NewApp(client, &config.Config{})
	model, _ := app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = model.(App)

	model, _ = app.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	app = model.(App)
	require.True(t, app.helpOpen)

	help := app.View().Content
	cleanHelp := components.SanitizeText(help)
	// "esc to close" comes from renderHelp body; "help" from the full key binding list.
	assert.Contains(t, cleanHelp, "esc to close")
	assert.Contains(t, strings.ToLower(cleanHelp), "help")

	// Trigger quit confirm by creating an unsaved context draft.
	app = NewApp(client, &config.Config{})
	model, _ = app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app = model.(App)
	app.know.view = contextViewAdd
	app.know.addTitle = "draft"

	model, _ = app.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	app = model.(App)
	require.True(t, app.quitConfirm)

	quit := app.View().Content
	cleanQuit := components.SanitizeText(quit)
	assert.Contains(t, cleanQuit, "You have unsaved changes.")
	assert.Contains(t, cleanQuit, "Quit")
	assert.Contains(t, cleanQuit, "anyway?")
}

// TestAppTabWantsArrowsAndCanExitToTabNav handles test app tab wants arrows and can exit to tab nav.
func TestAppTabWantsArrowsAndCanExitToTabNav(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	app.tab = tabKnow
	assert.True(t, app.tabWantsArrows())

	app.tab = tabEntities
	app.entities.view = entitiesViewList
	assert.False(t, app.tabWantsArrows())
	app.entities.view = entitiesViewDetail
	assert.True(t, app.tabWantsArrows())

	app = NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.entities.view = entitiesViewList
	app.entities.dataTable.SetRows([]table.Row{{"one"}, {"two"}})
	assert.True(t, app.canExitToTabNav())
	app.entities.dataTable.MoveDown(1)
	assert.False(t, app.canExitToTabNav())

	app.tab = tabHistory
	app.history.view = historyViewList
	app.history.filtering = false
	assert.True(t, app.canExitToTabNav())
}

func TestAppViewRendersToastFeedbackBranch(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 100
	app.height = 32
	app.startupChecking = false
	app.toast = &appToast{level: "success", text: "toast branch hit"}

	out := components.SanitizeText(app.View().Content)
	assert.Contains(t, out, "toast branch hit")
}

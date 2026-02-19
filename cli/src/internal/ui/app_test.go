package ui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEntityPaletteActions(t *testing.T) {
	actions, selections := buildSearchPaletteActions(
		[]api.Entity{{ID: "ent-123456789", Name: "Alpha", Type: "tool"}},
		nil,
		nil,
	)

	require.Len(t, actions, 1)
	assert.Equal(t, "entity:ent-123456789", actions[0].ID)
	assert.Equal(t, "Alpha", actions[0].Label)
	assert.Equal(t, "tool · ent-1234", actions[0].Desc)
	selection, ok := selections["entity:ent-123456789"]
	require.True(t, ok)
	require.NotNil(t, selection.entity)
	assert.Equal(t, "ent-123456789", selection.entity.ID)
}

func TestFilterPalette(t *testing.T) {
	items := []paletteAction{
		{ID: "tab:inbox", Label: "Inbox", Desc: "Approvals"},
		{ID: "tab:jobs", Label: "Jobs", Desc: "Tasks"},
	}
	filtered := filterPalette(items, "job")

	require.Len(t, filtered, 1)
	assert.Equal(t, "tab:jobs", filtered[0].ID)
}

func TestRunPaletteActionEntityJump(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteSelections = map[string]paletteSelection{
		"entity:ent-1": {
			entity: &api.Entity{ID: "ent-1", Name: "Alpha", Type: "person"},
		},
	}
	action := paletteAction{ID: "entity:ent-1", Label: "Alpha"}

	model, _ := app.runPaletteAction(action)
	updated := model.(App)

	assert.Equal(t, tabEntities, updated.tab)
	require.NotNil(t, updated.entities.detail)
	assert.Equal(t, "ent-1", updated.entities.detail.ID)
	assert.Equal(t, entitiesViewDetail, updated.entities.view)
}

func TestRunPaletteActionProfileSections(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	model, _ := app.runPaletteAction(paletteAction{ID: "tab:settings"})
	updated := model.(App)
	assert.Equal(t, tabProfile, updated.tab)

	model, _ = app.runPaletteAction(paletteAction{ID: "profile:keys"})
	updated = model.(App)
	assert.Equal(t, tabProfile, updated.tab)
	assert.Equal(t, 0, updated.profile.section)

	model, _ = updated.runPaletteAction(paletteAction{ID: "profile:agents"})
	updated = model.(App)
	assert.Equal(t, tabProfile, updated.tab)
	assert.Equal(t, 1, updated.profile.section)

	model, _ = updated.runPaletteAction(paletteAction{ID: "profile:taxonomy"})
	updated = model.(App)
	assert.Equal(t, tabProfile, updated.tab)
	assert.Equal(t, 2, updated.profile.section)
}

func TestRenderTabsUsesInactiveStyleWhenTabNavDisabled(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = false

	out := app.renderTabs()
	assert.Contains(t, out, TabInactiveStyle.Render("Entities"))
}

func TestTabNavAllowsActionKeys(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabRelations
	app.tabNav = true
	app.rels.view = relsViewList

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	updated := model.(App)

	assert.False(t, updated.tabNav)
	assert.Equal(t, relsViewCreateSourceSearch, updated.rels.view)
}

func TestPaletteModeSwitchesBetweenCommandAndSearch(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.openPaletteCommand()

	require.True(t, app.paletteCommandMode())
	assert.Equal(t, "", app.paletteQuery)

	app.paletteQuery = "alpha"
	require.False(t, app.paletteCommandMode())

	app.paletteSearchQuery = "alpha"
	model, _ := app.Update(paletteSearchLoadedMsg{
		query:    "alpha",
		entities: []api.Entity{{ID: "ent-1", Name: "Alpha"}},
	})
	updated := model.(App)
	require.Len(t, updated.paletteFiltered, 1)
	assert.Equal(t, "entity:ent-1", updated.paletteFiltered[0].ID)
	assert.False(t, updated.paletteSearchLoading)
}

func TestUnifiedPaletteRemovesDedicatedSearchTabActions(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	assert.NotContains(t, tabNames, "Search")
	for _, action := range app.paletteActions {
		assert.NotEqual(t, "tab:search", action.ID)
		assert.NotEqual(t, "search:semantic", action.ID)
	}
}

func TestHelpToggle(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	updated := model.(App)
	assert.True(t, updated.helpOpen)

	model, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated = model.(App)
	assert.False(t, updated.helpOpen)
}

func TestQuitConfirmWhenUnsaved(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.know.view = contextViewAdd
	app.know.fields[fieldTitle].value = "draft"

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	updated := model.(App)

	assert.True(t, updated.quitConfirm)
	assert.Nil(t, cmd)
}

func TestQuitConfirmAccepts(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.quitConfirm = true

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated := model.(App)

	assert.True(t, updated.quitConfirm)
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	assert.True(t, ok)
}

func TestQuitConfirmCancels(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.quitConfirm = true

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	updated := model.(App)

	assert.False(t, updated.quitConfirm)
}

func TestRenderPaletteSanitizesEntries(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 80
	app.paletteFiltered = []paletteAction{
		{ID: "tab:jobs", Label: "\x1b[2Jbad", Desc: "desc\x1b[0m"},
	}

	out := app.renderPalette()
	assert.False(t, strings.Contains(out, "\x1b"))
}

func TestAppClearsErrorOnInput(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.err = "oops"

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	updated := model.(App)

	assert.Empty(t, updated.err)
}

func TestClampBodyForViewportSupportsScrollMarkers(t *testing.T) {
	lines := make([]string, 0, 24)
	for i := 1; i <= 24; i++ {
		lines = append(lines, "line "+strconv.Itoa(i))
	}
	body := strings.Join(lines, "\n")

	topScroll, clipped := clampBodyForViewport(body, 18, 3, 4, 0)
	assert.True(t, clipped)
	assert.Contains(t, topScroll, "... ↓ more")
	assert.NotContains(t, topScroll, "... ↑ more")

	midScroll, _ := clampBodyForViewport(body, 18, 3, 4, 6)
	assert.Contains(t, midScroll, "... ↑ more")
	assert.Contains(t, midScroll, "... ↓ more")

	endScroll, _ := clampBodyForViewport(body, 18, 3, 4, 99)
	assert.Contains(t, endScroll, "... ↑ more")
	assert.NotContains(t, endScroll, "... ↓ more")
}

func TestClampBodyForViewportRespectsAvailableViewportLines(t *testing.T) {
	lines := make([]string, 0, 40)
	for i := 1; i <= 40; i++ {
		lines = append(lines, "line "+strconv.Itoa(i))
	}
	body := strings.Join(lines, "\n")

	// Tight viewport to exercise clipping without invading tab/footer space.
	clamped, _ := clampBodyForViewport(body, 14, 3, 4, 0)
	got := strings.Split(clamped, "\n")
	available := 14 - 3 - 4

	assert.LessOrEqual(t, len(got), available)
}

func TestAppBodyScrollHotkeys(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	app = model.(App)
	assert.Equal(t, 8, app.bodyScroll)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	app = model.(App)
	assert.Equal(t, 0, app.bodyScroll)
}

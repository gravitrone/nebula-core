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
		"alpha",
		[]api.Entity{{ID: "ent-123456789", Name: "Alpha", Type: "tool"}},
		nil,
		nil,
		nil,
		nil,
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

func TestBuildSearchPaletteActionsIncludesRelationshipHits(t *testing.T) {
	actions, selections := buildSearchPaletteActions(
		"owns",
		nil,
		nil,
		nil,
		[]api.Relationship{{
			ID:         "rel-1",
			Type:       "owns",
			Status:     "active",
			SourceName: "alpha",
			TargetName: "beta",
			SourceID:   "ent-a",
			TargetID:   "ent-b",
		}},
		nil,
		nil,
		nil,
	)

	require.Len(t, actions, 1)
	assert.Equal(t, "relationship:rel-1", actions[0].ID)
	assert.Contains(t, strings.ToLower(actions[0].Label), "owns")
	selection, ok := selections["relationship:rel-1"]
	require.True(t, ok)
	require.NotNil(t, selection.rel)
	assert.Equal(t, "rel-1", selection.rel.ID)
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

func TestRunPaletteActionRelationshipJump(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteSelections = map[string]paletteSelection{
		"relationship:rel-1": {
			rel: &api.Relationship{ID: "rel-1", Type: "owns"},
		},
	}
	action := paletteAction{ID: "relationship:rel-1", Label: "owns"}

	model, _ := app.runPaletteAction(action)
	updated := model.(App)

	assert.Equal(t, tabRelations, updated.tab)
	assert.Equal(t, relsViewDetail, updated.rels.view)
	require.NotNil(t, updated.rels.detail)
	assert.Equal(t, "rel-1", updated.rels.detail.ID)
}

func TestRunPaletteActionLogJump(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteSelections = map[string]paletteSelection{
		"log:log-1": {
			log: &api.Log{ID: "log-1", LogType: "event"},
		},
	}
	action := paletteAction{ID: "log:log-1", Label: "log"}

	model, _ := app.runPaletteAction(action)
	updated := model.(App)

	assert.Equal(t, tabLogs, updated.tab)
	assert.Equal(t, logsViewDetail, updated.logs.view)
	require.NotNil(t, updated.logs.detail)
	assert.Equal(t, "log-1", updated.logs.detail.ID)
}

func TestRunPaletteActionFileJump(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteSelections = map[string]paletteSelection{
		"file:file-1": {
			file: &api.File{ID: "file-1", Filename: "test.txt"},
		},
	}
	action := paletteAction{ID: "file:file-1", Label: "file"}

	model, _ := app.runPaletteAction(action)
	updated := model.(App)

	assert.Equal(t, tabFiles, updated.tab)
	assert.Equal(t, filesViewDetail, updated.files.view)
	require.NotNil(t, updated.files.detail)
	assert.Equal(t, "file-1", updated.files.detail.ID)
}

func TestRunPaletteActionProtocolJump(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteSelections = map[string]paletteSelection{
		"protocol:proto-1": {
			proto: &api.Protocol{ID: "proto-1", Name: "test-protocol"},
		},
	}
	action := paletteAction{ID: "protocol:proto-1", Label: "protocol"}

	model, _ := app.runPaletteAction(action)
	updated := model.(App)

	assert.Equal(t, tabProtocols, updated.tab)
	assert.Equal(t, protocolsViewDetail, updated.protocols.view)
	require.NotNil(t, updated.protocols.detail)
	assert.Equal(t, "proto-1", updated.protocols.detail.ID)
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

func TestRenderTabsShowsActiveTabWhenTabNavDisabled(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = false

	out := app.renderTabs()
	assert.Contains(t, out, TabActiveStyle.Render("Entities"))
}

func TestRenderTabsShowsFocusedStyleWhenTabNavEnabled(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = true

	out := app.renderTabs()
	assert.Contains(t, out, TabFocusStyle.Render("Entities"))
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

func TestTabNavDownMovesIntoModeLineFocus(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = true
	app.entities.view = entitiesViewList

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := model.(App)

	assert.False(t, updated.tabNav)
	assert.True(t, updated.entities.modeFocus)
}

func TestTabNavDownMovesIntoSettingsSectionFocus(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.tabNav = true

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	updated := model.(App)

	assert.False(t, updated.tabNav)
	assert.True(t, updated.profile.sectionFocus)
}

func TestPaletteModeSwitchesBetweenCommandAndSearch(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.openPaletteCommand()

	require.True(t, app.paletteCommandMode())
	assert.Equal(t, "/", app.paletteQuery)

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

func TestRowHighlightEnabledRequiresListFocus(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = false
	app.entities.view = entitiesViewList

	assert.True(t, app.rowHighlightEnabled())

	app.entities.modeFocus = true
	assert.False(t, app.rowHighlightEnabled())
}

func TestRowHighlightEnabledDisabledInTabNav(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.tabNav = true
	app.entities.view = entitiesViewList

	assert.False(t, app.rowHighlightEnabled())
}

func TestRowHighlightEnabledDisabledWhenSettingsSectionFocused(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.tabNav = false
	app.profile.sectionFocus = true

	assert.False(t, app.rowHighlightEnabled())
}

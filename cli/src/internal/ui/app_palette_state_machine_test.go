package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAppPaletteOpenFilterAndExecuteTab handles test app palette open filter and execute tab.
func TestAppPaletteOpenFilterAndExecuteTab(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
	})

	app := NewApp(client, &config.Config{})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = model.(App)
	assert.True(t, app.paletteOpen)

	for _, r := range "/job" {
		model, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		app = model.(App)
	}

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(App)

	assert.False(t, app.paletteOpen)
	assert.Equal(t, tabJobs, app.tab)
}

// TestAppPaletteArrowKeysMoveSelection handles test app palette arrow keys move selection.
func TestAppPaletteArrowKeysMoveSelection(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
	})

	app := NewApp(client, &config.Config{})
	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = model.(App)
	assert.True(t, app.paletteOpen)
	assert.Equal(t, 0, app.paletteIndex)
	require.Greater(t, len(app.paletteFiltered), 1)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = model.(App)
	assert.Equal(t, 1, app.paletteIndex)

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyUp})
	app = model.(App)
	assert.Equal(t, 0, app.paletteIndex)
}

// TestAppPaletteTextSearchLoadsAndJumpsToDetail handles test app palette text search loads and jumps to detail.
func TestAppPaletteTextSearchLoadsAndJumpsToDetail(t *testing.T) {
	var gotQuery string
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			gotQuery = r.URL.Query().Get("search_text")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "ent-1", "name": "Alpha", "type": "person"},
				},
			}))
			return
		}
		if r.URL.Path == "/api/context" ||
			r.URL.Path == "/api/jobs" ||
			r.URL.Path == "/api/relationships" ||
			r.URL.Path == "/api/logs" ||
			r.URL.Path == "/api/files" ||
			r.URL.Path == "/api/protocols" {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	app := NewApp(client, &config.Config{})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	app = model.(App)

	// Remove the leading slash to switch from command mode to search mode.
	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	app = model.(App)

	// Typing plain text in search mode triggers API queries.
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	app = model.(App)
	require.NotNil(t, cmd)
	msg := cmd()

	model, _ = app.Update(msg)
	app = model.(App)

	assert.Equal(t, "a", gotQuery)
	assert.False(t, app.paletteSearchLoading)
	require.Len(t, app.paletteFiltered, 1)
	assert.True(t, strings.HasPrefix(app.paletteFiltered[0].ID, "entity:"))

	model, _ = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app = model.(App)

	assert.False(t, app.paletteOpen)
	assert.Equal(t, tabEntities, app.tab)
	require.NotNil(t, app.entities.detail)
	assert.Equal(t, "ent-1", app.entities.detail.ID)
	assert.Equal(t, entitiesViewDetail, app.entities.view)
}

// TestApplySearchSelectionSwitchesTabAndSetsDetail handles test apply search selection switches tab and sets detail.
func TestApplySearchSelectionSwitchesTabAndSetsDetail(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	model, _ := app.Update(searchSelectionMsg{
		kind:   "entity",
		entity: &api.Entity{ID: "ent-9", Name: "Zeta"},
	})
	updated := model.(App)

	assert.Equal(t, tabEntities, updated.tab)
	require.NotNil(t, updated.entities.detail)
	assert.Equal(t, "ent-9", updated.entities.detail.ID)
	assert.Equal(t, entitiesViewDetail, updated.entities.view)
}

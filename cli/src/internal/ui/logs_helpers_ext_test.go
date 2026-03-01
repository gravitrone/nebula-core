package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogsModeLineAndModeKeysMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 80
	model.view = logsViewList

	line := components.SanitizeText(model.renderModeLine())
	assert.Contains(t, line, "Add")
	assert.Contains(t, line, "Library")

	model.modeFocus = true
	updated, cmd := model.handleModeKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewAdd, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)
}

func TestLogsLoadScopeOptionsNilClient(t *testing.T) {
	model := NewLogsModel(nil)
	assert.Nil(t, model.loadScopeOptions())
}

func TestLogsLoadDetailRelationshipsSuccessAndError(t *testing.T) {
	now := time.Now()
	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/relationships/log/log-1":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id":          "rel-1",
					"source_type": "log",
					"source_id":   "log-1",
					"target_type": "entity",
					"target_id":   "ent-1",
					"type":        "linked-to",
					"status":      "active",
					"created_at":  now,
				}},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	model := NewLogsModel(client)
	msg := model.loadDetailRelationships("log-1")().(logRelationshipsLoadedMsg) //nolint:forcetypeassert
	require.Len(t, msg.relationships, 1)
	assert.Equal(t, "rel-1", msg.relationships[0].ID)

	msg = model.loadDetailRelationships("log-2")().(logRelationshipsLoadedMsg) //nolint:forcetypeassert
	assert.Equal(t, "log-2", msg.id)
	assert.Nil(t, msg.relationships)
}

func TestLogsHandleListKeysSearchAndFilterMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.allItems = []api.Log{
		{ID: "log-1", LogType: "workout", Status: "active"},
		{ID: "log-2", LogType: "study", Status: "active"},
	}
	model.applyLogSearch()
	updated := model

	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	assert.Equal(t, "w", updated.searchBuf)
	assert.Equal(t, "workout", updated.searchSuggest)
	require.Len(t, updated.items, 1)

	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, "workout", updated.searchBuf)

	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "workou", updated.searchBuf)

	updated, _ = updated.handleListKeys(tea.KeyMsg{Type: tea.KeyCtrlU})
	assert.Equal(t, "", updated.searchBuf)
	require.Len(t, updated.items, 2)

	updated, cmd := updated.handleListKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	require.Nil(t, cmd)
	assert.True(t, updated.filtering)

	updated, _ = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	assert.Equal(t, "s", updated.searchBuf)
	updated, _ = updated.handleFilterInput(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.searchBuf)
}

func TestLogsHandleAddKeysStatusAndBackspaceMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewAdd
	model.addFocus = logFieldStatus

	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.addStatusIdx)

	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = logFieldTags
	updated.addTags = []string{"alpha"}
	updated.addTagBuf = "x"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "", updated.addTagBuf)
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Empty(t, updated.addTags)

	updated.addFocus = logFieldType
	updated.addType = "work"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "wor", updated.addType)

	updated.addFocus = logFieldTimestamp
	updated.addTimestamp = "2026-02-27"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "2026-02-2", updated.addTimestamp)

	updated.addSaved = true
	updated.addType = "workout"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addType)
}

func TestLogsHandleAddKeysAdditionalBranches(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewAdd

	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	model.addSaving = false
	model.addSaved = true
	model.addType = "kept"
	updated, cmd = model.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	require.Nil(t, cmd)
	assert.True(t, updated.addSaved)
	assert.Equal(t, "kept", updated.addType)

	model.addSaved = false
	model.modeFocus = true
	updated, cmd = model.handleAddKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)

	updated.view = logsViewAdd
	updated.modeFocus = false
	updated.addFocus = logFieldType
	updated.addType = ""
	updated, cmd = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	require.Nil(t, cmd)
	assert.Equal(t, "e", updated.addType)
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, "e ", updated.addType)

	updated.addFocus = logFieldTimestamp
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'6'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'7'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'T'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Z'}})
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	beforeTS := updated.addTimestamp
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyTab})
	assert.Equal(t, beforeTS, updated.addTimestamp)

	updated.addFocus = logFieldValue
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.addValue.Active)
	updated.addValue.Active = false

	updated.addFocus = logFieldMeta
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.addMeta.Active)
	updated.addMeta.Active = false

	updated.addFocus = 0
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, updated.modeFocus)
}

func TestLogsHandleEditKeysStatusAndTagMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "event", Status: "active", Timestamp: time.Now()}
	model.startEdit()
	model.editFocus = logEditFieldStatus

	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.editStatusIdx)

	updated.editFocus = logEditFieldTags
	updated.editTags = []string{"alpha"}
	updated.editTagBuf = "x"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "", updated.editTagBuf)
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Empty(t, updated.editTags)

	updated.editFocus = logEditFieldValue
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.editValue.Active)
	updated.editValue.Active = false

	updated.editFocus = logEditFieldMeta
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, updated.editMeta.Active)
	updated.editMeta.Active = false

	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, logsViewDetail, updated.view)
}

func TestLogsSaveAddAndEditValidationBranches(t *testing.T) {
	model := NewLogsModel(nil)
	model.addType = "event"
	model.addTimestamp = "not-a-date"
	updated, cmd := model.saveAdd()
	assert.Nil(t, cmd)
	assert.Contains(t, updated.addErr, "timestamp")

	model.addTimestamp = ""
	model.addValue.Buffer = "invalid"
	updated, cmd = model.saveAdd()
	assert.Nil(t, cmd)
	assert.NotEmpty(t, updated.addErr)

	model = NewLogsModel(nil)
	model.detail = &api.Log{ID: "log-1", Timestamp: time.Now()}
	model.startEdit()
	model.editValue.Buffer = "invalid"
	updated, cmd = model.saveEdit()
	assert.Nil(t, cmd)
	assert.NotEmpty(t, updated.errText)

	model.editValue.Buffer = ""
	model.editMeta.Buffer = "invalid"
	updated, cmd = model.saveEdit()
	assert.Nil(t, cmd)
	assert.NotEmpty(t, updated.errText)
}

func TestLogsSearchHelpersAndFormatLine(t *testing.T) {
	model := NewLogsModel(nil)
	model.allItems = []api.Log{
		{ID: "log-1", LogType: "workout", Status: "active", Timestamp: time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)},
		{ID: "log-2", LogType: "study", Status: "inactive", Timestamp: time.Date(2026, 2, 28, 0, 0, 0, 0, time.UTC)},
	}

	model.searchBuf = "work"
	model.applyLogSearch()
	require.Len(t, model.items, 1)
	assert.Equal(t, "workout", model.searchSuggest)

	model.searchBuf = "zzz"
	model.applyLogSearch()
	assert.Empty(t, model.items)

	line := formatLogLine(api.Log{LogType: "", Status: "active", Timestamp: time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)})
	clean := components.SanitizeText(line)
	assert.Contains(t, clean, "log")
	assert.Contains(t, clean, "2026-02-27")
	assert.True(t, strings.Contains(clean, "active"))
}

func TestParseLogTimestampWithMinuteLayout(t *testing.T) {
	ts, err := parseLogTimestamp("2026-02-27 13:45")
	require.NoError(t, err)
	require.NotNil(t, ts)
	assert.Equal(t, 2026, ts.Year())
	assert.Equal(t, 13, ts.Hour())
	assert.Equal(t, 45, ts.Minute())
}

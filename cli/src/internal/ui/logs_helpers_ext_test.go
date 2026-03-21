package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
	updated, cmd := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewAdd, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)
}

func TestLogsLoadScopeOptionsNilClient(t *testing.T) {
	model := NewLogsModel(nil)
	assert.Nil(t, model.loadScopeOptions())
}

func TestLogsLoadScopeOptionsSuccessAndError(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/audit/scopes" || r.Method != http.MethodGet {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "scope-2", "name": "private"},
					{"id": "scope-1", "name": "public"},
				},
			}))
		})

		model := NewLogsModel(client)
		cmd := model.loadScopeOptions()
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(logsScopesLoadedMsg)
		require.True(t, ok)
		assert.Equal(t, []string{"private", "public"}, loaded.options)
	})

	t.Run("error", func(t *testing.T) {
		_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet {
				http.Error(w, `{"error":{"code":"SCOPES_FAILED","message":"scope fetch failed"}}`, http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewLogsModel(client)
		cmd := model.loadScopeOptions()
		require.NotNil(t, cmd)
		msg := cmd()
		errOut, ok := msg.(errMsg)
		require.True(t, ok)
		assert.ErrorContains(t, errOut.err, "SCOPES_FAILED")
	})
}

func TestLogsLoadLogsSuccessAndError(t *testing.T) {
	now := time.Now().UTC()

	t.Run("success path includes active status filter", func(t *testing.T) {
		_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/logs" && r.Method == http.MethodGet {
				assert.Equal(t, "active", r.URL.Query().Get("status_category"))
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]any{
						{
							"id":         "log-1",
							"log_type":   "workout",
							"timestamp":  now,
							"value":      map[string]any{"note": "x"},
							"status":     "active",
							"tags":       []string{"core"},
							"metadata":   map[string]any{},
							"created_at": now,
							"updated_at": now,
						},
					},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewLogsModel(client)
		cmd := model.loadLogs()
		require.NotNil(t, cmd)
		msg := cmd()
		loaded, ok := msg.(logsLoadedMsg)
		require.True(t, ok)
		require.Len(t, loaded.items, 1)
		assert.Equal(t, "log-1", loaded.items[0].ID)
	})

	t.Run("error path returns errMsg", func(t *testing.T) {
		_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/logs" && r.Method == http.MethodGet {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]any{"code": "INTERNAL", "message": "logs failed"},
				}))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		})

		model := NewLogsModel(client)
		cmd := model.loadLogs()
		require.NotNil(t, cmd)
		msg := cmd()
		em, ok := msg.(errMsg)
		require.True(t, ok)
		require.Error(t, em.err)
		assert.Contains(t, strings.ToLower(em.err.Error()), "logs failed")
	})
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

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: 'w', Text: "w"})
	assert.Equal(t, "w", updated.searchBuf)
	assert.Equal(t, "workout", updated.searchSuggest)
	require.Len(t, updated.items, 1)

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, "workout", updated.searchBuf)

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "workou", updated.searchBuf)

	updated, _ = updated.handleListKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	assert.Equal(t, "", updated.searchBuf)
	require.Len(t, updated.items, 2)

	updated, cmd := updated.handleListKeys(tea.KeyPressMsg{Code: 'f', Text: "f"})
	require.Nil(t, cmd)
	assert.True(t, updated.filtering)

	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: 's', Text: "s"})
	assert.Equal(t, "s", updated.searchBuf)
	updated, _ = updated.handleFilterInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.filtering)
	assert.Equal(t, "", updated.searchBuf)
}

func TestLogsHandleAddKeysStatusAndBackspaceMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewAdd
	model.addFocus = logFieldStatus

	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.addStatusIdx)

	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = logFieldTags
	updated.addTags = []string{"alpha"}
	updated.addTagBuf = "x"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.addTagBuf)
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Empty(t, updated.addTags)

	updated.addFocus = logFieldType
	updated.addType = "work"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "wor", updated.addType)

	updated.addFocus = logFieldTimestamp
	updated.addTimestamp = "2026-02-27"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "2026-02-2", updated.addTimestamp)

	updated.addSaved = true
	updated.addType = "workout"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addType)
}

func TestLogsHandleAddKeysAdditionalBranches(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewAdd

	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, model, updated)

	model.addSaving = false
	model.addSaved = true
	model.addType = "kept"
	updated, cmd = model.handleAddKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.True(t, updated.addSaved)
	assert.Equal(t, "kept", updated.addType)

	model.addSaved = false
	model.modeFocus = true
	updated, cmd = model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)

	updated.view = logsViewAdd
	updated.modeFocus = false
	updated.addFocus = logFieldType
	updated.addType = ""
	updated, cmd = updated.handleAddKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, "e", updated.addType)
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "e ", updated.addType)

	updated.addFocus = logFieldTimestamp
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '2', Text: "2"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '0', Text: "0"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '2', Text: "2"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '6', Text: "6"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '-', Text: "-"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '0', Text: "0"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '2', Text: "2"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '-', Text: "-"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '2', Text: "2"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '7', Text: "7"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'T', Text: "T"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '1', Text: "1"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '3', Text: "3"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: ':', Text: ":"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '4', Text: "4"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '5', Text: "5"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: 'Z', Text: "Z"})
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: '+', Text: "+"})
	beforeTS := updated.addTimestamp
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, beforeTS, updated.addTimestamp)

	updated.addFocus = logFieldValue
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.addValue.Active)
	updated.addValue.Active = false

	updated.addFocus = logFieldMeta
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.addMeta.Active)
	updated.addMeta.Active = false

	updated.addFocus = 0
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.True(t, updated.modeFocus)
}

func TestLogsRenderAddBranchMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 96

	model.addFocus = logFieldType
	model.addType = "event"
	out := stripANSI(model.renderAdd())
	assert.Contains(t, out, "Type:")
	assert.Contains(t, out, "event")

	model.addFocus = logFieldTimestamp
	model.addTimestamp = "2026-03-01T12:45Z"
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "Timestamp:")
	assert.Contains(t, out, "2026-03-01T12:45Z")

	model.addFocus = logFieldStatus
	model.addStatusIdx = 1
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "Status:")
	assert.Contains(t, out, logStatusOptions[1])

	model.addFocus = logFieldTags
	model.addTags = []string{"alpha"}
	model.addTagBuf = "beta"
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "[alpha]")
	assert.Contains(t, out, "beta")

	model.addFocus = logFieldValue
	model.addValue.Buffer = "group | field | value"
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "group | field | value")

	model.addFocus = logFieldMeta
	model.addMeta.Buffer = "meta | key | value"
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "meta | key | value")

	model.addErr = "bad metadata"
	model.addSaved = true
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "bad metadata")
	assert.Contains(t, out, "Saved.")

	empty := NewLogsModel(nil)
	empty.width = 80
	empty.addFocus = logFieldMeta
	out = stripANSI(empty.renderAdd())
	assert.Contains(t, out, "Type:")
	assert.Contains(t, out, "Timestamp:")
	assert.Contains(t, out, "Metadata:")
	assert.Contains(t, out, "  -")
}

func TestLogsHandleEditKeysStatusAndTagMatrix(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "event", Status: "active", Timestamp: time.Now()}
	model.startEdit()
	model.editFocus = logEditFieldStatus

	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.editStatusIdx)

	updated.editFocus = logEditFieldTags
	updated.editTags = []string{"alpha"}
	updated.editTagBuf = "x"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.editTagBuf)
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Empty(t, updated.editTags)

	updated.editFocus = logEditFieldValue
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.editValue.Active)
	updated.editValue.Active = false

	updated.editFocus = logEditFieldMeta
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.True(t, updated.editMeta.Active)
	updated.editMeta.Active = false

	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
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

	model.addValue.Buffer = "meta | field | value"
	model.addMeta.Buffer = "invalid"
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

func TestLogsStartEditNilDetailAndSaveEditRequestError(t *testing.T) {
	model := NewLogsModel(nil)
	model.editType = "keep"
	model.startEdit()
	assert.Equal(t, "keep", model.editType)

	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/logs/log-1" && r.Method == http.MethodPatch {
			http.Error(w, `{"error":{"code":"LOG_UPDATE_FAILED","message":"update failed"}}`, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model = NewLogsModel(client)
	model.detail = &api.Log{ID: "log-1", LogType: "workout", Status: "active", Timestamp: time.Now().UTC()}
	model.startEdit()
	model.editMeta.Buffer = ""
	model.editValue.Buffer = ""
	updated, cmd := model.saveEdit()
	require.NotNil(t, cmd)
	assert.True(t, updated.editSaving)

	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "LOG_UPDATE_FAILED")
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

	noStatus := components.SanitizeText(formatLogLine(api.Log{
		LogType:   "audit",
		Status:    "",
		Timestamp: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Metadata:  api.JSONMap{"owner": "alxx"},
	}))
	assert.Contains(t, noStatus, "audit")
	assert.Contains(t, noStatus, "2026-03-01")
	assert.NotContains(t, noStatus, " ·  · ")
	assert.Contains(t, strings.ToLower(noStatus), "alxx")
}

func TestParseLogTimestampWithMinuteLayout(t *testing.T) {
	ts, err := parseLogTimestamp("2026-02-27 13:45")
	require.NoError(t, err)
	require.NotNil(t, ts)
	assert.Equal(t, 2026, ts.Year())
	assert.Equal(t, 13, ts.Hour())
	assert.Equal(t, 45, ts.Minute())
}

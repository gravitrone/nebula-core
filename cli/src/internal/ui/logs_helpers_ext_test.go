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

func TestLogsHandleAddKeysSavingAndSavedBranchesHelper(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewAdd

	// addSaving blocks key input.
	model.addSaving = true
	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.addSaving)

	// addSaved + non-Esc is no-op.
	model.addSaving = false
	model.addSaved = true
	model.addType = "kept"
	updated, cmd = model.handleAddKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.Nil(t, cmd)
	assert.True(t, updated.addSaved)
	assert.Equal(t, "kept", updated.addType)

	// addSaved + Esc resets form.
	updated.addSaved = true
	updated.addType = "workout"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addType)
}

func TestLogsHandleAddKeysModeToggleHelper(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewAdd
	model.addSaved = false
	model.modeFocus = true

	// modeFocus + right goes through Update (which checks modeFocus before dispatching to handleAddKeys).
	updated, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, logsViewList, updated.view)

	// Nil form initializes on first key and returns non-nil cmd.
	model2 := NewLogsModel(nil)
	model2.view = logsViewAdd
	model2.addForm = nil
	_, cmd2 := model2.handleAddKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	assert.NotNil(t, cmd2)
}

func TestLogsRenderAddBranchMatrixHelper(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 96

	// nil form -> Initializing.
	out := stripANSI(model.renderAdd())
	assert.Contains(t, out, "Initializing")

	// saving -> Saving.
	model.addSaving = true
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "Saving")

	// saved -> Log saved.
	model.addSaving = false
	model.addSaved = true
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "Log saved")

	// with form + metadata buffers.
	model.addSaved = false
	model.addForm = nil
	model.initAddForm()
	model.addValue.Buffer = "group | field | value"
	model.addMeta.Buffer = "meta | key | value"
	model.addErr = "bad metadata"
	out = stripANSI(model.renderAdd())
	assert.Contains(t, out, "group | field | value")
	assert.Contains(t, out, "meta | key | value")
	assert.Contains(t, out, "bad metadata")
}

func TestLogsHandleEditKeysStatusAndTagMatrixHelper(t *testing.T) {
	model := NewLogsModel(nil)
	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "event", Status: "active", Timestamp: time.Now()}
	model.startEdit()

	assert.Equal(t, "active", model.editStatus)
	assert.NotNil(t, model.editForm)

	// Esc exits to detail.
	updated, _ := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, logsViewDetail, updated.view)

	// editSaving blocks.
	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.True(t, updated.editSaving)
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

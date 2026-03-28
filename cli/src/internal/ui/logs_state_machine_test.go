package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testLogsClient handles test logs client.
func testLogsClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *api.Client) {
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, api.NewClient(srv.URL, "test-key")
}

// TestLogsInitLoadsLogsAndScopes handles test logs init loads logs and scopes.
func TestLogsInitLoadsLogsAndScopes(t *testing.T) {
	now := time.Now()
	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/logs") && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "log-1",
						"log_type":   "workout",
						"timestamp":  now,
						"value":      map[string]any{"note": "x"},
						"status":     "active",
						"tags":       []string{},
						"metadata":   map[string]any{},
						"created_at": now,
						"updated_at": now,
					},
				},
			})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":            "scope-1",
						"name":          "public",
						"description":   nil,
						"agent_count":   0,
						"entity_count":  0,
						"context_count": 0,
					},
				},
			})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewLogsModel(client)
	model, cmd := model.Update(runCmdFirst(model.Init()))

	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.False(t, model.loading)
	assert.Len(t, model.items, 1)
	assert.Equal(t, "log-1", model.items[0].ID)
	assert.Contains(t, model.scopeOptions, "public")
}

// TestLogsAddValidationErrorOnEmpty handles test logs add validation error on empty.
func TestLogsAddValidationErrorOnEmpty(t *testing.T) {
	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {})
	model := NewLogsModel(client)
	model.view = logsViewAdd
	// saveAdd directly with empty type validates immediately.
	model, _ = model.saveAdd()
	assert.Equal(t, "Type is required", model.addErr)
}

// TestLogsListNavigationOpensDetailAndReturnsToList handles test logs list navigation opens detail and returns to list.
func TestLogsListNavigationOpensDetailAndReturnsToList(t *testing.T) {
	now := time.Now()
	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/logs") && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "log-1",
						"log_type":   "workout",
						"timestamp":  now,
						"value":      map[string]any{"note": "x"},
						"status":     "active",
						"tags":       []string{"t1"},
						"metadata":   map[string]any{},
						"created_at": now,
						"updated_at": now,
					},
					{
						"id":         "log-2",
						"log_type":   "study",
						"timestamp":  now,
						"value":      map[string]any{},
						"status":     "active",
						"tags":       []string{},
						"metadata":   map[string]any{},
						"created_at": now,
						"updated_at": now,
					},
				},
			})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "s1", "name": "public"}}})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewLogsModel(client)
	model, cmd := model.Update(runCmdFirst(model.Init()))
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	require.Len(t, model.items, 2)

	// Navigate down to second item.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	assert.Equal(t, 1, model.dataTable.Cursor())

	// Open detail.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, logsViewDetail, model.view)
	require.NotNil(t, model.detail)
	assert.Equal(t, "log-2", model.detail.ID)

	// Back to list.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, logsViewList, model.view)
	assert.Nil(t, model.detail)
}

// TestLogsDetailRendersRelationshipsSection handles test logs detail renders relationships section.
func TestLogsDetailRendersRelationshipsSection(t *testing.T) {
	now := time.Now()
	model := NewLogsModel(nil)
	model.width = 90
	model.view = logsViewDetail
	model.detail = &api.Log{
		ID:        "log-1",
		LogType:   "event",
		Status:    "active",
		Content: "note: x",
		Notes:   "",
		CreatedAt: now,
		UpdatedAt: now,
	}
	model.detailRels = []api.Relationship{
		{
			ID:         "rel-1",
			SourceType: "log",
			SourceID:   "log-1",
			SourceName: "event",
			TargetType: "entity",
			TargetID:   "ent-1",
			TargetName: "Bro",
			Type:       "related-to",
			Status:     "active",
			CreatedAt:  now,
		},
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "related-to")
	assert.Contains(t, out, "Bro")
}

// TestLogsAddFlowCommitsTagsAndSaves handles test logs add flow commits tags and saves.
func TestLogsAddFlowCommitsTagsAndSaves(t *testing.T) {
	now := time.Now()
	var created api.CreateLogInput
	var posted bool
	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/logs") && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "s1", "name": "public"}}})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/logs" && r.Method == http.MethodPost:
			posted = true
			require.NoError(t, json.NewDecoder(r.Body).Decode(&created))
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"id":         "log-1",
				"log_type":   created.LogType,
				"timestamp":  now,
				"status":     created.Status,
				"tags":       created.Tags,
				"content":    created.Content,
				"notes":      created.Notes,
				"created_at": now,
				"updated_at": now,
			}})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewLogsModel(client)
	model, cmd := model.Update(runCmdFirst(model.Init()))
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	// Toggle into Add mode via mode line focus.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.True(t, model.modeFocus)
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, logsViewAdd, model.view)

	// Set form fields directly (huh forms don't support programmatic field navigation).
	model.addType = "workout"
	model.addStatus = "active"
	model.addTagStr = "alpha"

	// Save by calling saveAdd directly.
	var saveCmd tea.Cmd
	model, saveCmd = model.saveAdd()
	require.NotNil(t, saveCmd)
	msg := saveCmd()
	model, cmd = model.Update(msg)
	require.NotNil(t, cmd)

	// Reload logs and scopes.
	model, cmd = model.Update(runCmdFirst(cmd))
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	require.True(t, posted)
	assert.Equal(t, "workout", created.LogType)
	assert.Equal(t, "active", created.Status)
	assert.Equal(t, []string{"alpha"}, created.Tags)
	assert.True(t, model.addSaved)
}

// TestLogsEditFlowSavesPatchAndReturnsToList handles test logs edit flow saves patch and returns to list.
func TestLogsEditFlowSavesPatchAndReturnsToList(t *testing.T) {
	now := time.Now()
	var patched api.UpdateLogInput
	var patchedID string
	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/logs") && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":         "log-1",
						"log_type":   "workout",
						"timestamp":  now,
						"value":      map[string]any{"note": "x"},
						"status":     "active",
						"tags":       []string{},
						"metadata":   map[string]any{},
						"created_at": now,
						"updated_at": now,
					},
				},
			})
			require.NoError(t, err)
			return
		case r.URL.Path == "/api/audit/scopes" && r.Method == http.MethodGet:
			err := json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "s1", "name": "public"}}})
			require.NoError(t, err)
			return
		case strings.HasPrefix(r.URL.Path, "/api/logs/") && r.Method == http.MethodPatch:
			patchedID = strings.TrimPrefix(r.URL.Path, "/api/logs/")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": patchedID}})
			require.NoError(t, err)
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewLogsModel(client)
	model, cmd := model.Update(runCmdFirst(model.Init()))
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	// Open detail and then edit.
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, logsViewDetail, model.view)
	model, _ = model.Update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	assert.Equal(t, logsViewEdit, model.view)

	// Set edit fields directly (huh forms don't support programmatic field navigation).
	model.editTagStr = "beta"

	// Save by calling saveEdit directly.
	var saveCmd tea.Cmd
	model, saveCmd = model.saveEdit()
	require.NotNil(t, saveCmd)
	msg := saveCmd()
	model, cmd = model.Update(msg)
	require.NotNil(t, cmd)

	// Reload logs and scopes (post-update path).
	model, cmd = model.Update(runCmdFirst(cmd))
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	assert.Equal(t, "log-1", patchedID)
	require.NotNil(t, patched.Tags)
	assert.Equal(t, []string{"beta"}, *patched.Tags)
	assert.Equal(t, logsViewList, model.view)
}

// TestParseLogTimestamp handles test parse log timestamp.
func TestParseLogTimestamp(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ts, err := parseLogTimestamp("")
		require.NoError(t, err)
		assert.Nil(t, ts)
	})

	t.Run("date-only", func(t *testing.T) {
		ts, err := parseLogTimestamp("2026-02-13")
		require.NoError(t, err)
		require.NotNil(t, ts)
		assert.Equal(t, "2026-02-13", ts.Format("2006-01-02"))
	})

	t.Run("rfc3339", func(t *testing.T) {
		ts, err := parseLogTimestamp("2026-02-13T10:11:12Z")
		require.NoError(t, err)
		require.NotNil(t, ts)
		assert.Equal(t, "2026-02-13T10:11:12Z", ts.UTC().Format(time.RFC3339))
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := parseLogTimestamp("nope")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp:")
	})
}

// TestLogsRenderAddEditAndTagHelpers handles test logs render add edit and tag helpers.
func TestLogsRenderAddEditAndTagHelpers(t *testing.T) {
	now := time.Now()
	model := NewLogsModel(nil)
	model.width = 96

	// Tags are now stored as comma-separated string.
	model.addTagStr = "alpha"
	assert.Equal(t, "alpha", model.addTagStr)

	// renderAdd with nil form shows Initializing.
	addView := components.SanitizeText(model.renderAdd())
	assert.NotEmpty(t, addView)

	// resetAddForm clears all fields.
	model.addType = "event"
	model.addTimestamp = now.Format(time.RFC3339)
	model.addStatus = "inactive"
	model.addSaved = true
	model.resetAddForm()
	assert.Equal(t, "", model.addType)
	assert.Equal(t, "active", model.addStatus)
	assert.False(t, model.addSaved)

	// startEdit loads fields from detail.
	model.detail = &api.Log{
		ID:        "log-1",
		LogType:   "event",
		Status:    "active",
		Tags:      []string{"core"},
		Content: "k: v",
		Notes:   "scope: public",
		Timestamp: now,
	}
	model.startEdit()
	assert.Equal(t, "active", model.editStatus)
	assert.Equal(t, "core", model.editTagStr)
	assert.NotNil(t, model.editForm)

	// Add a tag via editTagStr.
	model.editTagStr = "core, beta"

	// renderEdit shows form output.
	editView := components.SanitizeText(model.renderEdit())
	assert.NotEmpty(t, editView)
}

// TestLogsFormsRenderMetadataPreviewTable handles test logs forms render metadata preview table.
func TestLogsFormsRenderMetadataPreviewTable(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 100
	model.view = logsViewAdd
	model.addForm = nil
	model.initAddForm()
	model.addMeta.Buffer = "profile | timezone | Europe/Warsaw"
	model.addValue.Buffer = "ops | board | nebula-core"

	addView := components.SanitizeText(model.renderAdd())
	assert.Contains(t, addView, "profile | timezone | Europe/Warsaw")
	assert.Contains(t, addView, "ops | board | nebula-core")

	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "event", Status: "active"}
	model.startEdit()
	model.editMeta.Buffer = "state | env | dev"
	model.editValue.Buffer = "state | build | local"

	editView := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editView, "state | env | dev")
	assert.Contains(t, editView, "state | build | local")
}

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	cmd := model.Init()
	require.NotNil(t, cmd)
	msg := cmd()
	model, cmd = model.Update(msg)

	require.NotNil(t, cmd)
	msg = cmd()
	model, _ = model.Update(msg)

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

	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
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
	cmd := model.Init()
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	require.Len(t, model.items, 2)

	// Navigate down to second item.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 1, model.list.Selected())

	// Open detail.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, logsViewDetail, model.view)
	require.NotNil(t, model.detail)
	assert.Equal(t, "log-2", model.detail.ID)

	// Back to list.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
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
		Value:     api.JSONMap{"note": "x"},
		Metadata:  api.JSONMap{},
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
				"value":      created.Value,
				"metadata":   created.Metadata,
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
	cmd := model.Init()
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	// Toggle into Add mode via mode line focus.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.True(t, model.modeFocus)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, logsViewAdd, model.view)

	// Fill type.
	for _, r := range "workout" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	assert.Equal(t, "workout", model.addType)

	// Move to Tags field.
	for i := 0; i < 3; i++ {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	assert.Equal(t, logFieldTags, model.addFocus)

	// Commit tag and dedupe.
	for _, r := range "alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, []string{"alpha"}, model.addTags)

	for _, r := range "alpha" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.Equal(t, []string{"alpha"}, model.addTags)

	// Save.
	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)

	// Reload logs and scopes.
	model, cmd = model.Update(cmd())
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
	cmd := model.Init()
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)
	model, _ = model.Update(cmd())

	// Open detail and then edit.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, logsViewDetail, model.view)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	assert.Equal(t, logsViewEdit, model.view)

	// Move focus to tags and add one tag.
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, logEditFieldTags, model.editFocus)
	for _, r := range "beta" {
		model, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, []string{"beta"}, model.editTags)

	model, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	require.NotNil(t, cmd)
	model, cmd = model.Update(cmd())
	require.NotNil(t, cmd)

	// Reload logs and scopes (post-update path).
	model, cmd = model.Update(cmd())
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

	model.addFocus = logFieldTags
	model.addTagBuf = "alpha"
	model.commitAddTag()
	addTags := components.SanitizeText(model.renderAddTags(true))
	assert.Contains(t, addTags, "[alpha]")
	assert.Contains(t, addTags, "█")

	addView := components.SanitizeText(model.renderAdd())
	assert.Contains(t, addView, "Type")
	assert.Contains(t, addView, "Status")

	model.addType = "event"
	model.addTimestamp = now.Format(time.RFC3339)
	model.addStatusIdx = 2
	model.addSaved = true
	model.resetAddForm()
	assert.Equal(t, "", model.addType)
	assert.Equal(t, 0, model.addFocus)
	assert.False(t, model.addSaved)

	model.detail = &api.Log{
		ID:        "log-1",
		LogType:   "event",
		Status:    "active",
		Tags:      []string{"core"},
		Value:     api.JSONMap{"k": "v"},
		Metadata:  api.JSONMap{"scope": "public"},
		Timestamp: now,
	}
	model.startEdit()
	model.editFocus = logEditFieldTags
	model.editTagBuf = "beta"
	model.commitEditTag()

	editTags := components.SanitizeText(model.renderEditTags(true))
	assert.Contains(t, editTags, "[beta]")

	editView := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editView, "Status")
	assert.Contains(t, editView, "Tags")
}

// TestLogsFormsRenderMetadataPreviewTable handles test logs forms render metadata preview table.
func TestLogsFormsRenderMetadataPreviewTable(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 100
	model.view = logsViewAdd
	model.addFocus = logFieldMeta
	model.addMeta.Buffer = "profile | timezone | Europe/Warsaw"
	model.addValue.Buffer = "ops | board | nebula-core"

	addView := components.SanitizeText(model.renderAdd())
	assert.Contains(t, addView, "profile | timezone | Europe/Warsaw")
	assert.Contains(t, addView, "ops | board | nebula-core")

	model.view = logsViewEdit
	model.detail = &api.Log{ID: "log-1", LogType: "event"}
	model.editFocus = logEditFieldMeta
	model.editMeta.Buffer = "state | env | dev"
	model.editValue.Buffer = "state | build | local"

	editView := components.SanitizeText(model.renderEdit())
	assert.Contains(t, editView, "state | env | dev")
	assert.Contains(t, editView, "state | build | local")
}

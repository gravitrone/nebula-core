package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesRenderListSideBySidePreviewBranch(t *testing.T) {
	now := time.Now().UTC()
	size := int64(1234)

	model := NewFilesModel(nil)
	model.width = 220
	model.items = []api.File{
		{
			ID:        "file-1",
			Filename:  "alpha.txt",
			FilePath:  "/tmp/alpha.txt",
			Status:    "active",
			SizeBytes: &size,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	model.dataTable.SetRows([]table.Row{{"row-1"}})
	model.dataTable.SetCursor(0)

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "alpha.txt")
	assert.Contains(t, out, "Selected")
}

func TestFilesHandleAddKeysBackspaceWithNilFormInits(t *testing.T) {
	model := NewFilesModel(nil)
	model.addName = "ab"

	// With nil form, any key press initializes the form and returns Init cmd.
	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.NotNil(t, cmd)
	assert.NotNil(t, updated.addForm)
}

func TestFilesSaveAddCreateErrorBranch(t *testing.T) {
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/files" && r.Method == http.MethodPost:
			http.Error(w, `{"error":{"code":"FILE_CREATE_FAILED","message":"create failed"}}`, http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewFilesModel(client)
	model.addName = "alpha.txt"
	model.addPath = "/tmp/alpha.txt"

	updated, cmd := model.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)

	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "FILE_CREATE_FAILED")
}

func TestFilesEditTagStrMultiTagBranch(t *testing.T) {
	model := NewFilesModel(nil)
	model.editTagStr = "alpha, beta"

	tags := parseCommaSeparated(model.editTagStr)
	assert.Equal(t, []string{"alpha", "beta"}, tags)
}

func TestLogsRenderListOutOfRangeAndTagBranches(t *testing.T) {
	now := time.Now().UTC()

	model := NewLogsModel(nil)
	model.width = 44
	model.items = []api.Log{{
		ID:        "log-1",
		LogType:   "event",
		Status:    "active",
		Timestamp: now,
	}}
	model.dataTable.SetRows([]table.Row{{"row-1"}})
	model.dataTable.SetCursor(0)

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "event")

	// Tags are now stored as comma-separated string in addTagStr/editTagStr.
	model.addTagStr = "a, b"
	assert.Equal(t, "a, b", model.addTagStr)
	model.editTagStr = "x, y"
	assert.Equal(t, "x, y", model.editTagStr)
}

func TestLogsHandleAddKeysEscapeResets(t *testing.T) {
	// With addSaved=true, Esc resets the form and returns nil cmd.
	model := NewLogsModel(nil)
	model.addType = "event"
	model.addTagStr = "tmp"
	model.addSaved = true

	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addType)
	assert.Equal(t, "", updated.addTagStr)
}

func TestLogsRenderAddValueMetaFallbackAndSaveAddErrorBranch(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 100
	model.addType = "event"

	// With nil form, renderAdd shows "Initializing..."
	out := components.SanitizeText(model.renderAdd())
	assert.NotEmpty(t, out)

	_, client := testLogsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/logs" && r.Method == http.MethodPost:
			var body map[string]any
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			http.Error(w, `{"error":{"code":"LOG_CREATE_FAILED","message":"create failed"}}`, http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model = NewLogsModel(client)
	model.addType = "event"
	model.addTimestamp = "2026-03-04"

	updated, cmd := model.saveAdd()
	require.NotNil(t, cmd)
	assert.True(t, updated.addSaving)

	msg := cmd()
	errOut, ok := msg.(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "LOG_CREATE_FAILED")
}

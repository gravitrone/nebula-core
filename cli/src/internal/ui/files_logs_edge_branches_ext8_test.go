package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
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
	model.list.SetItems([]string{"row-1"})

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "alpha.txt")
	assert.Contains(t, out, "Selected")
}

func TestFilesHandleAddKeysNameBackspaceBranch(t *testing.T) {
	model := NewFilesModel(nil)
	model.addFocus = fileFieldName
	model.addName = "ab"

	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.addName)
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

func TestFilesRenderEditTagsMultiTagSpacingBranch(t *testing.T) {
	model := NewFilesModel(nil)
	model.editTags = []string{"alpha", "beta"}

	out := stripANSI(model.renderEditTags(false))
	assert.Contains(t, out, "[alpha] [beta]")
}

func TestLogsRenderListOutOfRangeAndTagSpacingBranches(t *testing.T) {
	now := time.Now().UTC()

	model := NewLogsModel(nil)
	model.width = 44
	model.items = []api.Log{{
		ID:        "log-1",
		LogType:   "event",
		Status:    "active",
		Timestamp: now,
	}}
	// Include one stale visible row to hit abs-idx guard branch.
	model.list.SetItems([]string{"row-1", "ghost"})

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "event")
	assert.NotContains(t, out, "ghost")

	model.addTags = []string{"a", "b"}
	addTags := stripANSI(model.renderAddTags(false))
	assert.Contains(t, addTags, "[a] [b]")

	model.editTags = []string{"x", "y"}
	editTags := stripANSI(model.renderEditTags(false))
	assert.Contains(t, editTags, "[x] [y]")
}

func TestLogsHandleAddKeysUpBackAndDefaultBackspaceBranches(t *testing.T) {
	model := NewLogsModel(nil)
	model.addFocus = logFieldTimestamp

	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, logFieldType, updated.addFocus)

	updated.addType = "event"
	updated.addTagBuf = "tmp"
	updated, cmd = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.addType)
	assert.Equal(t, "", updated.addTagBuf)

	updated.addFocus = logFieldMeta
	updated, cmd = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, logFieldMeta, updated.addFocus)
}

func TestLogsRenderAddValueMetaFallbackAndSaveAddErrorBranch(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 100
	model.addType = "event"
	model.addTimestamp = "2026-03-04"

	out := components.SanitizeText(model.renderAdd())
	assert.Contains(t, out, "Value:")
	assert.Contains(t, out, "Metadata:")
	assert.Contains(t, out, "  -")

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

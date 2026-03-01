package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAndFormatFileSizeMatrix(t *testing.T) {
	size, err := parseFileSize("  ")
	require.NoError(t, err)
	assert.Nil(t, size)

	size, err = parseFileSize("2048")
	require.NoError(t, err)
	require.NotNil(t, size)
	assert.Equal(t, int64(2048), *size)

	_, err = parseFileSize("-1")
	assert.ErrorContains(t, err, "non-negative integer")

	_, err = parseFileSize("abc")
	assert.ErrorContains(t, err, "non-negative integer")

	assert.Equal(t, "12 B", formatFileSize(12))
	assert.Equal(t, "2.0 KB", formatFileSize(2048))
	assert.Equal(t, "1.0 MB", formatFileSize(1024*1024))
	assert.Equal(t, "1.0 GB", formatFileSize(1024*1024*1024))
}

func TestAppendCharFormatFormValueAndDerefString(t *testing.T) {
	value := "ab"
	appendChar(&value, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	assert.Equal(t, "abc", value)
	appendChar(&value, tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, "abc ", value)
	appendChar(&value, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, "abc ", value)

	assert.Equal(t, "-", formatFormValue("   ", false))
	assert.Contains(t, stripANSI(formatFormValue("value", true)), "value")
	assert.Equal(t, "", derefString(nil))
	text := "x"
	assert.Equal(t, "x", derefString(&text))
}

func TestFilesModeKeysAndToggleModeMatrix(t *testing.T) {
	model := NewFilesModel(nil)
	model.modeFocus = true

	updated, cmd := model.handleModeKeys(tea.KeyMsg{Type: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated.view = filesViewList
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, filesViewAdd, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyMsg{Type: tea.KeyLeft})
	require.Nil(t, cmd)
	assert.Equal(t, filesViewList, updated.view)
}

func TestFilesHandleDetailKeysBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewDetail
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a"}

	updated, cmd := model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated, _ = model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	assert.True(t, updated.metaExpanded)

	updated, _ = model.handleDetailKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	assert.Equal(t, filesViewEdit, updated.view)

	updated, _ = updated.handleDetailKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, filesViewList, updated.view)
	assert.Nil(t, updated.detail)
	assert.Nil(t, updated.detailRels)
}

func TestFilesLoadDetailRelationshipsSuccessAndError(t *testing.T) {
	now := time.Now()
	calls := 0
	_, client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Path {
		case "/api/relationships/file/file-1":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id":          "rel-1",
					"source_type": "entity",
					"source_id":   "ent-1",
					"target_type": "file",
					"target_id":   "file-1",
					"type":        "has-file",
					"status":      "active",
					"created_at":  now,
				}},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	model := NewFilesModel(client)
	msg := model.loadDetailRelationships("file-1")().(fileRelationshipsLoadedMsg) //nolint:forcetypeassert
	require.Len(t, msg.relationships, 1)
	assert.Equal(t, "rel-1", msg.relationships[0].ID)

	msg = model.loadDetailRelationships("file-2")().(fileRelationshipsLoadedMsg) //nolint:forcetypeassert
	assert.Equal(t, "file-2", msg.id)
	assert.Nil(t, msg.relationships)
	assert.Equal(t, 2, calls)
}

func TestFilesHandleAddKeysStatusTagsAndSavedBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewAdd
	model.addFocus = fileFieldStatus
	model.addStatusIdx = 0

	updated, cmd := model.handleAddKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.addStatusIdx)

	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyLeft})
	assert.Equal(t, 0, updated.addStatusIdx)

	updated.addFocus = fileFieldTags
	updated.addTags = []string{"alpha"}
	updated.addTagBuf = "z"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "", updated.addTagBuf)
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Empty(t, updated.addTags)

	updated.addSaved = true
	updated.addName = "Alpha.txt"
	updated.addPath = "/tmp/alpha.txt"
	updated, _ = updated.handleAddKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, updated.addSaved)
	assert.Equal(t, "", updated.addName)
	assert.Equal(t, "", updated.addPath)
}

func TestFilesHandleEditKeysStatusTagsAndBackBranches(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewEdit
	model.detail = &api.File{ID: "file-1", Filename: "Alpha.txt", FilePath: "/tmp/a"}
	model.startEdit()
	model.editFocus = fileFieldStatus
	model.editStatusIdx = 0

	updated, cmd := model.handleEditKeys(tea.KeyMsg{Type: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.editStatusIdx)

	updated.editFocus = fileFieldTags
	updated.editTags = []string{"alpha"}
	updated.editTagBuf = "z"
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Equal(t, "", updated.editTagBuf)
	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyBackspace})
	assert.Empty(t, updated.editTags)

	updated, _ = updated.handleEditKeys(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Equal(t, filesViewDetail, updated.view)
}

func TestRenderFilePreviewAndLineFallbacks(t *testing.T) {
	model := NewFilesModel(nil)
	now := time.Now()

	empty := api.File{ID: "file-1", Filename: "", Status: "", CreatedAt: now, UpdatedAt: time.Time{}}
	assert.Equal(t, "", model.renderFilePreview(empty, 0))

	preview := model.renderFilePreview(empty, 40)
	assert.Contains(t, preview, "Selected")
	assert.Contains(t, preview, "Status")
	assert.Contains(t, preview, "At")

	line := formatFileLine(api.File{Filename: "Alpha.txt", Status: "active", CreatedAt: now, UpdatedAt: now})
	assert.Contains(t, line, "Alpha.txt")
	assert.Contains(t, line, "active")
}

func TestFormatFileLineBranchMatrix(t *testing.T) {
	mime := "text/plain"
	size := int64(2048)
	line := formatFileLine(api.File{
		Filename: "Alpha.txt",
		MimeType: &mime,
		SizeBytes: &size,
		Status:   "active",
		Metadata: api.JSONMap{"group": map[string]any{"field": "value"}},
	})
	assert.Contains(t, line, "Alpha.txt")
	assert.Contains(t, line, "text/plain")
	assert.Contains(t, line, "2.0 KB")
	assert.Contains(t, line, "active")
	assert.Contains(t, strings.ToLower(line), "field")
	assert.Contains(t, strings.ToLower(line), "value")

	emptyMime := ""
	line = formatFileLine(api.File{
		Filename: "",
		MimeType: &emptyMime,
		Status:   "",
		Metadata: api.JSONMap{},
	})
	assert.Equal(t, "file", line)

	line = formatFileLine(api.File{
		Filename: "Name",
		Metadata: api.JSONMap{"k": "v"},
	})
	assert.Contains(t, line, "Name")
	assert.Contains(t, strings.ToLower(line), "v")
	assert.NotContains(t, line, " ·  · ")
}

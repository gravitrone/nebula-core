package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesUpdateUnknownMessageReturnsNoop(t *testing.T) {
	model := NewFilesModel(nil)
	model.view = filesViewDetail
	model.searchBuf = "alpha"

	updated, cmd := model.Update(struct{ name string }{name: "noop"})
	require.Nil(t, cmd)
	assert.Equal(t, filesViewDetail, updated.view)
	assert.Equal(t, "alpha", updated.searchBuf)
}

func TestFilesRenderListLoadingAndSmallWidthBranches(t *testing.T) {
	now := time.Now()
	size := int64(2048)

	model := NewFilesModel(nil)
	model.width = 34
	model.loading = true
	assert.Contains(t, components.SanitizeText(model.renderList()), "Loading files")

	model.loading = false
	model.items = []api.File{
		{
			ID:        "file-1",
			Filename:  "",
			Status:    "",
			CreatedAt: now,
		},
		{
			ID:        "file-2",
			Filename:  "Beta.txt",
			Status:    "active",
			SizeBytes: &size,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	model.dataTable.SetRows([]table.Row{{"row-1"}, {"row-2"}})
	model.dataTable.SetCursor(0)
	model.searchBuf = "be"
	model.searchSuggest = "Beta.txt"
	model.modeFocus = true

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "2 total")
	assert.Contains(t, out, "search:")
	assert.Contains(t, out, "next:")
	assert.Contains(t, out, "Beta.txt")
}

func TestFilesRenderFilePreviewOptionalFieldBranches(t *testing.T) {
	now := time.Now()
	size := int64(1024)
	mime := "text/plain"
	checksum := "abc123"

	model := NewFilesModel(nil)
	out := components.SanitizeText(model.renderFilePreview(api.File{
		ID:        "file-1",
		Filename:  "",
		FilePath:  "/tmp/example.txt",
		Status:    "",
		SizeBytes: &size,
		MimeType:  &mime,
		Checksum:  &checksum,
		Tags:      []string{"docs", "notes"},
		Notes: "preview",
		CreatedAt: now,
	}, 42))

	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Path")
	assert.Contains(t, out, "MIME")
	assert.Contains(t, out, "SHA")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "Notes")
}

func TestFilesHandleListKeysSpaceAndEscapeSearchBranches(t *testing.T) {
	now := time.Now()
	model := NewFilesModel(nil)
	model.all = nil
	model.applyFileSearch()

	updated, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeySpace})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)

	updated.all = []api.File{
		{ID: "file-1", Filename: "Alpha.txt", Status: "active", CreatedAt: now},
	}
	updated.applyFileSearch()
	updated.searchBuf = "Alpha"
	updated.searchSuggest = "Alpha.txt"
	updated, cmd = updated.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.searchBuf)
	assert.Equal(t, "", updated.searchSuggest)
}

func TestFilesRenderDetailFallbackAndChecksumBranches(t *testing.T) {
	now := time.Now()
	checksum := "deadbeef"

	model := NewFilesModel(nil)
	model.width = 80
	fallback := components.SanitizeText(model.renderDetail())
	assert.Contains(t, fallback, "No files found")

	model.detail = &api.File{
		ID:        "file-1",
		Filename:  "alpha.txt",
		FilePath:  "/tmp/alpha.txt",
		Checksum:  &checksum,
		CreatedAt: now,
	}
	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "Checksum")
	assert.Contains(t, out, "deadbeef")
}

package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchCommandEmptyQueryClearsState(t *testing.T) {
	model := NewSearchModel(nil)
	model.loading = true
	model.items = []searchEntry{{id: "ent-1"}}
	model.dataTable.SetRows([]table.Row{{"ent-1"}})

	cmd := model.search("   ")
	assert.Nil(t, cmd)
	assert.False(t, model.loading)
	assert.Empty(t, model.items)
	assert.Empty(t, model.dataTable.Rows())
}

func TestSearchCommandSemanticFailureReturnsErrMsg(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/search/semantic" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewSearchModel(client)
	model.mode = searchModeSemantic
	cmd := model.search("memory")
	require.NotNil(t, cmd)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestSearchCommandTextFailureAtEntityQueryReturnsErrMsg(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/entities" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewSearchModel(client)
	cmd := model.search("alpha")
	require.NotNil(t, cmd)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestSearchCommandTextFailureAtContextQueryReturnsErrMsg(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case "/api/context":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	cmd := model.search("alpha")
	require.NotNil(t, cmd)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestSearchCommandTextFailureAtJobsQueryReturnsErrMsg(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/entities":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case "/api/context":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
		case "/api/jobs":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	cmd := model.search("alpha")
	require.NotNil(t, cmd)
	_, ok := cmd().(errMsg)
	assert.True(t, ok)
}

func TestSearchEmitSelectionFetchFailuresReturnErrMsg(t *testing.T) {
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	model := NewSearchModel(client)

	cases := []struct {
		name string
		kind string
	}{
		{name: "entity", kind: "entity"},
		{name: "context", kind: "context"},
		{name: "job", kind: "job"},
		{name: "log", kind: "log"},
		{name: "file", kind: "file"},
		{name: "protocol", kind: "protocol"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := model.emitSelection(searchEntry{kind: tc.kind, id: "missing-id"})
			require.NotNil(t, cmd)
			_, ok := cmd().(errMsg)
			assert.True(t, ok)
		})
	}
}

func TestSearchEmitSelectionUnknownKindPassesThrough(t *testing.T) {
	model := NewSearchModel(nil)
	cmd := model.emitSelection(searchEntry{kind: "custom-kind", id: "x-1"})
	require.NotNil(t, cmd)
	msg := cmd().(searchSelectionMsg)
	assert.Equal(t, "custom-kind", msg.kind)
	assert.Nil(t, msg.entity)
	assert.Nil(t, msg.context)
	assert.Nil(t, msg.job)
}

func TestRenderSearchPreviewShowsEntityContextAndJobDetails(t *testing.T) {
	model := NewSearchModel(nil)
	previewEntity := components.SanitizeText(model.renderSearchPreview(searchEntry{
		kind:  "entity",
		id:    "ent-1",
		label: "Alpha Project",
		desc:  "project info",
		entity: &api.Entity{
			Type:   "project",
			Status: "active",
			Tags:   []string{"core", "ops"},
		},
	}, 56))
	assert.Contains(t, previewEntity, "Kind")
	assert.Contains(t, previewEntity, "Type")
	assert.Contains(t, previewEntity, "Status")
	assert.Contains(t, previewEntity, "Tags")

	link := "https://example.com/runbook"
	content := "deploy checklist"
	previewContext := components.SanitizeText(model.renderSearchPreview(searchEntry{
		kind:  "context",
		id:    "ctx-1",
		label: "Runbook",
		desc:  "context info",
		context: &api.Context{
			SourceType: "doc",
			Status:     "active",
			URL:        &link,
			Tags:       []string{"ops"},
			Content:    &content,
		},
	}, 56))
	assert.Contains(t, previewContext, "Source")
	assert.Contains(t, previewContext, "URL")
	assert.Contains(t, previewContext, "Preview")

	priority := "high"
	desc := "incident response"
	previewJob := components.SanitizeText(model.renderSearchPreview(searchEntry{
		kind:  "job",
		id:    "job-1",
		label: "Fix outage",
		desc:  "job info",
		job: &api.Job{
			Status:      "active",
			Priority:    &priority,
			Description: &desc,
		},
	}, 56))
	assert.Contains(t, previewJob, "Priority")
	assert.Contains(t, previewJob, "Description")
}

func TestSearchViewRendersTableAndPreviewContent(t *testing.T) {
	model := NewSearchModel(nil)
	model.width = 120
	model.textInput.SetValue("alpha")
	model.items = []searchEntry{
		{
			kind:  "entity",
			id:    "ent-1",
			label: "Alpha Node",
			desc:  "desc123",
			entity: &api.Entity{
				Type:   "project",
				Status: "active",
			},
		},
	}
	model.dataTable.SetRows([]table.Row{{"Alpha Node"}})
	model.dataTable.SetCursor(0)

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Title")
	assert.Contains(t, out, "Kind")
	assert.Contains(t, out, "Info")
	// desc appears in both the table row and the preview panel.
	assert.GreaterOrEqual(t, strings.Count(out, "desc123"), 1)
}

func TestBuildPaletteSearchEntriesLogFallbacks(t *testing.T) {
	entries := buildPaletteSearchEntries(
		"",
		nil,
		nil,
		nil,
		nil,
		[]api.Log{{ID: "log-1"}},
		nil,
		nil,
	)
	require.Len(t, entries, 1)
	assert.Equal(t, "log", entries[0].kind)
	assert.Equal(t, "log", strings.TrimSpace(entries[0].label))
	assert.Contains(t, entries[0].desc, "log")
}

func TestFilterLogsFilesProtocolsByQuery(t *testing.T) {
	logs := []api.Log{
		{ID: "log-1", LogType: "event", Status: "active", Content: "deploy ok"},
	}
	filteredLogs := filterLogsByQuery(logs, "deploy")
	require.Len(t, filteredLogs, 1)
	assert.Equal(t, "log-1", filteredLogs[0].ID)

	mime := "text/markdown"
	files := []api.File{
		{ID: "file-1", Filename: "notes.md", FilePath: "vault/notes.md", MimeType: &mime},
	}
	filteredFiles := filterFilesByQuery(files, "markdown")
	require.Len(t, filteredFiles, 1)
	assert.Equal(t, "file-1", filteredFiles[0].ID)

	kind := "checklist"
	protos := []api.Protocol{
		{ID: "proto-1", Name: "ops", Title: "Ops Checklist", ProtocolType: &kind},
	}
	filteredProtocols := filterProtocolsByQuery(protos, "checklist")
	require.Len(t, filteredProtocols, 1)
	assert.Equal(t, "proto-1", filteredProtocols[0].ID)
}

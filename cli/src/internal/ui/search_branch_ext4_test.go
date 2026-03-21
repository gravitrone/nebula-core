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

func TestSearchUpdateBackClearsActiveQueryState(t *testing.T) {
	model := NewSearchModel(nil)
	model.query = "alpha"
	model.items = []searchEntry{{id: "ent-1"}}
	model.list.SetItems([]string{"ent-1"})
	model.loading = true

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	require.Nil(t, cmd)
	assert.Equal(t, "", updated.query)
	assert.Empty(t, updated.items)
	assert.Empty(t, updated.list.Items)
	assert.False(t, updated.loading)
}

func TestSearchViewCoversTinyWidthAndRowFallbacks(t *testing.T) {
	model := NewSearchModel(nil)
	model.width = 0
	model.query = "x"
	model.items = []searchEntry{
		{kind: "", id: "ent-1", label: "", desc: ""},
	}
	// Keep one extra list row so rel->abs index can exceed item slice bounds.
	model.list.SetItems([]string{"row-1", "row-2"})

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Selected")
}

func TestSearchViewUsesSideBySidePreviewWhenWide(t *testing.T) {
	model := NewSearchModel(nil)
	model.width = 220
	model.query = "alpha"
	model.items = []searchEntry{
		{
			kind:  "entity",
			id:    "ent-1",
			label: "Alpha",
			desc:  "project",
			entity: &api.Entity{
				Type:   "tool",
				Status: "active",
			},
		},
	}
	model.list.SetItems([]string{"Alpha"})

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "1 results")
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "Alpha")
}

func TestRenderSearchPreviewFallbackBranches(t *testing.T) {
	model := NewSearchModel(nil)
	assert.Equal(t, "", model.renderSearchPreview(searchEntry{}, 0))

	out := components.SanitizeText(model.renderSearchPreview(searchEntry{}, 32))
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "result")
	assert.Contains(t, out, "Kind")
}

func TestBuildSemanticAndSearchEntriesFallbacks(t *testing.T) {
	semantic := buildSemanticEntries([]api.SemanticSearchResult{
		{Kind: "entity", ID: "ent-1", Score: 0.55},
	})
	require.Len(t, semantic, 1)
	assert.Equal(t, "ent-1", semantic[0].label)

	items := buildSearchEntries(
		"",
		nil,
		[]api.Context{{ID: "ctx-1", Name: "Context Node"}},
		[]api.Job{{ID: "job-1", Title: "Job Node"}},
	)
	require.Len(t, items, 2)
	assert.Contains(t, items[0].desc, "context")
	assert.Contains(t, items[1].desc, "job")
}

func TestBuildPaletteSearchEntriesFileAndProtocolLabelFallbacks(t *testing.T) {
	items := buildPaletteSearchEntries(
		"",
		nil,
		nil,
		nil,
		nil,
		nil,
		[]api.File{{ID: "file-1"}},
		[]api.Protocol{{ID: "proto-1"}},
	)
	require.Len(t, items, 2)
	assert.Equal(t, shortID("file-1"), items[0].label)
	assert.Equal(t, shortID("proto-1"), items[1].label)
}

func TestSearchEmitSelectionFetchesContextAndJob(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, client := searchTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/context/ctx-1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":          "ctx-1",
					"title":       "Runbook",
					"name":        "Runbook",
					"source_type": "doc",
					"status":      "active",
					"tags":        []string{},
					"created_at":  now,
					"updated_at":  now,
				},
			}))
		case "/api/jobs/job-1":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":         "job-1",
					"title":      "Fix outage",
					"status":     "active",
					"created_at": now,
					"updated_at": now,
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewSearchModel(client)
	ctxMsg := model.emitSelection(searchEntry{kind: "context", id: "ctx-1"})().(searchSelectionMsg)
	require.NotNil(t, ctxMsg.context)
	assert.Equal(t, "ctx-1", ctxMsg.context.ID)

	jobMsg := model.emitSelection(searchEntry{kind: "job", id: "job-1"})().(searchSelectionMsg)
	require.NotNil(t, jobMsg.job)
	assert.Equal(t, "job-1", jobMsg.job.ID)
}

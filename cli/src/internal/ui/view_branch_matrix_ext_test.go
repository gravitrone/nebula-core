package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

func TestImportExportViewStepFormatBranch(t *testing.T) {
	model := NewImportExportModel(nil)
	model.width = 80
	model.step = stepFormat

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "JSON")
	assert.Contains(t, out, "CSV")
}

func TestRelationshipsViewDetailBranch(t *testing.T) {
	model := NewRelationshipsModel(nil)
	model.width = 100
	model.view = relsViewDetail
	rel := api.Relationship{
		ID:         "r1",
		SourceType: "entity",
		SourceID:   "e1",
		SourceName: "Alpha",
		TargetType: "entity",
		TargetID:   "e2",
		TargetName: "Beta",
		Type:       "related-to",
		Status:     "active",
		Properties: api.JSONMap{},
		CreatedAt:  time.Now().UTC(),
	}
	model.detail = &rel

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "related-to")
	assert.Contains(t, out, "Alpha")
	assert.Contains(t, out, "Beta")
}

func TestProfileViewAgentDetailBranch(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	model.width = 100
	desc := "test agent"
	now := time.Now().UTC()
	model.agentDetail = &api.Agent{
		ID:               "ag-1",
		Name:             "alpha",
		Status:           "active",
		Scopes:           []string{"public"},
		Capabilities:     []string{"read"},
		RequiresApproval: false,
		Description:      &desc,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "trusted")
	assert.Contains(t, out, "test agent")
}

func TestLogsRenderListAvailableColsFloorBranch(t *testing.T) {
	model := NewLogsModel(nil)
	model.width = 20
	model.loading = false
	now := time.Now().UTC()
	model.items = []api.Log{{
		ID:        "l1",
		LogType:   "event",
		Timestamp: now,
		Value:     api.JSONMap{"text": "x"},
		Status:    "active",
		Tags:      []string{"smoke"},
		Metadata:  api.JSONMap{},
		CreatedAt: now,
		UpdatedAt: now,
	}}
	model.list.SetItems([]string{"event"})

	out := components.SanitizeText(model.renderList())
	assert.Contains(t, out, "event")
}

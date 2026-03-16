package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestJobsRenderJobPreviewBranchMatrix(t *testing.T) {
	model := NewJobsModel(nil)
	assert.Equal(t, "", model.renderJobPreview(api.Job{}, 0))

	now := time.Now().UTC()
	out := components.SanitizeText(model.renderJobPreview(api.Job{
		ID:        "job-1",
		Title:     "",
		Status:    "",
		CreatedAt: now,
	}, 44))
	assert.Contains(t, out, "Selected")
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "Status: -")
	assert.Contains(t, out, "Priority: -")

	priority := "high"
	desc := "  this is a note  "
	updated := now.Add(2 * time.Hour)
	model.detail = &api.Job{ID: "job-2"}
	model.detailRels = []api.Relationship{{ID: "rel-1"}}
	model.detailContext = []api.Context{{ID: "ctx-1"}}
	out = components.SanitizeText(model.renderJobPreview(api.Job{
		ID:          "job-2",
		Title:       "Ship release",
		Status:      "active",
		Priority:    &priority,
		Description: &desc,
		CreatedAt:   now,
		UpdatedAt:   updated,
	}, 44))
	assert.Contains(t, out, "Ship release")
	assert.Contains(t, out, "Status: active")
	assert.Contains(t, out, "Priority: high")
	assert.Contains(t, out, "Links: 1")
	assert.Contains(t, out, "Context: 1")
	assert.Contains(t, out, "Notes:")

	other := components.SanitizeText(model.renderJobPreview(api.Job{
		ID:          "job-3",
		Title:       "Other",
		Status:      "pending",
		Priority:    &priority,
		Description: &desc,
		CreatedAt:   now,
		UpdatedAt:   updated,
	}, 44))
	assert.NotContains(t, other, "Links: ")
}

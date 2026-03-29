package ui

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileSelectedTaxonomyEmptyItemsBranch(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	// taxItems is empty, so selectedTaxonomy returns nil
	assert.Nil(t, model.selectedTaxonomy())
}

func TestProfileRenderTaxonomyAdditionalBranchMatrix(t *testing.T) {
	now := time.Now().UTC()
	desc := "core scope"

	t.Run("narrow layout without preview and stale visible row index", func(t *testing.T) {
		model := NewProfileModel(nil, &config.Config{})
		model.width = 72
		model.taxKind = 0
		model.taxLoading = false
		model.taxIncludeInactive = false
		model.taxSearch = ""
		model.sectionFocus = true
		model.taxItems = []api.TaxonomyEntry{{
			ID:        "scope-1",
			Name:      "",
			IsBuiltin: false,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		}}
		model.taxList.SetRows([]table.Row{{"row-1"}, {"stale-row"}})
		model.taxList.SetCursor(0)

		out := model.renderTaxonomy()
		// Count line is hidden when no search/filter is active.
		assert.NotEmpty(t, out)
	})

	t.Run("wide layout with preview and flags matrix", func(t *testing.T) {
		model := NewProfileModel(nil, &config.Config{})
		model.width = 160
		model.taxKind = 1
		model.taxLoading = false
		model.taxIncludeInactive = true
		model.taxSearch = "entity"
		model.taxItems = []api.TaxonomyEntry{{
			ID:          "type-1",
			Name:        "person",
			Description: &desc,
			IsBuiltin:   true,
			IsActive:    false,
			CreatedAt:   now,
			UpdatedAt:   now,
		}}
		model.taxList.SetRows([]table.Row{{formatTaxonomyLine(model.taxItems[0])}})
		model.taxList.SetCursor(0)

		selected := model.selectedTaxonomy()
		require.NotNil(t, selected)
		assert.Equal(t, "type-1", selected.ID)

		out := model.renderTaxonomy()
		assert.Contains(t, out, "include inactive: true")
		assert.Contains(t, out, "filter: entity")
		assert.Contains(t, out, "Selected")
		assert.Contains(t, out, "inactive")
	})

	t.Run("narrow layout with preview stacks table and preview", func(t *testing.T) {
		model := NewProfileModel(nil, &config.Config{})
		model.width = 92
		model.taxKind = 0
		model.taxLoading = false
		model.taxIncludeInactive = false
		model.taxSearch = "scope"
		model.taxItems = []api.TaxonomyEntry{{
			ID:        "scope-2",
			Name:      "private",
			IsBuiltin: false,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		}}
		model.taxList.SetRows([]table.Row{{formatTaxonomyLine(model.taxItems[0])}})
		model.taxList.SetCursor(0)

		out := model.renderTaxonomy()
		assert.Contains(t, out, "Selected")
		assert.Contains(t, out, "filter: scope")
	})

	t.Run("wide layout without preview when section focus suppresses preview", func(t *testing.T) {
		model := NewProfileModel(nil, &config.Config{})
		model.width = 180
		model.taxKind = 3
		model.taxLoading = false
		model.taxIncludeInactive = true
		model.taxSearch = ""
		model.sectionFocus = true
		model.taxItems = []api.TaxonomyEntry{{
			ID:        "log-1",
			Name:      "journal",
			IsBuiltin: true,
			IsActive:  true,
			CreatedAt: now,
			UpdatedAt: now,
		}}
		model.taxList.SetRows([]table.Row{{formatTaxonomyLine(model.taxItems[0])}})
		model.taxList.SetCursor(0)

		out := model.renderTaxonomy()
		assert.NotContains(t, out, "Selected")
	})

	t.Run("tiny width clamps column budgets and still renders", func(t *testing.T) {
		model := NewProfileModel(nil, &config.Config{})
		model.width = 24
		model.taxKind = 0
		model.taxLoading = false
		model.taxItems = []api.TaxonomyEntry{{
			ID:          "scope-3",
			Name:        "public",
			Description: &desc,
			IsBuiltin:   true,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		}}
		model.taxList.SetRows([]table.Row{{formatTaxonomyLine(model.taxItems[0])}})
		model.taxList.SetCursor(0)

		out := model.renderTaxonomy()
		assert.Contains(t, out, "Name")
		assert.Contains(t, out, "Flags")
		assert.Contains(t, out, "Description")
	})
}

func TestRenderTaxonomyPreviewEmptyNameFallsBackToLabel(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	item := api.TaxonomyEntry{
		ID:       "scope-9",
		Name:     "   ",
		IsActive: true,
	}

	preview := model.renderTaxonomyPreview(item, 32)
	assert.Contains(t, preview, "Selected")
	assert.Contains(t, preview, "taxonomy")
}

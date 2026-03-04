package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviewTagsMatrix(t *testing.T) {
	assert.Equal(t, "", previewTags(nil, 2))
	assert.Equal(t, "", previewTags([]string{"a"}, 0))
	assert.Equal(t, "alpha, beta", previewTags([]string{"alpha", "beta"}, 2))
	assert.Equal(t, "alpha, beta +1", previewTags([]string{"alpha", "beta", "gamma"}, 2))
}

func TestEntitiesPruneBulkSelection(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.bulkSelected = map[string]bool{
		"ent-1": true,
		"ent-2": true,
	}

	model.pruneBulkSelection([]api.Entity{{ID: "ent-2"}, {ID: ""}})
	assert.False(t, model.bulkSelected["ent-1"])
	assert.True(t, model.bulkSelected["ent-2"])
}

func TestEntitiesMatchesFiltersMatrix(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.scopeNames = map[string]string{
		"scope-public-id":  "public",
		"scope-private-id": "private",
	}
	item := api.Entity{
		ID:              "ent-1",
		Name:            "Alpha",
		Type:            "project",
		Status:          "active",
		PrivacyScopeIDs: []string{"scope-public-id"},
	}

	assert.True(t, model.matchesEntityFilters(item))

	model.filterTypes = map[string]bool{"project": true}
	assert.True(t, model.matchesEntityFilters(item))
	model.filterTypes = map[string]bool{"person": true}
	assert.False(t, model.matchesEntityFilters(item))
	model.filterTypes = map[string]bool{"project": true}
	item.Type = ""
	assert.False(t, model.matchesEntityFilters(item))
	item.Type = "project"

	model.filterTypes = nil
	model.filterStatus = map[string]bool{"active": true}
	assert.True(t, model.matchesEntityFilters(item))
	model.filterStatus = map[string]bool{"archived": true}
	assert.False(t, model.matchesEntityFilters(item))
	model.filterStatus = map[string]bool{"active": true}
	item.Status = "   "
	assert.False(t, model.matchesEntityFilters(item))
	item.Status = "active"

	model.filterStatus = nil
	model.filterScopes = map[string]bool{"public": true}
	assert.True(t, model.matchesEntityFilters(item))
	model.filterScopes = map[string]bool{"admin": true}
	assert.False(t, model.matchesEntityFilters(item))

	model.filterScopes = map[string]bool{"public": true}
	model.scopeNames["scope-empty-id"] = "   "
	item.PrivacyScopeIDs = []string{"scope-empty-id"}
	assert.False(t, model.matchesEntityFilters(item))

	item.PrivacyScopeIDs = []string{"scope-empty-id", "scope-public-id"}
	assert.True(t, model.matchesEntityFilters(item))
}

func TestEntitiesApplyFiltersPrunesSelectionAndList(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.scopeNames = map[string]string{"scope-public-id": "public"}
	now := time.Now()
	model.allItems = []api.Entity{
		{ID: "ent-1", Name: "Alpha", Type: "project", Status: "active", PrivacyScopeIDs: []string{"scope-public-id"}, CreatedAt: now},
		{ID: "ent-2", Name: "Beta", Type: "person", Status: "active", PrivacyScopeIDs: []string{"scope-public-id"}, CreatedAt: now},
	}
	model.bulkSelected = map[string]bool{"ent-1": true, "ent-2": true}
	model.filterTypes = map[string]bool{"project": true}

	model.applyEntityFilters()

	require.Len(t, model.items, 1)
	assert.Equal(t, "ent-1", model.items[0].ID)
	assert.True(t, model.bulkSelected["ent-1"])
	assert.False(t, model.bulkSelected["ent-2"])
	require.Len(t, model.list.Items, 1)
}

func TestFormatEntityHeaderAndJoinSegmentsMatrix(t *testing.T) {
	header := stripANSI(formatEntityHeader("Alpha Project", "project", 80))
	assert.Contains(t, header, "Alpha Project")
	assert.Contains(t, header, "project")

	short := stripANSI(formatEntityHeader("Very Long Entity Name", "project", 10))
	assert.NotEmpty(t, short)

	unknownType := stripANSI(formatEntityHeader("Alpha", "", 80))
	assert.Contains(t, unknownType, "?")

	joined := joinEntitySegments([]string{"one", "two", "three"}, 9)
	assert.Equal(t, "one · two", joined)
	assert.Equal(t, "", joinEntitySegments(nil, 10))
}

func TestFormatEntityLineWidthNormalizesWidthTypeAndSegments(t *testing.T) {
	line := stripANSI(formatEntityLineWidth(api.Entity{
		Name:   "Alpha Entity",
		Type:   "",
		Status: "",
		Tags:   []string{"tag-a", "tag-b", "tag-c"},
		Metadata: api.JSONMap{
			"summary": "metadata preview should show",
		},
	}, maxEntityLineLen+200))

	assert.Contains(t, line, "Alpha Entity")
	assert.Contains(t, line, "?")
	assert.Contains(t, line, "tag-a, tag-b +1")
	assert.Contains(t, line, "metadata preview should show")
	assert.LessOrEqual(t, len([]rune(line)), maxEntityLineLen)

	compact := stripANSI(formatEntityLineWidth(api.Entity{Name: "Alpha", Type: "person"}, 0))
	assert.Contains(t, compact, "Alpha")
	assert.Contains(t, compact, "person")
}

func TestRenderFilterPickerFocusedFacetWithoutSelectionCount(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 96
	model.filterFacet = entitiesFilterFacetType
	model.filterTypeSet = []string{"person", "project"}
	model.filterCursor[entitiesFilterFacetType] = 99
	model.filterTypes = map[string]bool{}
	model.filterStatus = map[string]bool{}
	model.filterScopes = map[string]bool{}

	out := stripANSI(model.renderFilterPicker())
	assert.Contains(t, out, "Filter Entities")
	assert.Contains(t, out, "Type")
	assert.NotContains(t, out, "Type (")
	assert.Contains(t, out, "No active filters")
	assert.Contains(t, out, "person")
	assert.Contains(t, out, "project")
}

func TestRenderFilterPickerTinyWidthClampWithActiveScopeSelection(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 1
	model.filterFacet = entitiesFilterFacetScope
	model.filterScopeSet = []string{"public"}
	model.filterScopes = map[string]bool{"public": true}
	model.filterCursor[entitiesFilterFacetScope] = 0

	out := stripANSI(model.renderFilterPicker())
	assert.Contains(t, out, "Filter Entities")
	assert.Contains(t, out, "Scope (1)")
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "Active: scope=1")
}

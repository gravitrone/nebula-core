package ui

import (
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesRenderEditTagsAndRenderEditBranches(t *testing.T) {
	model := NewEntitiesModel(nil)

	assert.Equal(t, "-", model.renderEditTags(false))

	model.editTags = []string{"alpha"}
	model.editTagBuf = "beta"
	out := components.SanitizeText(model.renderEditTags(false))
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")

	out = components.SanitizeText(model.renderEditTags(true))
	assert.Contains(t, out, "alpha")
	assert.Contains(t, out, "beta")
	assert.Contains(t, out, "█")

	// nil detail branch falls back to list rendering.
	model = NewEntitiesModel(nil)
	model.width = 80
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "No entities found")

	// detail matrix across focus branches.
	model = NewEntitiesModel(nil)
	model.width = 90
	model.detail = &api.Entity{
		ID:              "ent-1",
		Name:            "Alpha",
		Status:          "active",
		Tags:            []string{"alpha"},
		PrivacyScopeIDs: []string{"scope-1"},
	}
	model.scopeNames = map[string]string{"scope-1": "public"}
	model.scopeOptions = []string{"public", "private"}
	model.editTags = []string{"alpha"}
	model.editScopes = []string{"public"}
	model.editStatusIdx = 1

	model.editFocus = editFieldTags
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Entity: Alpha")
	assert.Contains(t, out, "Tags:")

	model.editFocus = editFieldStatus
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Status:")
	assert.Contains(t, out, "inactive")

	model.editFocus = editFieldScopes
	model.editScopeSelecting = true
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Scopes:")
	assert.Contains(t, out, "public")

	model.editScopeSelecting = false
	model.editFocus = editFieldScopes
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Scopes:")
	assert.Contains(t, out, "public")

	model.editMeta.Buffer = ""
	model.editFocus = editFieldMetadata
	model.editSaving = true
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Metadata:")
	assert.Contains(t, out, "-")
	assert.Contains(t, out, "Saving...")
}

func TestEntitiesSyncDetailMetadataRowsAndMetaInspectBranches(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 90
	model.height = 20
	model.detail = &api.Entity{
		ID:   "ent-1",
		Name: "Alpha",
		Metadata: map[string]any{
			"profile": map[string]any{
				"name": "Alpha",
				"bio":  "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9",
			},
		},
	}

	model.metaExpanded = false
	model.metaInspect = true
	model.metaInspectI = 99
	model.metaSelectMode = true
	model.metaSelected = map[int]bool{-1: true, 99: true}
	model.syncDetailMetadataRows()
	require.NotNil(t, model.metaList)
	require.NotNil(t, model.metaSelected)
	assert.False(t, model.metaInspect)
	assert.False(t, model.metaSelectMode)
	require.Greater(t, len(model.metaRows), 0)

	model.metaExpanded = true
	model.metaInspect = true
	model.metaInspectI = 999 // out-of-range should close
	model.metaSelected = map[int]bool{0: true, 999: true}
	model.syncDetailMetadataRows()
	assert.False(t, model.metaInspect)
	assert.True(t, model.metaSelected[0])
	assert.False(t, model.metaSelected[999])

	model.openMetaInspect(0)
	lines := model.metaInspectLines()
	require.NotEmpty(t, lines)
	rendered := components.SanitizeText(model.renderMetaInspect())
	assert.Contains(t, rendered, "Lines")

	model.metaInspect = false
	assert.Nil(t, model.metaInspectLines())
	assert.Equal(t, "", model.renderMetaInspect())
}

func TestEntitiesSyncDetailMetadataRowsInitializesListAndSelectionMap(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.detail = &api.Entity{ID: "ent-1", Name: "Alpha", Metadata: api.JSONMap{}}
	model.metaList = nil
	model.metaSelected = nil
	model.metaExpanded = false
	model.metaInspect = true
	model.metaInspectI = 0

	model.syncDetailMetadataRows()

	require.NotNil(t, model.metaList)
	require.NotNil(t, model.metaSelected)
	assert.Empty(t, model.metaRows)
	assert.False(t, model.metaInspect)
}

func TestEntitiesViewBranchMatrix(t *testing.T) {
	now := time.Now()
	base := NewEntitiesModel(nil)
	base.width = 90
	base.height = 24
	base.scopeNames = map[string]string{"scope-1": "public"}
	base.scopeOptions = []string{"public", "private"}
	base.items = []api.Entity{{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active", Tags: []string{"core"}, CreatedAt: now}}
	base.allItems = append([]api.Entity{}, base.items...)
	base.applyEntityFilters()
	base.detail = &api.Entity{ID: "ent-1", Name: "Alpha", Type: "person", Status: "active", Tags: []string{"core"}, CreatedAt: now}
	base.rels = []api.Relationship{{ID: "rel-1", SourceID: "ent-1", TargetID: "ent-2", TargetName: "Beta", Type: "uses", Status: "active", CreatedAt: now}}
	base.relList.SetItems([]string{"uses"})
	base.history = []api.AuditEntry{{ID: "a1", Action: "update", ChangedAt: now}}
	base.historyList = components.NewList(8)
	base.historyList.SetItems([]string{formatHistoryLine(base.history[0])})
	base.relateResults = []api.Entity{{ID: "ent-2", Name: "Beta", Type: "tool", Status: "active"}}
	base.relateList.SetItems([]string{"Beta"})
	base.relEditBuf = "{}"

	model := base
	model.addMeta.Active = true
	_ = components.SanitizeText(model.View())

	model = base
	model.editMeta.Active = true
	_ = components.SanitizeText(model.View())

	model = base
	model.view = entitiesViewList
	model.bulkPrompt = "Bulk Tags"
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Bulk Tags")

	model = base
	model.view = entitiesViewList
	model.filtering = true
	_ = components.SanitizeText(model.View())

	model = base
	model.view = entitiesViewAdd
	model.addSaving = true
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Saving")

	model = base
	model.view = entitiesViewAdd
	model.addSaved = true
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Entity saved")

	model = base
	model.view = entitiesViewAdd
	_ = components.SanitizeText(model.View())

	model = base
	model.view = entitiesViewSearch
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Search Entities")

	model = base
	model.view = entitiesViewEdit
	model.startEdit()
	_ = components.SanitizeText(model.View())

	model = base
	model.view = entitiesViewConfirm
	model.confirmKind = "entity-archive"
	model.confirmReturn = entitiesViewDetail
	_ = components.SanitizeText(model.View())

	model = base
	model.view = entitiesViewRelationships
	model.relLoading = true
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Loading relationships")

	model = base
	model.view = entitiesViewRelateSearch
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Search Entity")

	model = base
	model.view = entitiesViewRelateType
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Relationship Type")

	model = base
	model.view = entitiesViewRelEdit
	_ = components.SanitizeText(model.View())

	model = base
	model.view = entitiesViewDetail
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Alpha")

	model = base
	model.view = entitiesViewHistory
	model.historyLoading = true
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Loading history")

	model = base
	model.view = entitiesView(99)
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Library")
}

package ui

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestEntitiesRenderEditShowsFormAndSaving(t *testing.T) {
	// nil detail branch falls back to list rendering.
	model := NewEntitiesModel(nil)
	model.width = 80
	out := components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "No entities found")

	// detail with form renders entity name and form fields.
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
	model.editTagStr = "alpha"
	model.editStatus = "inactive"
	model.editScopeStr = "public"
	model.initEditForm()
	_ = model.editForm.Init()

	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Entity: Alpha")
	assert.Contains(t, out, "Tags")
	assert.Contains(t, out, "Status")
	assert.Contains(t, out, "Scopes")

	model.editSaving = true
	out = components.SanitizeText(model.renderEdit())
	assert.Contains(t, out, "Saving...")
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
	base.relTable.SetRows([]table.Row{{"uses"}})
	base.history = []api.AuditEntry{{ID: "a1", Action: "update", ChangedAt: now}}
	base.historyTable.SetRows([]table.Row{{formatHistoryLine(base.history[0])}})
	base.relateResults = []api.Entity{{ID: "ent-2", Name: "Beta", Type: "tool", Status: "active"}}
	base.relateTable.SetRows([]table.Row{{"Beta"}})
	base.relEditBuf = "{}"

	model := base
	model.view = entitiesViewList
	model.bulkPrompt = "Bulk Tags"
	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Bulk Tags")

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
	model.view = entitiesViewSearch
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Search Entities")

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

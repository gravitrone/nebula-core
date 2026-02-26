package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestApplySearchSelectionRoutesByKind(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	model, _ := app.applySearchSelection(searchSelectionMsg{
		kind:   "entity",
		entity: &api.Entity{ID: "ent-1"},
	})
	app = model.(App)
	assert.Equal(t, tabEntities, app.tab)
	assert.Equal(t, entitiesViewDetail, app.entities.view)
	assert.Equal(t, "ent-1", app.entities.detail.ID)

	model, _ = app.applySearchSelection(searchSelectionMsg{
		kind:    "context",
		context: &api.Context{ID: "ctx-1"},
	})
	app = model.(App)
	assert.Equal(t, tabKnow, app.tab)
	assert.Equal(t, contextViewDetail, app.know.view)
	assert.Equal(t, "ctx-1", app.know.detail.ID)

	model, _ = app.applySearchSelection(searchSelectionMsg{
		kind: "job",
		job:  &api.Job{ID: "job-1"},
	})
	app = model.(App)
	assert.Equal(t, tabJobs, app.tab)
	assert.Equal(t, "job-1", app.jobs.detail.ID)

	model, _ = app.applySearchSelection(searchSelectionMsg{
		kind: "relationship",
		rel:  &api.Relationship{ID: "rel-1"},
	})
	app = model.(App)
	assert.Equal(t, tabRelations, app.tab)
	assert.Equal(t, relsViewDetail, app.rels.view)
	assert.Equal(t, "rel-1", app.rels.detail.ID)

	model, _ = app.applySearchSelection(searchSelectionMsg{
		kind: "log",
		log:  &api.Log{ID: "log-1"},
	})
	app = model.(App)
	assert.Equal(t, tabLogs, app.tab)
	assert.Equal(t, logsViewDetail, app.logs.view)
	assert.Equal(t, "log-1", app.logs.detail.ID)

	model, _ = app.applySearchSelection(searchSelectionMsg{
		kind: "file",
		file: &api.File{ID: "file-1"},
	})
	app = model.(App)
	assert.Equal(t, tabFiles, app.tab)
	assert.Equal(t, filesViewDetail, app.files.view)
	assert.Equal(t, "file-1", app.files.detail.ID)

	model, _ = app.applySearchSelection(searchSelectionMsg{
		kind:  "protocol",
		proto: &api.Protocol{ID: "proto-1"},
	})
	app = model.(App)
	assert.Equal(t, tabProtocols, app.tab)
	assert.Equal(t, protocolsViewDetail, app.protocols.view)
	assert.Equal(t, "proto-1", app.protocols.detail.ID)
}

func TestApplySearchSelectionIgnoresNilPayloads(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabInbox

	model, _ := app.applySearchSelection(searchSelectionMsg{kind: "entity"})
	updated := model.(App)
	assert.Equal(t, tabInbox, updated.tab)

	model, _ = app.applySearchSelection(searchSelectionMsg{kind: "unknown"})
	updated = model.(App)
	assert.Equal(t, tabInbox, updated.tab)
}

func TestHasUnsavedMatrix(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	assert.False(t, app.hasUnsaved())

	app.inbox.rejecting = true
	assert.True(t, app.hasUnsaved())

	app = NewApp(nil, &config.Config{})
	app.entities.view = entitiesViewEdit
	assert.True(t, app.hasUnsaved())

	app = NewApp(nil, &config.Config{})
	app.rels.view = relsViewEdit
	assert.True(t, app.hasUnsaved())

	app = NewApp(nil, &config.Config{})
	app.know.view = contextViewAdd
	app.know.fields[fieldTitle].value = "draft"
	assert.True(t, app.hasUnsaved())

	app = NewApp(nil, &config.Config{})
	app.jobs.changingSt = true
	assert.True(t, app.hasUnsaved())

	app = NewApp(nil, &config.Config{})
	app.profile.creating = true
	assert.True(t, app.hasUnsaved())

	app = NewApp(nil, &config.Config{})
	app.profile.taxPromptMode = taxPromptFilter
	assert.True(t, app.hasUnsaved())
}

func TestContextHasInputMatrix(t *testing.T) {
	model := NewContextModel(nil)
	assert.False(t, contextHasInput(model))

	model.fields[fieldTitle].value = "hello"
	assert.True(t, contextHasInput(model))

	model = NewContextModel(nil)
	model.tagBuf = "tag-1"
	assert.True(t, contextHasInput(model))

	model = NewContextModel(nil)
	model.linkEntities = []api.Entity{{ID: "ent-1"}}
	assert.True(t, contextHasInput(model))
}

func TestCanExitToTabNavMatrix(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabInbox
	assert.True(t, app.canExitToTabNav())
	app.inbox.detail = &api.Approval{ID: "ap-1"}
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabEntities
	assert.True(t, app.canExitToTabNav())
	app.entities.list.SetItems([]string{"one", "two"})
	app.entities.list.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabRelations
	assert.True(t, app.canExitToTabNav())
	app.rels.list.SetItems([]string{"one", "two"})
	app.rels.list.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabKnow
	app.know.view = contextViewAdd
	app.know.focus = fieldTitle
	assert.True(t, app.canExitToTabNav())
	app.know.modeFocus = true
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabJobs
	assert.True(t, app.canExitToTabNav())
	app.jobs.list.SetItems([]string{"one", "two"})
	app.jobs.list.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabLogs
	assert.True(t, app.canExitToTabNav())
	app.logs.list.SetItems([]string{"one", "two"})
	app.logs.list.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabFiles
	assert.True(t, app.canExitToTabNav())
	app.files.list.SetItems([]string{"one", "two"})
	app.files.list.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabProtocols
	assert.True(t, app.canExitToTabNav())
	app.protocols.list.SetItems([]string{"one", "two"})
	app.protocols.list.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabHistory
	assert.True(t, app.canExitToTabNav())
	app.history.list.SetItems([]string{"one", "two"})
	app.history.list.Down()
	assert.False(t, app.canExitToTabNav())
}

func TestCanExitToTabNavProfileSections(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabProfile

	app.profile.sectionFocus = true
	assert.True(t, app.canExitToTabNav())

	app.profile.sectionFocus = false
	app.profile.section = 0
	assert.True(t, app.canExitToTabNav())
	app.profile.keyList.SetItems([]string{"k1", "k2"})
	app.profile.keyList.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.section = 1
	assert.True(t, app.canExitToTabNav())
	app.profile.agentList.SetItems([]string{"a1", "a2"})
	app.profile.agentList.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.section = 2
	assert.True(t, app.canExitToTabNav())
	app.profile.taxList.SetItems([]string{"t1", "t2"})
	app.profile.taxList.Down()
	assert.False(t, app.canExitToTabNav())

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.creating = true
	assert.False(t, app.canExitToTabNav())
}

func TestFocusModeLineForActiveTabMatrix(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.entities.view = entitiesViewList
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.entities.modeFocus)

	app = NewApp(nil, &config.Config{})
	app.tab = tabRelations
	app.rels.view = relsViewCreateSourceSearch
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.rels.modeFocus)

	app = NewApp(nil, &config.Config{})
	app.tab = tabKnow
	app.know.view = contextViewAdd
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.know.modeFocus)

	app = NewApp(nil, &config.Config{})
	app.tab = tabJobs
	app.jobs.view = jobsViewAdd
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.jobs.modeFocus)

	app = NewApp(nil, &config.Config{})
	app.tab = tabLogs
	app.logs.view = logsViewAdd
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.logs.modeFocus)

	app = NewApp(nil, &config.Config{})
	app.tab = tabFiles
	app.files.view = filesViewAdd
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.files.modeFocus)

	app = NewApp(nil, &config.Config{})
	app.tab = tabProtocols
	app.protocols.view = protocolsViewEdit
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.protocols.modeFocus)
}

func TestFocusModeLineForActiveTabProfileAndNegativeCases(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.tab = tabProfile
	assert.True(t, app.focusModeLineForActiveTab())
	assert.True(t, app.profile.sectionFocus)

	app = NewApp(nil, &config.Config{})
	app.tab = tabProfile
	app.profile.creating = true
	assert.False(t, app.focusModeLineForActiveTab())

	app = NewApp(nil, &config.Config{})
	app.tab = tabEntities
	app.entities.view = entitiesViewDetail
	assert.False(t, app.focusModeLineForActiveTab())
}

func TestTabIndexForKeyMatrix(t *testing.T) {
	for i, key := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"} {
		idx, ok := tabIndexForKey(key)
		assert.True(t, ok)
		assert.Equal(t, i, idx)
	}

	idx, ok := tabIndexForKey("0")
	assert.True(t, ok)
	assert.Equal(t, 9, idx)

	idx, ok = tabIndexForKey("x")
	assert.False(t, ok)
	assert.Equal(t, 0, idx)
}

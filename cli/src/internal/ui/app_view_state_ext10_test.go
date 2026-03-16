package ui

import (
	"fmt"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestAppViewStateKeyAdditionalBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	app.tab = tabInbox
	app.inbox.rejecting = true
	assert.Equal(t, fmt.Sprintf("tab:%d:inbox:reject", tabInbox), app.viewStateKey())

	app.inbox.rejecting = false
	app.inbox.confirming = true
	assert.Equal(t, fmt.Sprintf("tab:%d:inbox:confirm", tabInbox), app.viewStateKey())

	app.inbox.confirming = false
	app.tab = tabEntities
	app.entities.view = entitiesViewDetail
	app.entities.modeFocus = true
	app.entities.filtering = false
	assert.Equal(
		t,
		fmt.Sprintf("tab:%d:entities:%d:mode=%t:filter=%t", tabEntities, entitiesViewDetail, true, false),
		app.viewStateKey(),
	)
	app.tab = tabLogs
	app.logs.view = logsViewAdd
	app.logs.modeFocus = true
	app.logs.filtering = true
	assert.Equal(
		t,
		fmt.Sprintf("tab:%d:logs:%d:mode=%t:filter=%t", tabLogs, logsViewAdd, true, true),
		app.viewStateKey(),
	)

	app.tab = tabProtocols
	app.protocols.view = protocolsViewAdd
	app.protocols.modeFocus = true
	app.protocols.filtering = true
	assert.Equal(
		t,
		fmt.Sprintf("tab:%d:protocols:%d:mode=%t:filter=%t", tabProtocols, protocolsViewAdd, true, true),
		app.viewStateKey(),
	)

	app.tab = 99
	assert.Equal(t, "tab:99", app.viewStateKey())
}

func TestAppRowHighlightDisabledBranchesForRemainingTabs(t *testing.T) {
	app := NewApp(nil, &config.Config{})

	app.tabNav = false
	app.tab = tabRelations
	app.rels.modeFocus = true
	app.rels.view = relsViewList
	assert.False(t, app.rowHighlightEnabled())

	app.tab = tabKnow
	app.know.modeFocus = true
	app.know.view = contextViewList
	assert.False(t, app.rowHighlightEnabled())

	app.tab = tabJobs
	app.jobs.modeFocus = true
	app.jobs.view = jobsViewList
	app.jobs.detail = nil
	app.jobs.changingSt = false
	assert.False(t, app.rowHighlightEnabled())

	app.tab = tabLogs
	app.logs.modeFocus = true
	app.logs.view = logsViewList
	assert.False(t, app.rowHighlightEnabled())

	app.tab = tabProtocols
	app.protocols.modeFocus = true
	app.protocols.view = protocolsViewList
	assert.False(t, app.rowHighlightEnabled())
}

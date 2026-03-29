package ui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsHandleDetailKeysMatrix(t *testing.T) {
	base := NewJobsModel(nil)
	base.detail = &api.Job{ID: "job-1"}
	base.detailRels = []api.Relationship{{ID: "rel-1"}}

	updated, cmd := base.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.True(t, updated.modeFocus)

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: 's', Text: "s"})
	require.Nil(t, cmd)
	assert.True(t, updated.changingSt)
	assert.Equal(t, []string{"job-1"}, updated.statusTargets)

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: 'n', Text: "n"})
	require.Nil(t, cmd)
	assert.True(t, updated.creatingSubtask)

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: 'a', Text: "a"})
	require.Nil(t, cmd)
	assert.True(t, updated.contextCreating)

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
	require.Nil(t, cmd)
	assert.True(t, updated.contextLinking)

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: 'l', Text: "l"})
	require.Nil(t, cmd)
	assert.True(t, updated.linkingRel)
	assert.Equal(t, "", updated.linkInput.Value())

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: 'u', Text: "u"})
	require.Nil(t, cmd)
	assert.True(t, updated.unlinkingRel)
	assert.Equal(t, "", updated.unlinkInput.Value())

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, jobsViewEdit, updated.view)

	updated, cmd = base.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Nil(t, updated.detail)
	assert.Nil(t, updated.detailRels)
	assert.Equal(t, jobsViewList, updated.view)
}

func TestJobsToggleSelectedAdditionalBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.items = []api.Job{{ID: ""}, {ID: "job-2"}}
	model.dataTable.SetRows([]table.Row{{"empty"}, {"job-2"}})
	model.dataTable.SetCursor(0)

	// Empty ID branch does nothing.
	model.toggleSelected()
	assert.Empty(t, model.selected)

	// Invalid selected index branch (stale index after items shrink).
	model.dataTable.MoveDown(1) // selected -> 1
	model.items = []api.Job{{ID: "job-1"}}
	model.toggleSelected()
	assert.Empty(t, model.selected)

	// Valid ID toggles on and off.
	model.items = []api.Job{{ID: ""}, {ID: "job-2"}}
	model.toggleSelected()
	assert.Equal(t, map[string]bool{"job-2": true}, model.selected)
	model.toggleSelected()
	assert.Empty(t, model.selected)
}

func TestJobsHandleLinkAndUnlinkInputAdditionalBranches(t *testing.T) {
	updatedRelationshipPath := ""
	createdRelationshipBody := map[string]any{}
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/relationships":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&createdRelationshipBody))
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "rel-new"}})
			require.NoError(t, err)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/api/relationships/"):
			updatedRelationshipPath = r.URL.Path
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "rel-2"}})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	model := NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1"}
	model.detailRels = []api.Relationship{{ID: "rel-1"}, {ID: "rel-2"}}

	// Back exits link mode.
	model.linkingRel = true
	model.linkInput.SetValue("entity ent-1 owns")
	updated, cmd := model.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.linkingRel)
	assert.Equal(t, "", updated.linkInput.Value())

	// Default append branch.
	updated.linkingRel = true
	updated, cmd = updated.handleLinkInput(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, "e", updated.linkInput.Value())

	// Successful link create.
	updated.linkInput.SetValue("entity ent-1 owns")
	updated, cmd = updated.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(jobRelationshipChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "job", createdRelationshipBody["source_type"])
	assert.Equal(t, "job-1", createdRelationshipBody["source_id"])
	assert.Equal(t, "entity", createdRelationshipBody["target_type"])
	assert.Equal(t, "ent-1", createdRelationshipBody["target_id"])

	// Back exits unlink mode.
	updated.unlinkingRel = true
	updated.unlinkInput.SetValue("1")
	updated, cmd = updated.handleUnlinkInput(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.unlinkingRel)
	assert.Equal(t, "", updated.unlinkInput.Value())

	// Enter with empty value exits without cmd.
	updated.unlinkingRel = true
	updated.unlinkInput.SetValue("   ")
	updated, cmd = updated.handleUnlinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.Nil(t, cmd)
	assert.False(t, updated.unlinkingRel)
	assert.Equal(t, "", updated.unlinkInput.Value())

	// List-index unlink maps row number to relationship ID.
	updated.unlinkingRel = true
	updated.unlinkInput.SetValue("2")
	updated, cmd = updated.handleUnlinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg = cmd()
	_, ok = msg.(jobRelationshipChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "/api/relationships/rel-2", updatedRelationshipPath)
}

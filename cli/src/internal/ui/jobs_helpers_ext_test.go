package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsRetainSelectionPrunesMissingIDs(t *testing.T) {
	model := NewJobsModel(nil)
	model.allItems = []api.Job{{ID: "job-1"}, {ID: "job-2"}}
	model.selected = map[string]bool{"job-1": true, "ghost": true}

	model.retainSelection()
	assert.Equal(t, map[string]bool{"job-1": true}, model.selected)
}

func TestJobsLoadDetailRelationshipsSuccessAndError(t *testing.T) {
	now := time.Now()
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/relationships/job/job-1":
			err := json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{
					"id":          "rel-1",
					"source_type": "job",
					"source_id":   "job-1",
					"target_type": "entity",
					"target_id":   "ent-1",
					"type":        "assigned-to",
					"status":      "active",
					"created_at":  now,
				}},
			})
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	model := NewJobsModel(client)
	msg := model.loadDetailRelationships("job-1")().(jobRelationshipsLoadedMsg) //nolint:forcetypeassert
	require.Len(t, msg.relationships, 1)
	assert.Equal(t, "rel-1", msg.relationships[0].ID)

	msg = model.loadDetailRelationships("job-2")().(jobRelationshipsLoadedMsg) //nolint:forcetypeassert
	assert.Equal(t, "job-2", msg.id)
	assert.Nil(t, msg.relationships)
}

func TestJobsHandleModeKeysAndToggle(t *testing.T) {
	model := NewJobsModel(nil)
	model.modeFocus = true
	model.view = jobsViewList

	updated, cmd := model.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, jobsViewAdd, updated.view)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.NotNil(t, cmd)
	assert.Equal(t, jobsViewList, updated.view)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)

	updated.modeFocus = true
	updated, cmd = updated.handleModeKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.False(t, updated.modeFocus)
}

func TestJobsHandleAddKeysStatusPriorityBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.view = jobsViewAdd
	model.addFocus = jobFieldStatus

	updated, cmd := model.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.addStatusIdx)

	updated.addFocus = jobFieldPriority
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	assert.Equal(t, 1, updated.addPriorityIdx)

	updated.addFocus = jobFieldTitle
	updated.addFields[jobFieldTitle].value = "abc"
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "ab", updated.addFields[jobFieldTitle].value)

	updated.addFocus = 0
	updated, _ = updated.handleAddKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	assert.True(t, updated.modeFocus)
}

func TestJobsHandleEditKeysStatusPriorityDescriptionAndBack(t *testing.T) {
	desc := "hello"
	model := NewJobsModel(nil)
	model.view = jobsViewEdit
	model.detail = &api.Job{ID: "job-1", Status: "pending", Description: &desc}
	model.startEdit()

	model.editFocus = jobEditFieldStatus
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	require.Nil(t, cmd)
	assert.Equal(t, 1, updated.editStatusIdx)

	updated.editFocus = jobEditFieldPriority
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyRight})
	assert.Equal(t, 1, updated.editPriorityIdx)

	updated.editFocus = jobEditFieldDescription
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'x', Text: "x"})
	assert.Contains(t, updated.editDesc, "x")
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.NotContains(t, updated.editDesc, "x")
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, jobsViewDetail, updated.view)
}

func TestJobsHandleEditKeysAdditionalBranchMatrix(t *testing.T) {
	desc := "hello"
	model := NewJobsModel(nil)
	model.view = jobsViewEdit
	model.detail = &api.Job{ID: "job-1", Status: "pending", Description: &desc}
	model.startEdit()

	model.editSaving = true
	updated, cmd := model.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyDown})
	require.Nil(t, cmd)
	assert.Equal(t, model.editFocus, updated.editFocus)
	assert.True(t, updated.editSaving)

	updated.editSaving = false
	updated.editFocus = 0
	updated, cmd = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyUp})
	require.Nil(t, cmd)
	assert.Equal(t, 0, updated.editFocus)

	updated.editFocus = jobEditFieldStatus
	updated.editStatusIdx = 0
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, len(jobStatusOptions)-1, updated.editStatusIdx)
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: ' ', Text: " "})
	assert.Equal(t, 0, updated.editStatusIdx)

	updated.editFocus = jobEditFieldPriority
	updated.editPriorityIdx = 0
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyLeft})
	assert.Equal(t, len(jobPriorityOptions)-1, updated.editPriorityIdx)
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: ' ', Text: " "})
	assert.Equal(t, 0, updated.editPriorityIdx)

	updated.editFocus = jobEditFieldDescription
	updated.editDesc = "x"
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "", updated.editDesc)
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: 'z', Text: "z"})
	assert.Equal(t, "z", updated.editDesc)
	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, "z", updated.editDesc)

	updated, _ = updated.handleEditKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, jobsViewDetail, updated.view)
}

func TestJobsHandleStatusInputBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.changingSt = true
	model.statusInput.SetValue("act")

	updated, cmd := model.handleStatusInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "ac", updated.statusInput.Value())

	updated.statusInput.SetValue("active")
	updated.statusTargets = nil
	updated.detail = nil
	updated, cmd = updated.handleStatusInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.False(t, updated.changingSt)
	assert.Empty(t, updated.statusInput.Value())
}

func TestJobsHandleStatusInputEnterWithTargetsReturnsCommand(t *testing.T) {
	var calls []string
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			calls = append(calls, r.URL.Path)
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "job-1"}})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model := NewJobsModel(client)
	model.changingSt = true
	model.statusInput.SetValue("active")
	model.statusTargets = []string{"job-1"}
	model.selected = map[string]bool{"job-1": true}

	updated, cmd := model.handleStatusInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(jobStatusUpdatedMsg)
	require.True(t, ok)
	assert.Empty(t, updated.selected)
	assert.Equal(t, []string{"/api/jobs/job-1/status"}, calls)
}

func TestJobsHandleLinkInputInvalidAndBackspaceBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.detail = &api.Job{ID: "job-1"}
	model.linkingRel = true
	model.linkInput.SetValue("entity-only")

	updated, cmd := model.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(errMsg)
	require.True(t, ok)
	assert.False(t, updated.linkingRel)
	assert.Equal(t, "", updated.linkInput.Value())

	updated.linkingRel = true
	updated.linkInput.SetValue("ab")
	updated, cmd = updated.handleLinkInput(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "a", updated.linkInput.Value())
}

func TestJobsHandleUnlinkInputNilDetailAndDirectIDBranches(t *testing.T) {
	model := NewJobsModel(nil)
	model.unlinkingRel = true
	model.unlinkInput.SetValue("1")

	updated, cmd := model.handleUnlinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Nil(t, cmd)
	assert.False(t, updated.unlinkingRel)
	assert.Equal(t, "", updated.unlinkInput.Value())

	var updatedID string
	_, client := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			updatedID = r.URL.Path
			err := json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "rel-custom"}})
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	model = NewJobsModel(client)
	model.detail = &api.Job{ID: "job-1"}
	model.unlinkingRel = true
	model.unlinkInput.SetValue("rel-custom")
	updated, cmd = model.handleUnlinkInput(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(jobRelationshipChangedMsg)
	require.True(t, ok)
	assert.Equal(t, "/api/relationships/rel-custom", updatedID)
	assert.False(t, updated.unlinkingRel)
}

func TestJobsParsePositiveListIndexAndSelectionHelpers(t *testing.T) {
	assert.Equal(t, 0, parsePositiveListIndex(""))
	assert.Equal(t, 0, parsePositiveListIndex("abc"))
	assert.Equal(t, 12, parsePositiveListIndex("12"))

	model := NewJobsModel(nil)
	model.items = []api.Job{{ID: "job-1"}, {ID: "job-2"}}
	model.selected = map[string]bool{"job-2": true}
	assert.Equal(t, []string{"job-2"}, model.selectedIDs())
	assert.Equal(t, 1, model.selectedCount())

	model = NewJobsModel(nil)
	model.selected = map[string]bool{"orphan": true}
	assert.Equal(t, []string{"orphan"}, model.selectedIDs())
}

package ui

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsLoadJobsSuccessAndErrorBranches(t *testing.T) {
	_, okClient := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "job-1", "status": "pending", "title": "load branch", "created_at": time.Now()},
			},
		}))
	})

	model := NewJobsModel(okClient)
	msg := model.loadJobs()
	loaded, ok := msg.(jobsLoadedMsg)
	require.True(t, ok)
	require.Len(t, loaded.items, 1)
	assert.Equal(t, "job-1", loaded.items[0].ID)

	_, errClient := testJobsClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":"JOBS_FAILED","message":"jobs unavailable"}}`, http.StatusInternalServerError)
	})

	errModel := NewJobsModel(errClient)
	errOut, ok := errModel.loadJobs().(errMsg)
	require.True(t, ok)
	assert.ErrorContains(t, errOut.err, "JOBS_FAILED")
}

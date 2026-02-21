package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetJob handles test get job.
func TestGetJob(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.Path, "/api/jobs/")

		_, err := w.Write(jsonResponse(map[string]any{
			"id":     "2026Q1-0001",
			"title":  "Test job",
			"status": "pending",
		}))
		require.NoError(t, err)
	})

	job, err := client.GetJob("2026Q1-0001")
	require.NoError(t, err)
	assert.Equal(t, "2026Q1-0001", job.ID)
	assert.Equal(t, "pending", job.Status)
}

// TestCreateJob handles test create job.
func TestCreateJob(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)

		var body CreateJobInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "New task", body.Title)

		_, err := w.Write(jsonResponse(map[string]any{
			"id":     "2026Q1-0002",
			"title":  body.Title,
			"status": "pending",
		}))
		require.NoError(t, err)
	})

	job, err := client.CreateJob(CreateJobInput{
		Title: "New task",
	})
	require.NoError(t, err)
	assert.Equal(t, "2026Q1-0002", job.ID)
}

// TestUpdateJobStatus handles test update job status.
func TestUpdateJobStatus(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Contains(t, r.URL.Path, "/status")

		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "in-progress", body["status"])

		_, err := w.Write(jsonResponse(map[string]any{
			"id":     "2026Q1-0001",
			"title":  "Test",
			"status": "in-progress",
		}))
		require.NoError(t, err)
	})

	job, err := client.UpdateJobStatus("2026Q1-0001", "in-progress")
	require.NoError(t, err)
	assert.Equal(t, "in-progress", job.Status)
}

// TestCreateSubtask handles test create subtask.
func TestCreateSubtask(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Contains(t, r.URL.Path, "/subtasks")

		_, err := w.Write(jsonResponse(map[string]any{
			"id":     "2026Q1-0001-01",
			"title":  "Subtask",
			"status": "pending",
		}))
		require.NoError(t, err)
	})

	job, err := client.CreateSubtask("2026Q1-0001", map[string]string{
		"title": "Subtask",
	})
	require.NoError(t, err)
	assert.Equal(t, "2026Q1-0001-01", job.ID)
}

// TestQueryJobsWithFilters handles test query jobs with filters.
func TestQueryJobsWithFilters(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "pending", r.URL.Query().Get("status"))
		assert.Equal(t, "high", r.URL.Query().Get("priority"))

		_, err := w.Write(jsonResponse([]map[string]any{
			{"id": "j1", "title": "Task 1", "status": "pending"},
			{"id": "j2", "title": "Task 2", "status": "pending"},
		}))
		require.NoError(t, err)
	})

	jobs, err := client.QueryJobs(QueryParams{
		"status":   "pending",
		"priority": "high",
	})
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
}

// TestUpdateJobStatusInvalid handles test update job status invalid.
func TestUpdateJobStatusInvalid(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		b, _ := json.Marshal(map[string]any{
			"error": map[string]any{
				"code":    "INVALID_STATUS",
				"message": "invalid status transition",
			},
		})
		_, err := w.Write(b)
		require.NoError(t, err)
	})

	_, err := client.UpdateJobStatus("2026Q1-0001", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "INVALID_STATUS")
}

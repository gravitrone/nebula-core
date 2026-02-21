package api

import "fmt"

// --- Job Methods ---

func (c *Client) CreateJob(input CreateJobInput) (*Job, error) {
	data, err := c.post("/api/jobs", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Job](data)
}

// GetJob gets get job.
func (c *Client) GetJob(id string) (*Job, error) {
	data, err := c.get(fmt.Sprintf("/api/jobs/%s", id))
	if err != nil {
		return nil, err
	}
	return decodeOne[Job](data)
}

// QueryJobs handles query jobs.
func (c *Client) QueryJobs(params QueryParams) ([]Job, error) {
	data, err := c.get(buildQuery("/api/jobs", params))
	if err != nil {
		return nil, err
	}
	return decodeList[Job](data)
}

// UpdateJobStatus updates update job status.
func (c *Client) UpdateJobStatus(id, status string) (*Job, error) {
	body := map[string]string{"status": status}
	data, err := c.patch(fmt.Sprintf("/api/jobs/%s/status", id), body)
	if err != nil {
		return nil, err
	}
	return decodeOne[Job](data)
}

// UpdateJob updates update job.
func (c *Client) UpdateJob(id string, input UpdateJobInput) (*Job, error) {
	data, err := c.patch(fmt.Sprintf("/api/jobs/%s", id), input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Job](data)
}

// CreateSubtask creates create subtask.
func (c *Client) CreateSubtask(jobID string, input map[string]string) (*Job, error) {
	data, err := c.post(fmt.Sprintf("/api/jobs/%s/subtasks", jobID), input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Job](data)
}

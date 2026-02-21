package api

import "fmt"

// --- Approval Methods ---

func (c *Client) GetPendingApprovals() ([]Approval, error) {
	return c.GetPendingApprovalsWithParams(200, 0)
}

// GetPendingApprovalsWithParams gets get pending approvals with params.
func (c *Client) GetPendingApprovalsWithParams(limit, offset int) ([]Approval, error) {
	data, err := c.get(buildQuery("/api/approvals/pending", QueryParams{
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	}))
	if err != nil {
		return nil, err
	}
	return decodeList[Approval](data)
}

// GetApproval gets get approval.
func (c *Client) GetApproval(id string) (*Approval, error) {
	data, err := c.get(fmt.Sprintf("/api/approvals/%s", id))
	if err != nil {
		return nil, err
	}
	return decodeOne[Approval](data)
}

// ApproveRequest handles approve request.
func (c *Client) ApproveRequest(id string) (*Approval, error) {
	return c.ApproveRequestWithInput(id, nil)
}

// ApproveRequestWithInput handles approve request with input.
func (c *Client) ApproveRequestWithInput(id string, input *ApproveRequestInput) (*Approval, error) {
	var body any
	if input != nil {
		body = input
	}
	data, err := c.post(fmt.Sprintf("/api/approvals/%s/approve", id), body)
	if err != nil {
		return nil, err
	}
	return decodeOne[Approval](data)
}

// RejectRequest handles reject request.
func (c *Client) RejectRequest(id string, notes string) (*Approval, error) {
	body := map[string]string{"review_notes": notes}
	data, err := c.post(fmt.Sprintf("/api/approvals/%s/reject", id), body)
	if err != nil {
		return nil, err
	}
	return decodeOne[Approval](data)
}

// GetApprovalDiff gets get approval diff.
func (c *Client) GetApprovalDiff(id string) (*ApprovalDiff, error) {
	data, err := c.get(fmt.Sprintf("/api/approvals/%s/diff", id))
	if err != nil {
		return nil, err
	}
	return decodeOne[ApprovalDiff](data)
}

package api

import "fmt"

// --- Context Methods ---

func (c *Client) CreateContext(input CreateContextInput) (*Context, error) {
	data, err := c.post("/api/context", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Context](data)
}

// GetContext gets get context.
func (c *Client) GetContext(id string) (*Context, error) {
	data, err := c.get(fmt.Sprintf("/api/context/%s", id))
	if err != nil {
		return nil, err
	}
	return decodeOne[Context](data)
}

// QueryContext handles query context.
func (c *Client) QueryContext(params QueryParams) ([]Context, error) {
	data, err := c.get(buildQuery("/api/context", params))
	if err != nil {
		return nil, err
	}
	return decodeList[Context](data)
}

// UpdateContext updates update context.
func (c *Client) UpdateContext(id string, input UpdateContextInput) (*Context, error) {
	data, err := c.patch(fmt.Sprintf("/api/context/%s", id), input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Context](data)
}

// LinkContext handles link context.
func (c *Client) LinkContext(id, entityID string) error {
	body := map[string]string{"entity_id": entityID}
	_, err := c.post(fmt.Sprintf("/api/context/%s/link", id), body)
	return err
}

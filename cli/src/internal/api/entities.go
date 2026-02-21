package api

import "fmt"

// --- Entity Methods ---

func (c *Client) GetEntity(id string) (*Entity, error) {
	data, err := c.get(fmt.Sprintf("/api/entities/%s", id))
	if err != nil {
		return nil, err
	}
	return decodeOne[Entity](data)
}

// QueryEntities handles query entities.
func (c *Client) QueryEntities(params QueryParams) ([]Entity, error) {
	data, err := c.get(buildQuery("/api/entities", params))
	if err != nil {
		return nil, err
	}
	return decodeList[Entity](data)
}

// CreateEntity creates create entity.
func (c *Client) CreateEntity(input CreateEntityInput) (*Entity, error) {
	data, err := c.post("/api/entities", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Entity](data)
}

// UpdateEntity updates update entity.
func (c *Client) UpdateEntity(id string, input UpdateEntityInput) (*Entity, error) {
	data, err := c.patch(fmt.Sprintf("/api/entities/%s", id), input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Entity](data)
}

// SearchEntities handles search entities.
func (c *Client) SearchEntities(metadata map[string]any) ([]Entity, error) {
	data, err := c.post("/api/entities/search", map[string]any{"metadata_query": metadata})
	if err != nil {
		return nil, err
	}
	return decodeList[Entity](data)
}

// GetEntityHistory gets get entity history.
func (c *Client) GetEntityHistory(id string, limit int, offset int) ([]AuditEntry, error) {
	params := QueryParams{
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", offset),
	}
	data, err := c.get(buildQuery(fmt.Sprintf("/api/entities/%s/history", id), params))
	if err != nil {
		return nil, err
	}
	return decodeList[AuditEntry](data)
}

// RevertEntity handles revert entity.
func (c *Client) RevertEntity(id string, auditID string) (*Entity, error) {
	body := map[string]string{"audit_id": auditID}
	data, err := c.post(fmt.Sprintf("/api/entities/%s/revert", id), body)
	if err != nil {
		return nil, err
	}
	return decodeOne[Entity](data)
}

// BulkUpdateEntityTags handles bulk update entity tags.
func (c *Client) BulkUpdateEntityTags(input BulkUpdateEntityTagsInput) (*BulkUpdateResult, error) {
	data, err := c.post("/api/entities/bulk/tags", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[BulkUpdateResult](data)
}

// BulkUpdateEntityScopes handles bulk update entity scopes.
func (c *Client) BulkUpdateEntityScopes(input BulkUpdateEntityScopesInput) (*BulkUpdateResult, error) {
	data, err := c.post("/api/entities/bulk/scopes", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[BulkUpdateResult](data)
}

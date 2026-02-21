package api

import "fmt"

// --- Relationship Methods ---

func (c *Client) CreateRelationship(input CreateRelationshipInput) (*Relationship, error) {
	data, err := c.post("/api/relationships", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Relationship](data)
}

// GetRelationships gets get relationships.
func (c *Client) GetRelationships(sourceType, sourceID string) ([]Relationship, error) {
	data, err := c.get(fmt.Sprintf("/api/relationships/%s/%s", sourceType, sourceID))
	if err != nil {
		return nil, err
	}
	return decodeList[Relationship](data)
}

// QueryRelationships handles query relationships.
func (c *Client) QueryRelationships(params QueryParams) ([]Relationship, error) {
	data, err := c.get(buildQuery("/api/relationships", params))
	if err != nil {
		return nil, err
	}
	return decodeList[Relationship](data)
}

// UpdateRelationship updates update relationship.
func (c *Client) UpdateRelationship(id string, input UpdateRelationshipInput) (*Relationship, error) {
	data, err := c.patch(fmt.Sprintf("/api/relationships/%s", id), input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Relationship](data)
}

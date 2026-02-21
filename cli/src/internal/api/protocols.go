package api

import "fmt"

// --- Protocol Methods ---

func (c *Client) CreateProtocol(input CreateProtocolInput) (*Protocol, error) {
	data, err := c.post("/api/protocols", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Protocol](data)
}

// GetProtocol gets get protocol.
func (c *Client) GetProtocol(name string) (*Protocol, error) {
	data, err := c.get(fmt.Sprintf("/api/protocols/%s", name))
	if err != nil {
		return nil, err
	}
	return decodeOne[Protocol](data)
}

// QueryProtocols handles query protocols.
func (c *Client) QueryProtocols(params QueryParams) ([]Protocol, error) {
	data, err := c.get(buildQuery("/api/protocols", params))
	if err != nil {
		return nil, err
	}
	return decodeList[Protocol](data)
}

// UpdateProtocol updates update protocol.
func (c *Client) UpdateProtocol(name string, input UpdateProtocolInput) (*Protocol, error) {
	data, err := c.patch(fmt.Sprintf("/api/protocols/%s", name), input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Protocol](data)
}

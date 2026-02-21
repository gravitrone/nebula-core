package api

import "fmt"

// --- Agent Methods ---

func (c *Client) RegisterAgent(input RegisterAgentInput) (*AgentRegistration, error) {
	data, err := c.post("/api/agents/register", input)
	if err != nil {
		return nil, err
	}
	return decodeOne[AgentRegistration](data)
}

// ListAgents lists list agents.
func (c *Client) ListAgents(statusCategory string) ([]Agent, error) {
	params := QueryParams{}
	if statusCategory != "" {
		params["status_category"] = statusCategory
	}
	data, err := c.get(buildQuery("/api/agents/", params))
	if err != nil {
		return nil, err
	}
	return decodeList[Agent](data)
}

// GetAgent gets get agent.
func (c *Client) GetAgent(name string) (*Agent, error) {
	data, err := c.get(fmt.Sprintf("/api/agents/%s", name))
	if err != nil {
		return nil, err
	}
	return decodeOne[Agent](data)
}

// UpdateAgent updates update agent.
func (c *Client) UpdateAgent(id string, input UpdateAgentInput) (*Agent, error) {
	data, err := c.patch(fmt.Sprintf("/api/agents/%s", id), input)
	if err != nil {
		return nil, err
	}
	return decodeOne[Agent](data)
}

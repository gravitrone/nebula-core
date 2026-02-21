package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client wraps HTTP calls to the Nebula REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new API client.
func NewClient(baseURL, apiKey string, timeout ...time.Duration) *Client {
	httpTimeout := 30 * time.Second
	if len(timeout) > 0 && timeout[0] > 0 {
		httpTimeout = timeout[0]
	}
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

// SetAPIKey updates the bearer token used for subsequent requests.
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// WithTimeout clones the client with a different HTTP timeout.
func (c *Client) WithTimeout(timeout time.Duration) *Client {
	return NewClient(c.baseURL, c.apiKey, timeout)
}

// do executes an HTTP request and returns the raw response body.
func (c *Client) do(method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		if msg, ok := extractAPIErrorBody(respBody); ok {
			return nil, resp.StatusCode, fmt.Errorf("%s", msg)
		}
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, resp.StatusCode, nil
}

// get performs a GET request.
func (c *Client) get(path string) ([]byte, error) {
	body, _, err := c.do(http.MethodGet, path, nil)
	return body, err
}

// post performs a POST request.
func (c *Client) post(path string, body any) ([]byte, error) {
	b, _, err := c.do(http.MethodPost, path, body)
	return b, err
}

// patch performs a PATCH request.
func (c *Client) patch(path string, body any) ([]byte, error) {
	b, _, err := c.do(http.MethodPatch, path, body)
	return b, err
}

// del performs a DELETE request.
func (c *Client) del(path string) ([]byte, error) {
	b, _, err := c.do(http.MethodDelete, path, nil)
	return b, err
}

// decodeOne decodes a single-item API response.
func decodeOne[T any](data []byte) (*T, error) {
	var resp apiResponse[T]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp.Data, nil
}

// decodeList decodes a list API response.
func decodeList[T any](data []byte) ([]T, error) {
	var resp apiResponse[[]T]
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return resp.Data, nil
}

// buildQuery appends query params to a path.
func buildQuery(path string, params QueryParams) string {
	if len(params) == 0 {
		return path
	}
	q := url.Values{}
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	return path + "?" + q.Encode()
}

// extractAPIErrorBody handles extract apierror body.
func extractAPIErrorBody(body []byte) (string, bool) {
	if len(body) == 0 {
		return "", false
	}

	var envelope apiResponse[any]
	if err := json.Unmarshal(body, &envelope); err == nil && envelope.Error != nil {
		return formatAPIError(envelope.Error.Code, envelope.Error.Message)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", false
	}

	if msg, ok := parseErrorValue(payload["error"]); ok {
		return msg, true
	}
	if msg, ok := parseErrorValue(payload["detail"]); ok {
		return msg, true
	}
	return "", false
}

// parseErrorValue parses parse error value.
func parseErrorValue(raw any) (string, bool) {
	switch value := raw.(type) {
	case string:
		msg := strings.TrimSpace(value)
		if msg == "" {
			return "", false
		}
		return msg, true
	case map[string]any:
		if nested, ok := parseErrorValue(value["error"]); ok {
			return nested, true
		}
		code, _ := value["code"].(string)
		message, _ := value["message"].(string)
		return formatAPIError(code, message)
	}
	return "", false
}

// formatAPIError handles format apierror.
func formatAPIError(code, message string) (string, bool) {
	code = strings.TrimSpace(code)
	message = strings.TrimSpace(message)
	switch {
	case code != "" && message != "":
		return fmt.Sprintf("%s: %s", code, message), true
	case code != "":
		return code, true
	case message != "":
		return message, true
	default:
		return "", false
	}
}

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
			return nil, resp.StatusCode, fmt.Errorf(
				"%s",
				normalizeAPIError(resp.StatusCode, msg),
			)
		}
		return nil, resp.StatusCode, fmt.Errorf(
			"%s",
			normalizeAPIError(
				resp.StatusCode,
				fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody)),
			),
		)
	}

	return respBody, resp.StatusCode, nil
}

// normalizeAPIError keeps auth/multi-api recovery branches consistent across all callers.
func normalizeAPIError(statusCode int, msg string) string {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if isMultiAPIConflictText(lower) {
		return "MULTIPLE_API_INSTANCES_DETECTED: multiple api instances detected"
	}
	if shouldNormalizeInvalidAPIKey(statusCode, lower) {
		detail := normalizedAuthDetail(trimmed)
		if detail == "" {
			detail = "invalid api key"
		}
		return fmt.Sprintf("INVALID_API_KEY: %s", detail)
	}
	return trimmed
}

// shouldNormalizeInvalidAPIKey handles should normalize invalid apikey.
func shouldNormalizeInvalidAPIKey(statusCode int, lowerMsg string) bool {
	if statusCode == http.StatusUnauthorized {
		return true
	}
	if strings.Contains(lowerMsg, "invalid api key") ||
		strings.Contains(lowerMsg, "invalid_api_key") ||
		strings.Contains(lowerMsg, "auth_required") ||
		strings.Contains(lowerMsg, "missing or invalid authorization") ||
		strings.Contains(lowerMsg, "not logged in") ||
		strings.Contains(lowerMsg, "unauthorized") {
		return true
	}
	if statusCode == http.StatusForbidden {
		return strings.Contains(lowerMsg, "missing or invalid authorization") ||
			strings.Contains(lowerMsg, "invalid authorization") ||
			strings.Contains(lowerMsg, "invalid api key") ||
			strings.Contains(lowerMsg, "invalid_api_key") ||
			strings.Contains(lowerMsg, "api key")
	}
	return false
}

// normalizedAuthDetail handles normalized auth detail.
func normalizedAuthDetail(text string) string {
	detail := strings.TrimSpace(text)
	if code, parsed := parseErrorCode(detail); code != "" {
		detail = parsed
	}
	lower := strings.ToLower(detail)
	if strings.HasPrefix(lower, "invalid_api_key:") ||
		strings.HasPrefix(lower, "invalid api key:") ||
		strings.HasPrefix(lower, "auth_required:") ||
		strings.HasPrefix(lower, "auth required:") {
		if idx := strings.Index(detail, ":"); idx >= 0 {
			detail = strings.TrimSpace(detail[idx+1:])
			lower = strings.ToLower(detail)
		}
	}
	if strings.HasPrefix(lower, "http ") {
		if idx := strings.Index(detail, ":"); idx >= 0 {
			detail = strings.TrimSpace(detail[idx+1:])
		}
	}
	detail = strings.TrimSpace(detail)
	switch strings.ToLower(detail) {
	case "", "forbidden", "unauthorized", "invalid_api_key", "invalid api key", "auth_required", "auth required":
		return "invalid api key"
	default:
		return detail
	}
}

// parseErrorCode handles parse error code.
func parseErrorCode(text string) (string, string) {
	parts := strings.SplitN(text, ":", 2)
	if len(parts) != 2 {
		return "", text
	}
	code := strings.TrimSpace(parts[0])
	if code == "" || strings.HasPrefix(strings.ToUpper(code), "HTTP ") {
		return "", text
	}
	for _, r := range code {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return "", text
		}
	}
	return code, strings.TrimSpace(parts[1])
}

// isMultiAPIConflictText handles is multi apiconflict text.
func isMultiAPIConflictText(lowerMsg string) bool {
	return strings.Contains(lowerMsg, "multiple api instances detected") ||
		strings.Contains(lowerMsg, "multiple_api_instances_detected") ||
		strings.Contains(lowerMsg, "address already in use") ||
		strings.Contains(lowerMsg, "eaddrinuse") ||
		strings.Contains(lowerMsg, "errno 98") ||
		strings.Contains(lowerMsg, "errno 48")
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
		if msg, ok := formatAPIError(envelope.Error.Code, envelope.Error.Message); ok {
			return msg, true
		}
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

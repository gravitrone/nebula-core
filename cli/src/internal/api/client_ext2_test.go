package api

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errReadCloser struct{}

// Read handles read.
func (errReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("boom read")
}

// Close handles close.
func (errReadCloser) Close() error {
	return nil
}

func TestDoReturnsMarshalBodyErrorForUnsupportedPayload(t *testing.T) {
	client := NewClient("http://example.com", "nbl_testkey")

	_, status, err := client.do(http.MethodPost, "/api/entities", map[string]any{
		"bad": func() {},
	})

	require.Error(t, err)
	assert.Equal(t, 0, status)
	assert.ErrorContains(t, err, "marshal body")
}

func TestDoReturnsCreateRequestErrorForInvalidBaseURL(t *testing.T) {
	client := NewClient("://bad-url", "nbl_testkey")

	_, status, err := client.do(http.MethodGet, "/api/health", nil)

	require.Error(t, err)
	assert.Equal(t, 0, status)
	assert.ErrorContains(t, err, "create request")
}

func TestDoReturnsReadResponseErrorWhenBodyReadFails(t *testing.T) {
	client := NewClient("http://example.com", "nbl_testkey")
	client.httpClient = &http.Client{
		Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       errReadCloser{},
				Header:     make(http.Header),
			}, nil
		}),
	}

	_, status, err := client.do(http.MethodGet, "/api/health", nil)

	require.Error(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.ErrorContains(t, err, "read response")
}

func TestExtractAPIErrorBodyParsesTopLevelErrorString(t *testing.T) {
	msg, ok := extractAPIErrorBody([]byte(`{"error":"service exploded"}`))
	require.True(t, ok)
	assert.Equal(t, "service exploded", msg)
}

func TestNormalizeAPIErrorBlankMessageKeepsOriginalBlank(t *testing.T) {
	assert.Equal(t, "", normalizeAPIError(http.StatusUnauthorized, ""))
	assert.Equal(t, "  ", normalizeAPIError(http.StatusUnauthorized, "  "))
}

func TestParseErrorValueRejectsUnsupportedTypes(t *testing.T) {
	_, ok := parseErrorValue(io.NopCloser(nil))
	assert.False(t, ok)
}

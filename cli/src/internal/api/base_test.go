package api

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDefaultClientUsesDefaultBaseURL handles test new default client uses default base url.
func TestNewDefaultClientUsesDefaultBaseURL(t *testing.T) {
	var gotURL string
	client := NewDefaultClient("nbl_testkey")
	client.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		gotURL = r.URL.String()
		body := `{"data":{"id":"ent-1","name":"Alpha","tags":[]}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})

	_, err := client.GetEntity("ent-1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(gotURL, DefaultBaseURL))
}

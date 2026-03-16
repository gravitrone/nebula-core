package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClientConcurrentMutations handles test client concurrent mutations.
func TestClientConcurrentMutations(t *testing.T) {
	var count atomic.Int32
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/entities" {
			var body CreateEntityInput
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, "stress-entity", body.Name)
			count.Add(1)
			_, err := w.Write(jsonResponse(map[string]any{"id": "ent-1", "name": body.Name}))
			require.NoError(t, err)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	const workers = 50
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.CreateEntity(CreateEntityInput{
				Name:   "stress-entity",
				Type:   "person",
				Status: "active",
			})
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		assert.NoError(t, err)
	}
	assert.Equal(t, int32(workers), count.Load())
}

// TestClientHandlesMalformedJSON handles test client handles malformed json.
func TestClientHandlesMalformedJSON(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("not-json"))
		require.NoError(t, err)
	})

	_, err := client.GetEntity("ent-1")
	require.Error(t, err)
}

// TestClientUnicodePayload handles test client unicode payload.
func TestClientUnicodePayload(t *testing.T) {
	_, client := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body CreateContextInput
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "🚀", body.Content)
		_, err := w.Write(jsonResponse(map[string]any{"id": "k-1", "name": body.Title}))
		require.NoError(t, err)
	})

	_, err := client.CreateContext(CreateContextInput{
		Title:      "unicode",
		SourceType: "note",
		Content:    "🚀",
	})
	require.NoError(t, err)
}

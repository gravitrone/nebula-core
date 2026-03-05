package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPICmdApprovalsDiffFocusFiltersAndMaxLines(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	now := time.Now().UTC()
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/approvals/") && strings.HasSuffix(r.URL.Path, "/diff"):
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"approval_id":  "a1",
					"request_type": "update_entity",
					"changes": map[string]any{
						"status": map[string]any{
							"from": "active",
							"to":   "active",
						},
						"metadata.summary": map[string]any{
							"from": "line-1\nline-2\nline-3\nline-4",
							"to":   "line-1\nline-2\nline-3\nline-4\nline-5",
						},
						"tags": map[string]any{
							"from": []string{"a"},
							"to":   []string{"a", "b"},
						},
					},
					"created_at": now,
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(shutdown)

	t.Setenv(outputModeEnv, string(OutputModePlain))
	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"approvals", "diff", "a1",
		"--only", "changed",
		"--only", "section=metadata",
		"--max-lines", "2",
	})
	require.NoError(t, cmd.Execute())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
	changes, ok := payload["changes"].(map[string]any)
	require.True(t, ok)
	require.Len(t, changes, 1)
	metadata, ok := changes["metadata.summary"].(map[string]any)
	require.True(t, ok)
	fromValue := metadata["from"].(string)
	toValue := metadata["to"].(string)
	assert.Contains(t, fromValue, "... (+2 more lines)")
	assert.Contains(t, toValue, "... (+3 more lines)")
	assert.Equal(t, float64(1), payload["filtered_count"])
}

func TestAPICmdApprovalsDiffTableContract(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/approvals/") && strings.HasSuffix(r.URL.Path, "/diff") {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"approval_id":  "a1",
					"request_type": "update_entity",
					"changes": map[string]any{
						"tags": map[string]any{"from": []string{"a"}, "to": []string{"a", "b"}},
					},
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	t.Setenv(outputModeEnv, string(OutputModeTable))
	var out bytes.Buffer
	cmd := APICmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"approvals", "diff", "a1", "--max-lines", "2"})
	require.NoError(t, cmd.Execute())

	text := out.String()
	assert.Contains(t, text, "approval_id")
	assert.Contains(t, text, "tags")
	assert.Contains(t, text, "max_lines")
}

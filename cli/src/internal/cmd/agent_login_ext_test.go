package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("forced write failure")
}

func TestAgentListRendersNoAgentsMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/" && r.Method == http.MethodGet {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := AgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "No agents found.")
}

func TestAgentListRendersTrustedAgentWithoutDescription(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, (&config.Config{APIKey: "nbl_test", Username: "alxx"}).Save())

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/agents/" && r.Method == http.MethodGet {
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":                "agent-1",
						"name":              "trusted-agent",
						"status":            "active",
						"requires_approval": false,
						"scopes":            []string{"public"},
						"capabilities":      []string{"read"},
					},
				},
			}))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	cmd := AgentCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"list"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, out.String(), "trusted-agent")
	assert.Contains(t, out.String(), "trusted")
	assert.NotContains(t, out.String(), "trusted -")
}

func TestRunInteractiveLoginPromptWriteError(t *testing.T) {
	err := RunInteractiveLogin(strings.NewReader("alxx\n"), failWriter{})
	require.Error(t, err)
	assert.ErrorContains(t, err, "write prompt")
}

func TestRunInteractiveLoginLoginFailure(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"error": map[string]any{
					"code":    "UNAUTHORIZED",
					"message": "invalid username",
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	var out bytes.Buffer
	err := RunInteractiveLogin(strings.NewReader("alxx\n"), &out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "login failed")
}

func TestRunInteractiveLoginSaveConfigFailure(t *testing.T) {
	base := t.TempDir()
	fakeHomeFile := filepath.Join(base, "home-as-file")
	require.NoError(t, os.WriteFile(fakeHomeFile, []byte("x"), 0o600))
	t.Setenv("HOME", fakeHomeFile)

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/keys/login" && r.Method == http.MethodPost {
			_, _ = io.WriteString(w, `{"data":{"api_key":"nbl_test","entity_id":"ent-1","username":"alxx"}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	var out bytes.Buffer
	err := RunInteractiveLogin(strings.NewReader("alxx\n"), &out)
	require.Error(t, err)
	assert.ErrorContains(t, err, "save config")
}

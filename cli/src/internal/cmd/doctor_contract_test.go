package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoctorCommandOutputContracts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	serverDir := filepath.Join(t.TempDir(), "server")
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, "src", "nebula_api"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, ".venv", "bin"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(serverDir, "src", "nebula_api", "app.py"), []byte("# app"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(serverDir, ".venv", "bin", "uvicorn"), []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("NEBULA_SERVER_DIR", serverDir)

	shutdown := startDefaultAPIBaseServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(shutdown)

	for _, mode := range []OutputMode{OutputModeJSON, OutputModePlain, OutputModeTable} {
		t.Setenv(outputModeEnv, string(mode))
		var out bytes.Buffer
		cmd := DoctorCmd()
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{})
		require.NoError(t, cmd.Execute(), "mode=%s", mode)

		text := out.String()
		switch mode {
		case OutputModeTable:
			assert.Contains(t, text, "summary")
			assert.Contains(t, text, "config")
		default:
			var payload map[string]any
			require.NoError(t, json.Unmarshal(out.Bytes(), &payload), "mode=%s", mode)
			assert.Contains(t, payload, "checks")
			assert.Contains(t, payload, "ok")
		}
	}
}

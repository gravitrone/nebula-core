package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWaitForAPIHealthProbeUsesDefaultFactory(t *testing.T) {
	prevFactory := newDefaultClient
	t.Cleanup(func() { newDefaultClient = prevFactory })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	newDefaultClient = func(apiKey string, timeout ...time.Duration) *api.Client {
		if len(timeout) > 0 {
			return api.NewClient(srv.URL, apiKey, timeout[0])
		}
		return api.NewClient(srv.URL, apiKey)
	}

	status, err := waitForAPIHealthProbe()
	require.NoError(t, err)
	assert.Equal(t, "ok", status)
}

func TestRunStartCmdReturnsAcquireLockErrorWhenRuntimeDirNotWritable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.Chmod(runtimeDir(), 0o500))

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create api lock")
}

func TestNormalizeServerDirCandidateRejectsInvalidAbsolutePath(t *testing.T) {
	_, ok := normalizeServerDirCandidate("\x00nebula")
	assert.False(t, ok)
}

func TestResolveServerDirStopsParentWalkAtFilesystemRoot(t *testing.T) {
	t.Setenv("NEBULA_SERVER_DIR", "")
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(string(filepath.Separator)))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	_, err = resolveServerDir()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not locate server dir")
}

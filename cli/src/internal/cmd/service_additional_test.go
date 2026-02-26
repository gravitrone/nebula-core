package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitrone/nebula-core/cli/internal/api"
)

// TestWaitForAPIHealthReturnsTrueWhenHealthy handles waitForAPIHealth success on a healthy local endpoint.
func TestWaitForAPIHealthReturnsTrueWhenHealthy(t *testing.T) {
	previousProbe := waitForAPIHealthProbe
	t.Cleanup(func() {
		waitForAPIHealthProbe = previousProbe
	})
	attempts := 0
	waitForAPIHealthProbe = func() (string, error) {
		attempts++
		if attempts < 2 {
			return "", assert.AnError
		}
		return "ok", nil
	}

	assert.True(t, waitForAPIHealth(2*time.Second))
	assert.GreaterOrEqual(t, attempts, 2)
}

// TestWaitForAPIHealthReturnsFalseOnTimeout handles waitForAPIHealth timeout behavior when no API is reachable.
func TestWaitForAPIHealthReturnsFalseOnTimeout(t *testing.T) {
	previousProbe := waitForAPIHealthProbe
	t.Cleanup(func() {
		waitForAPIHealthProbe = previousProbe
	})
	attempts := 0
	waitForAPIHealthProbe = func() (string, error) {
		attempts++
		return "", assert.AnError
	}

	start := time.Now()
	assert.False(t, waitForAPIHealth(300*time.Millisecond))
	assert.GreaterOrEqual(t, time.Since(start), 250*time.Millisecond)
	assert.Greater(t, attempts, 0)
}

// TestResolveServerDirRejectsInvalidEnv handles invalid explicit server-dir overrides.
func TestResolveServerDirRejectsInvalidEnv(t *testing.T) {
	t.Setenv("NEBULA_SERVER_DIR", t.TempDir())

	_, err := resolveServerDir()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NEBULA_SERVER_DIR does not point")
}

// TestResolveServerDirFindsServerUnderWorkingDir handles cwd-based server discovery.
func TestResolveServerDirFindsServerUnderWorkingDir(t *testing.T) {
	root := t.TempDir()
	serverDir := filepath.Join(root, "server")
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, "src", "nebula_api"), 0o755))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(serverDir, "src", "nebula_api", "app.py"), []byte("app = None\n"), 0o644),
	)

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	t.Setenv("NEBULA_SERVER_DIR", "")
	got, err := resolveServerDir()
	require.NoError(t, err)
	expected, err := filepath.Abs(serverDir)
	require.NoError(t, err)
	assert.Equal(t, normalizePathPrefix(expected), normalizePathPrefix(got))
}

// TestRunStartCmdRejectsInvalidServerEnv handles invalid server path failures before process launch.
func TestRunStartCmdRejectsInvalidServerEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("NEBULA_SERVER_DIR", t.TempDir())

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NEBULA_SERVER_DIR does not point")
}

// TestRunStartCmdReturnsHelpfulErrorWhenUvicornMissing handles missing uvicorn setup errors.
func TestRunStartCmdReturnsHelpfulErrorWhenUvicornMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, "src", "nebula_api"), 0o755))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(serverDir, "src", "nebula_api", "app.py"), []byte("app = None\n"), 0o644),
	)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uvicorn not found")
	_, statErr := os.Stat(apiLockPath())
	assert.True(t, os.IsNotExist(statErr))
}

// TestRunStartCmdStartsManagedProcess handles the successful API start path with lock/state persistence.
func TestRunStartCmdStartsManagedProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicorn(t)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	setWaitForAPIProbe(t, func() (string, error) { return "ok", nil })
	setStartHealthTimeout(t, 300*time.Millisecond)

	var out bytes.Buffer
	require.NoError(t, runStartCmd(&out))
	text := strings.ToLower(out.String())
	assert.Contains(t, text, "running")
	assert.Contains(t, text, "nebula api")

	state, err := loadAPIState()
	require.NoError(t, err)
	require.Positive(t, state.PID)
	assert.Equal(t, api.DefaultAPIPort, state.Port)
	assert.Equal(t, serverDir, state.ServerDir)
	assert.True(t, processAlive(state.PID))

	lock, err := loadAPILock()
	require.NoError(t, err)
	assert.Equal(t, state.PID, lock.APIPID)

	var stopOut bytes.Buffer
	require.NoError(t, runStopCmd(&stopOut))
	assert.Contains(t, strings.ToLower(stopOut.String()), "stopped")
}

// TestRunStartCmdReportsStartingWhenHealthNotReady handles startup status when probe checks fail.
func TestRunStartCmdReportsStartingWhenHealthNotReady(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicorn(t)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	setWaitForAPIProbe(t, func() (string, error) { return "", assert.AnError })
	setStartHealthTimeout(t, 300*time.Millisecond)

	var out bytes.Buffer
	require.NoError(t, runStartCmd(&out))
	text := strings.ToLower(out.String())
	assert.Contains(t, text, "starting")
	assert.NotContains(t, text, "status | running")

	state, err := loadAPIState()
	require.NoError(t, err)
	require.Positive(t, state.PID)
	require.True(t, processAlive(state.PID))

	var stopOut bytes.Buffer
	require.NoError(t, runStopCmd(&stopOut))
}

// TestRunStopCmdReturnsErrorOnCorruptLock handles invalid lock-file parse failures.
func TestRunStopCmdReturnsErrorOnCorruptLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.WriteFile(apiLockPath(), []byte("{broken"), 0o600))

	var out bytes.Buffer
	err := runStopCmd(&out)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "read api lock"))
}

// createFakeServerDirWithUvicorn handles constructing a temporary server dir with a runnable uvicorn shim.
func createFakeServerDirWithUvicorn(t *testing.T) string {
	t.Helper()
	serverDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, "src", "nebula_api"), 0o755))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(serverDir, "src", "nebula_api", "app.py"), []byte("app = None\n"), 0o644),
	)
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, ".venv", "bin"), 0o755))
	uvicornScript := "#!/bin/sh\nsleep 30\n"
	require.NoError(
		t,
		os.WriteFile(filepath.Join(serverDir, ".venv", "bin", "uvicorn"), []byte(uvicornScript), 0o755),
	)
	return serverDir
}

// setWaitForAPIProbe handles replacing the health probe for deterministic wait-loop tests.
func setWaitForAPIProbe(t *testing.T, probe func() (string, error)) {
	t.Helper()
	previousProbe := waitForAPIHealthProbe
	waitForAPIHealthProbe = probe
	t.Cleanup(func() {
		waitForAPIHealthProbe = previousProbe
	})
}

// setStartHealthTimeout handles overriding startup health polling timeout for deterministic tests.
func setStartHealthTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()
	previousTimeout := startHealthTimeout
	startHealthTimeout = timeout
	t.Cleanup(func() {
		startHealthTimeout = previousTimeout
	})
}

// normalizePathPrefix handles macOS /private path aliases for robust path equality assertions.
func normalizePathPrefix(path string) string {
	path = filepath.Clean(path)
	return strings.TrimPrefix(path, "/private")
}

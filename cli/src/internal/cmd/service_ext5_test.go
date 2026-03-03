package cmd

import (
	"bytes"
	"os/exec"
	"runtime"
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

func TestRunStartCmdReturnsExitedErrorWhenProcessDiesBeforeHealth(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicornScript(
		t,
		"#!/bin/sh\nexit 1\n",
	)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	setWaitForAPIProbe(t, func() (string, error) { return "", assert.AnError })
	setStartHealthTimeout(t, 100*time.Millisecond)

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api failed to start")
	_, stateErr := loadAPIState()
	assert.True(t, os.IsNotExist(stateErr))
	_, lockErr := loadAPILock()
	assert.True(t, os.IsNotExist(lockErr))
}

func TestProcessZombieDetectsExitedChildAndProcessAliveTreatsItAsDead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix zombie-state semantics required")
	}

	cmd := exec.Command("sh", "-c", "exit 0")
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Wait()
	})

	require.Eventually(t, func() bool {
		return processZombie(pid)
	}, 2*time.Second, 20*time.Millisecond)
	assert.False(t, processAlive(pid))
}

func TestProcessZombieReturnsFalseWhenPIDNotFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix ps behavior required")
	}
	assert.False(t, processZombie(2147483647))
}

func TestProcessZombieReturnsFalseForNonPositivePID(t *testing.T) {
	assert.False(t, processZombie(0))
	assert.False(t, processZombie(-42))
}

func TestProcessZombieReturnsFalseForLiveProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix ps behavior required")
	}

	cmd := exec.Command("sh", "-c", "sleep 30")
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	require.Eventually(t, func() bool {
		return processAlive(pid)
	}, time.Second, 20*time.Millisecond)
	assert.False(t, processZombie(pid))
}

func TestStopProcessIfAliveEscalatesToKillWhenProcessIgnoresTerm(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal semantics required")
	}

	cmd := exec.Command("sh", "-c", "trap '' TERM; sleep 30")
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	require.Eventually(t, func() bool {
		return processAlive(pid)
	}, time.Second, 20*time.Millisecond)

	stopProcessIfAlive(pid)

	require.Eventually(t, func() bool {
		return !processAlive(pid)
	}, 3*time.Second, 20*time.Millisecond)
}

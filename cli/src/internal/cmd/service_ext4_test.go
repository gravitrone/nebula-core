package cmd

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStartCmdReturnsRuntimeDirErrorWhenHomeIsFile(t *testing.T) {
	homeFile := filepath.Join(t.TempDir(), "home-file")
	require.NoError(t, os.WriteFile(homeFile, []byte("x"), 0o600))
	t.Setenv("HOME", homeFile)

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create runtime dir")
}

func TestRunStartCmdReturnsOpenLogErrorWhenLogPathIsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicorn(t)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)

	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.Mkdir(apiLogPath(), 0o700))

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open api log")
}

func TestRunStartCmdReturnsStartAPIErrorWhenUvicornNotExecutable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicorn(t)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	uvicornPath := filepath.Join(serverDir, ".venv", "bin", "uvicorn")
	require.NoError(t, os.Chmod(uvicornPath, 0o644))

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start api")
}

func TestRunStartCmdReturnsSaveStateErrorWhenRuntimeStatePathIsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicornScript(
		t,
		"#!/bin/sh\nexit 0\n",
	)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)

	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.Mkdir(apiStatePath(), 0o700))

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write runtime state")
}

func TestRunStartCmdKillsProcessWhenSaveStateFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicornScript(
		t,
		"#!/bin/sh\nsleep 30\n",
	)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	uvicornPath := filepath.Join(serverDir, ".venv", "bin", "uvicorn")

	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.Mkdir(apiStatePath(), 0o700))

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write runtime state")

	require.Eventually(t, func() bool {
		out, psErr := exec.Command("ps", "-axo", "command").Output()
		if psErr != nil {
			return false
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, uvicornPath) && strings.Contains(line, "nebula_api.app:app") {
				return false
			}
		}
		return true
	}, 3*time.Second, 50*time.Millisecond)

	_, lockErr := os.Stat(apiLockPath())
	assert.True(t, os.IsNotExist(lockErr))
}

func TestRunStartCmdKillsProcessWhenPortConflictDetected(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicornScript(
		t,
		"#!/bin/sh\necho 'ERROR: [Errno 98] Address already in use' >&2\nsleep 30\n",
	)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	setWaitForAPIProbe(t, func() (string, error) { return "", assert.AnError })
	setStartHealthTimeout(t, 100*time.Millisecond)
	uvicornPath := filepath.Join(serverDir, ".venv", "bin", "uvicorn")

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "multiple api instances detected")

	require.Eventually(t, func() bool {
		out, psErr := exec.Command("ps", "-axo", "command").Output()
		if psErr != nil {
			return false
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, uvicornPath) && strings.Contains(line, "nebula_api.app:app") {
				return false
			}
		}
		return true
	}, 3*time.Second, 50*time.Millisecond)

	_, stateErr := os.Stat(apiStatePath())
	assert.True(t, os.IsNotExist(stateErr))
	_, lockErr := os.Stat(apiLockPath())
	assert.True(t, os.IsNotExist(lockErr))
}

func TestRunStopCmdUsesStatePIDWhenLockAPIPIDMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	lock := apiLockState{
		OwnerPID: os.Getpid(),
		APIPID:   0,
	}
	lockRaw, err := json.Marshal(lock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), lockRaw, 0o600))

	state := &apiRuntimeState{
		PID:       999999,
		Port:      api.DefaultAPIPort,
		ServerDir: "/tmp/server",
		LogPath:   "/tmp/log",
	}
	require.NoError(t, saveAPIState(state))

	var out bytes.Buffer
	require.NoError(t, runStopCmd(&out))
	assert.Contains(t, strings.ToLower(out.String()), "cleaned stale runtime files")
}

func TestRunStopCmdPrefersLiveStatePIDWhenLockPIDConflicts(t *testing.T) {
	if !processAlive(1) {
		t.Skip("pid 1 not available on this host")
	}

	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicorn(t)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	setWaitForAPIProbe(t, func() (string, error) { return "ok", nil })
	setStartHealthTimeout(t, 300*time.Millisecond)

	var startOut bytes.Buffer
	require.NoError(t, runStartCmd(&startOut))
	state, err := loadAPIState()
	require.NoError(t, err)
	require.Positive(t, state.PID)
	require.True(t, processAlive(state.PID))

	lock := apiLockState{
		OwnerPID: os.Getpid(),
		APIPID:   1, // live conflicting pid that is not our managed API pid
	}
	lockRaw, err := json.Marshal(lock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), lockRaw, 0o600))

	var out bytes.Buffer
	require.NoError(t, runStopCmd(&out))
	assert.Contains(t, strings.ToLower(out.String()), "stopped")
	_, stateErr := os.Stat(apiStatePath())
	assert.True(t, os.IsNotExist(stateErr))
	_, lockErr := os.Stat(apiLockPath())
	assert.True(t, os.IsNotExist(lockErr))
}

func TestRunStopCmdReturnsStopAPIErrorForProtectedPID(t *testing.T) {
	if !processAlive(1) {
		t.Skip("pid 1 not available on this host")
	}

	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	lock := apiLockState{
		OwnerPID: os.Getpid(),
		APIPID:   1,
	}
	lockRaw, err := json.Marshal(lock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), lockRaw, 0o600))

	var out bytes.Buffer
	err = runStopCmd(&out)
	if err == nil {
		t.Skip("signal to pid 1 unexpectedly succeeded on this host")
	}
	assert.Contains(t, err.Error(), "stop api")
}

func TestRunLogsCmdUsesMinimumContentWidthWhenColumnsTiny(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("COLUMNS", "1")
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(
		t,
		os.WriteFile(apiLogPath(), []byte("this is a very long log line for width clamp\n"), 0o600),
	)

	var out bytes.Buffer
	require.NoError(t, runLogsCmd(&out, true, 50))
	assert.Contains(t, out.String(), "Nebula API Logs")
}

func TestDetectStartupFailureUsesDefaultTimeoutWhenNonPositive(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "api.log")
	require.NoError(t, os.WriteFile(logPath, []byte(""), 0o600))

	conflict, exited := detectStartupFailure(logPath, os.Getpid(), 0)
	assert.False(t, conflict)
	assert.False(t, exited)
}

func TestResolveServerDirReturnsErrorWhenNoCandidates(t *testing.T) {
	t.Setenv("NEBULA_SERVER_DIR", "")
	cwd, err := os.Getwd()
	require.NoError(t, err)
	tmp := t.TempDir()
	require.NoError(t, os.Chdir(tmp))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	_, err = resolveServerDir()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not locate server dir")
}

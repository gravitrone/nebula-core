package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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

// TestWaitForAPIHealthUsesDefaultTimeoutWhenNonPositive handles the fallback timeout branch.
func TestWaitForAPIHealthUsesDefaultTimeoutWhenNonPositive(t *testing.T) {
	previousProbe := waitForAPIHealthProbe
	t.Cleanup(func() {
		waitForAPIHealthProbe = previousProbe
	})
	attempts := 0
	waitForAPIHealthProbe = func() (string, error) {
		attempts++
		if attempts >= 2 {
			return "ok", nil
		}
		return "", assert.AnError
	}

	assert.True(t, waitForAPIHealth(0))
	assert.GreaterOrEqual(t, attempts, 2)
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

// TestResolveServerDirFindsNestedNebulaCoreServer handles nested repo discovery without hardcoded vault paths.
func TestResolveServerDirFindsNestedNebulaCoreServer(t *testing.T) {
	root := t.TempDir()
	serverDir := filepath.Join(root, "workspace", "project", "nebula-core", "server")
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

// TestRunStopCmdFallsBackToLiveStatePIDWhenLockPIDIsDead ensures stop targets live runtime-state pid if lock pid is stale.
func TestRunStopCmdFallsBackToLiveStatePIDWhenLockPIDIsDead(t *testing.T) {
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
	t.Cleanup(func() {
		if processAlive(state.PID) {
			if proc, findErr := os.FindProcess(state.PID); findErr == nil {
				_ = proc.Kill()
			}
		}
	})

	staleLock := apiLockState{
		OwnerPID:  os.Getpid(),
		APIPID:    999999,
		CreatedAt: time.Now().UTC(),
	}
	rawLock, err := json.Marshal(staleLock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), rawLock, 0o600))

	var stopOut bytes.Buffer
	require.NoError(t, runStopCmd(&stopOut))
	assert.Contains(t, strings.ToLower(stopOut.String()), "stopped")
	_, stateErr := loadAPIState()
	assert.True(t, os.IsNotExist(stateErr))
	_, lockErr := loadAPILock()
	assert.True(t, os.IsNotExist(lockErr))
}

// TestRunStartCmdDetectsMultiAPIConflictMessage handles address-in-use startup failures with explicit recovery guidance.
func TestRunStartCmdDetectsMultiAPIConflictMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicornScript(
		t,
		"#!/bin/sh\necho 'ERROR: [Errno 98] Address already in use' >&2\nsleep 2\n",
	)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	setWaitForAPIProbe(t, func() (string, error) { return "", assert.AnError })
	setStartHealthTimeout(t, 300*time.Millisecond)

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "multiple api instances detected")

	_, stateErr := loadAPIState()
	assert.True(t, os.IsNotExist(stateErr))
	_, lockErr := loadAPILock()
	assert.True(t, os.IsNotExist(lockErr))
}

// TestRunStartCmdDetectsMultiAPIConflictMessageAfterDelayedLog handles delayed conflict logs from startup races.
func TestRunStartCmdDetectsMultiAPIConflictMessageAfterDelayedLog(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	serverDir := createFakeServerDirWithUvicornScript(
		t,
		"#!/bin/sh\nsleep 0.25\necho 'ERROR: [Errno 98] Address already in use' >&2\nsleep 0.25\necho 'ERROR: [Errno 98] Address already in use' >&2\nsleep 2\n",
	)
	t.Setenv("NEBULA_SERVER_DIR", serverDir)
	setWaitForAPIProbe(t, func() (string, error) { return "", assert.AnError })
	setStartHealthTimeout(t, 100*time.Millisecond)

	var out bytes.Buffer
	err := runStartCmd(&out)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "multiple api instances detected")

	_, stateErr := loadAPIState()
	assert.True(t, os.IsNotExist(stateErr))
	_, lockErr := loadAPILock()
	assert.True(t, os.IsNotExist(lockErr))
}

// TestDetectStartupFailureDetectsConflictFromLog handles conflict detection when log marker is already present.
func TestDetectStartupFailureDetectsConflictFromLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "api.log")
	require.NoError(
		t,
		os.WriteFile(logPath, []byte("ERROR: [Errno 98] Address already in use"), 0o600),
	)

	conflict, exited := detectStartupFailure(logPath, os.Getpid(), 80*time.Millisecond)
	assert.True(t, conflict)
	assert.True(t, exited)
}

// TestDetectStartupFailureReturnsNoFailureWhenProcessAlive handles no-failure result while process stays alive.
func TestDetectStartupFailureReturnsNoFailureWhenProcessAlive(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "api.log")
	require.NoError(t, os.WriteFile(logPath, []byte(""), 0o600))

	conflict, exited := detectStartupFailure(logPath, os.Getpid(), 80*time.Millisecond)
	assert.False(t, conflict)
	assert.False(t, exited)
}

// TestDetectStartupFailureReturnsExitedWhenProcessDies handles process-exit detection without conflict marker.
func TestDetectStartupFailureReturnsExitedWhenProcessDies(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "api.log")
	require.NoError(t, os.WriteFile(logPath, []byte("regular startup logs"), 0o600))

	conflict, exited := detectStartupFailure(logPath, 999999, 80*time.Millisecond)
	assert.False(t, conflict)
	assert.True(t, exited)
}

// TestDetectStartupFailureHandlesUnreadableLogPath ensures log read failures do
// not produce false conflict signals.
func TestDetectStartupFailureHandlesUnreadableLogPath(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "api.log")
	require.NoError(t, os.MkdirAll(logPath, 0o700))

	conflictAlive, exitedAlive := detectStartupFailure(logPath, os.Getpid(), 80*time.Millisecond)
	assert.False(t, conflictAlive)
	assert.False(t, exitedAlive)

	conflictDead, exitedDead := detectStartupFailure(logPath, 999999, 80*time.Millisecond)
	assert.False(t, conflictDead)
	assert.True(t, exitedDead)
}

// TestSaveAPIStateRejectsNilState handles nil input validation branch.
func TestSaveAPIStateRejectsNilState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	err := saveAPIState(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api runtime state is nil")
}

// TestSaveAPIStateReturnsRuntimeDirError handles runtime-dir creation errors.
func TestSaveAPIStateReturnsRuntimeDirError(t *testing.T) {
	homeFile := filepath.Join(t.TempDir(), "home-file")
	require.NoError(t, os.WriteFile(homeFile, []byte("x"), 0o600))
	t.Setenv("HOME", homeFile)

	err := saveAPIState(&apiRuntimeState{PID: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create runtime dir")
}

func TestSaveAPIStateReturnsMarshalRuntimeStateError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	prevMarshal := marshalRuntimeStateJSON
	t.Cleanup(func() {
		marshalRuntimeStateJSON = prevMarshal
	})
	marshalRuntimeStateJSON = func(_ *apiRuntimeState) ([]byte, error) {
		return nil, errors.New("boom")
	}

	err := saveAPIState(&apiRuntimeState{PID: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal runtime state")
}

func TestSaveAPIStateReturnsWriteStateErrorWhenStatePathIsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.MkdirAll(apiStatePath(), 0o700))

	err := saveAPIState(&apiRuntimeState{PID: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write runtime state")
}

func TestSaveAPIStateReturnsWritePIDErrorWhenPIDPathIsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.MkdirAll(apiPIDPath(), 0o700))

	err := saveAPIState(&apiRuntimeState{PID: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is a directory")
}

// TestLoadAPIStateReturnsParseErrorOnInvalidJSON handles runtime-state parse failures.
func TestLoadAPIStateReturnsParseErrorOnInvalidJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.WriteFile(apiStatePath(), []byte("{bad-json"), 0o600))

	_, err := loadAPIState()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse runtime state")
}

// TestUpdateAPILockPIDReturnsWriteErrorWhenPathIsDir handles lock write failures.
func TestUpdateAPILockPIDReturnsWriteErrorWhenPathIsDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(apiLockPath(), 0o700))

	err := updateAPILockPID(os.Getpid())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write api lock")
}

// TestUpdateAPILockPIDRejectsNonPositivePID ensures invalid process IDs are rejected.
func TestUpdateAPILockPIDRejectsNonPositivePID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	err := updateAPILockPID(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid api pid")

	err = updateAPILockPID(-7)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid api pid")

	_, statErr := os.Stat(apiLockPath())
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestUpdateAPILockPIDWritesValidLockState covers the successful lock-file write branch.
func TestUpdateAPILockPIDWritesValidLockState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	require.NoError(t, updateAPILockPID(4242))
	lock, err := loadAPILock()
	require.NoError(t, err)
	assert.Equal(t, 4242, lock.APIPID)
	assert.Equal(t, os.Getpid(), lock.OwnerPID)
	assert.False(t, lock.CreatedAt.IsZero())
}

// TestAcquireAPILockRecoversWhenLockPathIsDir handles stale lock-directory recovery.
func TestAcquireAPILockRecoversWhenLockPathIsDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(apiLockPath(), 0o700))

	require.NoError(t, acquireAPILock())
	lock, err := loadAPILock()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), lock.OwnerPID)
}

// TestAcquireAPILockReturnsRuntimeDirErrorWhenHomeIsFile handles mkdir failures for runtime dir.
func TestAcquireAPILockReturnsRuntimeDirErrorWhenHomeIsFile(t *testing.T) {
	homeFile := filepath.Join(t.TempDir(), "home-file")
	require.NoError(t, os.WriteFile(homeFile, []byte("x"), 0o600))
	t.Setenv("HOME", homeFile)

	err := acquireAPILock()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create runtime dir")
}

// TestAcquireAPILockReturnsCreateLockErrorOnPermissionDenied ensures lock creation
// failures surface a clear create-api-lock error when runtime dir is not writable.
func TestAcquireAPILockReturnsCreateLockErrorOnPermissionDenied(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.Chmod(runtimeDir(), 0o500))
	t.Cleanup(func() {
		_ = os.Chmod(runtimeDir(), 0o700)
	})

	err := acquireAPILock()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create api lock")
}

// TestAcquireAPILockRecoversFromCorruptStateWhenLockHasNoPID covers stale lock + unreadable state recovery.
func TestAcquireAPILockRecoversFromCorruptStateWhenLockHasNoPID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	lock := apiLockState{
		OwnerPID:  11111,
		APIPID:    0,
		CreatedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(lock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), raw, 0o600))
	require.NoError(t, os.WriteFile(apiStatePath(), []byte("{broken"), 0o600))

	require.NoError(t, acquireAPILock())
	loaded, err := loadAPILock()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), loaded.OwnerPID)
	assert.Zero(t, loaded.APIPID)
}

// TestAcquireAPILockTreatsLiveOwnerWithoutAPIPIDAsHeld prevents lock stealing during startup races.
func TestAcquireAPILockTreatsLiveOwnerWithoutAPIPIDAsHeld(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	lock := apiLockState{
		OwnerPID:  os.Getpid(),
		APIPID:    0,
		CreatedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(lock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), raw, 0o600))

	err = acquireAPILock()
	require.Error(t, err)
	var held *apiLockHeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, os.Getpid(), held.PID)

	loaded, loadErr := loadAPILock()
	require.NoError(t, loadErr)
	assert.Equal(t, os.Getpid(), loaded.OwnerPID)
	assert.Zero(t, loaded.APIPID)
}

// TestAcquireAPILockCorruptLockButLiveRuntimeStateReturnsHeldError ensures runtime-state fallback blocks duplicate starts.
func TestAcquireAPILockCorruptLockButLiveRuntimeStateReturnsHeldError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.WriteFile(apiLockPath(), []byte("{broken"), 0o600))
	require.NoError(
		t,
		saveAPIState(&apiRuntimeState{
			PID:       os.Getpid(),
			Port:      api.DefaultAPIPort,
			ServerDir: "/tmp/server",
			LogPath:   "/tmp/log",
			StartedAt: time.Now().UTC(),
		}),
	)

	err := acquireAPILock()
	require.Error(t, err)
	var held *apiLockHeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, os.Getpid(), held.PID)
}

// TestAcquireAPILockFallsBackToRuntimeStateWhenLockPIDIsDead ensures dead lock PID metadata does not bypass live runtime state.
func TestAcquireAPILockFallsBackToRuntimeStateWhenLockPIDIsDead(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	staleLock := apiLockState{
		OwnerPID:  11111,
		APIPID:    999999,
		CreatedAt: time.Now().UTC(),
	}
	rawLock, err := json.Marshal(staleLock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), rawLock, 0o600))
	require.NoError(
		t,
		saveAPIState(&apiRuntimeState{
			PID:       os.Getpid(),
			Port:      api.DefaultAPIPort,
			ServerDir: "/tmp/server",
			LogPath:   "/tmp/log",
			StartedAt: time.Now().UTC(),
		}),
	)

	err = acquireAPILock()
	require.Error(t, err)
	var held *apiLockHeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, os.Getpid(), held.PID)
}

// TestRunLogsCmdReturnsReadErrorWhenLogPathIsDirectory handles non-file log path errors.
func TestRunLogsCmdReturnsReadErrorWhenLogPathIsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(apiLogPath(), 0o700))

	var out bytes.Buffer
	err := runLogsCmd(&out, true, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read api log")
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

// createFakeServerDirWithUvicornScript handles constructing a temporary server dir with a custom uvicorn shim script.
func createFakeServerDirWithUvicornScript(t *testing.T, uvicornScript string) string {
	t.Helper()
	serverDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, "src", "nebula_api"), 0o755))
	require.NoError(
		t,
		os.WriteFile(filepath.Join(serverDir, "src", "nebula_api", "app.py"), []byte("app = None\n"), 0o644),
	)
	require.NoError(t, os.MkdirAll(filepath.Join(serverDir, ".venv", "bin"), 0o755))
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

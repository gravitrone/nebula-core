package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitrone/nebula-core/cli/internal/api"
)

// TestTailLinesSkipsBlankAndLimits handles test tail lines skips blank and limits.
func TestTailLinesSkipsBlankAndLimits(t *testing.T) {
	lines := []string{"", "a", " ", "b", "c", ""}
	out := tailLines(lines, 2)
	assert.Equal(t, []string{"b", "c"}, out)
}

// TestNormalizeServerDirCandidate handles test normalize server dir candidate.
func TestNormalizeServerDirCandidate(t *testing.T) {
	tmp := t.TempDir()
	_, ok := normalizeServerDirCandidate(tmp)
	assert.False(t, ok)

	valid := filepath.Join(tmp, "server")
	require.NoError(t, os.MkdirAll(filepath.Join(valid, "src", "nebula_api"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(valid, "src", "nebula_api", "app.py"), []byte("app = None\n"), 0o644))

	dir, ok := normalizeServerDirCandidate(valid)
	assert.True(t, ok)
	assert.Equal(t, valid, dir)
}

// TestResolveServerDirUsesEnv handles test resolve server dir uses env.
func TestResolveServerDirUsesEnv(t *testing.T) {
	valid := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(valid, "src", "nebula_api"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(valid, "src", "nebula_api", "app.py"), []byte("app = None\n"), 0o644))

	t.Setenv("NEBULA_SERVER_DIR", valid)
	got, err := resolveServerDir()
	require.NoError(t, err)
	assert.Equal(t, valid, got)
}

// TestRunLogsCmdWithoutLogFileShowsFriendlyMessage handles test run logs cmd without log file shows friendly message.
func TestRunLogsCmdWithoutLogFileShowsFriendlyMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	require.NoError(t, runLogsCmd(&out, true, 50))
	assert.Contains(t, out.String(), "No API logs yet")
}

// TestRunLogsCmdUsesDefaultTailWhenNonPositive ensures non-positive tail values
// still render recent API logs.
func TestRunLogsCmdUsesDefaultTailWhenNonPositive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.WriteFile(apiLogPath(), []byte("first\nsecond\nthird\n"), 0o600))

	var out bytes.Buffer
	require.NoError(t, runLogsCmd(&out, true, 0))
	text := out.String()
	assert.Contains(t, text, "Nebula API Logs")
	assert.Contains(t, text, "first")
	assert.Contains(t, text, "third")
}

// TestRunStartCmdUsesLiveState verifies start reports already-running when a live
// runtime state PID exists.
func TestRunStartCmdUsesLiveState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	state := &apiRuntimeState{
		PID:       os.Getpid(),
		Port:      api.DefaultAPIPort,
		ServerDir: "/tmp/nebula/server",
		LogPath:   "/tmp/nebula/api.log",
		StartedAt: time.Now().UTC(),
	}
	require.NoError(t, saveAPIState(state))

	var out bytes.Buffer
	require.NoError(t, runStartCmd(&out))
	text := strings.ToLower(out.String())
	assert.Contains(t, text, "already running")
	assert.Contains(t, out.String(), strconv.Itoa(os.Getpid()))
}

// TestRunStartCmdReportsHeldLockPID ensures start reports a running API when
// lock ownership is held by a live process.
func TestRunStartCmdReportsHeldLockPID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, acquireAPILock())
	require.NoError(t, updateAPILockPID(os.Getpid()))

	var out bytes.Buffer
	require.NoError(t, runStartCmd(&out))
	text := strings.ToLower(out.String())
	assert.Contains(t, text, "already running")
	assert.Contains(t, out.String(), strconv.Itoa(os.Getpid()))
}

// TestAPIStateRoundTrip handles test apistate round trip.
func TestAPIStateRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	state := &apiRuntimeState{
		PID:       12345,
		Port:      8765,
		ServerDir: "/tmp/nebula/server",
		LogPath:   "/tmp/nebula/api.log",
		StartedAt: time.Now().UTC().Round(time.Second),
	}

	require.NoError(t, saveAPIState(state))
	loaded, err := loadAPIState()
	require.NoError(t, err)
	assert.Equal(t, state.PID, loaded.PID)
	assert.Equal(t, state.Port, loaded.Port)
	assert.Equal(t, state.ServerDir, loaded.ServerDir)
	assert.Equal(t, state.LogPath, loaded.LogPath)
	assert.True(t, loaded.StartedAt.Equal(state.StartedAt))
}

// TestAcquireAPILockRejectsLiveAPIPID ensures duplicate starts are blocked when the
// lock is owned by a live API pid.
func TestAcquireAPILockRejectsLiveAPIPID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	require.NoError(t, acquireAPILock())
	require.NoError(t, updateAPILockPID(os.Getpid()))

	err := acquireAPILock()
	require.Error(t, err)
	var held *apiLockHeldError
	require.ErrorAs(t, err, &held)
	assert.Equal(t, os.Getpid(), held.PID)
}

// TestAcquireAPILockRejectsLivePIDFromRuntimeState ensures stale lock metadata
// without APIPID still blocks when runtime state tracks a live process.
func TestAcquireAPILockRejectsLivePIDFromRuntimeState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	lock := apiLockState{
		OwnerPID:  11111,
		APIPID:    0,
		CreatedAt: time.Now().UTC(),
	}
	rawLock, err := json.Marshal(lock)
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

// TestAcquireAPILockCleansStaleFiles ensures stale runtime files are removed and
// lock acquisition still succeeds.
func TestAcquireAPILockCleansStaleFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	staleLock := apiLockState{
		OwnerPID:  42,
		APIPID:    999999,
		CreatedAt: time.Now().UTC(),
	}
	rawLock, err := json.Marshal(staleLock)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.WriteFile(apiLockPath(), rawLock, 0o600))

	staleState := &apiRuntimeState{
		PID:       999999,
		Port:      api.DefaultAPIPort,
		ServerDir: "/tmp/stale",
		LogPath:   "/tmp/stale.log",
		StartedAt: time.Now().UTC(),
	}
	require.NoError(t, saveAPIState(staleState))

	require.NoError(t, acquireAPILock())
	lock, err := loadAPILock()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), lock.OwnerPID)
	assert.Zero(t, lock.APIPID)
}

// TestAcquireAPILockRecoversFromCorruptLock ensures malformed lock content is
// treated as stale state and replaced by a valid lock.
func TestAcquireAPILockRecoversFromCorruptLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.WriteFile(apiLockPath(), []byte("{bad-json"), 0o600))

	require.NoError(t, acquireAPILock())
	lock, err := loadAPILock()
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), lock.OwnerPID)
}

// TestRunStopCmdRefusesUnmanagedRunningProcess ensures stop will not kill a process
// that was not started by managed lock ownership.
func TestRunStopCmdRefusesUnmanagedRunningProcess(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	state := &apiRuntimeState{
		PID:       os.Getpid(),
		Port:      api.DefaultAPIPort,
		ServerDir: "/tmp/nebula/server",
		LogPath:   "/tmp/nebula/api.log",
		StartedAt: time.Now().UTC(),
	}
	require.NoError(t, saveAPIState(state))

	var out bytes.Buffer
	require.NoError(t, runStopCmd(&out))
	text := strings.ToLower(out.String())
	assert.Contains(t, text, "refusing to stop unmanaged process")

	_, err := loadAPIState()
	require.NoError(t, err)
}

// TestRunStopCmdNoState reports a clean not-running message when there is no
// runtime state or lockfile.
func TestRunStopCmdNoState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	require.NoError(t, runStopCmd(&out))
	assert.Contains(t, strings.ToLower(out.String()), "api is not running")
}

// TestRunStopCmdCleansStaleLock verifies stale lockfiles are cleaned when no live
// target process can be found.
func TestRunStopCmdCleansStaleLock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	lock := apiLockState{
		OwnerPID:  os.Getpid(),
		APIPID:    999999,
		CreatedAt: time.Now().UTC(),
	}
	rawLock, err := json.Marshal(lock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), rawLock, 0o600))
	require.NoError(t, os.WriteFile(apiPIDPath(), []byte(strconv.Itoa(lock.APIPID)), 0o600))

	var out bytes.Buffer
	require.NoError(t, runStopCmd(&out))
	assert.Contains(t, strings.ToLower(out.String()), "cleaned stale runtime files")
	_, statErr := os.Stat(apiLockPath())
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// TestRunStopCmdCleansLockWithoutPID verifies stale lock records without a
// usable pid are removed cleanly.
func TestRunStopCmdCleansLockWithoutPID(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))

	lock := apiLockState{
		OwnerPID:  os.Getpid(),
		APIPID:    0,
		CreatedAt: time.Now().UTC(),
	}
	rawLock, err := json.Marshal(lock)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(apiLockPath(), rawLock, 0o600))

	var out bytes.Buffer
	require.NoError(t, runStopCmd(&out))
	assert.Contains(t, strings.ToLower(out.String()), "stale lock cleaned")
	_, statErr := os.Stat(apiLockPath())
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

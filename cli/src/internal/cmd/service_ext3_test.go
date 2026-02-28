package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeServerDirCandidateHandlesEmptyAndInvalidAbs(t *testing.T) {
	_, ok := normalizeServerDirCandidate("")
	assert.False(t, ok)

	_, ok = normalizeServerDirCandidate("\x00bad")
	assert.False(t, ok)
}

func TestProcessAliveReturnsFalseForNonPositivePID(t *testing.T) {
	assert.False(t, processAlive(0))
	assert.False(t, processAlive(-1))
}

func TestSaveAPIStateReturnsWriteRuntimeStateErrorWhenStatePathIsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.Mkdir(apiStatePath(), 0o700))

	err := saveAPIState(&apiRuntimeState{
		PID:       12345,
		Port:      7777,
		ServerDir: "/tmp/server",
		LogPath:   "/tmp/log",
		StartedAt: time.Now().UTC(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write runtime state")
}

func TestSaveAPIStateReturnsPIDWriteErrorWhenPIDPathIsDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(t, os.Mkdir(apiPIDPath(), 0o700))

	err := saveAPIState(&apiRuntimeState{
		PID:       54321,
		Port:      7777,
		ServerDir: "/tmp/server",
		LogPath:   "/tmp/log",
		StartedAt: time.Now().UTC(),
	})
	require.Error(t, err)
}

func TestAcquireAPILockReturnsCreateErrorWhenRuntimeDirNotWritable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o500))
	t.Cleanup(func() {
		_ = os.Chmod(runtimeDir(), 0o700)
	})

	err := acquireAPILock()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create api lock")
}

func TestAcquireAPILockReturnsFailedToAcquireAfterRetryLoop(t *testing.T) {
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

	// Prevent stale lock removal so both retry attempts observe os.ErrExist.
	require.NoError(t, os.Chmod(filepath.Dir(apiLockPath()), 0o500))
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Dir(apiLockPath()), 0o700)
	})

	err = acquireAPILock()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to acquire api lock")
}

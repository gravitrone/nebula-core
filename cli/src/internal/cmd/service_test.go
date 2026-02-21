package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

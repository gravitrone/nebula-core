package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartCmdMetadata(t *testing.T) {
	cmd := StartCmd()
	assert.Equal(t, "start", cmd.Use)
	assert.Contains(t, cmd.Short, "Start local Nebula API")
}

func TestStopCmdMetadata(t *testing.T) {
	cmd := StopCmd()
	assert.Equal(t, "stop", cmd.Use)
	assert.Contains(t, cmd.Short, "Stop local Nebula API")
}

func TestLogsCmdMetadataAndFlags(t *testing.T) {
	cmd := LogsCmd()
	assert.Equal(t, "logs", cmd.Use)
	assert.Contains(t, cmd.Short, "Show local Nebula logs")

	apiOnlyFlag := cmd.Flags().Lookup("api")
	require.NotNil(t, apiOnlyFlag)
	tailFlag := cmd.Flags().Lookup("tail")
	require.NotNil(t, tailFlag)
	assert.Equal(t, "120", tailFlag.DefValue)
}

func TestStartCmdRunEUsesRunStartCmd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	state := &apiRuntimeState{
		PID:       os.Getpid(),
		Port:      api.DefaultAPIPort,
		ServerDir: "/tmp/nebula/server",
		LogPath:   "/tmp/nebula/api.log",
		StartedAt: time.Now().UTC(),
	}
	require.NoError(t, saveAPIState(state))

	cmd := StartCmd()
	require.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.RunE(cmd, nil))
}

func TestStopCmdRunEUsesRunStopCmd(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := StopCmd()
	require.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.RunE(cmd, nil))
}

func TestLogsCmdRunERendersTailFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(runtimeDir(), 0o700))
	require.NoError(
		t,
		os.WriteFile(apiLogPath(), []byte("line-1\nline-2\nline-3\n"), 0o600),
	)

	cmd := LogsCmd()
	require.NoError(t, cmd.Flags().Set("tail", "2"))
	require.NotNil(t, cmd.RunE)
	require.NoError(t, cmd.RunE(cmd, nil))
}

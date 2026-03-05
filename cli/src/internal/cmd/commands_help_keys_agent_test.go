package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyNebulaHelpHandlesNilRoot ensures nil roots are ignored safely.
func TestApplyNebulaHelpHandlesNilRoot(t *testing.T) {
	assert.NotPanics(t, func() {
		ApplyNebulaHelp(nil)
	})
}

// TestApplyNebulaHelpRendersVisibleSortedSections ensures help output only
// exposes visible subcommands/flags and keeps deterministic ordering.
func TestApplyNebulaHelpRendersVisibleSortedSections(t *testing.T) {
	root := &cobra.Command{
		Use:   "nebula",
		Short: "Nebula root command",
	}
	root.PersistentFlags().StringP("config", "c", "", "config path")
	root.PersistentFlags().String("secret", "", "hidden flag")
	require.NoError(t, root.PersistentFlags().MarkHidden("secret"))

	root.AddCommand(
		&cobra.Command{Use: "zeta", Short: "zeta cmd", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "alpha", Short: "alpha cmd", Run: func(*cobra.Command, []string) {}},
		&cobra.Command{Use: "hidden", Short: "hidden cmd", Hidden: true, Run: func(*cobra.Command, []string) {}},
	)

	ApplyNebulaHelp(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())

	text := out.String()
	assert.Contains(t, text, "nebula alpha")
	assert.Contains(t, text, "nebula zeta")
	assert.NotContains(t, text, "nebula hidden")
	assert.Contains(t, text, "-c, --config")
	assert.NotContains(t, text, "secret")
}

// TestApplyNebulaHelpFallbackForUnknownTarget ensures the custom help command
// falls back to root output when target resolution fails.
func TestApplyNebulaHelpFallbackForUnknownTarget(t *testing.T) {
	root := &cobra.Command{Use: "nebula", Short: "Nebula root command"}
	ApplyNebulaHelp(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"help", "does-not-exist"})
	require.NoError(t, root.Execute())

	text := out.String()
	assert.Contains(t, text, "nebula")
}

// TestServiceCommandConstructors ensures all service commands expose expected
// entry points and log flags.
func TestServiceCommandConstructors(t *testing.T) {
	start := StartCmd()
	stop := StopCmd()
	logs := LogsCmd()

	assert.Equal(t, "start", start.Use)
	assert.Equal(t, "stop", stop.Use)
	assert.Equal(t, "logs", logs.Use)
	assert.NotNil(t, logs.Flags().Lookup("api"))
	assert.NotNil(t, logs.Flags().Lookup("tail"))
}

// TestAPILockHeldErrorFormatting ensures lock-held error strings stay stable.
func TestAPILockHeldErrorFormatting(t *testing.T) {
	var nilErr *apiLockHeldError
	assert.Equal(t, "api lock is held", nilErr.Error())
	assert.Equal(t, "api lock is held", (&apiLockHeldError{}).Error())
	assert.Equal(t, "api lock is held by pid 123", (&apiLockHeldError{PID: 123}).Error())
}

// TestKeysAndAgentCommandWiring ensures top-level groups expose required
// command paths.
func TestKeysAndAgentCommandWiring(t *testing.T) {
	keys := KeysCmd()
	agent := AgentCmd()

	assert.Equal(t, "keys", keys.Use)
	assert.Equal(t, "agent", agent.Use)
	assert.NotNil(t, keys.Commands())
	assert.NotNil(t, agent.Commands())

	var keySubs []string
	for _, command := range keys.Commands() {
		keySubs = append(keySubs, command.Name())
	}
	assert.Contains(t, keySubs, "list")
	assert.Contains(t, keySubs, "create")
	assert.Contains(t, keySubs, "revoke")

	var agentSubs []string
	for _, command := range agent.Commands() {
		agentSubs = append(agentSubs, command.Name())
	}
	assert.Contains(t, agentSubs, "register")
	assert.Contains(t, agentSubs, "list")
}

// TestKeysCommandsReturnNotLoggedInWithoutConfig ensures key commands fail
// clearly when no local config exists.
func TestKeysCommandsReturnNotLoggedInWithoutConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "list", cmd: keysListCmd()},
		{name: "create", cmd: keysCreateCmd(), args: []string{"demo-key"}},
		{name: "revoke", cmd: keysRevokeCmd(), args: []string{"key-id"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			tc.cmd.SetOut(&out)
			tc.cmd.SetErr(&out)
			tc.cmd.SetArgs(tc.args)

			err := tc.cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not logged in")
		})
	}
}

// TestAgentCommandsReturnNotLoggedInWithoutConfig ensures agent command flows
// return the same login prerequisite errors without local config.
func TestAgentCommandsReturnNotLoggedInWithoutConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cases := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "register", cmd: agentRegisterCmd(), args: []string{"demo-agent"}},
		{name: "list", cmd: agentListCmd()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			tc.cmd.SetOut(&out)
			tc.cmd.SetErr(&out)
			tc.cmd.SetArgs(tc.args)

			err := tc.cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not logged in")
		})
	}
}

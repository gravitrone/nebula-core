package cmd

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLoginCmdRejectsEmptyUsername handles test login cmd rejects empty username.
func TestLoginCmdRejectsEmptyUsername(t *testing.T) {
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Provide an empty username line.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_, _ = io.WriteString(w, "\n")
	_ = w.Close()
	os.Stdin = r

	cmd := LoginCmd()
	cmd.SetArgs([]string{})
	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "username is required")
}

// TestAgentCmdUnknownSubcommandDeterministicError handles test agent cmd unknown subcommand deterministic error.
func TestAgentCmdUnknownSubcommandDeterministicError(t *testing.T) {
	cmd := AgentCmd()
	cmd.SetArgs([]string{"nope"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

// TestAgentCmdHelpWorks handles test agent cmd help works.
func TestAgentCmdHelpWorks(t *testing.T) {
	cmd := AgentCmd()
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	assert.NoError(t, err)
}

// TestKeysCmdNotLoggedInErrors handles test keys cmd not logged in errors.
func TestKeysCmdNotLoggedInErrors(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	assert.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		assert.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cmd := KeysCmd()
	cmd.SetArgs([]string{"list"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

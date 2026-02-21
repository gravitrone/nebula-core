package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runHelpOutput runs run help output.
func runHelpOutput(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	require.NoError(t, root.Execute())
	return out.String()
}

// TestRootHelpUsesNebulaBoxLayout handles test root help uses nebula box layout.
func TestRootHelpUsesNebulaBoxLayout(t *testing.T) {
	output := runHelpOutput(t, "--help")
	assert.Contains(t, output, "╭")
	assert.Contains(t, output, "[ Help ]")
	assert.Contains(t, output, "command")
	assert.Contains(t, output, "nebula")
	assert.NotContains(t, output, "Usage:\n  nebula")
}

// TestSubcommandHelpUsesNebulaBoxLayout handles test subcommand help uses nebula box layout.
func TestSubcommandHelpUsesNebulaBoxLayout(t *testing.T) {
	output := runHelpOutput(t, "keys", "--help")
	assert.Contains(t, output, "╭")
	assert.Contains(t, output, "[ Help ]")
	assert.Contains(t, output, "nebula keys")
	assert.Contains(t, output, "/list")
	assert.NotContains(t, output, "Usage:\n  nebula keys")
}

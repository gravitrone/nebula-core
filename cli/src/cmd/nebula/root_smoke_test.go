package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunTUIMissingConfigReturnsError handles test run tuimissing config returns error.
func TestRunTUIMissingConfigReturnsError(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	_ = inW.Close()
	_ = outW.Close()
	os.Stdin = inR
	os.Stdout = outW
	defer func() {
		_, _ = io.Copy(io.Discard, outR)
		_ = outR.Close()
		_ = inR.Close()
	}()

	err = runTUI()
	assert.Error(t, err)
}

// TestMainHelpFlagDoesNotExit handles test main help flag does not exit.
func TestMainHelpFlagDoesNotExit(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"nebula", "--help"}
	defer func() { os.Args = oldArgs }()

	// main() should return normally for help (no os.Exit).
	main()
}

// TestRunTUIReturnsConfigPermissionError ensures invalid config permissions fail fast.
func TestRunTUIReturnsConfigPermissionError(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() {
		require.NoError(t, os.Setenv("HOME", oldHome))
	}()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	cfgPath := filepath.Join(cfgDir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte("api_key: nbl_test\n"), 0o644))

	err := runTUI()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config permissions too open")
}

// TestIsInteractiveTerminalHandlesNilFile ensures nil handles are treated as non-interactive.
func TestIsInteractiveTerminalHandlesNilFile(t *testing.T) {
	assert.False(t, isInteractiveTerminal(nil))
}

package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCommandRegistersExpectedSubcommands(t *testing.T) {
	root := newRootCommand()
	assert.Equal(t, "nebula", root.Use)
	assert.NotNil(t, root.RunE)

	for _, name := range []string{"login", "agent", "keys", "start", "stop", "logs"} {
		cmd, _, err := root.Find([]string{name})
		require.NoError(t, err)
		require.NotNil(t, cmd)
		assert.Equal(t, name, cmd.Name())
	}
}

func TestRunTUIMissingConfigWritesLoginHint(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() { require.NoError(t, os.Setenv("HOME", oldHome)) }()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	inR, inW, err := os.Pipe()
	require.NoError(t, err)
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	defer func() {
		_ = inR.Close()
		_ = inW.Close()
		_ = outR.Close()
		_ = outW.Close()
	}()

	_ = inW.Close() // keep stdin non-interactive
	os.Stdin = inR
	os.Stdout = outW

	err = runTUI()
	require.Error(t, err)

	require.NoError(t, outW.Close())
	out, readErr := io.ReadAll(outR)
	require.NoError(t, readErr)
	assert.Contains(t, string(out), "not logged in. run 'nebula login' first")
}

func TestRunTUIReturnsParseConfigError(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() { require.NoError(t, os.Setenv("HOME", oldHome)) }()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	cfgPath := filepath.Join(cfgDir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte("api_key: [broken\n"), 0o600))

	err := runTUI()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestRunTUIWithValidConfigNonTTYReturnsTUIError(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() { require.NoError(t, os.Setenv("HOME", oldHome)) }()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	cfgPath := filepath.Join(cfgDir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte("api_key: nbl_test\n"), 0o600))

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	inFile, err := os.CreateTemp(t.TempDir(), "stdin-*")
	require.NoError(t, err)
	defer func() { _ = inFile.Close() }()
	outFile, err := os.CreateTemp(t.TempDir(), "stdout-*")
	require.NoError(t, err)
	defer func() { _ = outFile.Close() }()

	os.Stdin = inFile
	os.Stdout = outFile

	err = runTUI()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tui error")
}

func TestRunTUIMissingConfigWithCharDevicesAttemptsTUI(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() { require.NoError(t, os.Setenv("HOME", oldHome)) }()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	in, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("dev null unavailable:", err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		t.Skip("dev null unavailable:", err)
	}
	defer func() { _ = out.Close() }()

	// /dev/null is a char device, so runTUI should follow the interactive branch.
	os.Stdin = in
	os.Stdout = out

	err = runTUI()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tui error")
	assert.NotContains(t, err.Error(), "config not found")
}

func TestIsInteractiveTerminalReturnsFalseForRegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "regular-*")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	assert.False(t, isInteractiveTerminal(f))
}

func TestRootCommandRunEDelegatesToRunTUI(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() { require.NoError(t, os.Setenv("HOME", oldHome)) }()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	inR, inW, err := os.Pipe()
	require.NoError(t, err)
	outR, outW, err := os.Pipe()
	require.NoError(t, err)
	defer func() {
		_ = inR.Close()
		_ = inW.Close()
		_ = outR.Close()
		_ = outW.Close()
	}()

	_ = inW.Close()
	os.Stdin = inR
	os.Stdout = outW

	root := newRootCommand()
	root.SetArgs([]string{})
	err = root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config not found")
}

func TestRunTUIReturnsNilWhenProgramRunnerSucceeds(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() { require.NoError(t, os.Setenv("HOME", oldHome)) }()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	cfgPath := filepath.Join(cfgDir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte("api_key: nbl_test\n"), 0o600))

	previousRunner := runBubbleTUI
	t.Cleanup(func() { runBubbleTUI = previousRunner })

	var captured tea.Model
	runBubbleTUI = func(app tea.Model) error {
		captured = app
		return nil
	}

	err := runTUI()
	require.NoError(t, err)
	assert.NotNil(t, captured)
}

func TestRunTUIWrapsProgramRunnerError(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	require.NoError(t, os.Setenv("HOME", dir))
	defer func() { require.NoError(t, os.Setenv("HOME", oldHome)) }()

	cfgDir := filepath.Join(dir, ".nebula")
	require.NoError(t, os.MkdirAll(cfgDir, 0o700))
	cfgPath := filepath.Join(cfgDir, "config")
	require.NoError(t, os.WriteFile(cfgPath, []byte("api_key: nbl_test\n"), 0o600))

	previousRunner := runBubbleTUI
	t.Cleanup(func() { runBubbleTUI = previousRunner })

	runBubbleTUI = func(_ tea.Model) error {
		return errors.New("runner boom")
	}

	err := runTUI()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tui error: runner boom")
}

func TestIsInteractiveTerminalReturnsFalseForClosedFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "closed-*")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	assert.False(t, isInteractiveTerminal(f))
}

func TestIsInteractiveTerminalMatchesDeviceBitForDevNull(t *testing.T) {
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Skip("dev null unavailable:", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	require.NoError(t, err)
	expected := info.Mode()&os.ModeCharDevice != 0
	assert.Equal(t, expected, isInteractiveTerminal(f))
}

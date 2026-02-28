package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainHelpPathDoesNotExit(t *testing.T) {
	t.Setenv("NEBULA_URL", "http://127.0.0.1:1")
	origArgs := os.Args
	origStdout := os.Stdout
	defer func() {
		os.Args = origArgs
		os.Stdout = origStdout
	}()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	os.Args = []string{"nebula", "--help"}

	main()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Contains(t, string(out), "Nebula")
}

func TestMainExecuteErrorExitsWithCodeOne(t *testing.T) {
	if os.Getenv("NEBULA_TEST_MAIN_EXEC_ERROR") == "1" {
		os.Args = []string{"nebula", "nope-subcommand"}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainExecuteErrorExitsWithCodeOne")
	cmd.Env = append(os.Environ(), "NEBULA_TEST_MAIN_EXEC_ERROR=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.Error(t, err)

	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, stderr.String(), "unknown command")
}

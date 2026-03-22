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
	defer func() {
		_ = r.Close()
	}()
	os.Stdout = w
	os.Args = []string{"nebula", "--help"}

	var out bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&out, r)
		readDone <- copyErr
	}()

	main()

	require.NoError(t, w.Close())
	require.NoError(t, <-readDone)
	assert.Contains(t, out.String(), "Nebula")
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

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, stderr.String(), "unknown command")
}

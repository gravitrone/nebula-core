package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCopyTextToClipboardReturnsNotFoundWhenNoBinaryExists handles test copy text to clipboard returns not found when no binary exists.
func TestCopyTextToClipboardReturnsNotFoundWhenNoBinaryExists(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	err := copyTextToClipboard("hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clipboard utility not found")
}

// TestCopyTextToClipboardUsesFirstAvailableCandidate handles test copy text to clipboard uses first available candidate.
func TestCopyTextToClipboardUsesFirstAvailableCandidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script clipboard shim is unix-only")
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "pbcopy")
	script := "#!/bin/sh\ncat >/dev/null\nexit 0\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	t.Setenv("PATH", tmp)

	err := copyTextToClipboard("line1\r\nline2")
	require.NoError(t, err)
}

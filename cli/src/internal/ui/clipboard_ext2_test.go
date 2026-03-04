package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyTextToClipboardCommandFailureBranch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script clipboard shim is unix-only")
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "pbcopy")
	script := "#!/bin/sh\nexit 1\n"
	require.NoError(t, os.WriteFile(bin, []byte(script), 0o755))
	t.Setenv("PATH", tmp)

	err := copyTextToClipboard("hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clipboard copy failed")
}

func TestCopyTextToClipboardFallsBackToNextCandidateAfterFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script clipboard shim is unix-only")
	}

	tmp := t.TempDir()
	pbcopy := filepath.Join(tmp, "pbcopy")
	wlCopy := filepath.Join(tmp, "wl-copy")
	out := filepath.Join(tmp, "clipboard.txt")

	require.NoError(t, os.WriteFile(pbcopy, []byte("#!/bin/sh\nexit 1\n"), 0o755))
	require.NoError(t, os.WriteFile(wlCopy, []byte("#!/bin/sh\nexit 1\n"), 0o755))
	t.Setenv("PATH", tmp)

	err := copyTextToClipboard("line1\r\nline2")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "clipboard copy failed")

	// wl-copy in candidate list does not receive args; add xclip shim to verify fallback path.
	xclip := filepath.Join(tmp, "xclip")
	require.NoError(
		t,
		os.WriteFile(xclip, []byte("#!/bin/sh\n/bin/cat > \""+out+"\"\n"), 0o755),
	)

	err = copyTextToClipboard("line1\r\nline2")
	require.NoError(t, err)

	raw, readErr := os.ReadFile(out)
	require.NoError(t, readErr)
	assert.Equal(t, "line1\nline2", string(raw))
}

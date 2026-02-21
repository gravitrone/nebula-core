package ui

import (
	"fmt"
	"os/exec"
	"strings"
)

// copyTextToClipboard handles copy text to clipboard.
func copyTextToClipboard(text string) error {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	candidates := [][]string{
		{"pbcopy"},
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"clip"},
	}

	var lastErr error
	for _, candidate := range candidates {
		if len(candidate) == 0 {
			continue
		}
		bin, err := exec.LookPath(candidate[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(bin, candidate[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return fmt.Errorf("clipboard copy failed: %w", lastErr)
	}
	return fmt.Errorf("clipboard utility not found (pbcopy/wl-copy/xclip/xsel/clip)")
}

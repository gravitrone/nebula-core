package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
)

// TestTaxonomyLineSanitizeGapRepro handles test taxonomy line sanitize gap repro.
func TestTaxonomyLineSanitizeGapRepro(t *testing.T) {
	item := api.TaxonomyEntry{
		Name:      "\x1b]8;;https://evil.example\x07click\x1b]8;;\x07\nsecond-line",
		IsBuiltin: true,
		IsActive:  true,
	}

	out := formatTaxonomyLine(item)
	if strings.Contains(out, "\x1b]8;;") {
		t.Fatalf("unexpected OSC escape sequence in output: %q", out)
	}
	if strings.Contains(out, "\n") {
		t.Fatalf("unexpected newline in output: %q", out)
	}
}

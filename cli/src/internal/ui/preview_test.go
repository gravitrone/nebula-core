package ui

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// TestFormatScopePreview handles test format scope preview.
func TestFormatScopePreview(t *testing.T) {
	got := formatScopePreview([]string{"public", "admin"})
	want := "[public] [admin]"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}

	if empty := formatScopePreview(nil); empty != "-" {
		t.Fatalf("expected dash for empty scopes, got %q", empty)
	}
}

// TestRenderPreviewRowScopesSupportsCommaAndBracketFormats handles test render preview row scopes supports comma and bracket formats.
func TestRenderPreviewRowScopesSupportsCommaAndBracketFormats(t *testing.T) {
	row := renderPreviewRow("Scopes", "public, admin", 80)
	clean := components.SanitizeText(row)
	if !strings.Contains(clean, "Scopes:") {
		t.Fatalf("expected scopes label, got %q", clean)
	}
	if !strings.Contains(clean, "public") || !strings.Contains(clean, "admin") {
		t.Fatalf("expected rendered scope badges with public and admin, got %q", clean)
	}

	row = renderPreviewRow("Scopes", "[public] [admin]", 80)
	clean = components.SanitizeText(row)
	if !strings.Contains(clean, "public") || !strings.Contains(clean, "admin") {
		t.Fatalf("expected bracket scopes with public and admin, got %q", clean)
	}
}

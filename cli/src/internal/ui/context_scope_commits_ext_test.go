package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextNormalizeScopeBranchMatrix(t *testing.T) {
	// Whitespace-only and # prefix produce empty string.
	assert.Equal(t, "", normalizeScope("   "))
	assert.Equal(t, "", normalizeScope("#"))

	// Normalization: trim, strip #, lowercase, collapse spaces to dashes.
	assert.Equal(t, "public", normalizeScope(" Public "))
	assert.Equal(t, "team-scope", normalizeScope("Team Scope"))
	assert.Equal(t, "admin_team", normalizeScope("#Admin_Team"))

	// Dedup removes repeated entries after normalization.
	scopes := []string{"public", "public", "team-scope"}
	scopes = dedup(scopes)
	assert.Equal(t, []string{"public", "team-scope"}, scopes)
}

func TestContextNormalizeScopeListDedupsViaParseAndNormalize(t *testing.T) {
	// Parse comma-separated, normalize each, dedup.
	scopeStr := "private, Admin Team, #private"
	scopes := parseCommaSeparated(scopeStr)
	for i, s := range scopes {
		scopes[i] = normalizeScope(s)
	}
	scopes = normalizeScopeList(scopes)
	assert.Contains(t, scopes, "private")
	assert.Contains(t, scopes, "admin-team")
	assert.Len(t, scopes, 2) // #private normalizes to "private" which deduplicates
}

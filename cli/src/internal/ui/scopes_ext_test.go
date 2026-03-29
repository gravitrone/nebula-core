package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeNameListEmptyReturnsNil(t *testing.T) {
	assert.Nil(t, scopeNameList(nil))
	assert.Nil(t, scopeNameList(map[string]string{}))
}

func TestScopeNameListSortedAndSkipsEmpty(t *testing.T) {
	got := scopeNameList(map[string]string{
		"c": "public",
		"a": "",
		"b": "private",
		"d": "admin",
	})
	assert.Equal(t, []string{"admin", "private", "public"}, got)
}

func TestToggleScopeAddAndRemove(t *testing.T) {
	start := []string{"public", "private"}

	removed := toggleScope(start, "private")
	assert.Equal(t, []string{"public"}, removed)

	added := toggleScope(removed, "admin")
	assert.Equal(t, []string{"public", "admin"}, added)
}

func TestRenderScopePillsStates(t *testing.T) {
	assert.Equal(t, "-", renderScopePills(nil, false))

	focusedEmpty := stripANSI(renderScopePills(nil, true))
	assert.Contains(t, focusedEmpty, "█")

	focusedWithScopes := stripANSI(renderScopePills([]string{"public", "private"}, true))
	assert.Contains(t, focusedWithScopes, "[public]")
	assert.Contains(t, focusedWithScopes, "[private]")
	assert.Contains(t, focusedWithScopes, "█")
}

func TestRenderScopeBadgeTrimAndEmpty(t *testing.T) {
	assert.Equal(t, "", renderScopeBadge("   "))

	publicBadge := stripANSI(renderScopeBadge(" public "))
	assert.Contains(t, publicBadge, "public")
}

func TestRenderScopeBadgeSupportsKnownAndUnknownScopes(t *testing.T) {
	known := []string{"public", "private", "sensitive", "admin"}
	for _, scope := range known {
		clean := stripANSI(renderScopeBadge(scope))
		assert.Contains(t, clean, scope)
	}

	custom := stripANSI(renderScopeBadge("custom-scope"))
	assert.Contains(t, custom, "custom-scope")
}

func TestRenderScopeOptionsNoScopesAvailableMessage(t *testing.T) {
	out := stripANSI(renderScopeOptions(nil, nil, 0))
	assert.Contains(t, out, "no scopes available")
}

func TestRenderScopeOptionsCursorAndSelectionMatrix(t *testing.T) {
	out := stripANSI(renderScopeOptions(
		[]string{"private"},
		[]string{"public", "private", "admin"},
		2,
	))
	assert.Contains(t, out, "public")
	assert.Contains(t, out, "[private]")
	assert.Contains(t, out, "admin")
}

func TestRenderScopeOptionsFallbackOptionsFromSelected(t *testing.T) {
	out := stripANSI(renderScopeOptions([]string{"sensitive"}, []string{}, 0))
	require.NotEmpty(t, out)
	assert.Contains(t, out, "[sensitive]")
}

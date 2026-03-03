package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataGroupAndFieldHandlesEmptyAndSinglePath(t *testing.T) {
	group, field := metadataGroupAndField("")
	assert.Equal(t, "-", group)
	assert.Equal(t, "-", field)

	group, field = metadataGroupAndField("profile")
	assert.Equal(t, "-", group)
	assert.Equal(t, "profile", field)
}

func TestMetadataGroupAndFieldHumanizesContextSegments(t *testing.T) {
	group, field := metadataGroupAndField("context_segments[0].text")
	assert.Equal(t, "context", group)
	assert.Equal(t, "segment 1.text", field)

	group, field = metadataGroupAndField("profile.context_segments.1")
	assert.Equal(t, "profile", group)
	assert.Equal(t, "context segment 2", field)
}

func TestMetadataGroupAndFieldHumanizesNumericSegments(t *testing.T) {
	group, field := metadataGroupAndField("profile.aliases[2]")
	assert.Equal(t, "profile", group)
	assert.Equal(t, "aliases.item 3", field)
}

func TestSplitMetadataPathNormalizesBracketNotation(t *testing.T) {
	parts := splitMetadataPath(" profile.aliases[2].name ")
	assert.Equal(t, []string{"profile", "aliases", "2", "name"}, parts)

	assert.Nil(t, splitMetadataPath("   "))
}

func TestMetadataColumnWidthsClampSmallContentWidth(t *testing.T) {
	group, field, value := metadataColumnWidths(20)
	assert.Equal(t, 10, group)
	assert.Equal(t, 14, field)
	assert.Equal(t, 14, value)
}

func TestMetadataColumnWidthsNeverExceedUsableWidth(t *testing.T) {
	for _, contentWidth := range []int{0, 10, 20, 34, 40, 48, 80} {
		group, field, value := metadataColumnWidths(contentWidth)
		effectiveContent := contentWidth
		if effectiveContent < 40 {
			effectiveContent = 40
		}
		usable := effectiveContent - 2
		if usable < 38 {
			usable = 38
		}
		assert.LessOrEqual(t, group+field+value, usable)
	}
}

func TestMetadataColumnWidthsScalesForLargerWidth(t *testing.T) {
	group, field, value := metadataColumnWidths(80)
	assert.Equal(t, 17, group)
	assert.Equal(t, 23, field)
	assert.Equal(t, 38, value)
}

func TestFlattenMetadataMapRowsHandlesEmptyMapWithPrefix(t *testing.T) {
	rows := []metadataDisplayRow{}
	flattenMetadataMapRows("root", map[string]any{}, &rows)
	assert.Equal(t, []metadataDisplayRow{{field: "root", value: "None"}}, rows)
}

func TestFlattenMetadataMapRowsFormatsScopesBadgesAndEmptyScopes(t *testing.T) {
	rows := []metadataDisplayRow{}
	flattenMetadataMapRows(
		"",
		map[string]any{
			"scopes": []any{"public", "admin"},
		},
		&rows,
	)
	assert.Equal(t, []metadataDisplayRow{{field: "scopes", value: "[public] [admin]"}}, rows)

	rows = []metadataDisplayRow{}
	flattenMetadataMapRows("", map[string]any{"scopes": []any{}}, &rows)
	assert.Equal(t, []metadataDisplayRow{{field: "scopes", value: "None"}}, rows)
}

func TestFlattenMetadataListRowsHandlesEmptyAndScalarLists(t *testing.T) {
	rows := []metadataDisplayRow{}
	flattenMetadataListRows("tags", []any{}, &rows)
	assert.Equal(t, []metadataDisplayRow{{field: "tags", value: "None"}}, rows)

	rows = []metadataDisplayRow{}
	flattenMetadataListRows("tags", []any{"a", 1}, &rows)
	assert.Equal(t, []metadataDisplayRow{{field: "tags", value: "a, 1"}}, rows)
}

func TestFlattenMetadataListRowsFormatsTextWithScopes(t *testing.T) {
	rows := []metadataDisplayRow{}
	flattenMetadataListRows(
		"context_segments",
		[]any{map[string]any{"text": " hi ", "scopes": []any{"public", "admin"}}},
		&rows,
	)
	assert.Equal(
		t,
		[]metadataDisplayRow{{field: "context_segments[0]", value: "[public] [admin] hi"}},
		rows,
	)
}

func TestFlattenMetadataListRowsFlattensMapWithoutText(t *testing.T) {
	rows := []metadataDisplayRow{}
	flattenMetadataListRows(
		"items",
		[]any{map[string]any{"name": "alpha"}},
		&rows,
	)
	assert.Equal(t, []metadataDisplayRow{{field: "items[0].name", value: "alpha"}}, rows)
}

func TestMetadataLinesPlainFormatsScopesFromSliceValues(t *testing.T) {
	lines := metadataLinesPlain(map[string]any{"scopes": []any{}}, 0)
	assert.Equal(t, []string{"scopes: None"}, lines)

	lines = metadataLinesPlain(map[string]any{"scopes": []any{"PUBLIC", "private"}}, 0)
	assert.Equal(t, []string{"scopes: [public] [private]"}, lines)
}

func TestParseJSONStructuredStringHandlesQuotedJSONAndRejectsScalars(t *testing.T) {
	parsed, ok := parseJSONStructuredString("\"{\\\"k\\\":\\\"v\\\"}\"")
	assert.True(t, ok)
	assert.Equal(t, map[string]any{"k": "v"}, parsed)

	parsed, ok = parseJSONStructuredString("[1,2]")
	assert.True(t, ok)
	assert.Equal(t, []any{float64(1), float64(2)}, parsed)

	_, ok = parseJSONStructuredString("\"42\"")
	assert.False(t, ok)
}

func TestParseStringSliceHandlesAnySliceAndDefaultBranch(t *testing.T) {
	assert.Equal(
		t,
		[]string{"public"},
		parseStringSlice([]any{" Public ", "", 1, nil}),
	)
	assert.Nil(t, parseStringSlice(123))
}

func TestParseStringSliceSkipsNonStringEntriesInAnySlice(t *testing.T) {
	assert.Equal(
		t,
		[]string{"public", "admin"},
		parseStringSlice([]any{"public", map[string]any{"x": 1}, true, " admin "}),
	)
}

func TestParseStringSliceParsesJSONArrayStringPayloads(t *testing.T) {
	assert.Equal(
		t,
		[]string{"public", "admin"},
		parseStringSlice(`[" Public ", "#admin", 1, null, ""]`),
	)
	assert.Equal(
		t,
		[]string{"public", "private"},
		parseStringSlice(`"[\"Public\", \"private\"]"`),
	)
}

func TestParseStringSliceRejectsJSONObjectStringPayloads(t *testing.T) {
	assert.Nil(t, parseStringSlice(`{"scope":"public"}`))
	assert.Nil(t, parseStringSlice(`"{\"scope\":\"public\"}"`))
}

func TestParseStringSliceParsesQuotedScalarStringPayloads(t *testing.T) {
	assert.Equal(
		t,
		[]string{"public", "private"},
		parseStringSlice(`"public, #private"`),
	)
	assert.Equal(t, []string{"public"}, parseStringSlice(`"public"`))
}

func TestScopeBadgesTextSkipsBlankEntries(t *testing.T) {
	assert.Equal(
		t,
		[]string{"[public]", "[admin]"},
		scopeBadgesText([]string{"public", "", " admin "}),
	)
	assert.Nil(t, scopeBadgesText(nil))
}

package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- formatAuditValue ---

func TestFormatAuditValueTypeConfusion(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name   string
		input  any
		expect string
		check  func(t *testing.T, got string)
	}{
		{name: "nil returns None", input: nil, expect: "None"},
		{name: "empty string returns None", input: "", expect: "None"},
		{name: "whitespace only returns None", input: "  ", expect: "None"},
		{name: "tab only returns None", input: "\t", expect: "None"},
		{name: "literal nil string returns None", input: "<nil>", expect: "None"},
		{name: "single dash returns None", input: "-", expect: "None"},
		{name: "double dash returns None", input: "--", expect: "None"},
		{name: "normal text preserved", input: "normal text", expect: "normal text"},
		{name: "hello world preserved", input: "hello world", expect: "hello world"},
		{
			name:  "zero time.Time returns None",
			input: time.Time{},
			check: func(t *testing.T, got string) {
				// formatLocalTimeFull already handles zero -> "None"
				assert.Equal(t, "None", got, "zero time.Time should return None")
			},
		},
		{
			name:  "valid time.Time formats non-empty",
			input: now,
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				assert.NotEqual(t, "None", got)
				assert.Contains(t, got, now.Local().Format("2006-01-02"))
			},
		},
		{name: "int 42", input: 42, expect: "42"},
		{name: "float64 3.14", input: 3.14, expect: "3.14"},
		{name: "bool true", input: true, expect: "true"},
		{name: "bool false", input: false, expect: "false"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuditValue(tc.input)
			if tc.check != nil {
				tc.check(t, got)
			} else {
				assert.Equal(t, tc.expect, got)
			}
		})
	}
}

func TestFormatAuditValueTimestampParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, got string)
	}{
		{
			name:  "RFC3339Nano with offset",
			input: "2026-03-29T16:09:05.525229+00:00",
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "2026-03-29")
				assert.NotEqual(t, "None", got)
				// Must not return raw ISO string
				assert.NotContains(t, got, "T16:09:05")
			},
		},
		{
			name:  "RFC3339 Zulu",
			input: "2024-01-15T10:30:00Z",
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "2024-01-15")
				assert.NotContains(t, got, "T10:30")
			},
		},
		{
			name:  "RFC3339 with positive offset",
			input: "2024-01-15T10:30:00+02:00",
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "2024-01-15")
				assert.NotContains(t, got, "T10:30")
			},
		},
		{
			name:  "date only is not a timestamp",
			input: "2024-01-15",
			check: func(t *testing.T, got string) {
				assert.Equal(t, "2024-01-15", got, "date-only string should be returned as-is")
			},
		},
		{
			name:  "not a timestamp returned as-is",
			input: "not-a-timestamp",
			check: func(t *testing.T, got string) {
				assert.Equal(t, "not-a-timestamp", got)
			},
		},
		{
			name:  "invalid date values returned as-is",
			input: "2024-13-45T99:99:99Z",
			check: func(t *testing.T, got string) {
				assert.Equal(t, "2024-13-45T99:99:99Z", got)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuditValue(tc.input)
			tc.check(t, got)
		})
	}
}

func TestFormatAuditValueJSONParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, got string)
	}{
		{
			name:  "JSON object parsed via metadataLinesPlain",
			input: `{"key": "value"}`,
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "key")
				assert.Contains(t, got, "value")
				assert.NotEqual(t, "None", got)
			},
		},
		{
			name:  "JSON array parsed as comma-separated",
			input: `["a", "b", "c"]`,
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "a")
				assert.Contains(t, got, "b")
				assert.Contains(t, got, "c")
			},
		},
		{
			name:  "JSON quoted string not unwrapped because inner is not object or array",
			input: `"just a string"`,
			check: func(t *testing.T, got string) {
				// parseJSONStructuredString checks HasPrefix("{") or HasPrefix("[") first.
				// A JSON-quoted plain string like "just a string" starts with `"`,
				// so the function unwraps the quotes to get the inner string, but then
				// the inner string doesn't start with { or [, so it returns false.
				// The outer string then goes through timestamp parsing (fails) and
				// SanitizeText which preserves the quotes.
				assert.Equal(t, `"just a string"`, got)
			},
		},
		{
			name:  "deeply nested JSON object",
			input: `{"nested": {"deep": true}}`,
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "nested")
				assert.Contains(t, got, "deep")
			},
		},
		{
			name:  "JSON null returns None",
			input: `null`,
			check: func(t *testing.T, got string) {
				// "null" is only 4 chars, parseJSONStructuredString requires len >= 2
				// and it doesn't start with { or [, so it falls through.
				// Then it fails RFC3339 parsing, goes to humanizeGoMapString (noop),
				// then SanitizeText("null") -> "null"
				assert.Equal(t, "null", got)
			},
		},
		{
			name:  "empty JSON object returns None",
			input: `{}`,
			check: func(t *testing.T, got string) {
				// Parsed as map[string]any with 0 keys, metadataLinesPlain returns
				// empty slice, so formatAuditValue returns "None"
				assert.Equal(t, "None", got)
			},
		},
		{
			name:  "empty JSON array returns None",
			input: `[]`,
			check: func(t *testing.T, got string) {
				// Parsed as []any with 0 elements -> "None"
				assert.Equal(t, "None", got)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuditValue(tc.input)
			tc.check(t, got)
		})
	}
}

func TestFormatAuditValueArrayFormatting(t *testing.T) {
	tests := []struct {
		name   string
		input  []any
		expect string
	}{
		{name: "empty array returns None", input: []any{}, expect: "None"},
		{name: "string elements comma-separated", input: []any{"a", "b", "c"}, expect: "a, b, c"},
		{name: "integer elements comma-separated", input: []any{1, 2, 3}, expect: "1, 2, 3"},
		{name: "nil element renders as nil", input: []any{nil}, expect: "<nil>"},
		{name: "single element no comma", input: []any{"single"}, expect: "single"},
		{name: "mixed types", input: []any{"a", 1, true, nil}, expect: "a, 1, true, <nil>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuditValue(tc.input)
			assert.Equal(t, tc.expect, got)
		})
	}
}

func TestFormatAuditValueMaliciousStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		check func(t *testing.T, got string)
	}{
		{
			name:  "SQL injection does not crash",
			input: "'; DROP TABLE audit;--",
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				assert.NotEqual(t, "None", got)
				// Content preserved through sanitize (no SQL-specific stripping)
				assert.Contains(t, got, "DROP TABLE")
			},
		},
		{
			name:  "XSS payload does not crash",
			input: "<script>alert('xss')</script>",
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// SanitizeText does not strip HTML, only control chars/ANSI
				assert.Contains(t, got, "script")
			},
		},
		{
			name:  "null bytes stripped by SanitizeText",
			input: "hello\x00world",
			check: func(t *testing.T, got string) {
				assert.NotContains(t, got, "\x00", "null bytes must be stripped")
				assert.Contains(t, got, "hello")
				assert.Contains(t, got, "world")
			},
		},
		{
			name:  "very long string does not OOM",
			input: strings.Repeat("a", 100_000),
			check: func(t *testing.T, got string) {
				assert.NotEmpty(t, got)
				// Should be the same length (no control chars to strip)
				assert.Equal(t, 100_000, len(got))
			},
		},
		{
			name:  "ANSI escape sequences stripped",
			input: "\x1b[31mred\x1b[0m",
			check: func(t *testing.T, got string) {
				assert.NotContains(t, got, "\x1b")
				assert.Contains(t, got, "red")
			},
		},
		{
			name:  "literal None string is indistinguishable from nil sentinel",
			input: "None",
			check: func(t *testing.T, got string) {
				// "None" is a normal string, not in the sentinel set ("", "<nil>", "-", "--")
				// so it passes through. This means formatAuditValue("None") == formatAuditValue(nil).
				// Documenting this ambiguity.
				assert.Equal(t, "None", got)
				assert.Equal(t, formatAuditValue(nil), got, "string 'None' is indistinguishable from nil")
			},
		},
		{
			name:  "triple dash not treated as sentinel",
			input: "---",
			check: func(t *testing.T, got string) {
				// Only "-" and "--" are sentinels
				assert.Equal(t, "---", got)
			},
		},
		{
			name:  "newlines preserved in SanitizeText",
			input: "line1\nline2\nline3",
			check: func(t *testing.T, got string) {
				assert.Contains(t, got, "\n")
				assert.Contains(t, got, "line1")
				assert.Contains(t, got, "line3")
			},
		},
		{
			name:  "bidirectional override chars stripped",
			input: "normal\u202Eoverride\u202Ctext",
			check: func(t *testing.T, got string) {
				assert.NotContains(t, got, "\u202E", "bidi override must be stripped")
				assert.Contains(t, got, "normal")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatAuditValue(tc.input)
			tc.check(t, got)
		})
	}
}

// --- humanizeAuditField ---

func TestHumanizeAuditField(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "empty string", input: "", expect: ""},
		{name: "simple word capitalized", input: "simple", expect: "Simple"},
		{name: "snake_case to Title Case", input: "snake_case", expect: "Snake Case"},
		{name: "kebab-case to Title Case", input: "kebab-case", expect: "Kebab Case"},
		{name: "entity_id uppercases ID", input: "entity_id", expect: "Entity ID"},
		{name: "bare id uppercases", input: "id", expect: "ID"},
		{name: "status_id uppercases ID", input: "status_id", expect: "Status ID"},
		{
			name:  "privacy_scope_ids uppercases IDS",
			input: "privacy_scope_ids",
			// Fixed: the code now matches both "id" and "ids".
			expect: "Privacy Scope IDS",
		},
		{name: "multi-part a_b_c", input: "a_b_c", expect: "A B C"},
		{
			name:  "leading/trailing underscores empty parts skipped",
			input: "__empty__parts__",
			// After split by "_": ["", "", "empty", "", "parts", "", ""]
			// Empty parts are skipped after TrimSpace
			expect: "Empty Parts",
		},
		{
			name:  "UPPER_CASE parts lowercased then capitalized",
			input: "UPPER_CASE",
			// strings.ToLower("UPPER") = "upper", then ToUpper("u")+"pper" = "Upper"
			expect: "Upper Case",
		},
		{
			name:  "mixed case MiXeD_CaSe",
			input: "MiXeD_CaSe",
			// ToLower("MiXeD") = "mixed", then ToUpper("m")+"ixed" = "Mixed"
			expect: "Mixed Case",
		},
		{
			name:  "single underscore",
			input: "_",
			// Split produces ["", ""], both empty, skipped. len(out)==0 -> SanitizeOneLine("_")
			expect: components.SanitizeOneLine("_"),
		},
		{
			name:  "whitespace only returns SanitizeOneLine result",
			input: "  ",
			// TrimSpace("  ") == "" -> returns ""
			expect: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := humanizeAuditField(tc.input)
			assert.Equal(t, tc.expect, got)
		})
	}
}

// --- parseAuditValuesMap ---

func TestParseAuditValuesMap(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectNil bool
		check     func(t *testing.T, m map[string]any)
	}{
		{name: "empty string returns nil", input: "", expectNil: true},
		{
			name:  "empty object returns empty map",
			input: "{}",
			check: func(t *testing.T, m map[string]any) {
				require.NotNil(t, m)
				assert.Empty(t, m)
			},
		},
		{
			name:  "simple key-value",
			input: `{"key": "value"}`,
			check: func(t *testing.T, m map[string]any) {
				require.NotNil(t, m)
				assert.Equal(t, "value", m["key"])
			},
		},
		{name: "non-JSON returns nil", input: "not json", expectNil: true},
		{name: "JSON array returns nil", input: `[1, 2, 3]`, expectNil: true},
		{
			name:  "nested map preserved",
			input: `{"nested": {"a": 1}}`,
			check: func(t *testing.T, m map[string]any) {
				require.NotNil(t, m)
				inner, ok := m["nested"].(map[string]any)
				require.True(t, ok, "nested value should be map[string]any")
				assert.Equal(t, float64(1), inner["a"])
			},
		},
		{name: "truncated JSON returns nil", input: `{"key": "val`, expectNil: true},
		{name: "JSON null returns nil", input: "null", expectNil: true},
		{
			name:  "numeric values as float64",
			input: `{"count": 42}`,
			check: func(t *testing.T, m map[string]any) {
				require.NotNil(t, m)
				assert.Equal(t, float64(42), m["count"])
			},
		},
		{
			name:  "special characters in keys",
			input: `{"key with spaces": "ok", "key\nwith\nnewlines": "ok2"}`,
			check: func(t *testing.T, m map[string]any) {
				require.NotNil(t, m)
				assert.Equal(t, "ok", m["key with spaces"])
				assert.Equal(t, "ok2", m["key\nwith\nnewlines"])
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAuditValuesMap(tc.input)
			if tc.expectNil {
				assert.Nil(t, got)
			} else {
				tc.check(t, got)
			}
		})
	}
}

// --- buildAuditDiffRows ---

func TestBuildAuditDiffRowsRedteam(t *testing.T) {
	tests := []struct {
		name  string
		entry api.AuditEntry
		check func(t *testing.T, rows []components.DiffRow)
	}{
		{
			name:  "empty old and new values returns nil",
			entry: api.AuditEntry{OldValues: "", NewValues: ""},
			check: func(t *testing.T, rows []components.DiffRow) {
				assert.Nil(t, rows)
			},
		},
		{
			name: "same values in changed fields are skipped",
			entry: api.AuditEntry{
				OldValues:     `{"status": "active"}`,
				NewValues:     `{"status": "active"}`,
				ChangedFields: []string{"status"},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				// formatAuditValue("active") == formatAuditValue("active"), so row is skipped.
				// NOTE: returns empty slice (not nil) because rows is pre-allocated with make().
				// This is a minor inconsistency with the nil return for empty keys.
				assert.Empty(t, rows)
			},
		},
		{
			name: "different values produce diff row",
			entry: api.AuditEntry{
				OldValues:     `{"status": "active"}`,
				NewValues:     `{"status": "archived"}`,
				ChangedFields: []string{"status"},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				require.Len(t, rows, 1)
				assert.Equal(t, "Status", rows[0].Label)
				assert.Equal(t, "active", rows[0].From)
				assert.Equal(t, "archived", rows[0].To)
			},
		},
		{
			name: "no changed_fields uses union of old and new keys",
			entry: api.AuditEntry{
				OldValues:     `{"name": "old_name"}`,
				NewValues:     `{"name": "new_name", "type": "agent"}`,
				ChangedFields: nil,
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				require.NotNil(t, rows)
				// "name" differs, "type" only in new (old is nil -> "None", new is "agent")
				labels := make([]string, len(rows))
				for i, r := range rows {
					labels[i] = r.Label
				}
				assert.Contains(t, labels, "Name")
				assert.Contains(t, labels, "Type")
			},
		},
		{
			name: "empty string in changed_fields is skipped",
			entry: api.AuditEntry{
				OldValues:     `{"name": "a"}`,
				NewValues:     `{"name": "b"}`,
				ChangedFields: []string{"", "name", ""},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				require.Len(t, rows, 1)
				assert.Equal(t, "Name", rows[0].Label)
			},
		},
		{
			name: "invalid old_values JSON still processes new values",
			entry: api.AuditEntry{
				OldValues:     `not valid json`,
				NewValues:     `{"status": "active"}`,
				ChangedFields: []string{"status"},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				require.Len(t, rows, 1)
				assert.Equal(t, "Status", rows[0].Label)
				assert.Equal(t, "None", rows[0].From, "missing key in nil old map -> nil -> None")
				assert.Equal(t, "active", rows[0].To)
			},
		},
		{
			name: "field in changed_fields but absent from both maps",
			entry: api.AuditEntry{
				OldValues:     `{"a": "1"}`,
				NewValues:     `{"a": "2"}`,
				ChangedFields: []string{"missing_field"},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				// Both old["missing_field"] and new["missing_field"] are nil -> "None" == "None"
				// Row skipped, but returns empty slice (not nil) due to pre-allocation.
				assert.Empty(t, rows)
			},
		},
		{
			name: "humanized labels for snake_case fields",
			entry: api.AuditEntry{
				OldValues:     `{"entity_id": "old-id"}`,
				NewValues:     `{"entity_id": "new-id"}`,
				ChangedFields: []string{"entity_id"},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				require.Len(t, rows, 1)
				assert.Equal(t, "Entity ID", rows[0].Label)
			},
		},
		{
			name: "rows are sorted alphabetically by key",
			entry: api.AuditEntry{
				OldValues:     `{"zebra": "1", "alpha": "1"}`,
				NewValues:     `{"zebra": "2", "alpha": "2"}`,
				ChangedFields: []string{"zebra", "alpha"},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				require.Len(t, rows, 2)
				assert.Equal(t, "Alpha", rows[0].Label)
				assert.Equal(t, "Zebra", rows[1].Label)
			},
		},
		{
			name: "old value nil and new value present",
			entry: api.AuditEntry{
				OldValues:     `{}`,
				NewValues:     `{"created_at": "2024-01-15T10:30:00Z"}`,
				ChangedFields: []string{"created_at"},
			},
			check: func(t *testing.T, rows []components.DiffRow) {
				require.Len(t, rows, 1)
				assert.Equal(t, "None", rows[0].From)
				assert.NotEqual(t, "None", rows[0].To)
				assert.Contains(t, rows[0].To, "2024-01-15")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows := buildAuditDiffRows(tc.entry)
			tc.check(t, rows)
		})
	}
}

package components

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sanitizeRenderedDiff(t *testing.T, rendered string) string {
	t.Helper()
	return strings.TrimRight(SanitizeText(rendered), " ")
}

func TestDigitWidth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		n    int
		want int
	}{
		{name: "zero", n: 0, want: 1},
		{name: "one", n: 1, want: 1},
		{name: "single_digit_nine", n: 9, want: 1},
		{name: "two_digits_ten", n: 10, want: 2},
		{name: "two_digits_ninety_nine", n: 99, want: 2},
		{name: "three_digits_one_hundred", n: 100, want: 3},
		{name: "six_digits", n: 999999, want: 6},
		{name: "negative_one", n: -1, want: 1},
		{name: "max_int64_constant", n: math.MaxInt64, want: 19},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, digitWidth(tt.n))
		})
	}
}

func TestTruncateStr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s        string
		maxWidth int
		want     string
	}{
		{name: "exact_fit", s: "hello", maxWidth: 5, want: "hello"},
		{name: "truncate_ascii", s: "hello", maxWidth: 3, want: "hel"},
		{name: "zero_width", s: "hello", maxWidth: 0, want: ""},
		{name: "empty_input", s: "", maxWidth: 5, want: ""},
		{name: "very_large_width", s: "abc", maxWidth: 100, want: "abc"},
		{name: "unicode_cjk_width_four", s: "日本語", maxWidth: 4, want: "日本"},
		{name: "unicode_cjk_width_five", s: "日本語", maxWidth: 5, want: "日本"},
		{name: "unicode_cjk_width_six", s: "日本語", maxWidth: 6, want: "日本語"},
		{name: "long_repeated_ascii", s: strings.Repeat("a", 10000), maxWidth: 5, want: "aaaaa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateStr(tt.s, tt.maxWidth))
		})
	}
}

func TestParseUnifiedDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		rawLines []string
		want     []DiffLine
	}{
		{name: "nil_input_returns_nil", rawLines: nil, want: nil},
		{name: "empty_slice_returns_nil", rawLines: []string{}, want: nil},
		{
			name:     "single_hunk_header",
			rawLines: []string{"@@ -1,3 +1,4 @@"},
			want: []DiffLine{
				{Kind: DiffHunk, Text: "@@ -1,3 +1,4 @@"},
			},
		},
		{
			name:     "single_added_line",
			rawLines: []string{"+added"},
			want: []DiffLine{
				{Kind: DiffAdd, After: 0, Text: "added"},
			},
		},
		{
			name:     "single_removed_line",
			rawLines: []string{"-removed"},
			want: []DiffLine{
				{Kind: DiffDelete, Before: 0, Text: "removed"},
			},
		},
		{
			name:     "space_prefixed_context_strips_leading_space",
			rawLines: []string{" context"},
			want: []DiffLine{
				{Kind: DiffContext, Before: 0, After: 0, Text: "context"},
			},
		},
		{
			name:     "context_without_prefix_is_preserved",
			rawLines: []string{"noprefix"},
			want: []DiffLine{
				{Kind: DiffContext, Before: 0, After: 0, Text: "noprefix"},
			},
		},
		{
			name:     "empty_line_is_context",
			rawLines: []string{""},
			want: []DiffLine{
				{Kind: DiffContext, Before: 0, After: 0, Text: ""},
			},
		},
		{
			name:     "bare_plus_has_empty_text",
			rawLines: []string{"+"},
			want: []DiffLine{
				{Kind: DiffAdd, After: 0, Text: ""},
			},
		},
		{
			name:     "bare_minus_has_empty_text",
			rawLines: []string{"-"},
			want: []DiffLine{
				{Kind: DiffDelete, Before: 0, Text: ""},
			},
		},
		{
			name: "full_sequence_increments_line_numbers",
			rawLines: []string{
				"@@ -1,3 +1,4 @@",
				" context",
				"-removed",
				"+added",
			},
			want: []DiffLine{
				{Kind: DiffHunk, Text: "@@ -1,3 +1,4 @@"},
				{Kind: DiffContext, Before: 1, After: 1, Text: "context"},
				{Kind: DiffDelete, Before: 2, Text: "removed"},
				{Kind: DiffAdd, After: 2, Text: "added"},
			},
		},
		{
			name: "multiple_hunks_reset_line_numbers",
			rawLines: []string{
				"@@ -1,1 +1,1 @@",
				" first",
				"@@ -10,5 +20,3 @@",
				" second",
				"-gone",
				"+new",
			},
			want: []DiffLine{
				{Kind: DiffHunk, Text: "@@ -1,1 +1,1 @@"},
				{Kind: DiffContext, Before: 1, After: 1, Text: "first"},
				{Kind: DiffHunk, Text: "@@ -10,5 +20,3 @@"},
				{Kind: DiffContext, Before: 10, After: 20, Text: "second"},
				{Kind: DiffDelete, Before: 11, Text: "gone"},
				{Kind: DiffAdd, After: 21, Text: "new"},
			},
		},
		{
			name:     "hunk_header_with_function_context_is_preserved",
			rawLines: []string{"@@ -10,5 +20,3 @@ func foo()"},
			want: []DiffLine{
				{Kind: DiffHunk, Text: "@@ -10,5 +20,3 @@ func foo()"},
			},
		},
		{
			name:     "ansi_prefixed_line_is_treated_as_context",
			rawLines: []string{"\x1b[31mred\x1b[0m"},
			want: []DiffLine{
				{Kind: DiffContext, Before: 0, After: 0, Text: "\x1b[31mred\x1b[0m"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseUnifiedDiff(tt.rawLines)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.Len(t, got, len(tt.want))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseUnifiedDiff_parseHunkHeader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		header     string
		wantBefore int
		wantAfter  int
	}{
		{
			name:       "standard_hunk_header",
			header:     "@@ -1,3 +1,4 @@",
			wantBefore: 1,
			wantAfter:  1,
		},
		{
			name:       "header_with_function_context",
			header:     "@@ -10,5 +20,3 @@ func foo()",
			wantBefore: 10,
			wantAfter:  20,
		},
		{
			name:       "malformed_header_falls_back_to_defaults",
			header:     "@@ nonsense @@",
			wantBefore: 1,
			wantAfter:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before, after := parseHunkHeader(tt.header)
			assert.Equal(t, tt.wantBefore, before)
			assert.Equal(t, tt.wantAfter, after)
		})
	}
}

func TestBuildFieldDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		fields []string
		before map[string]string
		after  map[string]string
		want   []DiffLine
	}{
		{name: "nil_fields_returns_nil", fields: nil, before: map[string]string{"x": "1"}, after: map[string]string{"x": "1"}, want: nil},
		{name: "empty_fields_returns_nil", fields: []string{}, before: map[string]string{"x": "1"}, after: map[string]string{"x": "1"}, want: nil},
		{
			name:   "single_same_value_is_context",
			fields: []string{"field"},
			before: map[string]string{"field": "value"},
			after:  map[string]string{"field": "value"},
			want: []DiffLine{
				{Kind: DiffContext, Before: 1, After: 1, Text: "field: value"},
			},
		},
		{
			name:   "single_different_value_is_delete_add_pair",
			fields: []string{"field"},
			before: map[string]string{"field": "before"},
			after:  map[string]string{"field": "after"},
			want: []DiffLine{
				{Kind: DiffDelete, Before: 1, Text: "field: before"},
				{Kind: DiffAdd, After: 1, Text: "field: after"},
			},
		},
		{
			name:   "missing_from_before_only_adds",
			fields: []string{"field"},
			before: map[string]string{},
			after:  map[string]string{"field": "after"},
			want: []DiffLine{
				{Kind: DiffAdd, After: 1, Text: "field: after"},
			},
		},
		{
			name:   "missing_from_after_only_deletes",
			fields: []string{"field"},
			before: map[string]string{"field": "before"},
			after:  map[string]string{},
			want: []DiffLine{
				{Kind: DiffDelete, Before: 1, Text: "field: before"},
			},
		},
		{
			name:   "missing_from_both_is_empty_context",
			fields: []string{"field"},
			before: map[string]string{},
			after:  map[string]string{},
			want: []DiffLine{
				{Kind: DiffContext, Before: 1, After: 1, Text: "field: "},
			},
		},
		{
			name:   "multiple_fields_increment_line_numbers_once_per_field",
			fields: []string{"same", "deleted", "changed", "added"},
			before: map[string]string{"same": "v", "deleted": "old", "changed": "before"},
			after:  map[string]string{"same": "v", "changed": "after", "added": "new"},
			want: []DiffLine{
				{Kind: DiffContext, Before: 1, After: 1, Text: "same: v"},
				{Kind: DiffDelete, Before: 2, Text: "deleted: old"},
				{Kind: DiffDelete, Before: 3, Text: "changed: before"},
				{Kind: DiffAdd, After: 3, Text: "changed: after"},
				{Kind: DiffAdd, After: 4, Text: "added: new"},
			},
		},
		{
			name:   "field_name_with_colon_is_rendered_verbatim",
			fields: []string{"my:field"},
			before: map[string]string{"my:field": "value"},
			after:  map[string]string{"my:field": "value"},
			want: []DiffLine{
				{Kind: DiffContext, Before: 1, After: 1, Text: "my:field: value"},
			},
		},
		{
			name:   "empty_field_name_keeps_separator",
			fields: []string{""},
			before: map[string]string{"": "value"},
			after:  map[string]string{"": "value"},
			want: []DiffLine{
				{Kind: DiffContext, Before: 1, After: 1, Text: ": value"},
			},
		},
		{
			name:   "nil_before_map_does_not_panic",
			fields: []string{"field"},
			before: nil,
			after:  map[string]string{"field": "after"},
			want: []DiffLine{
				{Kind: DiffAdd, After: 1, Text: "field: after"},
			},
		},
		{
			name:   "nil_after_map_does_not_panic",
			fields: []string{"field"},
			before: map[string]string{"field": "before"},
			after:  nil,
			want: []DiffLine{
				{Kind: DiffDelete, Before: 1, Text: "field: before"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFieldDiff(tt.fields, tt.before, tt.after)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.Len(t, got, len(tt.want))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDiffRowsToLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rows []DiffRow
		want []DiffLine
	}{
		{name: "nil_rows_return_nil", rows: nil, want: nil},
		{
			name: "multiline_label_and_values_are_sanitized_to_context",
			rows: []DiffRow{
				{Label: "  field\nname\t", From: " same\nvalue ", To: " same\tvalue "},
			},
			want: []DiffLine{
				{Kind: DiffContext, Before: 1, After: 1, Text: "field name: same value"},
			},
		},
		{
			name: "ansi_and_multiline_values_are_sanitized_before_diffing",
			rows: []DiffRow{
				{Label: "status\nfield", From: "\x1b[31mold\x1b[0m", To: "new\tvalue"},
			},
			want: []DiffLine{
				{Kind: DiffDelete, Before: 1, Text: "status field: old"},
				{Kind: DiffAdd, After: 1, Text: "status field: new value"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiffRowsToLines(tt.rows)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			require.Len(t, got, len(tt.want))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderDiffView_renderDiffLineDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		line   DiffLine
		before int
		after  int
		code   int
		want   func(DiffLine, int, int, int) string
	}{
		{
			name:   "dispatches_hunk_lines",
			line:   DiffLine{Kind: DiffHunk, Text: "@@ -1,1 +1,1 @@"},
			before: 2,
			after:  2,
			code:   8,
			want: func(line DiffLine, beforeW, afterW, codeW int) string {
				return renderHunkLine(line.Text, beforeW+1+afterW+1+2+codeW)
			},
		},
		{
			name:   "dispatches_add_lines",
			line:   DiffLine{Kind: DiffAdd, After: 7, Text: "added"},
			before: 2,
			after:  2,
			code:   8,
			want:   renderAddLine,
		},
		{
			name:   "dispatches_delete_lines",
			line:   DiffLine{Kind: DiffDelete, Before: 6, Text: "removed"},
			before: 2,
			after:  2,
			code:   8,
			want:   renderDeleteLine,
		},
		{
			name:   "defaults_to_context_rendering",
			line:   DiffLine{Kind: DiffContext, Before: 5, After: 5, Text: "context"},
			before: 2,
			after:  2,
			code:   8,
			want:   renderContextLine,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want(tt.line, tt.before, tt.after, tt.code), renderDiffLine(tt.line, tt.before, tt.after, tt.code))
		})
	}
}

func TestRenderDiffView_renderHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		render         func() string
		requiredParts  []string
		forbiddenParts []string
	}{
		{
			name: "render_hunk_line_preserves_header_text",
			render: func() string {
				return renderHunkLine("@@ -1,1 +1,1 @@ func foo()", 8)
			},
			requiredParts: []string{"@@ -1,1 +1,1 @@ func foo()"},
		},
		{
			name: "render_add_line_shows_symbol_and_truncates_code",
			render: func() string {
				return renderAddLine(DiffLine{Kind: DiffAdd, After: 7, Text: "abcdef"}, 2, 2, 4)
			},
			requiredParts:  []string{"7", "+ ", "abcd"},
			forbiddenParts: []string{"abcde"},
		},
		{
			name: "render_delete_line_shows_symbol_and_truncates_code",
			render: func() string {
				return renderDeleteLine(DiffLine{Kind: DiffDelete, Before: 9, Text: "uvwxyz"}, 2, 2, 4)
			},
			requiredParts:  []string{"9", "- ", "uvwx"},
			forbiddenParts: []string{"uvwxy"},
		},
		{
			name: "render_context_line_keeps_both_numbers_and_truncates_code",
			render: func() string {
				return renderContextLine(DiffLine{Kind: DiffContext, Before: 3, After: 4, Text: "ghijkl"}, 2, 2, 4)
			},
			requiredParts:  []string{"3", "4", "ghij"},
			forbiddenParts: []string{"ghijk"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clean := sanitizeRenderedDiff(t, tt.render())
			require.NotEmpty(t, clean)
			for _, part := range tt.requiredParts {
				assert.Contains(t, clean, part)
			}
			for _, part := range tt.forbiddenParts {
				assert.NotContains(t, clean, part)
			}
		})
	}
}

func TestRenderDiffView(t *testing.T) {
	t.Parallel()

	longText := "0123456789ABCDEF"
	manyLines := make([]DiffLine, 100)
	for i := range manyLines {
		manyLines[i] = DiffLine{
			Kind:   DiffContext,
			Before: i + 1,
			After:  i + 1,
			Text:   "line-" + strings.Repeat("x", 3),
		}
	}

	tests := []struct {
		name   string
		lines  []DiffLine
		width  int
		assert func(t *testing.T, got string)
	}{
		{
			name:  "nil_lines_return_empty",
			lines: nil,
			width: 80,
			assert: func(t *testing.T, got string) {
				assert.Equal(t, "", got)
			},
		},
		{
			name:  "empty_lines_return_empty",
			lines: []DiffLine{},
			width: 80,
			assert: func(t *testing.T, got string) {
				assert.Equal(t, "", got)
			},
		},
		{
			name: "zero_width_returns_empty",
			lines: []DiffLine{
				{Kind: DiffAdd, After: 1, Text: "added"},
			},
			width: 0,
			assert: func(t *testing.T, got string) {
				assert.Equal(t, "", got)
			},
		},
		{
			name: "negative_width_returns_empty",
			lines: []DiffLine{
				{Kind: DiffAdd, After: 1, Text: "added"},
			},
			width: -1,
			assert: func(t *testing.T, got string) {
				assert.Equal(t, "", got)
			},
		},
		{
			name: "width_one_still_renders_due_to_code_width_clamp",
			lines: []DiffLine{
				{Kind: DiffAdd, After: 1, Text: longText},
			},
			width: 1,
			assert: func(t *testing.T, got string) {
				clean := sanitizeRenderedDiff(t, got)
				require.NotEmpty(t, clean)
				assert.Contains(t, clean, "+ ")
				assert.Contains(t, clean, "0123456789")
				assert.NotContains(t, clean, "0123456789A")
			},
		},
		{
			name: "single_add_line_contains_plus",
			lines: []DiffLine{
				{Kind: DiffAdd, After: 1, Text: "added"},
			},
			width: 80,
			assert: func(t *testing.T, got string) {
				clean := sanitizeRenderedDiff(t, got)
				require.NotEmpty(t, clean)
				assert.Contains(t, clean, "+ ")
				assert.Contains(t, clean, "added")
			},
		},
		{
			name: "single_delete_line_contains_minus",
			lines: []DiffLine{
				{Kind: DiffDelete, Before: 1, Text: "removed"},
			},
			width: 80,
			assert: func(t *testing.T, got string) {
				clean := sanitizeRenderedDiff(t, got)
				require.NotEmpty(t, clean)
				assert.Contains(t, clean, "- ")
				assert.Contains(t, clean, "removed")
			},
		},
		{
			name: "single_context_line_renders_non_empty",
			lines: []DiffLine{
				{Kind: DiffContext, Before: 1, After: 1, Text: "context"},
			},
			width: 80,
			assert: func(t *testing.T, got string) {
				clean := sanitizeRenderedDiff(t, got)
				require.NotEmpty(t, clean)
				assert.Contains(t, clean, "context")
			},
		},
		{
			name: "single_hunk_line_renders_non_empty",
			lines: []DiffLine{
				{Kind: DiffHunk, Text: "@@ -1,1 +1,1 @@"},
			},
			width: 80,
			assert: func(t *testing.T, got string) {
				clean := sanitizeRenderedDiff(t, got)
				require.NotEmpty(t, clean)
				assert.Contains(t, clean, "@@ -1,1 +1,1 @@")
			},
		},
		{
			name: "very_large_width_renders_without_truncation",
			lines: []DiffLine{
				{Kind: DiffContext, Before: 123, After: 123, Text: longText},
			},
			width: 10000,
			assert: func(t *testing.T, got string) {
				clean := sanitizeRenderedDiff(t, got)
				require.NotEmpty(t, clean)
				assert.Contains(t, clean, longText)
			},
		},
		{
			name:  "one_hundred_lines_render_as_one_hundred_segments",
			lines: manyLines,
			width: 80,
			assert: func(t *testing.T, got string) {
				require.NotEmpty(t, got)
				segments := strings.Split(SanitizeText(got), "\n")
				require.Len(t, segments, 100)
				assert.Contains(t, segments[0], "line-xxx")
				assert.Contains(t, segments[99], "line-xxx")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, RenderDiffView(tt.lines, tt.width))
		})
	}
}

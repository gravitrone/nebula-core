package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

func TestParseApprovalDiffViewOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		only            []string
		maxLines        int
		wantErrContains string
		wantOnlyChanged bool
		wantSections    []string
		wantRawOnly     []string
		wantSectionsNil bool
	}{
		{
			name:            "maxLines=1 only=nil is valid with no filters",
			maxLines:        1,
			wantRawOnly:     nil,
			wantSectionsNil: true,
		},
		{
			name:            "maxLines=0 errors",
			maxLines:        0,
			wantErrContains: "invalid --max-lines 0",
		},
		{
			name:            "maxLines=-1 errors",
			maxLines:        -1,
			wantErrContains: "invalid --max-lines -1",
		},
		{
			name:            "changed filter sets OnlyChanged",
			only:            []string{"changed"},
			maxLines:        1,
			wantOnlyChanged: true,
			wantRawOnly:     []string{"changed"},
			wantSectionsNil: true,
		},
		{
			name:            "section=content sets content section",
			only:            []string{"section=content"},
			maxLines:        1,
			wantSections:    []string{"content"},
			wantRawOnly:     []string{"section=content"},
			wantSectionsNil: false,
		},
		{
			name:            "section= errors with missing section name",
			only:            []string{"section="},
			maxLines:        1,
			wantErrContains: "missing section name",
		},
		{
			name:            "garbage only value errors",
			only:            []string{"garbage"},
			maxLines:        1,
			wantErrContains: "invalid --only value",
		},
		{
			name:            "empty only token is skipped",
			only:            []string{""},
			maxLines:        1,
			wantRawOnly:     []string{""},
			wantSectionsNil: true,
		},
		{
			name:            "changed plus section=core sets both filters",
			only:            []string{"changed", "section=core"},
			maxLines:        1,
			wantOnlyChanged: true,
			wantSections:    []string{"core"},
			wantRawOnly:     []string{"changed", "section=core"},
			wantSectionsNil: false,
		},
		{
			name:            "section=meta canonicalizes to metadata",
			only:            []string{"section=meta"},
			maxLines:        1,
			wantSections:    []string{"metadata"},
			wantRawOnly:     []string{"section=meta"},
			wantSectionsNil: false,
		},
		{
			name:            "section=tag canonicalizes to tags",
			only:            []string{"section=tag"},
			maxLines:        1,
			wantSections:    []string{"tags"},
			wantRawOnly:     []string{"section=tag"},
			wantSectionsNil: false,
		},
		{
			name:            "section=scope canonicalizes to scopes",
			only:            []string{"section=scope"},
			maxLines:        1,
			wantSections:    []string{"scopes"},
			wantRawOnly:     []string{"section=scope"},
			wantSectionsNil: false,
		},
		{
			name:            "changed is case insensitive",
			only:            []string{"CHANGED"},
			maxLines:        1,
			wantOnlyChanged: true,
			wantRawOnly:     []string{"CHANGED"},
			wantSectionsNil: true,
		},
		{
			name:            "section=content is case insensitive",
			only:            []string{"SECTION=CONTENT"},
			maxLines:        1,
			wantSections:    []string{"content"},
			wantRawOnly:     []string{"SECTION=CONTENT"},
			wantSectionsNil: false,
		},
		{
			name:            "changed token is trimmed",
			only:            []string{"  changed  "},
			maxLines:        1,
			wantOnlyChanged: true,
			wantRawOnly:     []string{"  changed  "},
			wantSectionsNil: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			opts, err := parseApprovalDiffViewOptions(tc.only, tc.maxLines)
			if tc.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.maxLines, opts.MaxLines)
			assert.Equal(t, tc.wantOnlyChanged, opts.OnlyChanged)
			assert.Equal(t, tc.wantRawOnly, opts.RawOnly)
			if tc.wantSectionsNil {
				assert.Nil(t, opts.Sections)
				return
			}

			require.NotNil(t, opts.Sections)
			assert.Len(t, opts.Sections, len(tc.wantSections))
			for _, section := range tc.wantSections {
				_, ok := opts.Sections[section]
				assert.True(t, ok, "expected section %q to be present", section)
			}
		})
	}
}

func TestApprovalDiffAnyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		value         any
		want          string
		wantMultiline bool
	}{
		{name: "nil becomes None", value: nil, want: "None"},
		{name: "empty string becomes None", value: "", want: "None"},
		{name: "whitespace string becomes None", value: " ", want: "None"},
		{name: "non-empty string is preserved", value: "hello", want: "hello"},
		{name: "int marshals as json scalar", value: int(42), want: "42"},
		{name: "float marshals as json scalar", value: float64(3.14), want: "3.14"},
		{name: "bool marshals as json scalar", value: true, want: "true"},
		{
			name:          "map marshals as pretty json",
			value:         map[string]any{"a": "b"},
			want:          "{\n  \"a\": \"b\"\n}",
			wantMultiline: true,
		},
		{
			name:          "slice marshals as pretty json array",
			value:         []any{1, 2},
			want:          "[\n  1,\n  2\n]",
			wantMultiline: true,
		},
		{name: "string null remains literal null", value: "null", want: "null"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := approvalDiffAnyValue(tc.value)
			assert.Equal(t, tc.want, got)
			if tc.wantMultiline {
				assert.True(t, strings.Contains(got, "\n"))
			} else {
				assert.False(t, strings.Contains(got, "\n"))
			}
		})
	}
}

func TestApprovalDiffFieldValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    any
		wantFrom string
		wantTo   string
	}{
		{
			name:     "map from/to values are extracted",
			value:    map[string]any{"from": "old", "to": "new"},
			wantFrom: "old",
			wantTo:   "new",
		},
		{
			name:     "nil from becomes None",
			value:    map[string]any{"from": nil, "to": "new"},
			wantFrom: "None",
			wantTo:   "new",
		},
		{
			name:     "empty map yields None placeholders",
			value:    map[string]any{},
			wantFrom: "None",
			wantTo:   "None",
		},
		{
			name:     "direct string becomes to value",
			value:    "direct value",
			wantFrom: "None",
			wantTo:   "direct value",
		},
		{
			name:     "nil becomes None placeholders",
			value:    nil,
			wantFrom: "None",
			wantTo:   "None",
		},
		{
			name:     "direct int becomes json scalar to value",
			value:    int(42),
			wantFrom: "None",
			wantTo:   "42",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotFrom, gotTo := approvalDiffFieldValues(tc.value)
			assert.Equal(t, tc.wantFrom, gotFrom)
			assert.Equal(t, tc.wantTo, gotTo)
		})
	}
}

func TestApprovalDiffRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		changes  map[string]any
		maxLines int
		wantNil  bool
		wantRows []components.DiffRow
	}{
		{
			name:     "nil changes returns nil",
			changes:  nil,
			maxLines: 1,
			wantNil:  true,
		},
		{
			name:     "empty changes returns nil",
			changes:  map[string]any{},
			maxLines: 1,
			wantNil:  true,
		},
		{
			name: "single field builds one row",
			changes: map[string]any{
				"status": map[string]any{"from": "active", "to": "archived"},
			},
			maxLines: 1,
			wantRows: []components.DiffRow{
				{Label: "status", From: "active", To: "archived"},
			},
		},
		{
			name: "multiple fields are sorted alphabetically",
			changes: map[string]any{
				"zeta":  map[string]any{"from": "z1", "to": "z2"},
				"alpha": map[string]any{"from": "a1", "to": "a2"},
				"mid":   map[string]any{"from": "m1", "to": "m2"},
			},
			maxLines: 1,
			wantRows: []components.DiffRow{
				{Label: "alpha", From: "a1", To: "a2"},
				{Label: "mid", From: "m1", To: "m2"},
				{Label: "zeta", From: "z1", To: "z2"},
			},
		},
		{
			name: "non-map value uses None for from",
			changes: map[string]any{
				"name": "test",
			},
			maxLines: 1,
			wantRows: []components.DiffRow{
				{Label: "name", From: "None", To: "test"},
			},
		},
		{
			name: "multi-line values are clamped to max lines",
			changes: map[string]any{
				"content": map[string]any{"from": "old", "to": "line1\nline2\nline3"},
			},
			maxLines: 1,
			wantRows: []components.DiffRow{
				{Label: "content", From: "old", To: "line1\n... (+2 more lines)"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rows := approvalDiffRows(tc.changes, tc.maxLines)
			if tc.wantNil {
				assert.Nil(t, rows)
				return
			}

			require.NotNil(t, rows)
			assert.Equal(t, tc.wantRows, rows)
		})
	}
}

func TestClampDiffValueLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		maxLines int
		want     string
	}{
		{name: "maxLines=0 returns unchanged", value: "line1\nline2", maxLines: 0, want: "line1\nline2"},
		{name: "maxLines=-1 returns unchanged", value: "line1\nline2", maxLines: -1, want: "line1\nline2"},
		{name: "single line under limit is unchanged", value: "single line", maxLines: 1, want: "single line"},
		{
			name:     "maxLines=1 clamps multiline",
			value:    "line1\nline2\nline3",
			maxLines: 1,
			want:     "line1\n... (+2 more lines)",
		},
		{
			name:     "maxLines=2 clamps multiline",
			value:    "line1\nline2\nline3",
			maxLines: 2,
			want:     "line1\nline2\n... (+1 more lines)",
		},
		{name: "under limit remains unchanged", value: "a\nb\nc", maxLines: 5, want: "a\nb\nc"},
		{name: "empty string remains empty", value: "", maxLines: 1, want: ""},
		{name: "no newlines remains unchanged", value: "no newlines at all", maxLines: 3, want: "no newlines at all"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := clampDiffValueLines(tc.value, tc.maxLines)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCanonicalDiffSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		section string
		want    string
	}{
		{name: "core stays core", section: "core", want: "core"},
		{name: "metadata stays metadata", section: "metadata", want: "metadata"},
		{name: "meta aliases to metadata", section: "meta", want: "metadata"},
		{name: "tags stays tags", section: "tags", want: "tags"},
		{name: "tag aliases to tags", section: "tag", want: "tags"},
		{name: "scopes stays scopes", section: "scopes", want: "scopes"},
		{name: "scope aliases to scopes", section: "scope", want: "scopes"},
		{name: "content stays content", section: "content", want: "content"},
		{name: "source stays source", section: "source", want: "source"},
		{name: "unknown becomes other", section: "unknown", want: "other"},
		{name: "empty becomes other", section: "", want: "other"},
		{name: "case insensitive core", section: "CORE", want: "core"},
		{name: "trimmed meta aliases to metadata", section: "  meta  ", want: "metadata"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, canonicalDiffSection(tc.section))
		})
	}
}

func TestApprovalDiffSectionForLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		label string
		want  string
	}{
		{name: "content matches content", label: "content", want: "content"},
		{name: "content is case insensitive", label: "Content", want: "content"},
		{name: "scopes matches scopes", label: "scopes", want: "scopes"},
		{name: "scope substring maps to scopes", label: "privacy_scope_ids", want: "scopes"},
		{name: "tags matches tags", label: "tags", want: "tags"},
		{name: "tag substring maps to tags", label: "entity_tags", want: "tags"},
		{name: "source type maps to source", label: "source type", want: "source"},
		{name: "source substring maps to source", label: "source_path", want: "source"},
		{name: "title maps to core", label: "title", want: "core"},
		{name: "name maps to core", label: "name", want: "core"},
		{name: "status maps to core", label: "status", want: "core"},
		{name: "type maps to core", label: "type", want: "core"},
		{name: "metadata substring maps to metadata", label: "metadata_json", want: "metadata"},
		{name: "unknown label maps to other", label: "random_field", want: "other"},
		{name: "empty label maps to other", label: "", want: "other"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, approvalDiffSectionForLabel(tc.label))
		})
	}
}

func TestNormalizeDiffCompareValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty becomes none", value: "", want: "none"},
		{name: "None becomes none", value: "None", want: "none"},
		{name: "null becomes none", value: "null", want: "none"},
		{name: "nil placeholder becomes none", value: "<nil>", want: "none"},
		{name: "dash becomes none", value: "-", want: "none"},
		{name: "double dash becomes none", value: "--", want: "none"},
		{name: "hello stays hello", value: "hello", want: "hello"},
		{name: "HELLO lowercases", value: "HELLO", want: "hello"},
		{name: "spaced value trims", value: "  spaced  ", want: "spaced"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, normalizeDiffCompareValue(tc.value))
		})
	}
}

func TestApprovalDiffRowChanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		row  components.DiffRow
		want bool
	}{
		{name: "different values are changed", row: components.DiffRow{From: "a", To: "b"}, want: true},
		{name: "same values are unchanged", row: components.DiffRow{From: "same", To: "same"}, want: false},
		{name: "None and empty normalize to unchanged", row: components.DiffRow{From: "None", To: ""}, want: false},
		{name: "null and nil placeholder normalize to unchanged", row: components.DiffRow{From: "null", To: "<nil>"}, want: false},
		{name: "value to None is changed", row: components.DiffRow{From: "value", To: "None"}, want: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, approvalDiffRowChanged(tc.row))
		})
	}
}

func TestApplyApprovalDiffFilters(t *testing.T) {
	t.Parallel()

	mixedRows := []components.DiffRow{
		{Label: "status", From: "active", To: "archived"},
		{Label: "title", From: "same", To: "same"},
		{Label: "metadata_json", From: "old", To: "new"},
		{Label: "tags", From: "None", To: ""},
		{Label: "privacy_scope_ids", From: "scope-a", To: "scope-b"},
	}

	tests := []struct {
		name    string
		rows    []components.DiffRow
		opts    approvalDiffViewOptions
		wantNil bool
		want    []components.DiffRow
	}{
		{
			name:    "nil rows returns nil",
			rows:    nil,
			opts:    approvalDiffViewOptions{},
			wantNil: true,
		},
		{
			name:    "empty rows returns nil",
			rows:    []components.DiffRow{},
			opts:    approvalDiffViewOptions{},
			wantNil: true,
		},
		{
			name: "OnlyChanged keeps only changed rows",
			rows: mixedRows,
			opts: approvalDiffViewOptions{OnlyChanged: true},
			want: []components.DiffRow{
				{Label: "status", From: "active", To: "archived"},
				{Label: "metadata_json", From: "old", To: "new"},
				{Label: "privacy_scope_ids", From: "scope-a", To: "scope-b"},
			},
		},
		{
			name: "Sections filter keeps only core rows",
			rows: mixedRows,
			opts: approvalDiffViewOptions{Sections: map[string]struct{}{"core": {}}},
			want: []components.DiffRow{
				{Label: "status", From: "active", To: "archived"},
				{Label: "title", From: "same", To: "same"},
			},
		},
		{
			name: "OnlyChanged and Sections intersect",
			rows: mixedRows,
			opts: approvalDiffViewOptions{
				OnlyChanged: true,
				Sections:    map[string]struct{}{"core": {}},
			},
			want: []components.DiffRow{
				{Label: "status", From: "active", To: "archived"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := applyApprovalDiffFilters(tc.rows, tc.opts)
			if tc.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tc.want, got)
		})
	}
}

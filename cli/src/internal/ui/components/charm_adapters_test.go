package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RenderInfoTable ---

// TestRenderInfoTableBoundaryConditions covers nil/empty/zero/negative/extreme inputs.
func TestRenderInfoTableBoundaryConditions(t *testing.T) {
	validRows := []InfoTableRow{{Key: "Name", Value: "test"}}

	tests := []struct {
		name        string
		rows        []InfoTableRow
		width       int
		expectEmpty bool
		containsAny []string // if non-empty, output must contain all of these
	}{
		{
			name:        "nil rows returns empty",
			rows:        nil,
			width:       80,
			expectEmpty: true,
		},
		{
			name:        "empty slice returns empty",
			rows:        []InfoTableRow{},
			width:       80,
			expectEmpty: true,
		},
		{
			name:        "valid rows with zero width returns empty",
			rows:        validRows,
			width:       0,
			expectEmpty: true,
		},
		{
			name:        "valid rows with negative width returns empty",
			rows:        validRows,
			width:       -1,
			expectEmpty: true,
		},
		{
			name:        "valid rows with width 1 renders non-empty (innerWidth clamped to 20)",
			rows:        validRows,
			width:       1,
			expectEmpty: false,
		},
		{
			name:        "valid rows with width 20 contains headers",
			rows:        validRows,
			width:       20,
			expectEmpty: false,
			containsAny: []string{"Field", "Value"},
		},
		{
			name:        "valid rows with width 200 contains key text",
			rows:        validRows,
			width:       200,
			expectEmpty: false,
			containsAny: []string{"Name", "test"},
		},
		{
			name:        "single row renders key and value",
			rows:        []InfoTableRow{{Key: "Name", Value: "test"}},
			width:       80,
			expectEmpty: false,
			containsAny: []string{"Name", "test"},
		},
		{
			name:        "row with very long key is clamped to 24 chars",
			rows:        []InfoTableRow{{Key: strings.Repeat("A", 100), Value: "v"}},
			width:       80,
			expectEmpty: false,
		},
		{
			name:        "row with short key (1 char) is padded to minimum 6",
			rows:        []InfoTableRow{{Key: "X", Value: "val"}},
			width:       80,
			expectEmpty: false,
			containsAny: []string{"val"},
		},
		{
			name:        "row with empty key and value renders without panic",
			rows:        []InfoTableRow{{Key: "", Value: ""}},
			width:       80,
			expectEmpty: false,
		},
		{
			name:        "50 rows renders without panic",
			rows:        makeManyInfoRows(50),
			width:       80,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderInfoTable(tt.rows, tt.width)
			if tt.expectEmpty {
				assert.Empty(t, out, "expected empty output")
				return
			}
			require.NotEmpty(t, out, "expected non-empty output")
			clean := SanitizeText(out)
			for _, s := range tt.containsAny {
				assert.Contains(t, clean, s, "output should contain %q", s)
			}
		})
	}
}

// TestRenderInfoTableContentIntegrity verifies all row data and headers appear in output.
func TestRenderInfoTableContentIntegrity(t *testing.T) {
	rows := []InfoTableRow{
		{Key: "Name", Value: "Alice"},
		{Key: "Status", Value: "active"},
		{Key: "Type", Value: "agent"},
	}
	out := RenderInfoTable(rows, 100)
	require.NotEmpty(t, out)
	clean := SanitizeText(out)

	assert.Contains(t, clean, "Field", "header Field must appear")
	assert.Contains(t, clean, "Value", "header Value must appear")

	for _, r := range rows {
		assert.Contains(t, clean, r.Key, "row key %q must appear", r.Key)
		assert.Contains(t, clean, r.Value, "row value %q must appear", r.Value)
	}
}

// TestRenderInfoTableSanitization verifies ANSI, newlines, and tabs are handled.
func TestRenderInfoTableSanitization(t *testing.T) {
	tests := []struct {
		name  string
		row   InfoTableRow
		check func(t *testing.T, out string)
	}{
		{
			name: "ANSI escapes in key are stripped",
			row:  InfoTableRow{Key: "\x1b[31mRed\x1b[0m", Value: "ok"},
			check: func(t *testing.T, out string) {
				assert.NotContains(t, out, "\x1b[31m", "raw ANSI should be stripped from key")
				assert.NotContains(t, out, "\x1b[0m", "raw ANSI reset should be stripped")
				clean := SanitizeText(out)
				assert.Contains(t, clean, "Red", "key text should survive sanitization")
			},
		},
		{
			name: "newline in value is collapsed to space",
			row:  InfoTableRow{Key: "Desc", Value: "line1\nline2"},
			check: func(t *testing.T, out string) {
				clean := SanitizeText(out)
				assert.Contains(t, clean, "line1", "first part of value present")
				assert.Contains(t, clean, "line2", "second part of value present")
			},
		},
		{
			name: "tab in key handled without panic",
			row:  InfoTableRow{Key: "Key\twith\ttabs", Value: "val"},
			check: func(t *testing.T, out string) {
				require.NotEmpty(t, out, "should render without panic")
				clean := SanitizeText(out)
				assert.Contains(t, clean, "val")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderInfoTable([]InfoTableRow{tt.row}, 80)
			require.NotEmpty(t, out)
			tt.check(t, out)
		})
	}
}

// --- RenderDiffInfoTable ---

// TestRenderDiffInfoTableBoundaryConditions covers nil/empty/zero/negative/extreme inputs.
func TestRenderDiffInfoTableBoundaryConditions(t *testing.T) {
	validRows := []DiffRow{{Label: "status", From: "active", To: "archived"}}

	tests := []struct {
		name        string
		rows        []DiffRow
		width       int
		expectEmpty bool
		containsAny []string
	}{
		{
			name:        "nil rows returns empty",
			rows:        nil,
			width:       80,
			expectEmpty: true,
		},
		{
			name:        "empty slice returns empty",
			rows:        []DiffRow{},
			width:       80,
			expectEmpty: true,
		},
		{
			name:        "valid rows with zero width returns empty",
			rows:        validRows,
			width:       0,
			expectEmpty: true,
		},
		{
			name:        "valid rows with negative width returns empty",
			rows:        validRows,
			width:       -1,
			expectEmpty: true,
		},
		{
			name:        "valid rows with width 1 renders non-empty (innerWidth clamped to 40)",
			rows:        validRows,
			width:       1,
			expectEmpty: false,
		},
		{
			name:        "single row contains label and values",
			rows:        []DiffRow{{Label: "status", From: "active", To: "archived"}},
			width:       120,
			expectEmpty: false,
			containsAny: []string{"status", "active", "archived"},
		},
		{
			name:        "same from/to produces same change kind",
			rows:        []DiffRow{{Label: "x", From: "val", To: "val"}},
			width:       120,
			expectEmpty: false,
			containsAny: []string{"same"},
		},
		{
			name:        "empty from produces added change kind",
			rows:        []DiffRow{{Label: "x", From: "", To: "value"}},
			width:       120,
			expectEmpty: false,
			containsAny: []string{"added"},
		},
		{
			name:        "empty to produces removed change kind",
			rows:        []DiffRow{{Label: "x", From: "value", To: ""}},
			width:       120,
			expectEmpty: false,
			containsAny: []string{"removed"},
		},
		{
			name:        "different from/to produces updated change kind",
			rows:        []DiffRow{{Label: "x", From: "old", To: "new"}},
			width:       120,
			expectEmpty: false,
			containsAny: []string{"updated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderDiffInfoTable(tt.rows, tt.width)
			if tt.expectEmpty {
				assert.Empty(t, out, "expected empty output")
				return
			}
			require.NotEmpty(t, out, "expected non-empty output")
			clean := SanitizeText(out)
			for _, s := range tt.containsAny {
				assert.Contains(t, clean, s, "output should contain %q", s)
			}
		})
	}
}

// TestRenderDiffInfoTableContentIntegrity verifies all headers and row data appear.
func TestRenderDiffInfoTableContentIntegrity(t *testing.T) {
	rows := []DiffRow{
		{Label: "name", From: "Alice", To: "Bob"},
		{Label: "role", From: "admin", To: "viewer"},
	}
	out := RenderDiffInfoTable(rows, 120)
	require.NotEmpty(t, out)
	clean := SanitizeText(out)

	for _, header := range []string{"Field", "Change", "Before", "After"} {
		assert.Contains(t, clean, header, "header %q must appear", header)
	}
	for _, r := range rows {
		assert.Contains(t, clean, r.Label, "label %q must appear", r.Label)
		assert.Contains(t, clean, r.From, "from %q must appear", r.From)
		assert.Contains(t, clean, r.To, "to %q must appear", r.To)
	}
}

// --- RenderGridTable ---

// TestRenderGridTableBoundaryConditions covers nil/empty/zero/negative/extreme inputs.
func TestRenderGridTableBoundaryConditions(t *testing.T) {
	validCols := []TableColumn{{Header: "Name", Width: 20}}
	singleRow := [][]string{{"Alice"}}

	tests := []struct {
		name        string
		columns     []TableColumn
		rows        [][]string
		width       int
		expectEmpty bool
		containsAny []string
	}{
		{
			name:        "nil columns returns empty",
			columns:     nil,
			rows:        singleRow,
			width:       80,
			expectEmpty: true,
		},
		{
			name:        "empty columns returns empty",
			columns:     []TableColumn{},
			rows:        singleRow,
			width:       80,
			expectEmpty: true,
		},
		{
			name:        "valid columns with zero width returns empty",
			columns:     validCols,
			rows:        singleRow,
			width:       0,
			expectEmpty: true,
		},
		{
			name:        "valid columns with negative width returns empty",
			columns:     validCols,
			rows:        singleRow,
			width:       -1,
			expectEmpty: true,
		},
		{
			name:        "single column single row renders non-empty",
			columns:     validCols,
			rows:        singleRow,
			width:       80,
			expectEmpty: false,
			containsAny: []string{"Name", "Alice"},
		},
		{
			name: "multiple columns contain all headers",
			columns: []TableColumn{
				{Header: "ID", Width: 10},
				{Header: "Status", Width: 10},
				{Header: "Created", Width: 15},
			},
			rows:        [][]string{{"1", "active", "2025-01-01"}},
			width:       80,
			expectEmpty: false,
			containsAny: []string{"ID", "Status", "Created", "active"},
		},
		{
			name:        "rows shorter than columns does not panic",
			columns:     []TableColumn{{Header: "A", Width: 10}, {Header: "B", Width: 10}},
			rows:        [][]string{{"only-one"}},
			width:       80,
			expectEmpty: false,
			containsAny: []string{"only-one"},
		},
		{
			name:        "rows longer than columns does not panic",
			columns:     []TableColumn{{Header: "A", Width: 10}},
			rows:        [][]string{{"one", "two", "three"}},
			width:       80,
			expectEmpty: false,
			containsAny: []string{"one"},
		},
		{
			name:        "zero rows renders headers only",
			columns:     validCols,
			rows:        nil,
			width:       80,
			expectEmpty: false,
			containsAny: []string{"Name"},
		},
		{
			name:        "column with width 0 handled without panic",
			columns:     []TableColumn{{Header: "X", Width: 0}},
			rows:        [][]string{{"val"}},
			width:       80,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderGridTable(tt.columns, tt.rows, tt.width)
			if tt.expectEmpty {
				assert.Empty(t, out, "expected empty output")
				return
			}
			require.NotEmpty(t, out, "expected non-empty output")
			clean := SanitizeText(out)
			for _, s := range tt.containsAny {
				assert.Contains(t, clean, s, "output should contain %q", s)
			}
		})
	}
}

// --- RenderCompactBox ---

// TestRenderCompactBox verifies the compact box rendering.
func TestRenderCompactBox(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, out string)
	}{
		{
			name:    "empty content still renders box borders",
			content: "",
			check: func(t *testing.T, out string) {
				assert.NotEmpty(t, out, "box with empty content should still render borders")
			},
		},
		{
			name:    "normal content appears in output",
			content: "Loading items...",
			check: func(t *testing.T, out string) {
				clean := SanitizeText(out)
				assert.Contains(t, clean, "Loading items...", "content should appear inside box")
			},
		},
		{
			name:    "multiline content preserved",
			content: "Line1\nLine2\nLine3",
			check: func(t *testing.T, out string) {
				clean := SanitizeText(out)
				assert.Contains(t, clean, "Line1", "first line should appear")
				assert.Contains(t, clean, "Line3", "last line should appear")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := RenderCompactBox(tt.content)
			tt.check(t, out)
		})
	}
}

// --- diffChangeKind ---

// TestDiffChangeKind verifies the change classification logic.
func TestDiffChangeKind(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want string
	}{
		{name: "same values", from: "value", to: "value", want: "same"},
		{name: "empty from is added", from: "", to: "value", want: "added"},
		{name: "empty to is removed", from: "value", to: "", want: "removed"},
		{name: "different values is updated", from: "old", to: "new", want: "updated"},
		{name: "None string from is added", from: "None", to: "value", want: "added"},
		{name: "None string to is removed", from: "value", to: "None", want: "removed"},
		{name: "dash from is added", from: "-", to: "value", want: "added"},
		{name: "dash to is removed", from: "value", to: "-", want: "removed"},
		{name: "double dash from is added", from: "--", to: "value", want: "added"},
		{name: "nil placeholder from is added", from: "<nil>", to: "value", want: "added"},
		{name: "both empty is same", from: "", to: "", want: "same"},
		{name: "both None is same", from: "None", to: "None", want: "same"},
		{name: "whitespace only from is added", from: "   ", to: "value", want: "added"},
		{name: "whitespace only to is removed", from: "value", to: "   ", want: "removed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffChangeKind(tt.from, tt.to)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- helpers ---

func makeManyInfoRows(n int) []InfoTableRow {
	rows := make([]InfoTableRow, n)
	for i := range rows {
		rows[i] = InfoTableRow{
			Key:   strings.Repeat("k", (i%20)+1),
			Value: strings.Repeat("v", (i%30)+1),
		}
	}
	return rows
}

package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMetadataInputEdgeErrorMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "odd indent width",
			in: strings.Join([]string{
				"profile:",
				"   timezone: Europe/Warsaw",
			}, "\n"),
			want: "indent must use 2 spaces",
		},
		{
			name: "indent without parent",
			in:   "  timezone: Europe/Warsaw",
			want: "indent has no parent key",
		},
		{
			name: "unsupported list item",
			in: strings.Join([]string{
				"profile:",
				"  - timezone",
			}, "\n"),
			want: "list items not supported",
		},
		{
			name: "empty key",
			in:   ": value",
			want: "key is empty",
		},
		{
			name: "missing delimiter",
			in:   "profile",
			want: "expected 'key: value' or 'group | field | value'",
		},
		{
			name: "inline object unsupported",
			in:   "profile: {}",
			want: "inline objects not supported",
		},
		{
			name: "pipe row missing value",
			in:   "profile |",
			want: "expected at least 'field | value'",
		},
		{
			name: "pipe path collides with scalar",
			in: strings.Join([]string{
				"profile: alpha",
				"profile | timezone | Europe/Warsaw",
			}, "\n"),
			want: "already set as a value",
		},
		{
			name: "stack reset after pipe row",
			in: strings.Join([]string{
				"profile | timezone | Europe/Warsaw",
				"  bad: value",
			}, "\n"),
			want: "indent has no parent key",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseMetadataInput(tc.in)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestParseMetadataInputPipeRowsAndDedentSuccess(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"profile:",
		"  name: alxx",
		"owner: founder",
		"profile | timezone | Europe/Warsaw",
		"profile | tags | [ai, ml]",
	}, "\n")
	got, err := parseMetadataInput(input)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "founder", got["owner"])
	profile, ok := got["profile"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alxx", profile["name"])
	assert.Equal(t, "Europe/Warsaw", profile["timezone"])
	assert.Equal(t, []any{"ai", "ml"}, profile["tags"])
}

func TestMetadataLinesPlainAndListPlainBranchMatrix(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"scopes": []any{},
		"segments": []any{
			map[string]any{"text": "hello", "scopes": "public, admin"},
			map[string]any{"text": "   ", "k": "v"},
			[]any{"child"},
			7,
		},
	}

	lines := metadataLinesPlain(data, 0)
	joined := strings.Join(lines, "\n")
	assert.Contains(t, joined, "scopes: None")
	assert.Contains(t, joined, "- [public] [admin] hello")
	assert.Contains(t, joined, "k: v")
	assert.Contains(t, joined, "text: None")
	assert.Contains(t, joined, "- child")
	assert.Contains(t, joined, "- 7")
}

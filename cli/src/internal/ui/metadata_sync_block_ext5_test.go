package ui

import (
	"fmt"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncMetadataListBranchMatrix(t *testing.T) {
	rows := []metadataDisplayRow{
		{field: "k1", value: "v1"},
		{field: "k2", value: "v2"},
		{field: "k3", value: "v3"},
		{field: "k4", value: "v4"},
		{field: "k5", value: "v5"},
	}

	// nil list branch: should be a no-op.
	syncMetadataList(nil, rows, 0)

	list := components.NewList(3)
	list.Cursor = -4
	list.Offset = -2
	syncMetadataList(list, rows, 0)
	assert.Equal(t, 1, list.PageSize)
	assert.Equal(t, 0, list.Cursor)
	assert.Equal(t, 0, list.Offset)
	assert.Equal(t, []string{"k1", "k2", "k3", "k4", "k5"}, list.Items)

	list = components.NewList(3)
	list.Cursor = 99
	list.Offset = 99
	syncMetadataList(list, rows, 3)
	assert.Equal(t, 4, list.Cursor)
	assert.Equal(t, 2, list.Offset)

	list = components.NewList(3)
	list.Cursor = 1
	list.Offset = 3
	syncMetadataList(list, rows, 3)
	assert.Equal(t, 1, list.Cursor)
	assert.Equal(t, 1, list.Offset)

	empty := components.NewList(4)
	empty.Cursor = 7
	empty.Offset = 3
	syncMetadataList(empty, nil, 4)
	assert.Equal(t, 0, empty.Cursor)
	assert.Equal(t, 0, empty.Offset)
	assert.Empty(t, empty.Items)
}

func TestRenderMetadataBlockWithTitleClampsAndOverflow(t *testing.T) {
	data := map[string]any{}
	for i := 0; i < 30; i++ {
		data[fmt.Sprintf("k%02d", i)] = fmt.Sprintf("v%02d", i)
	}

	compactNarrow := components.SanitizeText(renderMetadataBlockWithTitle("Metadata", data, 12, false))
	// At very narrow widths, headers wrap across lines.
	assert.NotEmpty(t, compactNarrow)
	assert.Contains(t, compactNarrow, "Grou")

	compact := components.SanitizeText(renderMetadataBlockWithTitle("Metadata", data, 120, false))
	assert.Contains(t, compact, "+18 more rows (press m to expand)")
	assert.Contains(t, compact, "Group")
	assert.Contains(t, compact, "Field")
	assert.Contains(t, compact, "Value")

	expanded := components.SanitizeText(renderMetadataBlockWithTitle("Metadata", data, 120, true))
	assert.Contains(t, expanded, "+4 more rows (press m to expand)")
}

func TestSetMetadataPathBranchMatrix(t *testing.T) {
	root := map[string]any{}

	require.NoError(t, setMetadataPath(root, "profile.name", "alxx", 1))
	require.NoError(t, setMetadataPath(root, "profile.age", "18", 2))
	require.NoError(t, setMetadataPath(root, "profile.timezone", "Europe/Warsaw", 2))
	require.NoError(t, setMetadataPath(root, "status", "active", 3))
	assert.Equal(
		t,
		map[string]any{"name": "alxx", "age": "18", "timezone": "Europe/Warsaw"},
		root["profile"],
	)
	assert.Equal(t, "active", root["status"])

	err := setMetadataPath(root, "profile.name.first", "x", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already set as a value")

	err = setMetadataPath(root, "profile..timezone", "Europe/Warsaw", 4)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty key segment")

	err = setMetadataPath(root, "   ", "value", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty key segment")
}

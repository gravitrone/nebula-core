package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestRenderMetadataSelectableBlockReturnsNoneForEmptyRows(t *testing.T) {
	out := renderMetadataSelectableBlockWithTitle("Metadata", nil, 80, nil, nil, false)
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "None")
}

func TestRenderMetadataSelectableBlockUsesFallbackListWhenNil(t *testing.T) {
	rows := []metadataDisplayRow{{field: "owner", value: "alxx"}}
	out := renderMetadataSelectableBlockWithTitle("Metadata", rows, 80, nil, nil, false)
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "owner")
	assert.Contains(t, clean, "mode row")
	assert.Contains(t, clean, "metadata select mode")
}

func TestRenderMetadataSelectableBlockReturnsNoneWhenVisibleIsEmpty(t *testing.T) {
	rows := []metadataDisplayRow{{field: "owner", value: "alxx"}}
	list := components.NewList(0)
	list.Items = []string{"owner"}

	out := renderMetadataSelectableBlockWithTitle("Metadata", rows, 80, list, nil, false)
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "None")
}

func TestRenderMetadataSelectableBlockSkipsOutOfRangeVisibleRows(t *testing.T) {
	rows := []metadataDisplayRow{{field: "owner", value: "alxx"}}
	list := components.NewList(3)
	list.Items = []string{"owner", "orphan-visible-item"}
	list.Cursor = 0
	list.Offset = 0

	out := renderMetadataSelectableBlockWithTitle(
		"Metadata",
		rows,
		80,
		list,
		map[int]bool{
			-1: true,  // invalid, ignored
			0:  true,  // valid
			1:  true,  // invalid for rows slice, ignored
			2:  false, // exercises !v continue branch
		},
		true,
	)
	clean := components.SanitizeText(out)

	assert.Contains(t, clean, "selected 1")
	assert.Contains(t, clean, "copy")
	assert.NotContains(t, clean, "orphan-visible-item")
}

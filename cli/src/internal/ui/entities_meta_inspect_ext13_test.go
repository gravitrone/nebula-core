package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesMetaInspectPageSizeBounds(t *testing.T) {
	model := NewEntitiesModel(nil)

	model.height = 0
	assert.Equal(t, 10, model.metaInspectPageSize())

	model.height = 5
	assert.Equal(t, 6, model.metaInspectPageSize())

	model.height = 1000
	assert.Equal(t, 18, model.metaInspectPageSize())
}

func TestEntitiesMoveMetaInspectNoopAndShortContentClamp(t *testing.T) {
	model := NewEntitiesModel(nil)

	model.metaInspectO = 4
	model.moveMetaInspect(10)
	assert.Equal(t, 4, model.metaInspectO)

	model.metaInspect = true
	model.metaInspectI = 0
	model.metaRows = []metadataDisplayRow{{field: "profile.note", value: "single line"}}
	model.height = 120
	model.moveMetaInspect(10)
	assert.Equal(t, 0, model.metaInspectO)

	model.moveMetaInspect(-10)
	assert.Equal(t, 0, model.metaInspectO)
}

func TestEntitiesRenderMetaInspectScrollIndicatorsAndBounds(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.width = 90
	model.height = 30
	model.metaInspect = true
	model.metaInspectI = 0
	model.metaRows = []metadataDisplayRow{
		{
			field: "profile.note",
			value: strings.Join([]string{
				"line 01", "line 02", "line 03", "line 04", "line 05",
				"line 06", "line 07", "line 08", "line 09", "line 10",
				"line 11", "line 12", "line 13", "line 14", "line 15",
			}, "\n"),
		},
	}

	model.metaInspectO = 2
	out := components.SanitizeText(model.renderMetaInspect())
	assert.Contains(t, out, "... ↑ more")
	assert.Contains(t, out, "... ↓ more")

	model.metaInspectO = -7
	out = components.SanitizeText(model.renderMetaInspect())
	assert.Contains(t, out, "Lines 1-")

	model.metaInspectO = 999
	out = components.SanitizeText(model.renderMetaInspect())
	lines := model.metaInspectLines()
	require.NotEmpty(t, lines)
	assert.Contains(t, out, fmt.Sprintf("of %d", len(lines)))
	assert.Contains(t, out, "scroll")
}

func TestCompactJSONMarshalErrorBranch(t *testing.T) {
	assert.Equal(t, "", compactJSON(map[string]any{
		"bad": func() {},
	}))
}


package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderPaletteStateAndFallbackBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 18

	app.paletteInput.SetValue("/")
	app.paletteFiltered = nil
	app.paletteSearchLoading = false
	out := components.SanitizeText(app.renderPalette())
	assert.Contains(t, out, "Command")
	assert.Contains(t, out, "matching")

	app.paletteInput.SetValue("")
	out = components.SanitizeText(app.renderPalette())
	assert.Contains(t, out, "Search")
	assert.Contains(t, out, "Type")
	assert.Contains(t, out, "search")

	app.paletteInput.SetValue("abc")
	out = components.SanitizeText(app.renderPalette())
	assert.Contains(t, out, "search")
	assert.Contains(t, out, "results")

	app.paletteSearchLoading = true
	out = components.SanitizeText(app.renderPalette())
	assert.Contains(t, out, "Searching")
}

func TestRenderPaletteTableBranchesOnSmallWidth(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.width = 22
	app.paletteInput.SetValue("plain")
	app.paletteFiltered = []paletteAction{
		{ID: "x", Label: "", Desc: ""},
	}

	out := components.SanitizeText(app.renderPalette())
	assert.Contains(t, out, "Action")
	assert.Contains(t, out, "Description")
	assert.Contains(t, out, "-")
}

func TestRunStartupCheckCmdWithoutClientUsesDefaultPath(t *testing.T) {
	app := NewApp(nil, nil)
	app.client = nil
	app.config = nil

	cmd := app.runStartupCheckCmd()
	require.NotNil(t, cmd)

	msg, ok := cmd().(startupCheckedMsg)
	require.True(t, ok)
	assert.NotNil(t, msg)
}

func TestRunStartupCheckCmdWithoutClientUsesConfiguredKeyPath(t *testing.T) {
	app := NewApp(nil, &config.Config{APIKey: "branch-key"})
	app.client = nil

	cmd := app.runStartupCheckCmd()
	require.NotNil(t, cmd)

	msg, ok := cmd().(startupCheckedMsg)
	require.True(t, ok)
	assert.NotNil(t, msg)
}

func TestHandlePaletteKeysEdgeBranches(t *testing.T) {
	app := NewApp(nil, &config.Config{})
	app.paletteOpen = true
	app.paletteFiltered = nil
	app.paletteIndex = 0

	model, cmd := app.handlePaletteKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(App)
	assert.True(t, updated.paletteOpen)
	assert.Nil(t, cmd)

	model, cmd = updated.handlePaletteKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated = model.(App)
	assert.False(t, updated.paletteOpen)
	assert.Nil(t, cmd)

	updated.paletteOpen = true
	updated.paletteInput.SetValue("/x")
	model, cmd = updated.handlePaletteKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
	updated = model.(App)
	assert.Equal(t, "/", updated.paletteInput.Value())
	assert.Nil(t, cmd)
}

func TestBuildSearchPaletteActionsLabelFallbackBranch(t *testing.T) {
	actions, selections := buildSearchPaletteActions(
		"",
		[]api.Entity{{ID: "", Name: "", Type: ""}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	require.Len(t, actions, 1)
	assert.Equal(t, "entity:", actions[0].ID)
	assert.Equal(t, "entity", actions[0].Label)
	assert.Equal(t, "entity", actions[0].Desc)
	selection, ok := selections["entity:"]
	require.True(t, ok)
	require.NotNil(t, selection.entity)
}

func TestBuildSearchPaletteActionsUsesShortIDWhenLabelMissing(t *testing.T) {
	actions, _ := buildSearchPaletteActions(
		"",
		[]api.Entity{{ID: "entity-123456789", Name: "", Type: ""}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	require.Len(t, actions, 1)
	assert.Equal(t, "entity-1", actions[0].Label)
	assert.Equal(t, "entity · entity-1", actions[0].Desc)
}

func TestBuildSearchPaletteActionsDescFallbackBranch(t *testing.T) {
	actions, _ := buildSearchPaletteActions(
		"",
		[]api.Entity{{ID: "", Name: "Alpha", Type: "·"}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	require.Len(t, actions, 1)
	assert.Equal(t, "Alpha", actions[0].Label)
	assert.Equal(t, "entity", actions[0].Desc)
}

func TestBuildSearchPaletteActionsRelationshipLabelFallbackBranch(t *testing.T) {
	actions, _ := buildSearchPaletteActions(
		"",
		nil,
		nil,
		nil,
		[]api.Relationship{{
			ID:         "rel-1",
			Type:       "",
			Status:     "active",
			SourceName: "",
			TargetName: "",
			SourceID:   "ent-a",
			TargetID:   "ent-b",
		}},
		nil,
		nil,
		nil,
	)

	require.Len(t, actions, 1)
	assert.NotContains(t, actions[0].Label, "(")
	assert.Contains(t, actions[0].Label, "->")
}

func TestCenterBlockUniformEarlyReturnBranches(t *testing.T) {
	assert.Equal(t, "", centerBlockUniform("", 80))
	assert.Equal(t, "abcd", centerBlockUniform("abcd", 3))
	assert.Equal(t, "abcd", centerBlockUniform("abcd", 5))
}

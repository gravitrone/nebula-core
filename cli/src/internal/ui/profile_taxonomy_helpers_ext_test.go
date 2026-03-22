package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxonomyKindPathBounds(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})

	model.taxKind = -1
	assert.Equal(t, taxonomyKinds[0].Path, model.taxonomyKindPath())

	model.taxKind = len(taxonomyKinds)
	assert.Equal(t, taxonomyKinds[0].Path, model.taxonomyKindPath())

	model.taxKind = 2
	assert.Equal(t, taxonomyKinds[2].Path, model.taxonomyKindPath())
}

func TestTaxonomySetItemsAndSelectedBounds(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	now := time.Now()
	model.setTaxonomyItems([]api.TaxonomyEntry{{ID: "scope-1", Name: "public", CreatedAt: now, UpdatedAt: now}})
	require.Len(t, model.taxItems, 1)

	selected := model.selectedTaxonomy()
	require.NotNil(t, selected)
	assert.Equal(t, "scope-1", selected.ID)

	// With table.Model, SetCursor is clamped; clear rows to get cursor = -1.
	model.taxList.SetRows(nil)
	assert.Nil(t, model.selectedTaxonomy())
}

func TestTaxonomyPromptTitleModeMatrix(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})

	model.taxPromptMode = taxPromptCreateName
	assert.Equal(t, "New Taxonomy Name", model.taxonomyPromptTitle())

	model.taxPromptMode = taxPromptCreateDescription
	assert.Equal(t, "New Taxonomy Description (optional)", model.taxonomyPromptTitle())

	model.taxPromptMode = taxPromptEditName
	assert.Equal(t, "Edit Taxonomy Name", model.taxonomyPromptTitle())

	model.taxPromptMode = taxPromptEditDescription
	assert.Equal(t, "Edit Taxonomy Description (optional)", model.taxonomyPromptTitle())

	model.taxPromptMode = taxPromptNone
	assert.Equal(t, "Taxonomy", model.taxonomyPromptTitle())
}

func TestHandleTaxonomyPromptBackspaceRuneAndBack(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	model.taxPromptMode = taxPromptCreateName
	model.taxPromptBuf = "alxx"
	model.taxPendingName = "pending"
	model.taxPendingDesc = "desc"
	model.taxEditID = "scope-1"

	updated, cmd := model.handleTaxonomyPrompt(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Nil(t, cmd)
	assert.Equal(t, "alx", updated.taxPromptBuf)

	updated, _ = updated.handleTaxonomyPrompt(tea.KeyPressMsg{Code: tea.KeySpace})
	assert.Equal(t, "alx ", updated.taxPromptBuf)

	updated, _ = updated.handleTaxonomyPrompt(tea.KeyPressMsg{Code: 'z', Text: "z"})
	assert.Equal(t, "alx z", updated.taxPromptBuf)

	updated, _ = updated.handleTaxonomyPrompt(tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, taxPromptNone, updated.taxPromptMode)
	assert.Equal(t, "", updated.taxPromptBuf)
	assert.Equal(t, "", updated.taxPendingName)
	assert.Equal(t, "", updated.taxPendingDesc)
	assert.Equal(t, "", updated.taxEditID)
}

func TestSubmitTaxonomyPromptValidationAndTransitions(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})

	model.taxPromptMode = taxPromptCreateName
	model.taxPromptBuf = "   "
	updated, cmd := model.submitTaxonomyPrompt()
	require.NotNil(t, cmd)
	msg := cmd().(errMsg) //nolint:forcetypeassert
	assert.Contains(t, msg.err.Error(), "taxonomy name required")
	assert.Equal(t, taxPromptNone, updated.taxPromptMode)

	model.taxPromptMode = taxPromptCreateName
	model.taxPromptBuf = "  scopes-custom  "
	updated, cmd = model.submitTaxonomyPrompt()
	require.Nil(t, cmd)
	assert.Equal(t, taxPromptCreateDescription, updated.taxPromptMode)
	assert.Equal(t, "scopes-custom", updated.taxPendingName)

	model.taxPromptMode = taxPromptEditName
	model.taxPromptBuf = "  renamed  "
	model.taxPendingDesc = "desc"
	updated, cmd = model.submitTaxonomyPrompt()
	require.Nil(t, cmd)
	assert.Equal(t, taxPromptEditDescription, updated.taxPromptMode)
	assert.Equal(t, "renamed", updated.taxPendingName)
	assert.Equal(t, "desc", updated.taxPromptBuf)
}

func TestSubmitTaxonomyPromptFilterAndDefault(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	model.taxPromptMode = taxPromptFilter
	model.taxPromptBuf = "  private only "

	updated, cmd := model.submitTaxonomyPrompt()
	require.NotNil(t, cmd)
	assert.Equal(t, taxPromptNone, updated.taxPromptMode)
	assert.Equal(t, "private only", updated.taxSearch)
	assert.True(t, updated.taxLoading)

	updated.taxPromptMode = taxPromptNone
	updated, cmd = updated.submitTaxonomyPrompt()
	assert.Nil(t, cmd)
}

func TestTaxonomyArchiveActivateSelectedNoSelection(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})

	updated, cmd := model.taxonomyArchiveSelected()
	assert.Nil(t, cmd)
	assert.False(t, updated.taxLoading)

	updated, cmd = model.taxonomyActivateSelected()
	assert.Nil(t, cmd)
	assert.False(t, updated.taxLoading)
}

func TestRenderTaxonomyStateMessages(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	model.width = 90
	model.taxKind = 0

	model.taxPromptMode = taxPromptFilter
	model.taxPromptBuf = "scope"
	out := model.renderTaxonomy()
	assert.Contains(t, out, "Taxonomy Filter")

	model.taxPromptMode = taxPromptNone
	model.taxLoading = true
	out = model.renderTaxonomy()
	assert.Contains(t, out, "Loading taxonomy")

	model.taxLoading = false
	model.taxItems = nil
	out = model.renderTaxonomy()
	assert.Contains(t, out, "No taxonomy rows found")
}

func TestRenderTaxonomyPreviewIncludesOptionalFields(t *testing.T) {
	model := NewProfileModel(nil, &config.Config{})
	symmetric := true
	desc := "Primary scope"
	item := api.TaxonomyEntry{
		ID:          "scope-1",
		Name:        "Public",
		Description: &desc,
		IsBuiltin:   true,
		IsActive:    false,
		IsSymmetric: &symmetric,
		Metadata:    map[string]any{"owner": "alxx"},
	}

	assert.Equal(t, "", model.renderTaxonomyPreview(item, 0))
	preview := model.renderTaxonomyPreview(item, 40)
	assert.Contains(t, preview, "Status")
	assert.Contains(t, preview, "inactive")
	assert.Contains(t, preview, "Builtin")
	assert.Contains(t, preview, "Symmetric")
	assert.Contains(t, preview, "Meta")
}

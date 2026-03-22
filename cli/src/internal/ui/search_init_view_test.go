package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

// TestSearchInitReturnsFocusCmd handles test search init returns focus cmd.
func TestSearchInitReturnsFocusCmd(t *testing.T) {
	model := NewSearchModel(nil)
	assert.NotNil(t, model.Init())
}

// TestSearchViewRendersEmptyAndPopulatedStates handles test search view renders empty and populated states.
func TestSearchViewRendersEmptyAndPopulatedStates(t *testing.T) {
	model := NewSearchModel(nil)
	model.width = 80

	out := model.View()
	assert.Contains(t, out, "Query")
	assert.Contains(t, out, "Type to search.")

	// Inject a results message directly to avoid needing a live client.
	model.textInput.SetValue("a")
	model.mode = searchModeText
	model, _ = model.Update(searchResultsMsg{
		query:    "a",
		mode:     searchModeText,
		entities: []api.Entity{{ID: "ent-1", Name: "Alpha", Type: "person"}},
	})

	out = model.View()
	assert.Contains(t, out, "Query")
	assert.Contains(t, out, "a")
	assert.Contains(t, out, "Alpha")
}

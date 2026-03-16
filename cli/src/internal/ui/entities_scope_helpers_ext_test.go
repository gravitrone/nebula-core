package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEntitiesFormatEntityScopesHandlesEmptyInput(t *testing.T) {
	model := NewEntitiesModel(nil)

	assert.Equal(t, "-", model.formatEntityScopes(nil))
	assert.Equal(t, "-", model.formatEntityScopes([]string{}))
}

func TestEntitiesFormatEntityScopesFallsBackToShortIDs(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.scopeNames = map[string]string{
		"scope-public-id":  "public",
		"deadbeefcafebabe": "",
	}

	formatted := model.formatEntityScopes([]string{"scope-public-id", "deadbeefcafebabe", "1234567890abcdef"})
	assert.Contains(t, formatted, "[public]")
	assert.Contains(t, formatted, "[deadbeef]")
	assert.Contains(t, formatted, "[12345678]")
}

func TestEntitiesScopeNamesFromIDsPrefersNameAndKeepsUnknownRawID(t *testing.T) {
	model := NewEntitiesModel(nil)
	model.scopeNames = map[string]string{
		"scope-1": "public",
		"scope-2": "",
	}

	assert.Nil(t, model.scopeNamesFromIDs(nil))
	assert.Equal(t, []string{"public", "scope-2", "scope-3"}, model.scopeNamesFromIDs([]string{"scope-1", "scope-2", "scope-3"}))
}

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntitiesRenderModeLineStates(t *testing.T) {
	model := NewEntitiesModel(nil)

	model.view = entitiesViewList
	lineList := stripANSI(model.renderModeLine())
	assert.Contains(t, lineList, "Add")
	assert.Contains(t, lineList, "Library")

	model.view = entitiesViewAdd
	lineAdd := stripANSI(model.renderModeLine())
	assert.Contains(t, lineAdd, "Add")
	assert.Contains(t, lineAdd, "Library")

	model.modeFocus = true
	lineFocus := stripANSI(model.renderModeLine())
	assert.Contains(t, lineFocus, "Add")
	assert.Contains(t, lineFocus, "Library")

	model.view = entitiesViewList
	lineListFocus := stripANSI(model.renderModeLine())
	assert.Contains(t, lineListFocus, "Add")
	assert.Contains(t, lineListFocus, "Library")
	assert.NotEqual(t, "", lineList)
	assert.NotEqual(t, "", lineAdd)
	assert.NotEqual(t, "", lineFocus)
	assert.NotEqual(t, "", lineListFocus)
}

func TestEntitiesHandleListKeysBranchMatrix(t *testing.T) {
	newBase := func() EntitiesModel {
		m := NewEntitiesModel(nil)
		m.items = []api.Entity{
			{ID: "ent-1", Name: "Alpha", Type: "person"},
			{ID: "ent-2", Name: "Beta", Type: "tool"},
		}
		m.allItems = append([]api.Entity{}, m.items...)
		m.applyEntityFilters()
		m.bulkSelected = map[string]bool{}
		m.scopeNames = map[string]string{"s1": "public"}
		return m
	}

	t.Run("delegates to bulk prompt and filter handlers", func(t *testing.T) {
		model := newBase()
		model.bulkPrompt = "Bulk Tags (add:tag1,tag2)"
		model.bulkBuf = "abc"
		next, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
		assert.Nil(t, cmd)
		assert.Equal(t, "ab", next.bulkBuf)

		model = newBase()
		model.filtering = true
		next, cmd = model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
		assert.Nil(t, cmd)
		assert.False(t, next.filtering)
	})

	t.Run("mode focus delegates to mode handler", func(t *testing.T) {
		model := newBase()
		model.modeFocus = true
		next, _ := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyRight})
		assert.Equal(t, entitiesViewAdd, next.view)
		assert.False(t, next.modeFocus)
		assert.NotNil(t, next.addForm)
	})

	t.Run("navigation and selection branches", func(t *testing.T) {
		model := newBase()
		model.dataTable.SetCursor(0)

		next, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyDown})
		assert.Nil(t, cmd)
		assert.Equal(t, 1, next.dataTable.Cursor())

		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
		assert.Nil(t, cmd)
		assert.Equal(t, 0, next.dataTable.Cursor())

		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeyUp})
		assert.Nil(t, cmd)
		assert.True(t, next.modeFocus)

		next.modeFocus = false
		next.searchBuf = ""
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeySpace})
		assert.Nil(t, cmd)
		assert.True(t, next.isBulkSelected(0))

		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEnter})
		require.NotNil(t, cmd)
		assert.Equal(t, entitiesViewDetail, next.view)
		require.NotNil(t, next.detail)
		assert.Equal(t, "ent-1", next.detail.ID)
	})

	t.Run("search input and command-return branches", func(t *testing.T) {
		model := newBase()
		model.searchBuf = "al"
		model.searchSuggest = "alpha"

		next, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeyTab})
		require.NotNil(t, cmd)
		assert.True(t, next.loading)
		assert.Equal(t, "alpha", next.searchBuf)

		next.searchBuf = "alp"
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeyBackspace})
		require.NotNil(t, cmd)
		assert.Equal(t, "al", next.searchBuf)

		next.searchBuf = "alpha"
		next.searchSuggest = "alpha"
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
		require.NotNil(t, cmd)
		assert.Equal(t, "", next.searchBuf)
		assert.Equal(t, "", next.searchSuggest)

		next.searchBuf = "x"
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeySpace})
		require.NotNil(t, cmd)
		assert.Equal(t, "x ", next.searchBuf)

		next.searchBuf = "query"
		next.searchSuggest = "query-suggest"
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
		require.NotNil(t, cmd)
		assert.Equal(t, "", next.searchBuf)
		assert.Equal(t, "", next.searchSuggest)

		next.searchBuf = ""
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: tea.KeySpace})
		assert.Nil(t, cmd)
		assert.True(t, next.isBulkSelected(next.dataTable.Cursor()))

		next.searchBuf = ""
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: ' ', Text: " "})
		assert.Nil(t, cmd)
		assert.Equal(t, "", next.searchBuf)
	})

	t.Run("bulk action prompt and clear branches", func(t *testing.T) {
		model := newBase()
		model.bulkSelected = map[string]bool{"ent-1": true}

		next, cmd := model.handleListKeys(tea.KeyPressMsg{Code: 't', Text: "t"})
		assert.Nil(t, cmd)
		assert.Equal(t, "Bulk Tags (add:tag1,tag2)", next.bulkPrompt)
		assert.Equal(t, bulkTargetTags, next.bulkTarget)

		model = newBase()
		model.bulkSelected = map[string]bool{"ent-1": true}
		next, cmd = model.handleListKeys(tea.KeyPressMsg{Code: 'p', Text: "p"})
		assert.Nil(t, cmd)
		assert.Equal(t, "Bulk Scopes (add:scope1,scope2)", next.bulkPrompt)
		assert.Equal(t, bulkTargetScopes, next.bulkTarget)

		next.bulkPrompt = ""
		next, cmd = next.handleListKeys(tea.KeyPressMsg{Code: 'c', Text: "c"})
		assert.Nil(t, cmd)
		assert.Empty(t, next.bulkSelected)
	})

	t.Run("default rune branch", func(t *testing.T) {
		model := newBase()
		next, cmd := model.handleListKeys(tea.KeyPressMsg{Code: tea.KeySpace})
		assert.Nil(t, cmd)
		assert.Equal(t, "", next.searchBuf)

		next, cmd = model.handleListKeys(tea.KeyPressMsg{Code: 'z', Text: "z"})
		require.NotNil(t, cmd)
		assert.True(t, next.loading)
		assert.Equal(t, "z", next.searchBuf)
	})
}

package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

// TestListViewsEnterFilteringModeWithF handles test list views enter filtering mode with f.
func TestListViewsEnterFilteringModeWithF(t *testing.T) {
	keyF := tea.KeyPressMsg{Code: 'f', Text: "f"}

	t.Run("inbox", func(t *testing.T) {
		model := NewInboxModel(nil)
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("entities", func(t *testing.T) {
		model := NewEntitiesModel(nil)
		model.view = entitiesViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("relationships", func(t *testing.T) {
		model := NewRelationshipsModel(nil)
		model.view = relsViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("context", func(t *testing.T) {
		model := NewContextModel(nil)
		model.view = contextViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("jobs", func(t *testing.T) {
		model := NewJobsModel(nil)
		model.view = jobsViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("logs", func(t *testing.T) {
		model := NewLogsModel(nil)
		model.view = logsViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("files", func(t *testing.T) {
		model := NewFilesModel(nil)
		model.view = filesViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("protocols", func(t *testing.T) {
		model := NewProtocolsModel(nil)
		model.view = protocolsViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})

	t.Run("history", func(t *testing.T) {
		model := NewHistoryModel(nil)
		model.view = historyViewList
		model, _ = model.Update(keyF)
		assert.True(t, model.filtering)
	})
}

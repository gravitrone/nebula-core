package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtocolsViewSwitchesAcrossListAddDetailAndEdit(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.width = 90
	model.items = []api.Protocol{{ID: "proto-1", Name: "alpha", Title: "Alpha"}}
	model.dataTable.SetRows([]table.Row{{"alpha"}})
	model.dataTable.SetCursor(0)

	out := components.SanitizeText(model.View())
	assert.Contains(t, out, "Library")

	content := "proto content"
	version := "v1"
	typ := "ops"
	sourcePath := "/tmp/protocol.md"
	model.detail = &api.Protocol{
		ID:           "proto-1",
		Name:         "alpha",
		Title:        "Alpha Protocol",
		Version:      &version,
		ProtocolType: &typ,
		Content:      &content,
		Status:       "active",
		Tags:         []string{"tag-a"},
		AppliesTo:    []string{"entity"},
		SourcePath:   &sourcePath,
		Metadata:     api.JSONMap{"note": "hello"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	model.view = protocolsViewDetail
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Protocol")
	assert.Contains(t, out, "Alpha Protocol")

	model.startEdit()
	model.view = protocolsViewEdit
	out = components.SanitizeText(model.View())
	assert.Contains(t, out, "Title")
}

func TestProtocolsHandleDetailKeysEditAndBackPaths(t *testing.T) {
	content := "rules"
	model := NewProtocolsModel(nil)
	model.view = protocolsViewDetail
	model.detail = &api.Protocol{
		ID:        "proto-1",
		Name:      "alpha",
		Title:     "Alpha",
		Content:   &content,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	updated, cmd := model.handleDetailKeys(tea.KeyPressMsg{Code: 'e', Text: "e"})
	require.Nil(t, cmd)
	assert.Equal(t, protocolsViewEdit, updated.view)
	assert.Equal(t, "Alpha", updated.editFields[protoEditFieldTitle].value)
	assert.Equal(t, "rules", updated.editFields[protoEditFieldContent].value)

	updated, cmd = updated.handleDetailKeys(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, cmd)
	assert.Equal(t, protocolsViewList, updated.view)
	assert.Nil(t, updated.detail)
	assert.Nil(t, updated.detailRels)
}

func TestProtocolsRenderDetailIncludesOptionalSectionsAndRelationships(t *testing.T) {
	content := "playbook"
	version := "v2"
	typ := "ops"
	sourcePath := "/tmp/protocols/alpha.md"
	model := NewProtocolsModel(nil)
	model.width = 100
	model.detail = &api.Protocol{
		ID:           "proto-1",
		Name:         "alpha",
		Title:        "Alpha Protocol",
		Version:      &version,
		ProtocolType: &typ,
		Content:      &content,
		Status:       "active",
		Tags:         []string{"ops", "prod"},
		AppliesTo:    []string{"entity", "job"},
		SourcePath:   &sourcePath,
		Metadata:     api.JSONMap{"owner": "platform"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	model.detailRels = []api.Relationship{{
		ID:         "rel-1",
		SourceType: "protocol",
		SourceID:   "proto-1",
		SourceName: "alpha",
		TargetType: "entity",
		TargetID:   "ent-1",
		TargetName: "Entity One",
		Type:       "applies-to",
		Status:     "active",
		CreatedAt:  time.Now(),
	}}

	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "Version")
	assert.Contains(t, out, "Type")
	assert.Contains(t, out, "Applies To")
	assert.Contains(t, out, "Source Path")
	assert.Contains(t, out, "applies-to")
}

func TestProtocolsRenderDetailFallsBackToListWhenNoDetail(t *testing.T) {
	model := NewProtocolsModel(nil)
	model.width = 80
	model.items = []api.Protocol{{ID: "proto-1", Name: "alpha", Title: "Alpha"}}
	model.dataTable.SetRows([]table.Row{{"alpha"}})
	model.dataTable.SetCursor(0)

	out := components.SanitizeText(model.renderDetail())
	assert.Contains(t, out, "alpha")
}

package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type contextSavedMsg struct{}
type contextLinkResultsMsg struct{ items []api.Entity }
type contextListLoadedMsg struct{ items []api.Context }
type contextScopesLoadedMsg struct{ names map[string]string }
type contextDetailLoadedMsg struct {
	item          api.Context
	relationships []api.Relationship
}
type contextUpdatedMsg struct{ item api.Context }

// --- Constants ---

var contextTypes = []string{
	"note",
	"video",
	"article",
	"paper",
	"tool",
	"course",
	"thread",
}

var contextStatusOptions = []string{"active", "inactive"}

type contextView int

const (
	contextViewAdd contextView = iota
	contextViewList
	contextViewDetail
	contextViewEdit
)

// Field indices
const (
	fieldTitle    = 0
	fieldURL      = 1
	fieldType     = 2
	fieldTags     = 3
	fieldScopes   = 4
	fieldEntities = 5
	fieldNotes    = 6
	fieldCount    = 7
)

const (
	contextEditFieldTitle = iota
	contextEditFieldURL
	contextEditFieldType
	contextEditFieldStatus
	contextEditFieldTags
	contextEditFieldScopes
	contextEditFieldNotes
	contextEditFieldCount
)

// --- Context Model ---

// ContextModel handles adding context items manually.
type ContextModel struct {
	client              *api.Client
	fields              []formField
	typeIdx             int
	typeSelecting       bool
	scopeOptions        []string
	scopeIdx            int
	scopeSelecting      bool
	focus               int
	modeFocus           bool
	saved               bool
	saving              bool
	view                contextView
	errText             string
	tags                []string
	tagBuf              string
	scopes              []string
	scopeBuf            string
	linkSearching       bool
	linkLoading         bool
	linkQuery           string
	linkResults         []api.Entity
	linkList            *components.List
	linkEntities        []api.Entity
	list                *components.List
	allItems            []api.Context
	items               []api.Context
	filtering           bool
	filterBuf           string
	loadingList         bool
	detail              *api.Context
	detailRelationships []api.Relationship
	contextEditFields   []formField
	editFocus           int
	editTypeIdx         int
	editTypeSelecting   bool
	editScopeSelecting  bool
	editStatusIdx       int
	editTags            []string
	editTagBuf          string
	editScopes          []string
	editScopeBuf        string
	editSaving          bool
	contentExpanded     bool
	sourcePathExpanded  bool
	scopeNames          map[string]string
	width               int
	height              int
}

type formField struct {
	label string
	value string
}

// NewContextModel builds the context UI model.
func NewContextModel(client *api.Client) ContextModel {
	return ContextModel{
		client: client,
		fields: []formField{
			{label: "Title"},
			{label: "URL"},
			{label: "Type"},
			{label: "Tags"},
			{label: "Scopes"},
			{label: "Entities"},
			{label: "Notes"},
		},
		contextEditFields: []formField{
			{label: "Title"},
			{label: "URL"},
			{label: "Type"},
			{label: "Status"},
			{label: "Tags"},
			{label: "Scopes"},
			{label: "Notes"},
		},
		linkList: components.NewList(6),
		list:     components.NewList(10),
	}
}

// Init handles init.
func (m ContextModel) Init() tea.Cmd {
	m.saved = false
	m.errText = ""
	m.focus = 0
	m.modeFocus = false
	m.typeIdx = 0
	m.typeSelecting = false
	m.scopeIdx = 0
	m.scopeSelecting = false
	m.view = contextViewAdd
	m.tags = nil
	m.tagBuf = ""
	m.scopes = nil
	m.scopeBuf = ""
	m.linkSearching = false
	m.linkLoading = false
	m.linkQuery = ""
	m.linkResults = nil
	m.linkEntities = nil
	m.allItems = nil
	m.filtering = false
	m.filterBuf = ""
	m.detail = nil
	m.loadingList = false
	m.editFocus = 0
	m.editTypeIdx = 0
	m.editTypeSelecting = false
	m.editScopeSelecting = false
	m.editStatusIdx = statusIndex(contextStatusOptions, "active")
	m.editTags = nil
	m.editTagBuf = ""
	m.editScopes = nil
	m.editScopeBuf = ""
	m.editSaving = false
	m.contentExpanded = false
	m.sourcePathExpanded = false
	if m.scopeNames == nil {
		m.scopeNames = map[string]string{}
	}
	if m.linkList != nil {
		m.linkList.SetItems(nil)
	}
	if m.list != nil {
		m.list.SetItems(nil)
	}
	for i := range m.fields {
		m.fields[i].value = ""
	}
	return m.loadScopeNames()
}

// Update updates update.
func (m ContextModel) Update(msg tea.Msg) (ContextModel, tea.Cmd) {
	switch msg := msg.(type) {
	case contextSavedMsg:
		m.saving = false
		m.saved = true
		return m, nil

	case errMsg:
		m.saving = false
		m.editSaving = false
		m.errText = msg.err.Error()
		return m, nil
	case contextLinkResultsMsg:
		m.linkLoading = false
		m.linkResults = msg.items
		labels := make([]string, len(msg.items))
		for i, e := range msg.items {
			labels[i] = formatEntityLine(e)
		}
		if m.linkList != nil {
			m.linkList.SetItems(labels)
		}
		return m, nil

	case contextListLoadedMsg:
		m.loadingList = false
		m.allItems = append([]api.Context{}, msg.items...)
		m.applyContextFilter()
		return m, nil
	case contextScopesLoadedMsg:
		if m.scopeNames == nil {
			m.scopeNames = map[string]string{}
		}
		for id, name := range msg.names {
			m.scopeNames[id] = name
		}
		m.scopeOptions = scopeNameList(m.scopeNames)
		return m, nil
	case contextDetailLoadedMsg:
		m.detail = &msg.item
		m.detailRelationships = msg.relationships
		return m, nil
	case contextUpdatedMsg:
		m.editSaving = false
		m.detail = &msg.item
		m.view = contextViewDetail
		return m, nil

	case tea.KeyMsg:
		if m.view == contextViewList {
			return m.handleListKeys(msg)
		}
		if m.view == contextViewEdit {
			return m.handleEditKeys(msg)
		}
		if m.view == contextViewDetail {
			return m.handleDetailKeys(msg)
		}
		if m.linkSearching {
			return m.handleLinkSearch(msg)
		}
		if m.modeFocus {
			return m.handleModeKeys(msg)
		}
		// Type selector field - press space to enter, then space/left/right to cycle
		if m.focus == fieldType {
			if m.typeSelecting {
				switch {
				case isKey(msg, "left"):
					m.typeIdx = (m.typeIdx - 1 + len(contextTypes)) % len(contextTypes)
					return m, nil
				case isKey(msg, "right"):
					m.typeIdx = (m.typeIdx + 1) % len(contextTypes)
					return m, nil
				case isSpace(msg):
					m.typeSelecting = false
					return m, nil
				}
			} else if isSpace(msg) {
				m.typeSelecting = true
				return m, nil
			}
		}
		if m.focus == fieldScopes && m.scopeSelecting {
			switch {
			case isKey(msg, "left"):
				if len(m.scopeOptions) > 0 {
					m.scopeIdx = (m.scopeIdx - 1 + len(m.scopeOptions)) % len(m.scopeOptions)
				}
				return m, nil
			case isKey(msg, "right"):
				if len(m.scopeOptions) > 0 {
					m.scopeIdx = (m.scopeIdx + 1) % len(m.scopeOptions)
				}
				return m, nil
			case isSpace(msg):
				if len(m.scopeOptions) > 0 {
					scope := m.scopeOptions[m.scopeIdx]
					m.scopes = toggleScope(m.scopes, scope)
				}
				return m, nil
			case isEnter(msg), isBack(msg):
				m.scopeSelecting = false
				return m, nil
			}
		}

		switch {
		case isDown(msg):
			m.typeSelecting = false
			m.scopeSelecting = false
			m.focus = (m.focus + 1) % fieldCount
		case isUp(msg):
			if m.focus == 0 {
				m.typeSelecting = false
				m.scopeSelecting = false
				m.modeFocus = true
				return m, nil
			}
			m.typeSelecting = false
			m.scopeSelecting = false
			m.focus = (m.focus - 1 + fieldCount) % fieldCount
		case isKey(msg, "ctrl+s"):
			return m.save()
		case isBack(msg):
			m.resetForm()
		case isKey(msg, "backspace"):
			switch m.focus {
			case fieldTags:
				if len(m.tagBuf) > 0 {
					m.tagBuf = m.tagBuf[:len(m.tagBuf)-1]
				} else if len(m.tags) > 0 {
					m.tags = m.tags[:len(m.tags)-1]
				}
			case fieldScopes:
				if len(m.scopes) > 0 {
					m.scopes = m.scopes[:len(m.scopes)-1]
				}
			case fieldEntities:
				if len(m.linkEntities) > 0 {
					m.linkEntities = m.linkEntities[:len(m.linkEntities)-1]
				}
			default:
				if m.focus != fieldType {
					f := &m.fields[m.focus]
					if len(f.value) > 0 {
						f.value = f.value[:len(f.value)-1]
					}
				}
			}
		default:
			if m.focus == fieldTags {
				switch {
				case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
					m.commitTag()
				default:
					ch := msg.String()
					if len(ch) == 1 && ch != "," {
						m.tagBuf += ch
					}
				}
			} else if m.focus == fieldScopes {
				if isSpace(msg) {
					m.scopeSelecting = true
				}
			} else if m.focus == fieldEntities {
				if isEnter(msg) {
					m.startLinkSearch()
				}
			} else if m.focus != fieldType {
				ch := msg.String()
				if len(ch) == 1 || ch == " " {
					m.fields[m.focus].value += ch
				}
			}
		}
		if m.focus == fieldEntities && !m.linkSearching {
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				m.startLinkSearch()
				m.linkQuery += ch
				return m, m.updateLinkSearch()
			}
		}
	}
	return m, nil
}

// View handles view.
func (m ContextModel) View() string {
	if m.saving {
		return "  " + MutedStyle.Render("Saving...")
	}

	if m.saved {
		return components.Indent(components.Box(SuccessStyle.Render("Context saved! Press Esc to add another."), m.width), 1)
	}

	if m.linkSearching {
		return m.renderLinkSearch()
	}
	if m.filtering && m.view == contextViewList {
		return components.Indent(components.InputDialog("Filter Context", m.filterBuf), 1)
	}

	modeLine := m.renderModeLine()
	var body string
	switch m.view {
	case contextViewList:
		body = m.renderList()
	case contextViewDetail:
		body = m.renderDetail()
	case contextViewEdit:
		body = m.renderEdit()
	default:
		body = m.renderAdd()
	}
	if modeLine != "" {
		body = components.CenterLine(modeLine, m.width) + "\n\n" + body
	}
	return components.Indent(body, 1)
}

// renderAdd renders render add.
func (m ContextModel) renderAdd() string {
	var b strings.Builder
	for i, f := range m.fields {
		label := f.label

		switch i {
		case fieldType:
			// Type selector
			if i == m.focus && m.typeSelecting {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n  ")
				for j, t := range contextTypes {
					if j == m.typeIdx {
						b.WriteString(AccentStyle.Render("[" + t + "]"))
					} else {
						b.WriteString(MutedStyle.Render(" " + t + " "))
					}
					if j < len(contextTypes)-1 {
						b.WriteString(" ")
					}
				}
			} else if i == m.focus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + contextTypes[m.typeIdx]))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + contextTypes[m.typeIdx]))
			}
		case fieldTags:
			if i == m.focus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderTags(true)))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderTags(false)))
			}
		case fieldScopes:
			if i == m.focus && m.scopeSelecting {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + renderScopeOptions(m.scopes, m.scopeOptions, m.scopeIdx)))
			} else if i == m.focus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderScopes(true)))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderScopes(false)))
			}
		case fieldEntities:
			if i == m.focus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderLinkedEntities(true)))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderLinkedEntities(false)))
			}
		case m.focus:
			b.WriteString(SelectedStyle.Render("  " + label + ":"))
			b.WriteString("\n")
			b.WriteString(NormalStyle.Render("  " + f.value))
			b.WriteString(AccentStyle.Render("█"))
		default:
			b.WriteString(MutedStyle.Render("  " + label + ":"))
			b.WriteString("\n")
			val := f.value
			if val == "" {
				val = "-"
			}
			b.WriteString(NormalStyle.Render("  " + val))
		}

		if i < fieldCount-1 {
			b.WriteString("\n\n")
		}
	}

	if m.errText != "" {
		b.WriteString("\n\n")
		b.WriteString(components.ErrorBox("Error", m.errText, m.width))
	}

	return components.TitledBox("Add Context", b.String(), m.width)
}

// renderEdit renders render edit.
func (m ContextModel) renderEdit() string {
	var b strings.Builder
	for i, f := range m.contextEditFields {
		label := f.label
		switch i {
		case contextEditFieldType:
			if i == m.editFocus && m.editTypeSelecting {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n  ")
				for j, t := range contextTypes {
					if j == m.editTypeIdx {
						b.WriteString(AccentStyle.Render("[" + t + "]"))
					} else {
						b.WriteString(MutedStyle.Render(" " + t + " "))
					}
					if j < len(contextTypes)-1 {
						b.WriteString(" ")
					}
				}
			} else if i == m.editFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + contextTypes[m.editTypeIdx]))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + contextTypes[m.editTypeIdx]))
			}
		case contextEditFieldStatus:
			if i == m.editFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
			}
			b.WriteString("\n")
			status := contextStatusOptions[m.editStatusIdx]
			b.WriteString(NormalStyle.Render("  " + status))
		case contextEditFieldTags:
			if i == m.editFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderEditTags(true)))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderEditTags(false)))
			}
		case contextEditFieldScopes:
			if i == m.editFocus && m.editScopeSelecting {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + renderScopeOptions(m.editScopes, m.scopeOptions, m.scopeIdx)))
			} else if i == m.editFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderEditScopes(true)))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderEditScopes(false)))
			}
		default:
			if i == m.editFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + f.value))
				b.WriteString(AccentStyle.Render("█"))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				val := f.value
				if val == "" {
					val = "-"
				}
				b.WriteString(NormalStyle.Render("  " + val))
			}
		}

		if i < contextEditFieldCount-1 {
			b.WriteString("\n\n")
		}
	}

	if m.errText != "" {
		b.WriteString("\n\n")
		b.WriteString(components.ErrorBox("Error", m.errText, m.width))
	}

	if m.editSaving {
		b.WriteString("\n\n" + MutedStyle.Render("Saving..."))
	}

	return components.TitledBox("Edit Context", b.String(), m.width)
}

// renderModeLine renders render mode line.
func (m ContextModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("Library")
	if m.view == contextViewAdd {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("Library")
	}
	if m.modeFocus {
		if m.view == contextViewAdd {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("Library")
		}
	}
	return add + " " + list
}

// handleModeKeys handles handle mode keys.
func (m ContextModel) handleModeKeys(msg tea.KeyMsg) (ContextModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.modeFocus = false
		if m.view == contextViewEdit {
			m.editFocus = 0
		} else {
			m.focus = 0
		}
	case isUp(msg):
		m.modeFocus = false
	case isKey(msg, "left"), isKey(msg, "right"), isSpace(msg), isEnter(msg):
		return m.toggleMode()
	case isBack(msg):
		m.modeFocus = false
		if m.view == contextViewEdit {
			m.editFocus = 0
		} else {
			m.focus = 0
		}
	}
	return m, nil
}

// toggleMode handles toggle mode.
func (m ContextModel) toggleMode() (ContextModel, tea.Cmd) {
	m.modeFocus = false
	m.detail = nil
	m.contentExpanded = false
	m.sourcePathExpanded = false
	if m.view == contextViewAdd {
		m.view = contextViewList
		m.loadingList = true
		return m, m.loadContextList()
	}
	if m.view == contextViewDetail || m.view == contextViewEdit {
		m.view = contextViewList
		return m, nil
	}
	m.view = contextViewAdd
	return m, nil
}

// handleListKeys handles handle list keys.
func (m ContextModel) handleListKeys(msg tea.KeyMsg) (ContextModel, tea.Cmd) {
	if m.filtering {
		return m.handleFilterInput(msg)
	}
	switch {
	case isDown(msg):
		m.list.Down()
	case isUp(msg):
		if m.list.Selected() == 0 {
			m.modeFocus = true
		} else {
			m.list.Up()
		}
	case isEnter(msg):
		if idx := m.list.Selected(); idx < len(m.items) {
			item := m.items[idx]
			itemID := strings.TrimSpace(item.ID)
			if itemID == "" {
				return m, func() tea.Msg {
					return errMsg{fmt.Errorf("selected context is missing id")}
				}
			}
			m.detail = &item
			m.view = contextViewDetail
			return m, m.loadContextDetail(itemID)
		}
	case isKey(msg, "f"):
		m.filtering = true
		return m, nil
	case isBack(msg):
		m.view = contextViewAdd
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m ContextModel) handleFilterInput(msg tea.KeyMsg) (ContextModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
	case isBack(msg):
		m.filtering = false
		m.filterBuf = ""
		m.applyContextFilter()
	case isKey(msg, "backspace", "delete"):
		if len(m.filterBuf) > 0 {
			m.filterBuf = m.filterBuf[:len(m.filterBuf)-1]
			m.applyContextFilter()
		}
	default:
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
			if ch == " " && m.filterBuf == "" {
				return m, nil
			}
			m.filterBuf += ch
			m.applyContextFilter()
		}
	}
	return m, nil
}

// handleDetailKeys handles handle detail keys.
func (m ContextModel) handleDetailKeys(msg tea.KeyMsg) (ContextModel, tea.Cmd) {
	switch {
	case isUp(msg):
		m.modeFocus = true
	case isBack(msg):
		m.detail = nil
		m.detailRelationships = nil
		m.contentExpanded = false
		m.sourcePathExpanded = false
		m.view = contextViewList
	case isKey(msg, "e"):
		m.startEdit()
		m.view = contextViewEdit
	case isKey(msg, "c"):
		m.contentExpanded = !m.contentExpanded
	case isKey(msg, "v"):
		m.sourcePathExpanded = !m.sourcePathExpanded
	}
	return m, nil
}

// handleEditKeys handles handle edit keys.
func (m ContextModel) handleEditKeys(msg tea.KeyMsg) (ContextModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if m.modeFocus {
		return m.handleModeKeys(msg)
	}
	if m.editFocus == contextEditFieldType {
		if m.editTypeSelecting {
			switch {
			case isKey(msg, "left"):
				m.editTypeIdx = (m.editTypeIdx - 1 + len(contextTypes)) % len(contextTypes)
				return m, nil
			case isKey(msg, "right"):
				m.editTypeIdx = (m.editTypeIdx + 1) % len(contextTypes)
				return m, nil
			case isSpace(msg), isEnter(msg):
				m.editTypeSelecting = false
				return m, nil
			}
		} else if isSpace(msg) || isEnter(msg) {
			m.editTypeSelecting = true
			return m, nil
		}
	}
	if m.editFocus == contextEditFieldScopes && m.editScopeSelecting {
		switch {
		case isKey(msg, "left"):
			if len(m.scopeOptions) > 0 {
				m.scopeIdx = (m.scopeIdx - 1 + len(m.scopeOptions)) % len(m.scopeOptions)
			}
			return m, nil
		case isKey(msg, "right"):
			if len(m.scopeOptions) > 0 {
				m.scopeIdx = (m.scopeIdx + 1) % len(m.scopeOptions)
			}
			return m, nil
		case isSpace(msg):
			if len(m.scopeOptions) > 0 {
				scope := m.scopeOptions[m.scopeIdx]
				m.editScopes = toggleScope(m.editScopes, scope)
			}
			return m, nil
		case isEnter(msg), isBack(msg):
			m.editScopeSelecting = false
			return m, nil
		}
	}
	if m.editFocus == contextEditFieldStatus {
		switch {
		case isKey(msg, "left"):
			m.editStatusIdx = (m.editStatusIdx - 1 + len(contextStatusOptions)) % len(contextStatusOptions)
			return m, nil
		case isKey(msg, "right"), isSpace(msg):
			m.editStatusIdx = (m.editStatusIdx + 1) % len(contextStatusOptions)
			return m, nil
		}
	}

	switch {
	case isDown(msg):
		m.editTypeSelecting = false
		m.editScopeSelecting = false
		m.editFocus = (m.editFocus + 1) % contextEditFieldCount
	case isUp(msg):
		m.editTypeSelecting = false
		m.editScopeSelecting = false
		if m.editFocus == 0 {
			m.modeFocus = true
			return m, nil
		}
		m.editFocus = (m.editFocus - 1 + contextEditFieldCount) % contextEditFieldCount
	case isKey(msg, "ctrl+s"):
		return m.saveEdit()
	case isBack(msg):
		m.editScopeSelecting = false
		m.view = contextViewDetail
	case isKey(msg, "backspace"):
		switch m.editFocus {
		case contextEditFieldTags:
			if len(m.editTagBuf) > 0 {
				m.editTagBuf = m.editTagBuf[:len(m.editTagBuf)-1]
			} else if len(m.editTags) > 0 {
				m.editTags = m.editTags[:len(m.editTags)-1]
			}
		case contextEditFieldScopes:
			if len(m.editScopes) > 0 {
				m.editScopes = m.editScopes[:len(m.editScopes)-1]
			}
		default:
			if m.editFocus != contextEditFieldType && m.editFocus != contextEditFieldStatus {
				f := &m.contextEditFields[m.editFocus]
				if len(f.value) > 0 {
					f.value = f.value[:len(f.value)-1]
				}
			}
		}
	default:
		switch m.editFocus {
		case contextEditFieldTags:
			switch {
			case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
				m.commitEditTag()
			default:
				ch := msg.String()
				if len(ch) == 1 && ch != "," {
					m.editTagBuf += ch
				}
			}
		case contextEditFieldScopes:
			if isSpace(msg) {
				m.editScopeSelecting = true
			}
		default:
			if m.editFocus != contextEditFieldType && m.editFocus != contextEditFieldStatus {
				ch := msg.String()
				if len(ch) == 1 || ch == " " {
					m.contextEditFields[m.editFocus].value += ch
				}
			}
		}
	}
	return m, nil
}

// renderList renders render list.
func (m ContextModel) renderList() string {
	if m.loadingList {
		return components.Box(MutedStyle.Render("Loading context..."), m.width)
	}

	if len(m.items) == 0 {
		return components.EmptyStateBox(
			"Context",
			"No context found.",
			[]string{"Press tab to switch Add/Library", "Press / for command palette"},
			m.width,
		)
	}

	contentWidth := components.BoxContentWidth(m.width)
	visible := m.list.Visible()

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap
	}

	sepWidth := 1
	if b := lipgloss.RoundedBorder().Left; b != "" {
		sepWidth = lipgloss.Width(b)
	}

	// 4 columns -> 3 separators.
	availableCols := tableWidth - (3 * sepWidth)
	if availableCols < 30 {
		availableCols = 30
	}

	typeWidth := 10
	statusWidth := 11
	atWidth := compactTimeColumnWidth
	titleWidth := availableCols - (typeWidth + statusWidth + atWidth)
	if titleWidth < 12 {
		titleWidth = 12
	}
	cols := []components.TableColumn{
		{Header: "Title", Width: titleWidth, Align: lipgloss.Left},
		{Header: "Type", Width: typeWidth, Align: lipgloss.Left},
		{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.Context
	if idx := m.list.Selected(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}

	for i := range visible {
		absIdx := m.list.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.items) {
			continue
		}
		k := m.items[absIdx]

		title := components.ClampTextWidthEllipsis(components.SanitizeOneLine(contextTitle(k)), titleWidth)
		typ := strings.TrimSpace(components.SanitizeOneLine(k.SourceType))
		if typ == "" {
			typ = "note"
		}
		status := strings.TrimSpace(components.SanitizeOneLine(k.Status))
		if status == "" {
			status = "-"
		}
		at := k.UpdatedAt
		if at.IsZero() {
			at = k.CreatedAt
		}
		when := formatLocalTimeCompact(at)

		if m.list.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			title,
			components.ClampTextWidthEllipsis(typ, typeWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			when,
		})
	}
	if m.modeFocus {
		activeRowRel = -1
	}

	countLine := fmt.Sprintf("%d total", len(m.items))
	if query := strings.TrimSpace(m.filterBuf); query != "" {
		countLine = fmt.Sprintf("%s · filter: %s", countLine, query)
	}
	countLine = MutedStyle.Render(countLine)

	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderContextPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.TitledBox("Context", content, m.width)
}

// renderDetail renders render detail.
func (m ContextModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}

	k := m.detail
	rows := []components.TableRow{
		{Label: "ID", Value: k.ID},
		{Label: "Title", Value: contextTitle(*k)},
	}
	if k.SourceType != "" {
		rows = append(rows, components.TableRow{Label: "Type", Value: k.SourceType})
	}
	if k.Status != "" {
		rows = append(rows, components.TableRow{Label: "Status", Value: k.Status})
	}
	if k.URL != nil && strings.TrimSpace(*k.URL) != "" {
		rows = append(rows, components.TableRow{Label: "URL", Value: *k.URL})
	}
	if len(k.PrivacyScopeIDs) > 0 {
		rows = append(rows, components.TableRow{Label: "Scopes", Value: m.formatContextScopes(k.PrivacyScopeIDs)})
	}
	if len(k.Tags) > 0 {
		rows = append(rows, components.TableRow{Label: "Tags", Value: strings.Join(k.Tags, ", ")})
	}
	rows = append(rows, components.TableRow{Label: "Created", Value: formatLocalTimeFull(k.CreatedAt)})
	if !k.UpdatedAt.IsZero() {
		rows = append(rows, components.TableRow{Label: "Updated", Value: formatLocalTimeFull(k.UpdatedAt)})
	}
	if k.SourcePath != nil && strings.TrimSpace(*k.SourcePath) != "" {
		path := *k.SourcePath
		if !m.sourcePathExpanded {
			path = truncateString(path, 60)
		}
		rows = append(rows, components.TableRow{Label: "Source Path", Value: path})
	}

	sections := []string{components.Table("Context", rows, m.width)}
	if k.Content != nil && strings.TrimSpace(*k.Content) != "" {
		content := strings.TrimSpace(components.SanitizeText(*k.Content))
		if !m.contentExpanded {
			content = truncateString(content, 220)
		}
		sections = append(sections, components.TitledBox("Content", content, m.width))
	}
	if len(m.detailRelationships) > 0 {
		sections = append(sections, renderRelationshipSummaryTable("context", k.ID, m.detailRelationships, 6, m.width))
	}

	return strings.Join(sections, "\n\n")
}

// renderContextPreview renders render context preview.
func (m ContextModel) renderContextPreview(k api.Context, width int) string {
	if width <= 0 {
		return ""
	}

	title := components.SanitizeOneLine(contextTitle(k))
	typ := strings.TrimSpace(components.SanitizeOneLine(k.SourceType))
	if typ == "" {
		typ = "note"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(k.Status))
	if status == "" {
		status = "-"
	}
	at := k.UpdatedAt
	if at.IsZero() {
		at = k.CreatedAt
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Type", typ, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeCompact(at), width))

	if k.URL != nil && strings.TrimSpace(*k.URL) != "" {
		lines = append(lines, renderPreviewRow("URL", strings.TrimSpace(*k.URL), width))
	}
	if len(k.PrivacyScopeIDs) > 0 {
		lines = append(lines, renderPreviewRow("Scopes", m.formatContextScopes(k.PrivacyScopeIDs), width))
	}
	if len(k.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(k.Tags, ", "), width))
	}
	if m.detail != nil && m.detail.ID == k.ID && len(m.detailRelationships) > 0 {
		lines = append(
			lines,
			renderPreviewRow("Links", fmt.Sprintf("%d", len(m.detailRelationships)), width),
		)
	}

	snippet := ""
	if k.Content != nil {
		snippet = truncateString(strings.TrimSpace(components.SanitizeText(*k.Content)), 80)
	} else if k.URL != nil {
		snippet = truncateString(strings.TrimSpace(components.SanitizeText(*k.URL)), 80)
	}
	if strings.TrimSpace(snippet) != "" {
		lines = append(lines, renderPreviewRow("Preview", strings.TrimSpace(snippet), width))
	}

	return padPreviewLines(lines, width)
}

// startEdit handles start edit.
func (m *ContextModel) startEdit() {
	if m.detail == nil {
		return
	}
	k := m.detail
	m.contextEditFields[contextEditFieldTitle].value = contextTitle(*k)
	if k.URL != nil {
		m.contextEditFields[contextEditFieldURL].value = *k.URL
	} else {
		m.contextEditFields[contextEditFieldURL].value = ""
	}
	m.contextEditFields[contextEditFieldNotes].value = ""
	if k.Content != nil {
		m.contextEditFields[contextEditFieldNotes].value = *k.Content
	}
	m.editTypeIdx = statusIndex(contextTypes, k.SourceType)
	m.editStatusIdx = statusIndex(contextStatusOptions, k.Status)
	m.editTags = append([]string{}, k.Tags...)
	m.editTagBuf = ""
	m.editScopes = m.scopeNamesFromIDs(k.PrivacyScopeIDs)
	m.editScopeBuf = ""
	m.editScopeSelecting = false
	m.scopeIdx = 0
	m.editSaving = false
	m.editFocus = 0
}

// saveEdit handles save edit.
func (m ContextModel) saveEdit() (ContextModel, tea.Cmd) {
	if m.detail == nil {
		return m, nil
	}
	m.commitEditTag()
	title := strings.TrimSpace(m.contextEditFields[contextEditFieldTitle].value)
	url := strings.TrimSpace(m.contextEditFields[contextEditFieldURL].value)
	content := strings.TrimSpace(m.contextEditFields[contextEditFieldNotes].value)
	sourceType := contextTypes[m.editTypeIdx]
	status := contextStatusOptions[m.editStatusIdx]
	tags := normalizeBulkTags(m.editTags)
	scopes := normalizeBulkScopes(m.editScopes)

	input := api.UpdateContextInput{
		Title:      &title,
		URL:        &url,
		SourceType: &sourceType,
		Content:    &content,
		Status:     &status,
		Tags:       &tags,
		Scopes:     &scopes,
	}

	m.editSaving = true
	return m, func() tea.Msg {
		updated, err := m.client.UpdateContext(m.detail.ID, input)
		if err != nil {
			return errMsg{err}
		}
		return contextUpdatedMsg{item: *updated}
	}
}

// --- Helpers ---

func (m ContextModel) loadContextList() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryContext(api.QueryParams{})
		if err != nil {
			return errMsg{err}
		}
		return contextListLoadedMsg{items: items}
	}
}

// applyContextFilter handles apply context filter.
func (m *ContextModel) applyContextFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterBuf))
	if query == "" {
		m.items = append([]api.Context{}, m.allItems...)
	} else {
		filtered := make([]api.Context, 0, len(m.allItems))
		for _, item := range m.allItems {
			title := strings.ToLower(strings.TrimSpace(contextTitle(item)))
			typ := strings.ToLower(strings.TrimSpace(item.SourceType))
			status := strings.ToLower(strings.TrimSpace(item.Status))
			tags := strings.ToLower(strings.Join(item.Tags, " "))
			content := ""
			if item.Content != nil {
				content = strings.ToLower(strings.TrimSpace(*item.Content))
			}
			url := ""
			if item.URL != nil {
				url = strings.ToLower(strings.TrimSpace(*item.URL))
			}
			if strings.Contains(title, query) ||
				strings.Contains(typ, query) ||
				strings.Contains(status, query) ||
				strings.Contains(tags, query) ||
				strings.Contains(content, query) ||
				strings.Contains(url, query) {
				filtered = append(filtered, item)
			}
		}
		m.items = filtered
	}
	labels := make([]string, len(m.items))
	for i, item := range m.items {
		labels[i] = formatContextLine(item)
	}
	if m.list != nil {
		m.list.SetItems(labels)
	}
}

// loadContextDetail loads load context detail.
func (m ContextModel) loadContextDetail(id string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(id) == "" {
			return errMsg{fmt.Errorf("context id is required")}
		}
		item, err := m.client.GetContext(id)
		if err != nil {
			return errMsg{err}
		}
		rels, relErr := m.client.GetRelationships("context", id)
		if relErr != nil {
			rels = nil
		}
		return contextDetailLoadedMsg{item: *item, relationships: rels}
	}
}

// formatContextLine handles format context line.
func formatContextLine(k api.Context) string {
	t := components.SanitizeText(k.SourceType)
	if t == "" {
		t = "note"
	}
	name := truncateContextName(components.SanitizeText(contextTitle(k)), maxContextNameLen)
	line := fmt.Sprintf("%s %s", name, TypeBadgeStyle.Render(components.SanitizeText(t)))
	if status := strings.TrimSpace(components.SanitizeText(k.Status)); status != "" {
		line = fmt.Sprintf("%s · %s", line, status)
	}
	preview := ""
	if k.Content != nil {
		preview = truncateString(strings.TrimSpace(components.SanitizeText(*k.Content)), 40)
	} else if k.URL != nil {
		preview = truncateString(strings.TrimSpace(components.SanitizeText(*k.URL)), 40)
	}
	if preview != "" {
		line = fmt.Sprintf("%s · %s", line, preview)
	}
	return line
}

// contextTitle handles context title.
func contextTitle(k api.Context) string {
	title := strings.TrimSpace(k.Title)
	if title != "" {
		return title
	}
	title = strings.TrimSpace(k.Name)
	if title != "" {
		return title
	}
	return "(untitled)"
}

const maxContextNameLen = 80

// truncateContextName handles truncate context name.
func truncateContextName(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// loadScopeNames loads load scope names.
func (m ContextModel) loadScopeNames() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		scopes, err := m.client.ListAuditScopes()
		if err != nil {
			return errMsg{err}
		}
		names := map[string]string{}
		for _, scope := range scopes {
			names[scope.ID] = scope.Name
		}
		return contextScopesLoadedMsg{names: names}
	}
}

// formatContextScopes handles format context scopes.
func (m ContextModel) formatContextScopes(ids []string) string {
	if len(ids) == 0 {
		return "-"
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := m.scopeNames[id]; ok && name != "" {
			names = append(names, name)
		} else {
			names = append(names, id)
		}
	}
	return formatScopePreview(names)
}

// scopeNamesFromIDs handles scope names from ids.
func (m ContextModel) scopeNamesFromIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := m.scopeNames[id]; ok && name != "" {
			names = append(names, name)
		} else {
			names = append(names, id)
		}
	}
	return names
}

// resetForm handles reset form.
func (m *ContextModel) resetForm() {
	m.saved = false
	m.errText = ""
	m.typeSelecting = false
	m.focus = 0
	m.modeFocus = false
	m.typeIdx = 0
	m.scopeIdx = 0
	m.scopeSelecting = false
	m.tags = nil
	m.tagBuf = ""
	m.scopes = nil
	m.scopeBuf = ""
	m.linkSearching = false
	m.linkLoading = false
	m.linkQuery = ""
	m.linkResults = nil
	m.linkEntities = nil
	if m.linkList != nil {
		m.linkList.SetItems(nil)
	}
	for i := range m.fields {
		m.fields[i].value = ""
	}
}

// save handles save.
func (m ContextModel) save() (ContextModel, tea.Cmd) {
	title := strings.TrimSpace(m.fields[fieldTitle].value)
	if title == "" {
		m.errText = "Title is required"
		return m, nil
	}

	url := strings.TrimSpace(m.fields[fieldURL].value)
	sourceType := contextTypes[m.typeIdx]
	notes := strings.TrimSpace(m.fields[fieldNotes].value)

	m.commitTag()

	scopes := normalizeBulkScopes(m.scopes)
	if len(scopes) == 0 {
		scopes = []string{"private"}
	}

	input := api.CreateContextInput{
		Title:      title,
		URL:        url,
		SourceType: sourceType,
		Content:    notes,
		Scopes:     scopes,
		Tags:       m.tags,
	}

	linkIDs := make([]string, 0, len(m.linkEntities))
	for _, e := range m.linkEntities {
		linkIDs = append(linkIDs, e.ID)
	}

	m.saving = true
	return m, func() tea.Msg {
		created, err := m.client.CreateContext(input)
		if err != nil {
			return errMsg{err}
		}
		for _, id := range linkIDs {
			if err := m.client.LinkContext(created.ID, api.LinkContextInput{
				OwnerType: "entity",
				OwnerID:   id,
			}); err != nil {
				return errMsg{err}
			}
		}
		return contextSavedMsg{}
	}
}

// renderTags renders render tags.
func (m *ContextModel) renderTags(focused bool) string {
	if len(m.tags) == 0 && m.tagBuf == "" && !focused {
		return "-"
	}

	var b strings.Builder
	for i, t := range m.tags {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(AccentStyle.Render("[" + t + "]"))
	}
	if focused {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		if m.tagBuf != "" {
			b.WriteString(m.tagBuf)
		}
		b.WriteString(AccentStyle.Render("█"))
	} else if m.tagBuf != "" {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(MutedStyle.Render(m.tagBuf))
	}
	return b.String()
}

// renderEditTags renders render edit tags.
func (m *ContextModel) renderEditTags(focused bool) string {
	if len(m.editTags) == 0 && m.editTagBuf == "" && !focused {
		return "-"
	}

	var b strings.Builder
	for i, t := range m.editTags {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(AccentStyle.Render("[" + t + "]"))
	}
	if focused {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		if m.editTagBuf != "" {
			b.WriteString(m.editTagBuf)
		}
		b.WriteString(AccentStyle.Render("█"))
	} else if m.editTagBuf != "" {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(MutedStyle.Render(m.editTagBuf))
	}
	return b.String()
}

// renderScopes renders render scopes.
func (m *ContextModel) renderScopes(focused bool) string {
	return renderScopePills(m.scopes, focused)
}

// renderEditScopes renders render edit scopes.
func (m *ContextModel) renderEditScopes(focused bool) string {
	return renderScopePills(m.editScopes, focused)
}

// renderLinkedEntities renders render linked entities.
func (m *ContextModel) renderLinkedEntities(focused bool) string {
	if len(m.linkEntities) == 0 && !focused {
		return "-"
	}
	var b strings.Builder
	for i, e := range m.linkEntities {
		if i > 0 {
			b.WriteString(" ")
		}
		label := e.Name
		if label == "" {
			label = shortID(e.ID)
		}
		b.WriteString(AccentStyle.Render("[" + label + "]"))
	}
	return b.String()
}

// startLinkSearch handles start link search.
func (m *ContextModel) startLinkSearch() {
	m.linkSearching = true
	m.linkLoading = false
	m.linkQuery = ""
	m.linkResults = nil
	if m.linkList != nil {
		m.linkList.SetItems(nil)
	}
}

// handleLinkSearch handles handle link search.
func (m ContextModel) handleLinkSearch(msg tea.KeyMsg) (ContextModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.linkSearching = false
		m.linkLoading = false
		m.linkQuery = ""
		m.linkResults = nil
		if m.linkList != nil {
			m.linkList.SetItems(nil)
		}
	case isDown(msg):
		if m.linkList != nil {
			m.linkList.Down()
		}
	case isUp(msg):
		if m.linkList != nil {
			m.linkList.Up()
		}
	case isEnter(msg):
		if m.linkList != nil {
			if idx := m.linkList.Selected(); idx < len(m.linkResults) {
				m.addLinkedEntity(m.linkResults[idx])
			}
		}
		m.linkSearching = false
		m.linkLoading = false
		m.linkQuery = ""
		m.linkResults = nil
		if m.linkList != nil {
			m.linkList.SetItems(nil)
		}
	case isKey(msg, "backspace"):
		if len(m.linkQuery) > 0 {
			m.linkQuery = m.linkQuery[:len(m.linkQuery)-1]
			return m, m.updateLinkSearch()
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.linkQuery != "" {
			m.linkQuery = ""
			m.linkResults = nil
			if m.linkList != nil {
				m.linkList.SetItems(nil)
			}
			return m, nil
		}
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			if len(m.linkResults) > 0 {
				m.linkResults = nil
				if m.linkList != nil {
					m.linkList.SetItems(nil)
				}
			}
			m.linkQuery += msg.String()
			return m, m.updateLinkSearch()
		}
	}
	return m, nil
}

// renderLinkSearch renders render link search.
func (m ContextModel) renderLinkSearch() string {
	var b strings.Builder
	b.WriteString(MetaKeyStyle.Render("Search") + MetaPunctStyle.Render(": ") + SelectedStyle.Render(components.SanitizeText(m.linkQuery)))
	b.WriteString(AccentStyle.Render("█"))
	b.WriteString("\n\n")
	if m.linkLoading {
		b.WriteString(MutedStyle.Render("Searching..."))
	} else if strings.TrimSpace(m.linkQuery) == "" {
		b.WriteString(MutedStyle.Render("Type to search."))
	} else if len(m.linkResults) == 0 {
		b.WriteString(MutedStyle.Render("No matches."))
	} else {
		contentWidth := components.BoxContentWidth(m.width)
		visible := m.linkList.Visible()

		previewWidth := preferredPreviewWidth(contentWidth)

		gap := 3
		tableWidth := contentWidth
		sideBySide := contentWidth >= minSideBySideContentWidth
		if sideBySide {
			tableWidth = contentWidth - previewWidth - gap
		}

		sepWidth := 1
		if br := lipgloss.RoundedBorder().Left; br != "" {
			sepWidth = lipgloss.Width(br)
		}

		// 3 columns -> 2 separators.
		availableCols := tableWidth - (2 * sepWidth)
		if availableCols < 30 {
			availableCols = 30
		}

		typeWidth := 14
		statusWidth := 11
		nameWidth := availableCols - (typeWidth + statusWidth)
		if nameWidth < 16 {
			nameWidth = 16
			typeWidth = availableCols - (nameWidth + statusWidth)
			if typeWidth < 12 {
				typeWidth = 12
			}
		}

		cols := []components.TableColumn{
			{Header: "Name", Width: nameWidth, Align: lipgloss.Left},
			{Header: "Type", Width: typeWidth, Align: lipgloss.Left},
			{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		}

		tableRows := make([][]string, 0, len(visible))
		activeRowRel := -1
		var previewItem *api.Entity
		if idx := m.linkList.Selected(); idx >= 0 && idx < len(m.linkResults) {
			previewItem = &m.linkResults[idx]
		}

		for i := range visible {
			absIdx := m.linkList.RelToAbs(i)
			if absIdx < 0 || absIdx >= len(m.linkResults) {
				continue
			}
			e := m.linkResults[absIdx]

			name := strings.TrimSpace(components.SanitizeOneLine(e.Name))
			if name == "" {
				name = "entity"
			}
			typ := strings.TrimSpace(components.SanitizeOneLine(e.Type))
			if typ == "" {
				typ = "entity"
			}
			status := strings.TrimSpace(components.SanitizeOneLine(e.Status))
			if status == "" {
				status = "-"
			}

			if m.linkList.IsSelected(absIdx) {
				activeRowRel = len(tableRows)
			}
			tableRows = append(tableRows, []string{
				components.ClampTextWidthEllipsis(name, nameWidth),
				components.ClampTextWidthEllipsis(typ, typeWidth),
				components.ClampTextWidthEllipsis(status, statusWidth),
			})
		}

		countLine := MutedStyle.Render(fmt.Sprintf("%d results", len(m.linkResults)))
		table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
		preview := ""
		if previewItem != nil {
			content := m.renderLinkEntityPreview(*previewItem, previewBoxContentWidth(previewWidth))
			preview = renderPreviewBox(content, previewWidth)
		}

		body := table
		if sideBySide && preview != "" {
			body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
		} else if preview != "" {
			body = table + "\n\n" + preview
		}

		b.WriteString(countLine)
		b.WriteString("\n\n")
		b.WriteString(body)
	}
	return components.Indent(components.TitledBox("Link Entity", b.String(), m.width), 1)
}

// renderLinkEntityPreview renders render link entity preview.
func (m ContextModel) renderLinkEntityPreview(e api.Entity, width int) string {
	if width <= 0 {
		return ""
	}

	name := strings.TrimSpace(components.SanitizeOneLine(e.Name))
	if name == "" {
		name = "entity"
	}
	typ := strings.TrimSpace(components.SanitizeOneLine(e.Type))
	if typ == "" {
		typ = "entity"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(e.Status))
	if status == "" {
		status = "-"
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(name, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Type", typ, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	if len(e.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(e.Tags, ", "), width))
	}

	return padPreviewLines(lines, width)
}

// searchLinkEntities handles search link entities.
func (m ContextModel) searchLinkEntities(query string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryEntities(api.QueryParams{"search_text": query})
		if err != nil {
			return errMsg{err}
		}
		return contextLinkResultsMsg{items: items}
	}
}

// updateLinkSearch updates update link search.
func (m *ContextModel) updateLinkSearch() tea.Cmd {
	query := strings.TrimSpace(m.linkQuery)
	if query == "" {
		m.linkLoading = false
		m.linkResults = nil
		if m.linkList != nil {
			m.linkList.SetItems(nil)
		}
		return nil
	}
	m.linkLoading = true
	return m.searchLinkEntities(query)
}

// addLinkedEntity handles add linked entity.
func (m *ContextModel) addLinkedEntity(entity api.Entity) {
	for _, e := range m.linkEntities {
		if e.ID == entity.ID {
			return
		}
	}
	m.linkEntities = append(m.linkEntities, entity)
}

// commitTag handles commit tag.
func (m *ContextModel) commitTag() {
	raw := strings.TrimSpace(m.tagBuf)
	if raw == "" {
		m.tagBuf = ""
		return
	}

	tag := normalizeTag(raw)
	if tag == "" {
		m.tagBuf = ""
		return
	}

	for _, t := range m.tags {
		if t == tag {
			m.tagBuf = ""
			return
		}
	}
	m.tags = append(m.tags, tag)
	m.tagBuf = ""
}

// commitScope handles commit scope.
func (m *ContextModel) commitScope() {
	raw := strings.TrimSpace(m.scopeBuf)
	if raw == "" {
		m.scopeBuf = ""
		return
	}

	scope := normalizeScope(raw)
	if scope == "" {
		m.scopeBuf = ""
		return
	}

	for _, s := range m.scopes {
		if s == scope {
			m.scopeBuf = ""
			return
		}
	}
	m.scopes = append(m.scopes, scope)
	m.scopeBuf = ""
}

// commitEditTag handles commit edit tag.
func (m *ContextModel) commitEditTag() {
	raw := strings.TrimSpace(m.editTagBuf)
	if raw == "" {
		m.editTagBuf = ""
		return
	}

	tag := normalizeTag(raw)
	if tag == "" {
		m.editTagBuf = ""
		return
	}

	for _, t := range m.editTags {
		if t == tag {
			m.editTagBuf = ""
			return
		}
	}
	m.editTags = append(m.editTags, tag)
	m.editTagBuf = ""
}

// commitEditScope handles commit edit scope.
func (m *ContextModel) commitEditScope() {
	raw := strings.TrimSpace(m.editScopeBuf)
	if raw == "" {
		m.editScopeBuf = ""
		return
	}

	scope := normalizeScope(raw)
	if scope == "" {
		m.editScopeBuf = ""
		return
	}

	for _, s := range m.editScopes {
		if s == scope {
			m.editScopeBuf = ""
			return
		}
	}
	m.editScopes = append(m.editScopes, scope)
	m.editScopeBuf = ""
}

// normalizeTag handles normalize tag.
func normalizeTag(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Join(strings.Fields(s), "-")
	return s
}

// normalizeScope handles normalize scope.
func normalizeScope(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), "-")
	return s
}

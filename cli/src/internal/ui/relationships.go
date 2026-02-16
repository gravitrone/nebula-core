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

type relTabLoadedMsg struct{ items []api.Relationship }
type relTabNamesLoadedMsg struct{ names map[string]string }
type relTabSavedMsg struct{}
type relTabScopesLoadedMsg struct{ options []string }
type relTabEntityCacheLoadedMsg struct{ items []api.Entity }

type relTabResultsMsg struct {
	query string
	items []api.Entity
}

// --- View States ---

type relationshipsView int

const (
	relsViewList relationshipsView = iota
	relsViewDetail
	relsViewEdit
	relsViewConfirm
	relsViewCreateSourceSearch
	relsViewCreateSourceSelect
	relsViewCreateTargetSearch
	relsViewCreateTargetSelect
	relsViewCreateType
)

const (
	relsEditFieldStatus = iota
	relsEditFieldProperties
	relsEditFieldCount
)

var relsStatusOptions = []string{"active", "archived"}

// --- Relationships Model ---

type RelationshipsModel struct {
	client    *api.Client
	items     []api.Relationship
	list      *components.List
	loading   bool
	view      relationshipsView
	modeFocus bool
	width     int
	height    int

	names        map[string]string
	scopeOptions []string
	entityCache  []api.Entity

	detail        *api.Relationship
	metaExpanded  bool
	editFocus     int
	editStatusIdx int
	editMeta      MetadataEditor
	editSaving    bool

	confirmKind string

	// create flow
	createQuery       string
	createResults     []api.Entity
	createList        *components.List
	createSource      *api.Entity
	createTarget      *api.Entity
	createType        string
	createTypeResults []string
	createTypeList    *components.List
	createTypeNav     bool
	createLoading     bool

	typeOptions []string
}

// NewRelationshipsModel builds the relationships UI model.
func NewRelationshipsModel(client *api.Client) RelationshipsModel {
	return RelationshipsModel{
		client:         client,
		list:           components.NewList(12),
		createList:     components.NewList(8),
		createTypeList: components.NewList(6),
		view:           relsViewList,
		names:          map[string]string{},
	}
}

func (m RelationshipsModel) Init() tea.Cmd {
	m.view = relsViewList
	m.loading = true
	m.modeFocus = false
	m.metaExpanded = false
	m.editMeta.Reset()
	return tea.Batch(m.loadRelationships(), m.loadScopeOptions(), m.loadEntityCache())
}

func (m RelationshipsModel) Update(msg tea.Msg) (RelationshipsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case relTabLoadedMsg:
		m.loading = false
		m.items = msg.items
		m.list.SetItems(m.buildListLabels())
		m.typeOptions = uniqueRelationshipTypes(msg.items)
		return m, m.loadRelationshipNames(msg.items)

	case relTabNamesLoadedMsg:
		if m.names == nil {
			m.names = map[string]string{}
		}
		for id, name := range msg.names {
			m.names[id] = name
		}
		m.list.SetItems(m.buildListLabels())
		return m, nil
	case relTabScopesLoadedMsg:
		m.scopeOptions = msg.options
		m.editMeta.SetScopeOptions(m.scopeOptions)
		return m, nil
	case relTabEntityCacheLoadedMsg:
		m.entityCache = msg.items
		return m, nil

	case relTabResultsMsg:
		if strings.TrimSpace(msg.query) != strings.TrimSpace(m.createQuery) {
			return m, nil
		}
		m.createLoading = false
		m.createResults = msg.items
		labels := make([]string, len(msg.items))
		for i, e := range msg.items {
			labels[i] = formatEntityLine(e)
		}
		m.createList.SetItems(labels)
		return m, nil

	case relTabSavedMsg:
		m.editSaving = false
		m.loading = true
		m.view = relsViewList
		return m, m.loadRelationships()

	case errMsg:
		m.loading = false
		m.editSaving = false
		m.createLoading = false
		return m, nil

	case tea.KeyMsg:
		if m.editMeta.Active {
			m.editMeta.HandleKey(msg)
			return m, nil
		}
		if m.modeFocus {
			return m.handleModeKeys(msg)
		}
		switch m.view {
		case relsViewDetail:
			return m.handleDetailKeys(msg)
		case relsViewEdit:
			return m.handleEditKeys(msg)
		case relsViewConfirm:
			return m.handleConfirmKeys(msg)
		case relsViewCreateSourceSearch, relsViewCreateSourceSelect, relsViewCreateTargetSearch, relsViewCreateTargetSelect, relsViewCreateType:
			return m.handleCreateKeys(msg)
		default:
			return m.handleListKeys(msg)
		}
	}
	return m, nil
}

func (m RelationshipsModel) View() string {
	if m.editMeta.Active {
		return m.editMeta.Render(m.width)
	}

	modeLine := m.renderModeLine()
	var body string
	switch m.view {
	case relsViewDetail:
		body = m.renderDetail()
	case relsViewEdit:
		body = m.renderEdit()
	case relsViewConfirm:
		body = m.renderConfirm()
	case relsViewCreateSourceSearch, relsViewCreateSourceSelect, relsViewCreateTargetSearch, relsViewCreateTargetSelect, relsViewCreateType:
		body = m.renderCreate()
	default:
		body = m.renderList()
	}
	if modeLine != "" {
		body = components.CenterLine(modeLine, m.width) + "\n\n" + body
	}
	return components.Indent(body, 1)
}

// --- List ---

func (m RelationshipsModel) handleListKeys(msg tea.KeyMsg) (RelationshipsModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.list.Down()
	case isUp(msg):
		if m.list.Selected() == 0 {
			m.modeFocus = true
		} else {
			m.list.Up()
		}
	case isEnter(msg), isSpace(msg):
		if rel := m.selectedRelationship(); rel != nil {
			m.detail = rel
			m.view = relsViewDetail
		}
	case isKey(msg, "n"):
		m.startCreate()
		m.view = relsViewCreateSourceSearch
	}
	return m, nil
}

// --- Mode Line ---

func (m RelationshipsModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("Library")
	if m.isAddView() {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("Library")
	}
	line := add + " " + list
	if m.modeFocus {
		return SelectedStyle.Render("› " + line)
	}
	return line
}

func (m RelationshipsModel) handleModeKeys(msg tea.KeyMsg) (RelationshipsModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.modeFocus = false
	case isUp(msg):
		m.modeFocus = false
	case isKey(msg, "left"), isKey(msg, "right"), isSpace(msg), isEnter(msg):
		return m.toggleMode()
	case isBack(msg):
		m.modeFocus = false
	}
	return m, nil
}

func (m RelationshipsModel) toggleMode() (RelationshipsModel, tea.Cmd) {
	m.modeFocus = false
	if m.isAddView() {
		m.view = relsViewList
		return m, nil
	}
	m.startCreate()
	m.view = relsViewCreateSourceSearch
	return m, nil
}

func (m RelationshipsModel) isAddView() bool {
	switch m.view {
	case relsViewCreateSourceSearch, relsViewCreateSourceSelect, relsViewCreateTargetSearch, relsViewCreateTargetSelect, relsViewCreateType:
		return true
	default:
		return false
	}
}

func (m RelationshipsModel) renderList() string {
	if m.loading {
		return "  " + MutedStyle.Render("Loading relationships...")
	}

	if len(m.items) == 0 {
		return components.EmptyStateBox(
			"Relationships",
			"No relationships found.",
			[]string{"Press n to create", "Press / for command palette"},
			m.width,
		)
	}

	contentWidth := components.BoxContentWidth(m.width)
	visible := m.list.Visible()

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= 110
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap
		if tableWidth < 60 {
			sideBySide = false
			tableWidth = contentWidth
		}
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

	relWidth := 12
	statusWidth := 9
	atWidth := 11
	edgeWidth := availableCols - (relWidth + statusWidth + atWidth)
	if edgeWidth < 12 {
		edgeWidth = 12
	}
	cols := []components.TableColumn{
		{Header: "Rel", Width: relWidth, Align: lipgloss.Left},
		{Header: "Edge", Width: edgeWidth, Align: lipgloss.Left},
		{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.Relationship
	if idx := m.list.Selected(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}
	for i := range visible {
		absIdx := m.list.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.items) {
			continue
		}
		rel := m.items[absIdx]
		relType := strings.TrimSpace(components.SanitizeOneLine(rel.Type))
		if relType == "" {
			relType = "-"
		}
		source := m.displayNode(rel.SourceID, rel.SourceType, rel.SourceName)
		target := m.displayNode(rel.TargetID, rel.TargetType, rel.TargetName)
		edge := components.SanitizeOneLine(fmt.Sprintf("%s -> %s", source, target))
		status := strings.TrimSpace(components.SanitizeOneLine(rel.Status))
		if status == "" {
			status = "-"
		}
		when := formatLocalTimeCompact(rel.CreatedAt)

		if m.list.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(relType, relWidth),
			components.ClampTextWidthEllipsis(edge, edgeWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			when,
		})
	}

	title := "Relationships"
	countLine := MutedStyle.Render(fmt.Sprintf("%d total", len(m.items)))

	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderRelationshipPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.TitledBox(title, content, m.width)
}

// --- Detail ---

func (m RelationshipsModel) handleDetailKeys(msg tea.KeyMsg) (RelationshipsModel, tea.Cmd) {
	switch {
	case isUp(msg):
		m.modeFocus = true
	case isBack(msg):
		m.detail = nil
		m.metaExpanded = false
		m.view = relsViewList
	case isKey(msg, "e"):
		m.startEdit()
		m.view = relsViewEdit
	case isKey(msg, "d"):
		m.confirmKind = "archive"
		m.view = relsViewConfirm
	case isKey(msg, "m"):
		m.metaExpanded = !m.metaExpanded
	}
	return m, nil
}

func (m RelationshipsModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}
	rel := m.detail
	rows := []components.TableRow{
		{Label: "ID", Value: rel.ID},
		{Label: "Type", Value: rel.Type},
		{Label: "Status", Value: rel.Status},
		{Label: "Source", Value: m.displayNode(rel.SourceID, rel.SourceType, rel.SourceName)},
		{Label: "Target", Value: m.displayNode(rel.TargetID, rel.TargetType, rel.TargetName)},
		{Label: "Created", Value: formatLocalTimeFull(rel.CreatedAt)},
	}

	sections := []string{components.Table("Relationship", rows, m.width)}
	if len(rel.Properties) > 0 {
		props := renderMetadataBlock(map[string]any(rel.Properties), m.width, m.metaExpanded)
		if props != "" {
			sections = append(sections, props)
		}
	}

	return strings.Join(sections, "\n\n")
}

func (m RelationshipsModel) renderRelationshipPreview(rel api.Relationship, width int) string {
	if width <= 0 {
		return ""
	}
	relType := strings.TrimSpace(components.SanitizeOneLine(rel.Type))
	if relType == "" {
		relType = "-"
	}
	source := m.displayNode(rel.SourceID, rel.SourceType, rel.SourceName)
	target := m.displayNode(rel.TargetID, rel.TargetType, rel.TargetName)
	status := strings.TrimSpace(components.SanitizeOneLine(rel.Status))
	if status == "" {
		status = "-"
	}

	title := components.SanitizeOneLine(fmt.Sprintf("%s (%s -> %s)", relType, source, target))

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Rel", relType, width))
	lines = append(lines, renderPreviewRow("From", source, width))
	lines = append(lines, renderPreviewRow("To", target, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeCompact(rel.CreatedAt), width))

	return padPreviewLines(lines, width)
}

// --- Edit ---

func (m *RelationshipsModel) startEdit() {
	if m.detail == nil {
		return
	}
	m.editFocus = relsEditFieldStatus
	m.editStatusIdx = statusIndex(relsStatusOptions, m.detail.Status)
	m.editMeta.Reset()
	m.editMeta.Load(map[string]any(m.detail.Properties))
	m.editSaving = false
}

func (m RelationshipsModel) handleEditKeys(msg tea.KeyMsg) (RelationshipsModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	switch {
	case isDown(msg):
		m.editFocus = (m.editFocus + 1) % relsEditFieldCount
	case isUp(msg):
		if m.editFocus > 0 {
			m.editFocus = (m.editFocus - 1 + relsEditFieldCount) % relsEditFieldCount
		}
	case isBack(msg):
		m.view = relsViewDetail
	case isKey(msg, "ctrl+s"):
		return m.saveEdit()
	case isKey(msg, "backspace"):
		return m, nil
	default:
		if m.editFocus == relsEditFieldStatus {
			switch {
			case isKey(msg, "left"):
				m.editStatusIdx = (m.editStatusIdx - 1 + len(relsStatusOptions)) % len(relsStatusOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.editStatusIdx = (m.editStatusIdx + 1) % len(relsStatusOptions)
			}
		} else if m.editFocus == relsEditFieldProperties {
			if isEnter(msg) {
				m.editMeta.Active = true
			}
		}
	}
	return m, nil
}

func (m RelationshipsModel) renderEdit() string {
	status := relsStatusOptions[m.editStatusIdx]
	var b strings.Builder

	if m.editFocus == relsEditFieldStatus {
		b.WriteString(SelectedStyle.Render("> Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	} else {
		b.WriteString(MutedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	}

	b.WriteString("\n\n")

	if m.editFocus == relsEditFieldProperties {
		b.WriteString(SelectedStyle.Render("> Properties:"))
	} else {
		b.WriteString(MutedStyle.Render("  Properties:"))
	}
	b.WriteString("\n")
	props := renderMetadataInput(m.editMeta.Buffer)
	b.WriteString(NormalStyle.Render("  " + props))

	if m.editSaving {
		b.WriteString("\n\n" + MutedStyle.Render("Saving..."))
	}

	return components.TitledBox("Edit Relationship", b.String(), m.width)
}

func (m RelationshipsModel) saveEdit() (RelationshipsModel, tea.Cmd) {
	if m.detail == nil {
		return m, nil
	}
	status := relsStatusOptions[m.editStatusIdx]
	input := api.UpdateRelationshipInput{Status: &status}
	props, err := parseMetadataInput(m.editMeta.Buffer)
	if err != nil {
		return m, nil
	}
	props = mergeMetadataScopes(props, m.editMeta.Scopes)
	if len(props) > 0 {
		input.Properties = props
	}

	m.editSaving = true
	return m, func() tea.Msg {
		_, err := m.client.UpdateRelationship(m.detail.ID, input)
		if err != nil {
			return errMsg{err}
		}
		return relTabSavedMsg{}
	}
}

// --- Confirm ---

func (m RelationshipsModel) handleConfirmKeys(msg tea.KeyMsg) (RelationshipsModel, tea.Cmd) {
	switch {
	case isKey(msg, "y"), isEnter(msg):
		status := "archived"
		input := api.UpdateRelationshipInput{Status: &status}
		m.view = relsViewDetail
		return m, func() tea.Msg {
			_, err := m.client.UpdateRelationship(m.detail.ID, input)
			if err != nil {
				return errMsg{err}
			}
			return relTabSavedMsg{}
		}
	case isKey(msg, "n"), isBack(msg):
		m.view = relsViewDetail
	}
	return m, nil
}

func (m RelationshipsModel) renderConfirm() string {
	if m.detail == nil {
		return components.ConfirmDialog("Confirm", "Archive this relationship?")
	}

	summary := []components.TableRow{
		{Label: "Relationship", Value: m.detail.Type},
		{Label: "ID", Value: m.detail.ID},
		{Label: "Source", Value: m.displayNode(m.detail.SourceID, m.detail.SourceType, m.detail.SourceName)},
		{Label: "Target", Value: m.displayNode(m.detail.TargetID, m.detail.TargetType, m.detail.TargetName)},
	}
	diffs := []components.DiffRow{{
		Label: "status",
		From:  firstNonEmpty(m.detail.Status, "active"),
		To:    "archived",
	}}
	return components.ConfirmPreviewDialog("Archive Relationship", summary, diffs, m.width)
}

// --- Create ---

func (m *RelationshipsModel) startCreate() {
	m.createQuery = ""
	m.createResults = nil
	m.createList.SetItems(nil)
	m.createSource = nil
	m.createTarget = nil
	m.createType = ""
	m.createTypeResults = nil
	m.createTypeList.SetItems(nil)
	m.createTypeNav = false
	m.createLoading = false
}

func (m RelationshipsModel) handleCreateKeys(msg tea.KeyMsg) (RelationshipsModel, tea.Cmd) {
	switch m.view {
	case relsViewCreateSourceSearch:
		switch {
		case isBack(msg):
			if m.createQuery != "" {
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.createLoading = false
				return m, nil
			}
			m.view = relsViewList
		case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
			if m.createQuery != "" {
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.createLoading = false
				return m, nil
			}
			m.view = relsViewList
		case isDown(msg):
			m.createList.Down()
		case isUp(msg):
			m.createList.Up()
		case isEnter(msg):
			if idx := m.createList.Selected(); idx < len(m.createResults) {
				item := m.createResults[idx]
				m.createSource = &item
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateTargetSearch
			}
		case isKey(msg, "backspace", "delete"):
			if len(m.createQuery) > 0 {
				m.createQuery = m.createQuery[:len(m.createQuery)-1]
				return m, m.updateCreateSearch()
			}
		default:
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				if ch == " " && m.createQuery == "" {
					return m, nil
				}
				m.createQuery += ch
				return m, m.updateCreateSearch()
			}
		}
	case relsViewCreateSourceSelect:
		switch {
		case isBack(msg):
			m.view = relsViewCreateSourceSearch
		case isDown(msg):
			m.createList.Down()
		case isUp(msg):
			m.createList.Up()
		case isEnter(msg):
			if idx := m.createList.Selected(); idx < len(m.createResults) {
				item := m.createResults[idx]
				m.createSource = &item
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateTargetSearch
			}
		}
	case relsViewCreateTargetSearch:
		switch {
		case isBack(msg):
			if m.createQuery != "" {
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.createLoading = false
				return m, nil
			}
			m.view = relsViewCreateSourceSearch
		case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
			if m.createQuery != "" {
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.createLoading = false
				return m, nil
			}
			m.view = relsViewCreateSourceSearch
		case isDown(msg):
			m.createList.Down()
		case isUp(msg):
			m.createList.Up()
		case isEnter(msg):
			if idx := m.createList.Selected(); idx < len(m.createResults) {
				item := m.createResults[idx]
				m.createTarget = &item
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateType
				m.resetTypeSuggestions()
			}
		case isKey(msg, "backspace", "delete"):
			if len(m.createQuery) > 0 {
				m.createQuery = m.createQuery[:len(m.createQuery)-1]
				return m, m.updateCreateSearch()
			}
		default:
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				if ch == " " && m.createQuery == "" {
					return m, nil
				}
				m.createQuery += ch
				return m, m.updateCreateSearch()
			}
		}
	case relsViewCreateTargetSelect:
		switch {
		case isBack(msg):
			m.view = relsViewCreateTargetSearch
		case isDown(msg):
			m.createList.Down()
		case isUp(msg):
			m.createList.Up()
		case isEnter(msg):
			if idx := m.createList.Selected(); idx < len(m.createResults) {
				item := m.createResults[idx]
				m.createTarget = &item
				m.createQuery = ""
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateType
				m.resetTypeSuggestions()
			}
		}
	case relsViewCreateType:
		switch {
		case isBack(msg):
			m.view = relsViewCreateTargetSearch
		case isDown(msg):
			if len(m.createTypeResults) > 0 {
				m.createTypeNav = true
				m.createTypeList.Down()
			}
		case isUp(msg):
			if len(m.createTypeResults) > 0 {
				m.createTypeNav = true
				m.createTypeList.Up()
			}
		case isEnter(msg):
			kind := strings.TrimSpace(m.createType)
			if m.createTypeNav && len(m.createTypeResults) > 0 {
				if idx := m.createTypeList.Selected(); idx < len(m.createTypeResults) {
					kind = m.createTypeResults[idx]
				}
			}
			if kind == "" || m.createSource == nil || m.createTarget == nil {
				return m, nil
			}
			m.view = relsViewList
			m.loading = true
			return m, m.createRelationship(*m.createSource, *m.createTarget, kind)
		case isKey(msg, "backspace", "delete"):
			if len(m.createType) > 0 {
				m.createType = m.createType[:len(m.createType)-1]
				m.createTypeNav = false
				m.updateTypeSuggestions()
			}
		case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
			if m.createType != "" {
				m.createType = ""
				m.createTypeNav = false
				m.updateTypeSuggestions()
				return m, nil
			}
			m.view = relsViewCreateTargetSearch
		default:
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				if ch == " " && m.createType == "" {
					return m, nil
				}
				m.createType += ch
				m.createTypeNav = false
				m.updateTypeSuggestions()
			}
		}
	}
	return m, nil
}

func (m RelationshipsModel) renderCreate() string {
	switch m.view {
	case relsViewCreateSourceSearch, relsViewCreateSourceSelect:
		return m.renderCreateSearch("Source Entity")
	case relsViewCreateTargetSearch, relsViewCreateTargetSelect:
		return m.renderCreateSearch("Target Entity")
	case relsViewCreateType:
		return m.renderCreateType()
	}
	return ""
}

func (m RelationshipsModel) renderCreateSearch(title string) string {
	var b strings.Builder
	b.WriteString(MetaKeyStyle.Render("Search") + MetaPunctStyle.Render(": ") + SelectedStyle.Render(components.SanitizeText(m.createQuery)))
	b.WriteString(AccentStyle.Render("█"))
	b.WriteString("\n\n")

	if m.createLoading {
		b.WriteString(MutedStyle.Render("Searching..."))
	} else if strings.TrimSpace(m.createQuery) == "" {
		b.WriteString(MutedStyle.Render("Type to search."))
	} else if len(m.createResults) == 0 {
		b.WriteString(MutedStyle.Render("No matches."))
	} else {
		contentWidth := components.BoxContentWidth(m.width)
		visible := m.createList.Visible()

		previewWidth := preferredPreviewWidth(contentWidth)

		gap := 3
		tableWidth := contentWidth
		sideBySide := contentWidth >= 110
		if sideBySide {
			tableWidth = contentWidth - previewWidth - gap
			if tableWidth < 60 {
				sideBySide = false
				tableWidth = contentWidth
			}
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
		if idx := m.createList.Selected(); idx >= 0 && idx < len(m.createResults) {
			previewItem = &m.createResults[idx]
		}

		for i := range visible {
			absIdx := m.createList.RelToAbs(i)
			if absIdx < 0 || absIdx >= len(m.createResults) {
				continue
			}
			e := m.createResults[absIdx]

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

			if m.createList.IsSelected(absIdx) {
				activeRowRel = len(tableRows)
			}

			tableRows = append(tableRows, []string{
				components.ClampTextWidthEllipsis(name, nameWidth),
				components.ClampTextWidthEllipsis(typ, typeWidth),
				components.ClampTextWidthEllipsis(status, statusWidth),
			})
		}

		countLine := MutedStyle.Render(fmt.Sprintf("%d results", len(m.createResults)))
		table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
		preview := ""
		if previewItem != nil {
			content := m.renderCreateEntityPreview(*previewItem, previewBoxContentWidth(previewWidth))
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

	return components.TitledBox(title, b.String(), m.width)
}

func (m RelationshipsModel) renderCreateType() string {
	var b strings.Builder
	b.WriteString(MetaKeyStyle.Render("Type") + MetaPunctStyle.Render(": ") + SelectedStyle.Render(components.SanitizeText(m.createType)))
	b.WriteString(AccentStyle.Render("█"))
	b.WriteString("\n\n")

	if strings.TrimSpace(m.createType) == "" && len(m.typeOptions) == 0 {
		b.WriteString(MutedStyle.Render("Type a relationship type."))
	} else if len(m.createTypeResults) == 0 {
		b.WriteString(MutedStyle.Render("No suggestions."))
	} else {
		contentWidth := components.BoxContentWidth(m.width)
		visible := m.createTypeList.Visible()

		previewWidth := preferredPreviewWidth(contentWidth)

		gap := 3
		tableWidth := contentWidth
		sideBySide := contentWidth >= 110
		if sideBySide {
			tableWidth = contentWidth - previewWidth - gap
			if tableWidth < 60 {
				sideBySide = false
				tableWidth = contentWidth
			}
		}

		cols := []components.TableColumn{
			{Header: "Suggestion", Width: tableWidth, Align: lipgloss.Left},
		}

		tableRows := make([][]string, 0, len(visible))
		activeRowRel := -1
		var selectedSuggestion string
		if idx := m.createTypeList.Selected(); idx >= 0 && idx < len(m.createTypeResults) {
			selectedSuggestion = m.createTypeResults[idx]
		}

		for i := range visible {
			absIdx := m.createTypeList.RelToAbs(i)
			if absIdx < 0 || absIdx >= len(m.createTypeResults) {
				continue
			}
			s := strings.TrimSpace(components.SanitizeOneLine(m.createTypeResults[absIdx]))
			if s == "" {
				s = "-"
			}

			if m.createTypeList.IsSelected(absIdx) {
				activeRowRel = len(tableRows)
			}

			tableRows = append(tableRows, []string{
				components.ClampTextWidthEllipsis(s, tableWidth),
			})
		}

		countLine := MutedStyle.Render(fmt.Sprintf("%d suggestions", len(m.createTypeResults)))
		table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
		preview := ""
		if strings.TrimSpace(selectedSuggestion) != "" {
			content := m.renderCreateTypePreview(selectedSuggestion, previewBoxContentWidth(previewWidth))
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

	return components.TitledBox("Relationship Type", b.String(), m.width)
}

func (m RelationshipsModel) renderCreateEntityPreview(e api.Entity, width int) string {
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
	if metaPreview := metadataPreview(map[string]any(e.Metadata), 80); metaPreview != "" {
		lines = append(lines, renderPreviewRow("Meta", metaPreview, width))
	}

	return padPreviewLines(lines, width)
}

func (m RelationshipsModel) renderCreateTypePreview(suggestion string, width int) string {
	if width <= 0 {
		return ""
	}

	suggestion = strings.TrimSpace(components.SanitizeOneLine(suggestion))
	if suggestion == "" {
		suggestion = "relationship"
	}

	src := "-"
	if m.createSource != nil && strings.TrimSpace(m.createSource.Name) != "" {
		src = components.SanitizeOneLine(m.createSource.Name)
	}
	tgt := "-"
	if m.createTarget != nil && strings.TrimSpace(m.createTarget.Name) != "" {
		tgt = components.SanitizeOneLine(m.createTarget.Name)
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(suggestion, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Source", src, width))
	lines = append(lines, renderPreviewRow("Target", tgt, width))

	return padPreviewLines(lines, width)
}

// --- Helpers ---

func (m RelationshipsModel) loadRelationships() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryRelationships(api.QueryParams{
			"status_category": "active",
			"limit":           "50",
		})
		if err != nil {
			return errMsg{err}
		}
		return relTabLoadedMsg{items: items}
	}
}

func (m RelationshipsModel) loadRelationshipNames(items []api.Relationship) tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		names := map[string]string{}
		ids := map[string]string{}
		for _, rel := range items {
			if strings.TrimSpace(rel.SourceName) != "" {
				names[rel.SourceID] = rel.SourceName
			} else if rel.SourceID != "" {
				ids[rel.SourceID] = strings.TrimSpace(rel.SourceType)
			}
			if strings.TrimSpace(rel.TargetName) != "" {
				names[rel.TargetID] = rel.TargetName
			} else if rel.TargetID != "" {
				ids[rel.TargetID] = strings.TrimSpace(rel.TargetType)
			}
		}
		for id, typ := range ids {
			if _, ok := names[id]; ok {
				continue
			}
			switch typ {
			case "", "entity":
				ent, err := m.client.GetEntity(id)
				if err == nil && ent != nil && ent.Name != "" {
					names[id] = ent.Name
				}
			case "context":
				ki, err := m.client.GetContext(id)
				if err == nil && ki != nil && ki.Name != "" {
					names[id] = ki.Name
				}
			case "job":
				job, err := m.client.GetJob(id)
				if err == nil && job != nil && job.Title != "" {
					names[id] = job.Title
				}
			}
		}
		return relTabNamesLoadedMsg{names: names}
	}
}

func (m RelationshipsModel) buildListLabels() []string {
	labels := make([]string, len(m.items))
	for i, rel := range m.items {
		source := m.displayNode(rel.SourceID, rel.SourceType, rel.SourceName)
		target := m.displayNode(rel.TargetID, rel.TargetType, rel.TargetName)
		labels[i] = fmt.Sprintf(
			"%s · %s -> %s",
			components.SanitizeText(rel.Type),
			components.SanitizeText(source),
			components.SanitizeText(target),
		)
	}
	return labels
}

func (m RelationshipsModel) displayNode(id, typ, name string) string {
	if strings.TrimSpace(name) != "" {
		return components.SanitizeText(name)
	}
	if label, ok := m.names[id]; ok && label != "" {
		return components.SanitizeText(label)
	}
	switch strings.TrimSpace(typ) {
	case "", "entity":
		return "unknown entity"
	case "context":
		return "unknown context"
	case "job":
		return "unknown job"
	default:
		return "unknown"
	}
}

func (m RelationshipsModel) selectedRelationship() *api.Relationship {
	if len(m.items) == 0 {
		return nil
	}
	idx := m.list.Selected()
	if idx < 0 || idx >= len(m.items) {
		return nil
	}
	return &m.items[idx]
}

func (m RelationshipsModel) searchEntities(query string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryEntities(api.QueryParams{"search_text": query})
		if err != nil {
			return errMsg{err}
		}
		return relTabResultsMsg{query: query, items: items}
	}
}

func (m RelationshipsModel) createRelationship(source api.Entity, target api.Entity, relType string) tea.Cmd {
	return func() tea.Msg {
		input := api.CreateRelationshipInput{
			SourceType: "entity",
			SourceID:   source.ID,
			TargetType: "entity",
			TargetID:   target.ID,
			Type:       relType,
		}
		_, err := m.client.CreateRelationship(input)
		if err != nil {
			return errMsg{err}
		}
		return relTabSavedMsg{}
	}
}

func (m *RelationshipsModel) updateCreateSearch() tea.Cmd {
	query := strings.TrimSpace(m.createQuery)
	if query == "" {
		m.createLoading = false
		m.createResults = nil
		m.createList.SetItems(nil)
		return nil
	}
	if len(m.entityCache) > 0 {
		m.createLoading = false
		m.createResults = filterEntitiesByQuery(m.entityCache, query)
		labels := make([]string, len(m.createResults))
		for i, e := range m.createResults {
			labels[i] = formatEntityLine(e)
		}
		m.createList.SetItems(labels)
		return nil
	}
	m.createLoading = true
	return m.searchEntities(query)
}

func (m *RelationshipsModel) resetTypeSuggestions() {
	m.createTypeResults = filterRelationshipTypes(m.typeOptions, "")
	m.createTypeList.SetItems(m.createTypeResults)
	m.createTypeNav = false
}

func (m *RelationshipsModel) updateTypeSuggestions() {
	m.createTypeResults = filterRelationshipTypes(m.typeOptions, m.createType)
	m.createTypeList.SetItems(m.createTypeResults)
}

func (m RelationshipsModel) loadScopeOptions() tea.Cmd {
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
		return relTabScopesLoadedMsg{options: scopeNameList(names)}
	}
}

func (m RelationshipsModel) loadEntityCache() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		items, err := m.client.QueryEntities(api.QueryParams{})
		if err != nil {
			return errMsg{err}
		}
		return relTabEntityCacheLoadedMsg{items: items}
	}
}

func uniqueRelationshipTypes(items []api.Relationship) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, rel := range items {
		typ := strings.TrimSpace(rel.Type)
		if typ == "" {
			continue
		}
		lower := strings.ToLower(typ)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, lower)
	}
	return out
}

func filterRelationshipTypes(options []string, query string) []string {
	if len(options) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return append([]string{}, options...)
	}
	out := make([]string, 0, len(options))
	for _, opt := range options {
		if strings.Contains(strings.ToLower(opt), q) {
			out = append(out, opt)
		}
	}
	return out
}

package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type relTabLoadedMsg struct{ items []api.Relationship }
type relTabNamesLoadedMsg struct{ names map[string]string }
type relTabSavedMsg struct{}
type relTabScopesLoadedMsg struct{ options []string }
type relTabEntityCacheLoadedMsg struct{ items []api.Entity }
type relTabContextCacheLoadedMsg struct{ items []api.Context }
type relTabJobCacheLoadedMsg struct{ items []api.Job }

type relTabResultsMsg struct {
	query string
	items []relationshipCreateCandidate
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

var relsStatusOptions = []string{"active", "inactive"}

type relationshipCreateCandidate struct {
	ID       string
	NodeType string
	Name     string
	Kind     string
	Status   string
	Tags     []string
}

// --- Relationships Model ---

type RelationshipsModel struct {
	client    *api.Client
	items     []api.Relationship
	allItems  []api.Relationship
	list      *components.List
	loading   bool
	spinner   spinner.Model
	view      relationshipsView
	modeFocus   bool
	filtering   bool
	filterInput textinput.Model
	width       int
	height    int

	names        map[string]string
	scopeOptions []string
	entityCache  []api.Entity
	contextCache []api.Context
	jobCache     []api.Job

	detail        *api.Relationship
	metaExpanded  bool
	editFocus     int
	editStatusIdx int
	editMeta      MetadataEditor
	editSaving    bool

	confirmKind string

	// create flow
	createQueryInput  textinput.Model
	createResults     []relationshipCreateCandidate
	createList        *components.List
	createSource      *relationshipCreateCandidate
	createTarget      *relationshipCreateCandidate
	createTypeInput   textinput.Model
	createTypeResults []string
	createTypeList    *components.List
	createTypeNav     bool
	createLoading     bool

	typeOptions []string
}

// NewRelationshipsModel builds the relationships UI model.
func NewRelationshipsModel(client *api.Client) RelationshipsModel {
	return RelationshipsModel{
		client:           client,
		spinner:          components.NewNebulaSpinner(),
		filterInput:      components.NewNebulaTextInput("Filter relationships..."),
		createQueryInput: components.NewNebulaTextInput("Search nodes..."),
		createTypeInput:  components.NewNebulaTextInput("Relationship type..."),
		list:             components.NewList(12),
		createList:       components.NewList(8),
		createTypeList:   components.NewList(6),
		view:             relsViewList,
		names:            map[string]string{},
	}
}

// Init handles init.
func (m RelationshipsModel) Init() tea.Cmd {
	m.view = relsViewList
	m.loading = true
	m.modeFocus = false
	m.filtering = false
	m.filterInput.Reset()
	m.metaExpanded = false
	m.editMeta.Reset()
	return tea.Batch(
		m.loadRelationships(),
		m.loadScopeOptions(),
		m.loadEntityCache(),
		m.loadContextCache(),
		m.loadJobCache(),
		m.spinner.Tick,
	)
}

// Update updates update.
func (m RelationshipsModel) Update(msg tea.Msg) (RelationshipsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case relTabLoadedMsg:
		m.loading = false
		m.allItems = append([]api.Relationship{}, msg.items...)
		m.applyListFilter()
		m.typeOptions = uniqueRelationshipTypes(msg.items)
		return m, m.loadRelationshipNames(msg.items)

	case relTabNamesLoadedMsg:
		if m.names == nil {
			m.names = map[string]string{}
		}
		for id, name := range msg.names {
			m.names[id] = name
		}
		m.applyListFilter()
		return m, nil
	case relTabScopesLoadedMsg:
		m.scopeOptions = msg.options
		m.editMeta.SetScopeOptions(m.scopeOptions)
		return m, nil
	case relTabEntityCacheLoadedMsg:
		m.entityCache = msg.items
		return m, nil
	case relTabContextCacheLoadedMsg:
		m.contextCache = msg.items
		return m, nil
	case relTabJobCacheLoadedMsg:
		m.jobCache = msg.items
		return m, nil

	case relTabResultsMsg:
		if strings.TrimSpace(msg.query) != strings.TrimSpace(m.createQueryInput.Value()) {
			return m, nil
		}
		m.createLoading = false
		m.createResults = msg.items
		labels := make([]string, len(msg.items))
		for i, candidate := range msg.items {
			labels[i] = formatCreateCandidateLine(candidate)
		}
		m.createList.SetItems(labels)
		return m, nil

	case relTabSavedMsg:
		m.editSaving = false
		m.loading = true
		m.view = relsViewList
		return m, tea.Batch(m.loadRelationships(), m.spinner.Tick)

	case errMsg:
		m.loading = false
		m.editSaving = false
		m.createLoading = false
		return m, nil

	case tea.KeyPressMsg:
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

// View handles view.
func (m RelationshipsModel) View() string {
	if m.editMeta.Active {
		return m.editMeta.Render(m.width)
	}
	if m.filtering && m.view == relsViewList {
		return components.Indent(components.TextInputDialog("Filter Relationships", m.filterInput.View()), 1)
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

func (m RelationshipsModel) handleListKeys(msg tea.KeyPressMsg) (RelationshipsModel, tea.Cmd) {
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
	case isEnter(msg), isSpace(msg):
		if rel := m.selectedRelationship(); rel != nil {
			m.detail = rel
			m.view = relsViewDetail
		}
	case isKey(msg, "f"):
		m.filtering = true
		m.filterInput.Focus()
		return m, nil
	case isKey(msg, "n"):
		m.startCreate()
		m.view = relsViewCreateSourceSearch
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m RelationshipsModel) handleFilterInput(msg tea.KeyPressMsg) (RelationshipsModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
		m.filterInput.Blur()
	case isBack(msg):
		m.filtering = false
		m.filterInput.Reset()
		m.filterInput.Blur()
		m.applyListFilter()
	default:
		prev := m.filterInput.Value()
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		if m.filterInput.Value() != prev {
			m.applyListFilter()
		}
		return m, cmd
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
	if m.modeFocus {
		if m.isAddView() {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("Library")
		}
	}
	return add + " " + list
}

// handleModeKeys handles handle mode keys.
func (m RelationshipsModel) handleModeKeys(msg tea.KeyPressMsg) (RelationshipsModel, tea.Cmd) {
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

// toggleMode handles toggle mode.
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

// isAddView handles is add view.
func (m RelationshipsModel) isAddView() bool {
	switch m.view {
	case relsViewCreateSourceSearch, relsViewCreateSourceSelect, relsViewCreateTargetSearch, relsViewCreateTargetSelect, relsViewCreateType:
		return true
	default:
		return false
	}
}

// renderList renders render list.
func (m RelationshipsModel) renderList() string {
	if m.loading {
		return "  " + m.spinner.View() + " " + MutedStyle.Render("Loading relationships...")
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

	relWidth := 12
	statusWidth := 9
	atWidth := compactTimeColumnWidth
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
	if m.modeFocus {
		activeRowRel = -1
	}

	title := "Relationships"
	count := fmt.Sprintf("%d total", len(m.items))
	if query := strings.TrimSpace(m.filterInput.Value()); query != "" {
		count = fmt.Sprintf("%s · filter: %s", count, query)
	}
	countLine := MutedStyle.Render(count)

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

func (m RelationshipsModel) handleDetailKeys(msg tea.KeyPressMsg) (RelationshipsModel, tea.Cmd) {
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

// renderDetail renders render detail.
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

// renderRelationshipPreview renders render relationship preview.
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

// handleEditKeys handles handle edit keys.
func (m RelationshipsModel) handleEditKeys(msg tea.KeyPressMsg) (RelationshipsModel, tea.Cmd) {
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
		switch m.editFocus {
		case relsEditFieldStatus:
			switch {
			case isKey(msg, "left"):
				m.editStatusIdx = (m.editStatusIdx - 1 + len(relsStatusOptions)) % len(relsStatusOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.editStatusIdx = (m.editStatusIdx + 1) % len(relsStatusOptions)
			}
		case relsEditFieldProperties:
			if isEnter(msg) {
				m.editMeta.Active = true
			}
		}
	}
	return m, nil
}

// renderEdit renders render edit.
func (m RelationshipsModel) renderEdit() string {
	status := relsStatusOptions[m.editStatusIdx]
	var b strings.Builder

	if m.editFocus == relsEditFieldStatus {
		b.WriteString(SelectedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	} else {
		b.WriteString(MutedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	}

	b.WriteString("\n\n")

	if m.editFocus == relsEditFieldProperties {
		b.WriteString(SelectedStyle.Render("  Properties:"))
	} else {
		b.WriteString(MutedStyle.Render("  Properties:"))
	}
	b.WriteString("\n")
	props := renderMetadataEditorPreview(m.editMeta.Buffer, m.editMeta.Scopes, m.width, 6)
	if strings.TrimSpace(props) == "" {
		props = "-"
	}
	b.WriteString(NormalStyle.Render("  " + props))

	if m.editSaving {
		b.WriteString("\n\n" + MutedStyle.Render("Saving..."))
	}

	return components.TitledBox("Edit Relationship", b.String(), m.width)
}

// saveEdit handles save edit.
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

func (m RelationshipsModel) handleConfirmKeys(msg tea.KeyPressMsg) (RelationshipsModel, tea.Cmd) {
	switch {
	case isKey(msg, "y"), isEnter(msg):
		status := "inactive"
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

// renderConfirm renders render confirm.
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
		To:    "inactive",
	}}
	return components.ConfirmPreviewDialog("Archive Relationship", summary, diffs, m.width)
}

// --- Create ---

func (m *RelationshipsModel) startCreate() {
	m.createQueryInput.Reset()
	m.createQueryInput.Focus()
	m.createResults = nil
	m.createList.SetItems(nil)
	m.createSource = nil
	m.createTarget = nil
	m.createTypeInput.Reset()
	m.createTypeResults = nil
	m.createTypeList.SetItems(nil)
	m.createTypeNav = false
	m.createLoading = false
}

// handleCreateKeys handles handle create keys.
func (m RelationshipsModel) handleCreateKeys(msg tea.KeyPressMsg) (RelationshipsModel, tea.Cmd) {
	switch m.view {
	case relsViewCreateSourceSearch:
		switch {
		case isBack(msg):
			if m.createQueryInput.Value() != "" {
				m.createQueryInput.Reset()
				m.createResults = nil
				m.createList.SetItems(nil)
				m.createLoading = false
				return m, nil
			}
			m.createQueryInput.Blur()
			m.view = relsViewList
		case isDown(msg):
			m.createList.Down()
		case isUp(msg):
			m.createList.Up()
		case isEnter(msg):
			if idx := m.createList.Selected(); idx < len(m.createResults) {
				item := m.createResults[idx]
				m.createSource = &item
				m.createQueryInput.Reset()
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateTargetSearch
			}
		default:
			prev := m.createQueryInput.Value()
			var cmd tea.Cmd
			m.createQueryInput, cmd = m.createQueryInput.Update(msg)
			if m.createQueryInput.Value() != prev {
				return m, tea.Batch(cmd, m.updateCreateSearch())
			}
			return m, cmd
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
				m.createQueryInput.Reset()
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateTargetSearch
			}
		}
	case relsViewCreateTargetSearch:
		switch {
		case isBack(msg):
			if m.createQueryInput.Value() != "" {
				m.createQueryInput.Reset()
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
				m.createQueryInput.Reset()
				m.createQueryInput.Blur()
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateType
				m.createTypeInput.Focus()
				m.resetTypeSuggestions()
			}
		default:
			prev := m.createQueryInput.Value()
			var cmd tea.Cmd
			m.createQueryInput, cmd = m.createQueryInput.Update(msg)
			if m.createQueryInput.Value() != prev {
				return m, tea.Batch(cmd, m.updateCreateSearch())
			}
			return m, cmd
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
				m.createQueryInput.Reset()
				m.createQueryInput.Blur()
				m.createResults = nil
				m.createList.SetItems(nil)
				m.view = relsViewCreateType
				m.createTypeInput.Focus()
				m.resetTypeSuggestions()
			}
		}
	case relsViewCreateType:
		switch {
		case isBack(msg):
			m.createTypeInput.Blur()
			m.view = relsViewCreateTargetSearch
			m.createQueryInput.Focus()
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
			kind := strings.TrimSpace(m.createTypeInput.Value())
			if m.createTypeNav && len(m.createTypeResults) > 0 {
				if idx := m.createTypeList.Selected(); idx < len(m.createTypeResults) {
					kind = m.createTypeResults[idx]
				}
			}
			if kind == "" || m.createSource == nil || m.createTarget == nil {
				return m, nil
			}
			m.createTypeInput.Blur()
			m.view = relsViewList
			m.loading = true
			return m, tea.Batch(m.createRelationship(*m.createSource, *m.createTarget, kind), m.spinner.Tick)
		default:
			prev := m.createTypeInput.Value()
			var cmd tea.Cmd
			m.createTypeInput, cmd = m.createTypeInput.Update(msg)
			if m.createTypeInput.Value() != prev {
				m.createTypeNav = false
				m.updateTypeSuggestions()
			}
			return m, cmd
		}
	}
	return m, nil
}

// renderCreate renders render create.
func (m RelationshipsModel) renderCreate() string {
	switch m.view {
	case relsViewCreateSourceSearch, relsViewCreateSourceSelect:
		return m.renderCreateSearch("Source Node")
	case relsViewCreateTargetSearch, relsViewCreateTargetSelect:
		return m.renderCreateSearch("Target Node")
	case relsViewCreateType:
		return m.renderCreateType()
	}
	return ""
}

// renderCreateSearch renders render create search.
func (m RelationshipsModel) renderCreateSearch(title string) string {
	var b strings.Builder
	b.WriteString(m.createQueryInput.View())
	b.WriteString("\n\n")

	if m.createLoading {
		b.WriteString(MutedStyle.Render("Searching..."))
	} else if strings.TrimSpace(m.createQueryInput.Value()) == "" {
		b.WriteString(MutedStyle.Render("Type to search."))
	} else if len(m.createResults) == 0 {
		b.WriteString(MutedStyle.Render("No matches."))
	} else {
		contentWidth := components.BoxContentWidth(m.width)
		visible := m.createList.Visible()

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

		kindWidth := 18
		statusWidth := 11
		nameWidth := availableCols - (kindWidth + statusWidth)
		if nameWidth < 16 {
			nameWidth = 16
			kindWidth = availableCols - (nameWidth + statusWidth)
			if kindWidth < 12 {
				kindWidth = 12
			}
		}

		cols := []components.TableColumn{
			{Header: "Name", Width: nameWidth, Align: lipgloss.Left},
			{Header: "Kind", Width: kindWidth, Align: lipgloss.Left},
			{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		}

		tableRows := make([][]string, 0, len(visible))
		activeRowRel := -1
		var previewItem *relationshipCreateCandidate
		if idx := m.createList.Selected(); idx >= 0 && idx < len(m.createResults) {
			previewItem = &m.createResults[idx]
		}

		for i := range visible {
			absIdx := m.createList.RelToAbs(i)
			if absIdx < 0 || absIdx >= len(m.createResults) {
				continue
			}
			candidate := m.createResults[absIdx]

			name := strings.TrimSpace(components.SanitizeOneLine(candidate.Name))
			if name == "" {
				name = "node"
			}
			kind := strings.TrimSpace(components.SanitizeOneLine(candidate.Kind))
			if kind == "" {
				kind = strings.TrimSpace(components.SanitizeOneLine(candidate.NodeType))
			}
			if kind == "" {
				kind = "node"
			}
			status := strings.TrimSpace(components.SanitizeOneLine(candidate.Status))
			if status == "" {
				status = "-"
			}

			if m.createList.IsSelected(absIdx) {
				activeRowRel = len(tableRows)
			}

			tableRows = append(tableRows, []string{
				components.ClampTextWidthEllipsis(name, nameWidth),
				components.ClampTextWidthEllipsis(kind, kindWidth),
				components.ClampTextWidthEllipsis(status, statusWidth),
			})
		}

		countLine := MutedStyle.Render(fmt.Sprintf("%d results", len(m.createResults)))
		table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
		preview := ""
		if previewItem != nil {
			content := m.renderCreateNodePreview(*previewItem, previewBoxContentWidth(previewWidth))
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

// renderCreateType renders render create type.
func (m RelationshipsModel) renderCreateType() string {
	var b strings.Builder
	b.WriteString(m.createTypeInput.View())
	b.WriteString("\n\n")

	if strings.TrimSpace(m.createTypeInput.Value()) == "" && len(m.typeOptions) == 0 {
		b.WriteString(MutedStyle.Render("Type a relationship type."))
	} else if len(m.createTypeResults) == 0 {
		b.WriteString(MutedStyle.Render("No suggestions."))
	} else {
		contentWidth := components.BoxContentWidth(m.width)
		visible := m.createTypeList.Visible()

		previewWidth := preferredPreviewWidth(contentWidth)

		gap := 3
		tableWidth := contentWidth
		sideBySide := contentWidth >= minSideBySideContentWidth
		if sideBySide {
			tableWidth = contentWidth - previewWidth - gap
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

// renderCreateNodePreview renders render create node preview.
func (m RelationshipsModel) renderCreateNodePreview(candidate relationshipCreateCandidate, width int) string {
	if width <= 0 {
		return ""
	}

	name := strings.TrimSpace(components.SanitizeOneLine(candidate.Name))
	if name == "" {
		name = "node"
	}
	kind := strings.TrimSpace(components.SanitizeOneLine(candidate.Kind))
	if kind == "" {
		kind = strings.TrimSpace(components.SanitizeOneLine(candidate.NodeType))
	}
	if kind == "" {
		kind = "node"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(candidate.Status))
	if status == "" {
		status = "-"
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(name, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Kind", kind, width))
	lines = append(lines, renderPreviewRow("Node", candidate.NodeType, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	if len(candidate.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(candidate.Tags, ", "), width))
	}

	return padPreviewLines(lines, width)
}

// renderCreateTypePreview renders render create type preview.
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

// loadRelationshipNames loads load relationship names.
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

// buildListLabels builds build list labels.
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

// applyListFilter handles apply list filter.
func (m *RelationshipsModel) applyListFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	if query == "" {
		m.items = append([]api.Relationship{}, m.allItems...)
	} else {
		filtered := make([]api.Relationship, 0, len(m.allItems))
		for _, rel := range m.allItems {
			relType := strings.ToLower(strings.TrimSpace(rel.Type))
			status := strings.ToLower(strings.TrimSpace(rel.Status))
			source := strings.ToLower(m.displayNode(rel.SourceID, rel.SourceType, rel.SourceName))
			target := strings.ToLower(m.displayNode(rel.TargetID, rel.TargetType, rel.TargetName))
			if strings.Contains(relType, query) ||
				strings.Contains(status, query) ||
				strings.Contains(source, query) ||
				strings.Contains(target, query) {
				filtered = append(filtered, rel)
			}
		}
		m.items = filtered
	}
	if m.list != nil {
		m.list.SetItems(m.buildListLabels())
	}
}

// displayNode handles display node.
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

// selectedRelationship handles selected relationship.
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

// searchCreateNodes handles search create nodes.
func (m RelationshipsModel) searchCreateNodes(query string) tea.Cmd {
	return func() tea.Msg {
		entities, err := m.client.QueryEntities(api.QueryParams{"search_text": query})
		if err != nil {
			return errMsg{err}
		}
		contextItems, err := m.client.QueryContext(api.QueryParams{"search_text": query})
		if err != nil {
			return errMsg{err}
		}
		jobs, err := m.client.QueryJobs(api.QueryParams{"search_text": query})
		if err != nil {
			return errMsg{err}
		}
		candidates := combineCreateCandidates(entities, contextItems, jobs)
		return relTabResultsMsg{query: query, items: filterCreateCandidatesByQuery(candidates, query)}
	}
}

// createRelationship creates create relationship.
func (m RelationshipsModel) createRelationship(source relationshipCreateCandidate, target relationshipCreateCandidate, relType string) tea.Cmd {
	return func() tea.Msg {
		input := api.CreateRelationshipInput{
			SourceType: source.NodeType,
			SourceID:   source.ID,
			TargetType: target.NodeType,
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

// updateCreateSearch updates update create search.
func (m *RelationshipsModel) updateCreateSearch() tea.Cmd {
	query := strings.TrimSpace(m.createQueryInput.Value())
	if query == "" {
		m.createLoading = false
		m.createResults = nil
		m.createList.SetItems(nil)
		return nil
	}
	if len(m.entityCache) > 0 || len(m.contextCache) > 0 || len(m.jobCache) > 0 {
		m.createLoading = false
		candidates := combineCreateCandidates(m.entityCache, m.contextCache, m.jobCache)
		m.createResults = filterCreateCandidatesByQuery(candidates, query)
		labels := make([]string, len(m.createResults))
		for i, candidate := range m.createResults {
			labels[i] = formatCreateCandidateLine(candidate)
		}
		m.createList.SetItems(labels)
		return nil
	}
	m.createLoading = true
	return m.searchCreateNodes(query)
}

// resetTypeSuggestions handles reset type suggestions.
func (m *RelationshipsModel) resetTypeSuggestions() {
	m.createTypeResults = filterRelationshipTypes(m.typeOptions, "")
	m.createTypeList.SetItems(m.createTypeResults)
	m.createTypeNav = false
}

// updateTypeSuggestions updates update type suggestions.
func (m *RelationshipsModel) updateTypeSuggestions() {
	m.createTypeResults = filterRelationshipTypes(m.typeOptions, m.createTypeInput.Value())
	m.createTypeList.SetItems(m.createTypeResults)
}

// loadScopeOptions loads load scope options.
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

// loadEntityCache loads load entity cache.
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

// loadContextCache loads load context cache.
func (m RelationshipsModel) loadContextCache() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		items, err := m.client.QueryContext(api.QueryParams{})
		if err != nil {
			return errMsg{err}
		}
		return relTabContextCacheLoadedMsg{items: items}
	}
}

// loadJobCache loads load job cache.
func (m RelationshipsModel) loadJobCache() tea.Cmd {
	if m.client == nil {
		return nil
	}
	return func() tea.Msg {
		items, err := m.client.QueryJobs(api.QueryParams{})
		if err != nil {
			return errMsg{err}
		}
		return relTabJobCacheLoadedMsg{items: items}
	}
}

// uniqueRelationshipTypes handles unique relationship types.
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

// filterRelationshipTypes handles filter relationship types.
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

// combineCreateCandidates handles combine create candidates.
func combineCreateCandidates(entities []api.Entity, contextItems []api.Context, jobs []api.Job) []relationshipCreateCandidate {
	candidates := make([]relationshipCreateCandidate, 0, len(entities)+len(contextItems)+len(jobs))
	for _, entity := range entities {
		name := strings.TrimSpace(entity.Name)
		if name == "" {
			name = "entity"
		}
		kind := strings.TrimSpace(entity.Type)
		if kind == "" {
			kind = "entity"
		}
		status := strings.TrimSpace(entity.Status)
		if status == "" {
			status = "-"
		}
		candidates = append(candidates, relationshipCreateCandidate{
			ID:       entity.ID,
			NodeType: "entity",
			Name:     name,
			Kind:     "entity/" + kind,
			Status:   status,
			Tags:     append([]string{}, entity.Tags...),
		})
	}
	for _, contextItem := range contextItems {
		name := strings.TrimSpace(contextItem.Title)
		if name == "" {
			name = strings.TrimSpace(contextItem.Name)
		}
		if name == "" {
			name = "context"
		}
		kind := strings.TrimSpace(contextItem.SourceType)
		if kind == "" {
			kind = "note"
		}
		status := strings.TrimSpace(contextItem.Status)
		if status == "" {
			status = "-"
		}
		candidates = append(candidates, relationshipCreateCandidate{
			ID:       contextItem.ID,
			NodeType: "context",
			Name:     name,
			Kind:     "context/" + kind,
			Status:   status,
			Tags:     append([]string{}, contextItem.Tags...),
		})
	}
	for _, job := range jobs {
		name := strings.TrimSpace(job.Title)
		if name == "" {
			name = "job"
		}
		kind := "job"
		if job.Priority != nil && strings.TrimSpace(*job.Priority) != "" {
			kind = "job/" + strings.TrimSpace(*job.Priority)
		}
		status := strings.TrimSpace(job.Status)
		if status == "" {
			status = "-"
		}
		candidates = append(candidates, relationshipCreateCandidate{
			ID:       job.ID,
			NodeType: "job",
			Name:     name,
			Kind:     kind,
			Status:   status,
		})
	}
	return candidates
}

// filterCreateCandidatesByQuery handles filter create candidates by query.
func filterCreateCandidatesByQuery(candidates []relationshipCreateCandidate, query string) []relationshipCreateCandidate {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		return append([]relationshipCreateCandidate{}, candidates...)
	}
	out := make([]relationshipCreateCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		haystack := strings.ToLower(strings.Join([]string{
			candidate.Name,
			candidate.Kind,
			candidate.Status,
			candidate.NodeType,
			strings.Join(candidate.Tags, " "),
		}, " "))
		if strings.Contains(haystack, query) {
			out = append(out, candidate)
		}
	}
	return out
}

// formatCreateCandidateLine handles format create candidate line.
func formatCreateCandidateLine(candidate relationshipCreateCandidate) string {
	name := strings.TrimSpace(candidate.Name)
	if name == "" {
		name = "node"
	}
	kind := strings.TrimSpace(candidate.Kind)
	if kind == "" {
		kind = strings.TrimSpace(candidate.NodeType)
	}
	if kind == "" {
		kind = "node"
	}
	status := strings.TrimSpace(candidate.Status)
	if status == "" {
		status = "-"
	}
	return fmt.Sprintf("%s · %s · %s", name, kind, status)
}

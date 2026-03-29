package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type protocolsLoadedMsg struct{ items []api.Protocol }
type protocolCreatedMsg struct{}
type protocolUpdatedMsg struct{}
type protocolNotesSavedMsg struct{}
type protocolRelationshipsLoadedMsg struct {
	id            string
	relationships []api.Relationship
}

type protocolsView int

const (
	protocolsViewAdd protocolsView = iota
	protocolsViewList
	protocolsViewDetail
	protocolsViewEdit
)

const (
	protoFieldName = iota
	protoFieldTitle
	protoFieldVersion
	protoFieldType
	protoFieldApplies
	protoFieldStatus
	protoFieldTags
	protoFieldContent
	protoFieldMetadata
	protoFieldSourcePath
	protoFieldCount
)

const (
	protoEditFieldTitle = iota
	protoEditFieldVersion
	protoEditFieldType
	protoEditFieldApplies
	protoEditFieldStatus
	protoEditFieldTags
	protoEditFieldContent
	protoEditFieldMetadata
	protoEditFieldSourcePath
	protoEditFieldCount
)

var protocolStatusOptions = []string{"active", "inactive"}

// --- Protocols Model ---

type ProtocolsModel struct {
	client     *api.Client
	dataTable  table.Model
	items      []api.Protocol
	allItems   []api.Protocol
	loading    bool
	spinner    spinner.Model
	view       protocolsView
	detail     *api.Protocol
	detailRels []api.Relationship
	modeFocus  bool
	filtering  bool
	searchBuf  string
	width      int
	height     int

	// add
	addFields    []formField
	addFocus     int
	addStatusIdx int
	addTags      []string
	addTagBuf    string
	addApplies   []string
	addApplyBuf  string
	addMeta      MetadataEditor
	addSaving    bool
	addErr       string

	// edit
	editFields    []formField
	editFocus     int
	editStatusIdx int
	editTags      []string
	editTagBuf    string
	editApplies   []string
	editApplyBuf  string
	editMeta      MetadataEditor
	editSaving    bool

	// inline notes editing (split-pane)
	notesEditing  bool
	notesTextarea textarea.Model
	notesDirty    bool

}

// NewProtocolsModel builds the protocols UI model.
func NewProtocolsModel(client *api.Client) ProtocolsModel {
	return ProtocolsModel{
		client:    client,
		spinner:   components.NewNebulaSpinner(),
		dataTable: components.NewNebulaTable(nil, 12),
		view:      protocolsViewList,
		addFields: []formField{
			{label: "Name"},
			{label: "Title"},
			{label: "Version"},
			{label: "Type"},
			{label: "Applies To"},
			{label: "Status"},
			{label: "Tags"},
			{label: "Content"},
			{label: "Notes"},
			{label: "Source Path"},
		},
		editFields: []formField{
			{label: "Title"},
			{label: "Version"},
			{label: "Type"},
			{label: "Applies To"},
			{label: "Status"},
			{label: "Tags"},
			{label: "Content"},
			{label: "Notes"},
			{label: "Source Path"},
		},
	}
}

// Init handles init.
func (m ProtocolsModel) Init() tea.Cmd {
	m.loading = true
	m.view = protocolsViewList
	m.detail = nil
	m.detailRels = nil
	m.filtering = false
	m.searchBuf = ""
	m.modeFocus = false
	m.addFocus = 0
	m.addStatusIdx = statusIndex(protocolStatusOptions, "active")
	m.addTags = nil
	m.addTagBuf = ""
	m.addApplies = nil
	m.addApplyBuf = ""
	m.addMeta.Reset()
	m.addSaving = false
	m.addErr = ""
	m.editFocus = 0
	m.editStatusIdx = statusIndex(protocolStatusOptions, "active")
	m.editTags = nil
	m.editTagBuf = ""
	m.editApplies = nil
	m.editApplyBuf = ""
	m.editMeta.Reset()
	m.editSaving = false
	return tea.Batch(m.loadProtocols, m.spinner.Tick)
}

// Update updates update.
func (m ProtocolsModel) Update(msg tea.Msg) (ProtocolsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case protocolsLoadedMsg:
		m.loading = false
		m.allItems = msg.items
		m.applySearch()
		return m, nil
	case protocolCreatedMsg:
		m.addSaving = false
		m.addErr = ""
		m.loading = true
		return m, tea.Batch(m.loadProtocols, m.spinner.Tick)
	case protocolUpdatedMsg:
		m.editSaving = false
		m.detail = nil
		m.detailRels = nil
		m.view = protocolsViewList
		m.loading = true
		return m, tea.Batch(m.loadProtocols, m.spinner.Tick)
	case protocolRelationshipsLoadedMsg:
		if m.detail != nil && m.detail.ID == msg.id {
			m.detailRels = msg.relationships
		}
		return m, nil
	case protocolNotesSavedMsg:
		m.notesEditing = false
		m.notesDirty = false
		m.loading = true
		return m, tea.Batch(m.loadProtocols, m.spinner.Tick)
	case errMsg:
		m.loading = false
		m.addSaving = false
		m.editSaving = false
		m.notesEditing = false
		m.addErr = msg.err.Error()
		return m, nil

	case tea.KeyPressMsg:
		if m.notesEditing {
			return m.handleNotesEditKeys(msg)
		}
		if m.modeFocus {
			return m.handleModeKeys(msg)
		}
		switch m.view {
		case protocolsViewAdd:
			return m.handleAddKeys(msg)
		case protocolsViewDetail:
			return m.handleDetailKeys(msg)
		case protocolsViewEdit:
			return m.handleEditKeys(msg)
		default:
			return m.handleListKeys(msg)
		}
	}
	return m, nil
}

// View handles view.
func (m ProtocolsModel) View() string {
	if m.addMeta.Active {
		return m.addMeta.Render(m.width)
	}
	if m.editMeta.Active {
		return m.editMeta.Render(m.width)
	}
	if m.filtering && m.view == protocolsViewList {
		return components.Indent(components.InputDialog("Filter Protocols", m.searchBuf), 1)
	}
	switch m.view {
	case protocolsViewAdd:
		body := m.renderAdd()
		mode := m.renderModeLine()
		if mode != "" {
			body = components.CenterLine(mode, m.width) + "\n\n" + body
		}
		return components.Indent(body, 1)
	case protocolsViewDetail:
		return components.Indent(m.renderDetail(), 1)
	case protocolsViewEdit:
		body := m.renderEdit()
		mode := m.renderModeLine()
		if mode != "" {
			body = components.CenterLine(mode, m.width) + "\n\n" + body
		}
		return components.Indent(body, 1)
	default:
		body := m.renderList()
		mode := m.renderModeLine()
		if mode != "" {
			body = components.CenterLine(mode, m.width) + "\n\n" + body
		}
		return components.Indent(body, 1)
	}
}

// Hints returns the hint items for the current view state.
func (m ProtocolsModel) Hints() []components.HintItem {
	if m.addMeta.Active || m.editMeta.Active || m.filtering {
		return nil
	}
	if m.notesEditing {
		return []components.HintItem{
			{Key: "esc", Desc: "Cancel"},
			{Key: "ctrl+s", Desc: "Save"},
		}
	}
	if m.view != protocolsViewList {
		return nil
	}
	return []components.HintItem{
		{Key: "1-9/0", Desc: "Tabs"},
		{Key: "/", Desc: "Command"},
		{Key: "?", Desc: "Help"},
		{Key: "q", Desc: "Quit"},
		{Key: "\u2191/\u2193", Desc: "Scroll"},
		{Key: "enter", Desc: "View"},
		{Key: "a", Desc: "Add"},
		{Key: "e", Desc: "Edit"},
	}
}

// --- Loading ---

func (m ProtocolsModel) loadProtocols() tea.Msg {
	items, err := m.client.QueryProtocols(api.QueryParams{"status_category": "active"})
	if err != nil {
		return errMsg{err}
	}
	return protocolsLoadedMsg{items: items}
}

// applySearch handles apply search.
func (m *ProtocolsModel) applySearch() {
	query := strings.TrimSpace(strings.ToLower(m.searchBuf))
	if query == "" {
		m.items = append([]api.Protocol{}, m.allItems...)
	} else {
		filtered := make([]api.Protocol, 0)
		for _, item := range m.allItems {
			name := strings.ToLower(item.Name)
			title := strings.ToLower(item.Title)
			if strings.Contains(name, query) || strings.Contains(title, query) {
				filtered = append(filtered, item)
			}
		}
		m.items = filtered
	}
	rows := make([]table.Row, 0, len(m.items))
	for _, item := range m.items {
		name := components.SanitizeOneLine(item.Name)
		title := components.SanitizeOneLine(item.Title)
		label := name
		if strings.TrimSpace(item.Title) != "" {
			label = fmt.Sprintf("%s · %s", name, title)
		}
		rows = append(rows, table.Row{label})
	}
	m.dataTable.SetRows(rows)
	m.dataTable.SetCursor(0)
}

// --- Mode Line ---

func (m ProtocolsModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("Library")
	if m.view == protocolsViewAdd {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("Library")
	}
	if m.modeFocus {
		if m.view == protocolsViewAdd {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("Library")
		}
	}
	return add + " " + list
}

// handleModeKeys handles handle mode keys.
func (m ProtocolsModel) handleModeKeys(msg tea.KeyPressMsg) (ProtocolsModel, tea.Cmd) {
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
func (m ProtocolsModel) toggleMode() (ProtocolsModel, tea.Cmd) {
	m.modeFocus = false
	if m.view == protocolsViewAdd {
		m.view = protocolsViewList
		return m, nil
	}
	m.view = protocolsViewAdd
	return m, nil
}

// --- List ---

func (m ProtocolsModel) handleListKeys(msg tea.KeyPressMsg) (ProtocolsModel, tea.Cmd) {
	if m.filtering {
		return m.handleFilterInput(msg)
	}
	switch {
	case isDown(msg):
		m.dataTable.MoveDown(1)
	case isUp(msg):
		if m.dataTable.Cursor() <= 0 {
			m.modeFocus = true
		} else {
			m.dataTable.MoveUp(1)
		}
	case isKey(msg, "n"):
		m.view = protocolsViewAdd
		return m, nil
	case isKey(msg, "tab"):
		m.view = protocolsViewAdd
		return m, nil
	case isEnter(msg):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			m.detail = &m.items[idx]
			m.detailRels = nil
			m.view = protocolsViewDetail
			return m, m.loadDetailRelationships(m.items[idx].ID)
		}
	case isKey(msg, "e"):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			item := m.items[idx]
			m.notesEditing = true
			m.notesDirty = false
			m.notesTextarea = components.NewNebulaTextarea(36, 10)
			m.notesTextarea.SetValue(item.Notes)
			m.notesTextarea.Focus()
		}
	case isKey(msg, "f"):
		m.filtering = true
		return m, nil
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applySearch()
		}
	default:
		if ch := keyText(msg); ch != "" {
			m.searchBuf += ch
			m.applySearch()
		}
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m ProtocolsModel) handleFilterInput(msg tea.KeyPressMsg) (ProtocolsModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
	case isBack(msg):
		m.filtering = false
		m.searchBuf = ""
		m.applySearch()
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applySearch()
		}
	default:
		if ch := keyText(msg); ch != "" {
			m.searchBuf += ch
			m.applySearch()
		}
	}
	return m, nil
}

// --- Inline Notes Edit ---

// handleNotesEditKeys routes keys to the textarea when inline notes editing is active.
func (m ProtocolsModel) handleNotesEditKeys(msg tea.KeyPressMsg) (ProtocolsModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.notesEditing = false
		m.notesDirty = false
		return m, nil
	case isKey(msg, "ctrl+s"):
		return m.saveInlineNotes()
	}
	var cmd tea.Cmd
	m.notesTextarea, cmd = m.notesTextarea.Update(msg)
	m.notesDirty = true
	return m, cmd
}

// saveInlineNotes saves the current textarea content via the API.
func (m ProtocolsModel) saveInlineNotes() (ProtocolsModel, tea.Cmd) {
	if idx := m.dataTable.Cursor(); idx < 0 || idx >= len(m.items) {
		m.notesEditing = false
		return m, nil
	}
	item := m.items[m.dataTable.Cursor()]
	notes := m.notesTextarea.Value()
	return m, func() tea.Msg {
		input := api.UpdateProtocolInput{Notes: notes}
		_, err := m.client.UpdateProtocol(item.Name, input)
		if err != nil {
			return errMsg{err}
		}
		return protocolNotesSavedMsg{}
	}
}

// renderList renders render list.
func (m ProtocolsModel) renderList() string {
	if m.loading {
		return components.CenterLine(m.spinner.View()+" Loading protocols...", m.width)
	}
	contentWidth := components.BoxContentWidth(m.width)

	if len(m.items) == 0 {
		box := components.EmptyStateBox(
			"Protocols",
			"No protocols found.",
			[]string{"Press n to create", "Press / for command palette"},
			m.width,
		)
		return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, box)
	}

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap - components.TableBaseBorderWidth
	}

	// Each table cell has Padding(0,1) = 2 chars. 4 columns = 8 chars of padding.
	cellPadding := 4 * 2
	availableCols := tableWidth - cellPadding
	if availableCols < 30 {
		availableCols = 30
	}

	statusWidth := 11
	atWidth := compactTimeColumnWidth
	nameWidth := 18
	titleWidth := availableCols - (nameWidth + statusWidth + atWidth)
	if titleWidth < 14 {
		titleWidth = 14
		nameWidth = availableCols - (titleWidth + statusWidth + atWidth)
		if nameWidth < 12 {
			nameWidth = 12
		}
	}
	if titleWidth > 40 {
		titleWidth = 40
	}

	tableRows := make([]table.Row, len(m.items))
	for i, p := range m.items {
		name := strings.TrimSpace(components.SanitizeOneLine(p.Name))
		if name == "" {
			name = "protocol"
		}
		title := strings.TrimSpace(components.SanitizeOneLine(p.Title))
		if title == "" {
			title = "-"
		}
		status := strings.TrimSpace(components.SanitizeOneLine(p.Status))
		if status == "" {
			status = "-"
		}
		at := p.UpdatedAt
		if at.IsZero() {
			at = p.CreatedAt
		}
		tableRows[i] = table.Row{
			components.ClampTextWidthEllipsis(name, nameWidth),
			components.ClampTextWidthEllipsis(title, titleWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			formatLocalTimeCompact(at),
		}
	}

	m.dataTable.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Title", Width: titleWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "At", Width: atWidth},
	})
	actualTableWidth := nameWidth + titleWidth + statusWidth + atWidth + cellPadding
	m.dataTable.SetWidth(actualTableWidth)
	m.dataTable.SetRows(tableRows)

	countLine := ""
	if strings.TrimSpace(m.searchBuf) != "" {
		countLine = MutedStyle.Render(fmt.Sprintf("%d total · search: %s", len(m.items), strings.TrimSpace(m.searchBuf)))
	}

	tableView := components.TableBaseStyle.Render(m.dataTable.View())
	preview := ""
	if m.notesEditing {
		m.notesTextarea.SetWidth(previewWidth - 4)
		m.notesTextarea.SetHeight(10)
		preview = m.notesTextarea.View()
	} else {
		var previewItem *api.Protocol
		if !m.modeFocus {
			if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
				previewItem = &m.items[idx]
			}
		}
		if previewItem != nil {
			content := m.renderProtocolPreview(*previewItem, previewBoxContentWidth(previewWidth))
			preview = renderPreviewBox(content, previewWidth)
		}
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	result := body
	if countLine != "" {
		result += "\n" + countLine
	}
	return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, result)
}

// renderProtocolPreview renders render protocol preview.
func (m ProtocolsModel) renderProtocolPreview(p api.Protocol, width int) string {
	if width <= 0 {
		return ""
	}

	name := strings.TrimSpace(components.SanitizeOneLine(p.Name))
	if name == "" {
		name = "protocol"
	}
	title := strings.TrimSpace(components.SanitizeOneLine(p.Title))
	heading := name
	if title != "" {
		heading = title
	}
	status := strings.TrimSpace(components.SanitizeOneLine(p.Status))
	if status == "" {
		status = "-"
	}
	at := p.UpdatedAt
	if at.IsZero() {
		at = p.CreatedAt
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(heading, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Name", name, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeFull(at), width))
	if p.Version != nil && strings.TrimSpace(*p.Version) != "" {
		lines = append(lines, renderPreviewRow("Version", strings.TrimSpace(*p.Version), width))
	}
	if p.ProtocolType != nil && strings.TrimSpace(*p.ProtocolType) != "" {
		lines = append(lines, renderPreviewRow("Type", strings.TrimSpace(*p.ProtocolType), width))
	}
	if len(p.AppliesTo) > 0 {
		lines = append(lines, renderPreviewRow("Applies", strings.Join(p.AppliesTo, ", "), width))
	}
	if len(p.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(p.Tags, ", "), width))
	}
	if p.Content != nil && strings.TrimSpace(*p.Content) != "" {
		lines = append(lines, renderPreviewRow("Content", strings.TrimSpace(*p.Content), width))
	}
	if p.Notes != "" {
		lines = append(lines, renderPreviewRow("Notes", truncateString(p.Notes, 80), width))
	}

	return padPreviewLines(lines, width)
}

// --- Detail ---

func (m ProtocolsModel) handleDetailKeys(msg tea.KeyPressMsg) (ProtocolsModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.view = protocolsViewList
		m.detail = nil
		m.detailRels = nil
	case isKey(msg, "e"):
		m.startEdit()
		m.view = protocolsViewEdit
	}
	return m, nil
}

// renderDetail renders render detail.
func (m ProtocolsModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}
	p := m.detail
	infoRows := []components.InfoTableRow{
		{Key: "Name", Value: p.Name},
		{Key: "Title", Value: p.Title},
	}
	if p.Version != nil && *p.Version != "" {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Version", Value: *p.Version})
	}
	if p.ProtocolType != nil && *p.ProtocolType != "" {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Type", Value: *p.ProtocolType})
	}
	if len(p.AppliesTo) > 0 {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Applies To", Value: strings.Join(p.AppliesTo, ", ")})
	}
	if p.Status != "" {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Status", Value: p.Status})
	}
	if len(p.Tags) > 0 {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Tags", Value: strings.Join(p.Tags, ", ")})
	}
	if p.SourcePath != nil && *p.SourcePath != "" {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Source Path", Value: *p.SourcePath})
	}
	infoRows = append(infoRows, components.InfoTableRow{Key: "Created", Value: formatLocalTimeFull(p.CreatedAt)})
	if !p.UpdatedAt.IsZero() {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Updated", Value: formatLocalTimeFull(p.UpdatedAt)})
	}

	sections := []string{components.RenderInfoTable(infoRows, m.width)}
	if p.Content != nil && strings.TrimSpace(*p.Content) != "" {
		rendered := strings.TrimSpace(components.RenderMarkdown(
			components.SanitizeText(*p.Content), m.width-6,
		))
		sections = append(
			sections,
			components.TitledBox("Content", rendered, m.width),
		)
	}
	if p.Notes != "" {
		sections = append(sections, components.TitledBox("Notes", p.Notes, m.width))
	}
	if len(m.detailRels) > 0 {
		sections = append(sections, renderRelationshipSummaryTable("protocol", p.ID, m.detailRels, 6, m.width))
	}
	return strings.Join(sections, "\n\n")
}

// loadDetailRelationships loads load detail relationships.
func (m ProtocolsModel) loadDetailRelationships(protocolID string) tea.Cmd {
	return func() tea.Msg {
		rels, err := m.client.GetRelationships("protocol", protocolID)
		if err != nil {
			return protocolRelationshipsLoadedMsg{id: protocolID, relationships: nil}
		}
		return protocolRelationshipsLoadedMsg{id: protocolID, relationships: rels}
	}
}

// --- Add ---

func (m ProtocolsModel) handleAddKeys(msg tea.KeyPressMsg) (ProtocolsModel, tea.Cmd) {
	if m.addSaving {
		return m, nil
	}
	if m.addMeta.Active {
		if m.addMeta.HandleKey(msg) {
			m.addMeta.Active = false
		}
		return m, nil
	}
	switch {
	case isBack(msg):
		m.view = protocolsViewList
	case isDown(msg):
		m.addFocus = (m.addFocus + 1) % protoFieldCount
	case isUp(msg):
		m.addFocus = (m.addFocus - 1 + protoFieldCount) % protoFieldCount
	case isKey(msg, "ctrl+s"):
		return m.saveAdd()
	}

	if m.addFocus == protoFieldTags {
		return m.handleTagInput(msg, true)
	}
	if m.addFocus == protoFieldApplies {
		return m.handleApplyInput(msg, true)
	}
	if m.addFocus == protoFieldMetadata {
		if isEnter(msg) || isSpace(msg) {
			m.addMeta.Active = true
		}
		return m, nil
	}

	if m.addFocus == protoFieldStatus {
		if isKey(msg, "left") {
			m.addStatusIdx = (m.addStatusIdx - 1 + len(protocolStatusOptions)) % len(protocolStatusOptions)
			return m, nil
		}
		if isKey(msg, "right") {
			m.addStatusIdx = (m.addStatusIdx + 1) % len(protocolStatusOptions)
			return m, nil
		}
	}

	if ch := keyText(msg); ch != "" {
		m.addFields[m.addFocus].value += ch
		return m, nil
	}
	if isKey(msg, "backspace", "delete") {
		v := m.addFields[m.addFocus].value
		if v != "" {
			m.addFields[m.addFocus].value = v[:len(v)-1]
		}
	}
	return m, nil
}

// renderAdd renders render add.
func (m ProtocolsModel) renderAdd() string {
	rows := make([][2]string, 0, len(m.addFields))
	for i, f := range m.addFields {
		var value string
		switch i {
		case protoFieldStatus:
			value = protocolStatusOptions[m.addStatusIdx]
		case protoFieldTags:
			value = m.renderTags(m.addTags, m.addTagBuf)
		case protoFieldApplies:
			value = m.renderApplies(m.addApplies, m.addApplyBuf)
		case protoFieldMetadata:
			value = m.addMeta.Buffer
		default:
			value = formatFormValue(f.value, i == m.addFocus)
		}
		rows = append(rows, [2]string{f.label, value})
	}
	body := renderFormGrid("Add Protocol", rows, m.addFocus, m.width)
	if m.addErr != "" {
		body += "\n\n" + ErrorStyle.Render(m.addErr)
	}
	return body
}

// saveAdd handles save add.
func (m ProtocolsModel) saveAdd() (ProtocolsModel, tea.Cmd) {
	name := strings.TrimSpace(m.addFields[protoFieldName].value)
	if name == "" {
		m.addErr = "Name is required"
		return m, nil
	}
	title := strings.TrimSpace(m.addFields[protoFieldTitle].value)
	if title == "" {
		m.addErr = "Title is required"
		return m, nil
	}
	content := strings.TrimSpace(m.addFields[protoFieldContent].value)
	if content == "" {
		m.addErr = "Content is required"
		return m, nil
	}

	m.commitTag(true)
	m.commitApply(true)

	input := api.CreateProtocolInput{
		Name:         name,
		Title:        title,
		Version:      strings.TrimSpace(m.addFields[protoFieldVersion].value),
		Content:      content,
		ProtocolType: strings.TrimSpace(m.addFields[protoFieldType].value),
		AppliesTo:    append([]string{}, m.addApplies...),
		Status:       protocolStatusOptions[m.addStatusIdx],
		Tags:         append([]string{}, m.addTags...),
		Notes:        m.addMeta.Buffer,
		SourcePath:   stringPtr(strings.TrimSpace(m.addFields[protoFieldSourcePath].value)),
	}
	m.addSaving = true
	return m, func() tea.Msg {
		if _, err := m.client.CreateProtocol(input); err != nil {
			return errMsg{err}
		}
		return protocolCreatedMsg{}
	}
}

// --- Edit ---

func (m *ProtocolsModel) startEdit() {
	if m.detail == nil {
		return
	}
	p := m.detail
	for i := range m.editFields {
		m.editFields[i].value = ""
	}
	m.editFields[protoEditFieldTitle].value = p.Title
	if p.Version != nil {
		m.editFields[protoEditFieldVersion].value = *p.Version
	}
	if p.ProtocolType != nil {
		m.editFields[protoEditFieldType].value = *p.ProtocolType
	}
	m.editApplies = append([]string{}, p.AppliesTo...)
	m.editTags = append([]string{}, p.Tags...)
	m.editFields[protoEditFieldContent].value = ""
	if p.Content != nil {
		m.editFields[protoEditFieldContent].value = *p.Content
	}
	if p.SourcePath != nil {
		m.editFields[protoEditFieldSourcePath].value = *p.SourcePath
	}
	m.editStatusIdx = statusIndex(protocolStatusOptions, p.Status)
	m.editMeta.Buffer = p.Notes
	m.editFocus = 0
	m.editSaving = false
}

// handleEditKeys handles handle edit keys.
func (m ProtocolsModel) handleEditKeys(msg tea.KeyPressMsg) (ProtocolsModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if m.editMeta.Active {
		if m.editMeta.HandleKey(msg) {
			m.editMeta.Active = false
		}
		return m, nil
	}
	switch {
	case isBack(msg):
		m.view = protocolsViewDetail
	case isDown(msg):
		m.editFocus = (m.editFocus + 1) % protoEditFieldCount
	case isUp(msg):
		m.editFocus = (m.editFocus - 1 + protoEditFieldCount) % protoEditFieldCount
	case isKey(msg, "ctrl+s"):
		return m.saveEdit()
	}

	if m.editFocus == protoEditFieldTags {
		return m.handleTagInput(msg, false)
	}
	if m.editFocus == protoEditFieldApplies {
		return m.handleApplyInput(msg, false)
	}
	if m.editFocus == protoEditFieldMetadata {
		if isEnter(msg) || isSpace(msg) {
			m.editMeta.Active = true
		}
		return m, nil
	}
	if m.editFocus == protoEditFieldStatus {
		if isKey(msg, "left") {
			m.editStatusIdx = (m.editStatusIdx - 1 + len(protocolStatusOptions)) % len(protocolStatusOptions)
			return m, nil
		}
		if isKey(msg, "right") {
			m.editStatusIdx = (m.editStatusIdx + 1) % len(protocolStatusOptions)
			return m, nil
		}
	}
	if ch := keyText(msg); ch != "" {
		m.editFields[m.editFocus].value += ch
		return m, nil
	}
	if isKey(msg, "backspace", "delete") {
		v := m.editFields[m.editFocus].value
		if v != "" {
			m.editFields[m.editFocus].value = v[:len(v)-1]
		}
	}
	return m, nil
}

// renderEdit renders render edit.
func (m ProtocolsModel) renderEdit() string {
	rows := make([][2]string, 0, len(m.editFields))
	for i, f := range m.editFields {
		var value string
		switch i {
		case protoEditFieldStatus:
			value = protocolStatusOptions[m.editStatusIdx]
		case protoEditFieldTags:
			value = m.renderTags(m.editTags, m.editTagBuf)
		case protoEditFieldApplies:
			value = m.renderApplies(m.editApplies, m.editApplyBuf)
		case protoEditFieldMetadata:
			value = m.editMeta.Buffer
		default:
			value = formatFormValue(f.value, i == m.editFocus)
		}
		rows = append(rows, [2]string{f.label, value})
	}
	return renderFormGrid("Edit Protocol", rows, m.editFocus, m.width)
}

// saveEdit handles save edit.
func (m ProtocolsModel) saveEdit() (ProtocolsModel, tea.Cmd) {
	if m.detail == nil {
		return m, nil
	}
	m.commitTag(false)
	m.commitApply(false)
	input := api.UpdateProtocolInput{
		Title:        stringPtr(strings.TrimSpace(m.editFields[protoEditFieldTitle].value)),
		Version:      stringPtr(strings.TrimSpace(m.editFields[protoEditFieldVersion].value)),
		Content:      stringPtr(strings.TrimSpace(m.editFields[protoEditFieldContent].value)),
		ProtocolType: stringPtr(strings.TrimSpace(m.editFields[protoEditFieldType].value)),
		AppliesTo:    slicePtr(m.editApplies),
		Status:       stringPtr(protocolStatusOptions[m.editStatusIdx]),
		Tags:         slicePtr(m.editTags),
		Notes:        m.editMeta.Buffer,
		SourcePath:   stringPtr(strings.TrimSpace(m.editFields[protoEditFieldSourcePath].value)),
	}

	m.editSaving = true
	name := m.detail.Name
	return m, func() tea.Msg {
		if _, err := m.client.UpdateProtocol(name, input); err != nil {
			return errMsg{err}
		}
		return protocolUpdatedMsg{}
	}
}

// --- Helpers ---

func (m ProtocolsModel) renderTags(tags []string, buf string) string {
	out := append([]string{}, tags...)
	if strings.TrimSpace(buf) != "" {
		out = append(out, buf)
	}
	return strings.Join(out, ", ")
}

// renderApplies renders render applies.
func (m ProtocolsModel) renderApplies(items []string, buf string) string {
	out := append([]string{}, items...)
	if strings.TrimSpace(buf) != "" {
		out = append(out, buf)
	}
	return strings.Join(out, ", ")
}

// commitTag handles commit tag.
func (m *ProtocolsModel) commitTag(addMode bool) {
	buf := strings.TrimSpace(m.addTagBuf)
	if !addMode {
		buf = strings.TrimSpace(m.editTagBuf)
	}
	if buf == "" {
		return
	}
	tag := normalizeTag(buf)
	if tag == "" {
		if addMode {
			m.addTagBuf = ""
		} else {
			m.editTagBuf = ""
		}
		return
	}
	if addMode {
		m.addTags = append(m.addTags, tag)
		m.addTagBuf = ""
	} else {
		m.editTags = append(m.editTags, tag)
		m.editTagBuf = ""
	}
}

// commitApply handles commit apply.
func (m *ProtocolsModel) commitApply(addMode bool) {
	buf := strings.TrimSpace(m.addApplyBuf)
	if !addMode {
		buf = strings.TrimSpace(m.editApplyBuf)
	}
	if buf == "" {
		return
	}
	item := strings.TrimSpace(buf)
	if addMode {
		m.addApplies = append(m.addApplies, item)
		m.addApplyBuf = ""
	} else {
		m.editApplies = append(m.editApplies, item)
		m.editApplyBuf = ""
	}
}

// handleTagInput handles handle tag input.
func (m ProtocolsModel) handleTagInput(msg tea.KeyPressMsg, addMode bool) (ProtocolsModel, tea.Cmd) {
	if isKey(msg, "backspace", "delete") {
		if addMode {
			if len(m.addTagBuf) > 0 {
				m.addTagBuf = m.addTagBuf[:len(m.addTagBuf)-1]
			}
		} else {
			if len(m.editTagBuf) > 0 {
				m.editTagBuf = m.editTagBuf[:len(m.editTagBuf)-1]
			}
		}
		return m, nil
	}
	if isKey(msg, "enter", ",", " ", "space") {
		m.commitTag(addMode)
		return m, nil
	}
	if ch := keyText(msg); ch != "" {
		if addMode {
			m.addTagBuf += ch
		} else {
			m.editTagBuf += ch
		}
	}
	return m, nil
}

// handleApplyInput handles handle apply input.
func (m ProtocolsModel) handleApplyInput(msg tea.KeyPressMsg, addMode bool) (ProtocolsModel, tea.Cmd) {
	if isKey(msg, "backspace", "delete") {
		if addMode {
			if len(m.addApplyBuf) > 0 {
				m.addApplyBuf = m.addApplyBuf[:len(m.addApplyBuf)-1]
			}
		} else {
			if len(m.editApplyBuf) > 0 {
				m.editApplyBuf = m.editApplyBuf[:len(m.editApplyBuf)-1]
			}
		}
		return m, nil
	}
	if isKey(msg, "enter", ",", " ", "space") {
		m.commitApply(addMode)
		return m, nil
	}
	if ch := keyText(msg); ch != "" {
		if addMode {
			m.addApplyBuf += ch
		} else {
			m.editApplyBuf += ch
		}
	}
	return m, nil
}

// stringPtr handles string ptr.
func stringPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

// slicePtr handles slice ptr.
func slicePtr(items []string) *[]string {
	if len(items) == 0 {
		return nil
	}
	out := append([]string{}, items...)
	return &out
}

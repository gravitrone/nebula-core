package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type logsLoadedMsg struct{ items []api.Log }
type logCreatedMsg struct{}
type logUpdatedMsg struct{}
type logsScopesLoadedMsg struct{ options []string }
type logRelationshipsLoadedMsg struct {
	id            string
	relationships []api.Relationship
}

type logsView int

const (
	logsViewAdd logsView = iota
	logsViewList
	logsViewDetail
	logsViewEdit
)

// --- Logs Model ---

type LogsModel struct {
	client        *api.Client
	items         []api.Log
	allItems      []api.Log
	dataTable     table.Model
	loading       bool
	spinner       spinner.Model
	view          logsView
	modeFocus     bool
	filtering     bool
	searchBuf     string
	searchSuggest string
	detail        *api.Log
	detailRels    []api.Relationship
	errText       string
	addErr        string
	valueExpanded bool
	metaExpanded  bool
	width         int
	height        int
	scopeOptions  []string

	// add (huh form)
	addForm      *huh.Form
	addType      string
	addTimestamp string
	addStatus    string
	addTagStr    string
	addValue     MetadataEditor
	addMeta      MetadataEditor
	addSaving    bool
	addSaved     bool

	// edit (huh form)
	editForm      *huh.Form
	editType      string
	editTimestamp string
	editStatus    string
	editTagStr    string
	editValue     MetadataEditor
	editMeta      MetadataEditor
	editSaving    bool
}

// NewLogsModel builds the logs UI model.
func NewLogsModel(client *api.Client) LogsModel {
	return LogsModel{
		client:    client,
		spinner:   components.NewNebulaSpinner(),
		dataTable: components.NewNebulaTable(nil, 12),
		view:      logsViewList,
		addStatus: "active",
	}
}

// Init handles init.
func (m LogsModel) Init() tea.Cmd {
	m.loading = true
	m.view = logsViewList
	m.modeFocus = false
	m.filtering = false
	m.searchBuf = ""
	m.searchSuggest = ""
	m.detail = nil
	m.detailRels = nil
	m.errText = ""
	m.valueExpanded = false
	m.metaExpanded = false
	m.addStatus = "active"
	m.addTagStr = ""
	m.addType = ""
	m.addTimestamp = ""
	m.addForm = nil
	m.addValue.Reset()
	m.addMeta.Reset()
	m.addSaving = false
	m.addSaved = false
	m.editStatus = "active"
	m.editTagStr = ""
	m.editType = ""
	m.editTimestamp = ""
	m.editForm = nil
	m.editValue.Reset()
	m.editMeta.Reset()
	m.editSaving = false
	return tea.Batch(m.loadLogs(), m.spinner.Tick)
}

// Update updates update.
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case logsLoadedMsg:
		m.loading = false
		m.allItems = msg.items
		m.applyLogSearch()
		return m, m.loadScopeOptions()
	case logsScopesLoadedMsg:
		m.scopeOptions = msg.options
		m.addMeta.SetScopeOptions(m.scopeOptions)
		m.editMeta.SetScopeOptions(m.scopeOptions)
		return m, nil
	case logRelationshipsLoadedMsg:
		if m.detail != nil && m.detail.ID == msg.id {
			m.detailRels = msg.relationships
		}
		return m, nil
	case logCreatedMsg:
		m.addSaving = false
		m.addSaved = true
		m.loading = true
		return m, tea.Batch(m.loadLogs(), m.spinner.Tick)
	case logUpdatedMsg:
		m.editSaving = false
		m.detail = nil
		m.view = logsViewList
		m.loading = true
		return m, tea.Batch(m.loadLogs(), m.spinner.Tick)
	case errMsg:
		m.loading = false
		m.addSaving = false
		m.editSaving = false
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyPressMsg:
		if m.addValue.Active {
			m.addValue.HandleKey(msg)
			return m, nil
		}
		if m.addMeta.Active {
			m.addMeta.HandleKey(msg)
			return m, nil
		}
		if m.editValue.Active {
			m.editValue.HandleKey(msg)
			return m, nil
		}
		if m.editMeta.Active {
			m.editMeta.HandleKey(msg)
			return m, nil
		}
		if m.modeFocus {
			return m.handleModeKeys(msg)
		}
		switch m.view {
		case logsViewAdd:
			return m.handleAddKeys(msg)
		case logsViewEdit:
			return m.handleEditKeys(msg)
		case logsViewDetail:
			return m.handleDetailKeys(msg)
		default:
			return m.handleListKeys(msg)
		}
	}
	return m, nil
}

// View handles view.
func (m LogsModel) View() string {
	if m.addValue.Active {
		return m.addValue.Render(m.width)
	}
	if m.addMeta.Active {
		return m.addMeta.Render(m.width)
	}
	if m.editValue.Active {
		return m.editValue.Render(m.width)
	}
	if m.editMeta.Active {
		return m.editMeta.Render(m.width)
	}
	if m.filtering && m.view == logsViewList {
		return components.Indent(components.InputDialog("Filter Logs", m.searchBuf), 1)
	}
	modeLine := m.renderModeLine()
	var body string
	switch m.view {
	case logsViewAdd:
		body = m.renderAdd()
	case logsViewEdit:
		body = m.renderEdit()
	case logsViewDetail:
		body = m.renderDetail()
	default:
		body = m.renderList()
	}
	if modeLine != "" {
		body = components.CenterLine(modeLine, m.width) + "\n\n" + body
	}
	return components.Indent(body, 1)
}

// --- Mode Line ---

func (m LogsModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("Library")
	if m.view == logsViewAdd {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("Library")
	}
	if m.modeFocus {
		if m.view == logsViewAdd {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("Library")
		}
	}
	return add + " " + list
}

// handleModeKeys handles handle mode keys.
func (m LogsModel) handleModeKeys(msg tea.KeyPressMsg) (LogsModel, tea.Cmd) {
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
func (m LogsModel) toggleMode() (LogsModel, tea.Cmd) {
	m.modeFocus = false
	if m.view == logsViewAdd {
		m.view = logsViewList
		return m, nil
	}
	m.view = logsViewAdd
	m.addSaved = false
	return m, nil
}

// --- List View ---

func (m LogsModel) renderList() string {
	if m.loading {
		return "  " + m.spinner.View() + " " + MutedStyle.Render("Loading logs...")
	}
	if len(m.items) == 0 {
		return components.EmptyStateBox(
			"Logs",
			"No logs found.",
			[]string{"Press tab to switch Add/Library", "Press / for command palette"},
			m.width,
		)
	}

	contentWidth := components.BoxContentWidth(m.width)

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap
	}

	// Each table cell has Padding(0,1) = 2 chars. 4 columns = 8 chars of padding.
	cellPadding := 4 * 2
	availableCols := tableWidth - cellPadding
	if availableCols < 30 {
		availableCols = 30
	}

	statusWidth := 11
	atWidth := compactTimeColumnWidth
	typeWidth := 16
	valueWidth := availableCols - (typeWidth + statusWidth + atWidth)
	if valueWidth < 14 {
		valueWidth = 14
		typeWidth = availableCols - (valueWidth + statusWidth + atWidth)
		if typeWidth < 12 {
			typeWidth = 12
		}
	}

	tableRows := make([]table.Row, len(m.items))
	for i, l := range m.items {
		typ := strings.TrimSpace(components.SanitizeOneLine(l.LogType))
		if typ == "" {
			typ = "log"
		}
		status := strings.TrimSpace(components.SanitizeOneLine(l.Status))
		if status == "" {
			status = "-"
		}
		value := metadataPreview(map[string]any(l.Value), 80)
		if strings.TrimSpace(value) == "" {
			value = "-"
		}
		at := l.Timestamp
		if at.IsZero() {
			at = l.UpdatedAt
		}
		if at.IsZero() {
			at = l.CreatedAt
		}

		tableRows[i] = table.Row{
			components.ClampTextWidthEllipsis(typ, typeWidth),
			components.ClampTextWidthEllipsis(value, valueWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			formatLocalTimeCompact(at),
		}
	}

	m.dataTable.SetColumns([]table.Column{
		{Title: "Type", Width: typeWidth},
		{Title: "Value", Width: valueWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "At", Width: atWidth},
	})
	m.dataTable.SetWidth(tableWidth)
	m.dataTable.SetRows(tableRows)

	countLine := fmt.Sprintf("%d total", len(m.items))
	if strings.TrimSpace(m.searchBuf) != "" {
		countLine = fmt.Sprintf("%s · search: %s", countLine, strings.TrimSpace(m.searchBuf))
		if m.searchSuggest != "" && !strings.EqualFold(strings.TrimSpace(m.searchBuf), strings.TrimSpace(m.searchSuggest)) {
			countLine = fmt.Sprintf("%s · next: %s", countLine, strings.TrimSpace(m.searchSuggest))
		}
	}
	countLine = MutedStyle.Render(countLine)

	tableView := m.dataTable.View()
	preview := ""
	var previewItem *api.Log
	if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}
	if previewItem != nil {
		content := m.renderLogPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.TitledBox("Logs", content, m.width)
}

// renderLogPreview renders render log preview.
func (m LogsModel) renderLogPreview(l api.Log, width int) string {
	if width <= 0 {
		return ""
	}

	title := strings.TrimSpace(components.SanitizeOneLine(l.LogType))
	if title == "" {
		title = "log"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(l.Status))
	if status == "" {
		status = "-"
	}
	at := l.Timestamp
	if at.IsZero() {
		at = l.UpdatedAt
	}
	if at.IsZero() {
		at = l.CreatedAt
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeFull(at), width))
	if len(l.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(l.Tags, ", "), width))
	}
	if valuePreview := metadataPreview(map[string]any(l.Value), 120); valuePreview != "" {
		lines = append(lines, renderPreviewRow("Value", valuePreview, width))
	}
	if metaPreview := metadataPreview(map[string]any(l.Metadata), 80); metaPreview != "" {
		lines = append(lines, renderPreviewRow("Meta", metaPreview, width))
	}

	return padPreviewLines(lines, width)
}

// handleListKeys handles handle list keys.
func (m LogsModel) handleListKeys(msg tea.KeyPressMsg) (LogsModel, tea.Cmd) {
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
	case isEnter(msg), isSpace(msg):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			item := m.items[idx]
			m.detail = &item
			m.detailRels = nil
			m.view = logsViewDetail
			return m, m.loadDetailRelationships(item.ID)
		}
	case isKey(msg, "f"):
		m.filtering = true
		return m, nil
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applyLogSearch()
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.applyLogSearch()
		}
	case isBack(msg):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.applyLogSearch()
		}
	case isKey(msg, "tab"):
		if m.searchSuggest != "" && !strings.EqualFold(strings.TrimSpace(m.searchBuf), strings.TrimSpace(m.searchSuggest)) {
			m.searchBuf = m.searchSuggest
			m.applyLogSearch()
		}
	default:
		ch := keyText(msg)
		if ch != "" {
			m.searchBuf += ch
			m.applyLogSearch()
		}
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m LogsModel) handleFilterInput(msg tea.KeyPressMsg) (LogsModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
	case isBack(msg):
		m.filtering = false
		m.searchBuf = ""
		m.searchSuggest = ""
		m.applyLogSearch()
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applyLogSearch()
		}
	default:
		ch := keyText(msg)
		if ch != "" {
			if ch == " " && m.searchBuf == "" {
				return m, nil
			}
			m.searchBuf += ch
			m.applyLogSearch()
		}
	}
	return m, nil
}

// --- Detail View ---

func (m LogsModel) handleDetailKeys(msg tea.KeyPressMsg) (LogsModel, tea.Cmd) {
	switch {
	case isUp(msg):
		m.modeFocus = true
	case isBack(msg):
		m.detail = nil
		m.detailRels = nil
		m.valueExpanded = false
		m.metaExpanded = false
		m.view = logsViewList
	case isKey(msg, "e"):
		m.startEdit()
		m.view = logsViewEdit
	case isKey(msg, "v"):
		m.valueExpanded = !m.valueExpanded
	case isKey(msg, "m"):
		m.metaExpanded = !m.metaExpanded
	}
	return m, nil
}

// renderDetail renders render detail.
func (m LogsModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}
	l := m.detail
	rows := []components.TableRow{
		{Label: "ID", Value: l.ID},
		{Label: "Type", Value: l.LogType},
		{Label: "Timestamp", Value: formatLocalTimeFull(l.Timestamp)},
	}
	if l.Status != "" {
		rows = append(rows, components.TableRow{Label: "Status", Value: l.Status})
	}
	if len(l.Tags) > 0 {
		rows = append(rows, components.TableRow{Label: "Tags", Value: strings.Join(l.Tags, ", ")})
	}
	rows = append(rows, components.TableRow{Label: "Created", Value: formatLocalTimeFull(l.CreatedAt)})
	if !l.UpdatedAt.IsZero() {
		rows = append(rows, components.TableRow{Label: "Updated", Value: formatLocalTimeFull(l.UpdatedAt)})
	}

	sections := []string{components.Table("Log", rows, m.width)}
	if len(l.Value) > 0 {
		sections = append(sections, renderMetadataBlockWithTitle("Value", map[string]any(l.Value), m.width, m.valueExpanded))
	}
	if len(l.Metadata) > 0 {
		sections = append(sections, renderMetadataBlockWithTitle("Metadata", map[string]any(l.Metadata), m.width, m.metaExpanded))
	}
	if len(m.detailRels) > 0 {
		sections = append(sections, renderRelationshipSummaryTable("log", l.ID, m.detailRels, 6, m.width))
	}
	return strings.Join(sections, "\n\n")
}

// loadDetailRelationships loads load detail relationships.
func (m LogsModel) loadDetailRelationships(logID string) tea.Cmd {
	return func() tea.Msg {
		rels, err := m.client.GetRelationships("log", logID)
		if err != nil {
			return logRelationshipsLoadedMsg{id: logID, relationships: nil}
		}
		return logRelationshipsLoadedMsg{id: logID, relationships: rels}
	}
}

// --- Add View ---

// initAddForm initializes the huh add form.
func (m *LogsModel) initAddForm() {
	m.addForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Type").Value(&m.addType),
			huh.NewInput().Title("Timestamp").Description("YYYY-MM-DD or RFC3339").Value(&m.addTimestamp),
			huh.NewSelect[string]().Title("Status").Options(
				huh.NewOption("active", "active"),
				huh.NewOption("inactive", "inactive"),
			).Value(&m.addStatus),
			huh.NewInput().Title("Tags").Description("Comma-separated").Value(&m.addTagStr),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

func (m LogsModel) handleAddKeys(msg tea.KeyPressMsg) (LogsModel, tea.Cmd) {
	if m.addSaving {
		return m, nil
	}
	if m.addSaved {
		if isBack(msg) {
			m.resetAddForm()
		}
		return m, nil
	}
	if m.addForm == nil {
		m.initAddForm()
		return m, m.addForm.Init()
	}
	var formCmd tea.Cmd
	_, formCmd = m.addForm.Update(msg)
	if m.addForm.State == huh.StateCompleted {
		return m.saveAdd()
	}
	if m.addForm.State == huh.StateAborted {
		m.resetAddForm()
		return m, nil
	}
	return m, formCmd
}

// renderAdd renders render add.
func (m LogsModel) renderAdd() string {
	if m.addSaving {
		return components.Indent(MutedStyle.Render("Saving..."), 1)
	}
	if m.addSaved {
		var b strings.Builder
		b.WriteString(SuccessStyle.Render("Log saved!"))
		b.WriteString("\n\n" + MutedStyle.Render("Press Esc to add another."))
		return components.Indent(b.String(), 1)
	}
	if m.addForm == nil {
		return components.Indent(MutedStyle.Render("Initializing..."), 1)
	}
	var b strings.Builder
	b.WriteString(m.addForm.View())
	valuePreview := renderMetadataEditorPreview(m.addValue.Buffer, m.addValue.Scopes, m.width, 6)
	metaPreview := renderMetadataEditorPreview(m.addMeta.Buffer, m.addMeta.Scopes, m.width, 6)
	if valuePreview != "" {
		b.WriteString("\n" + MutedStyle.Render("Value:") + "\n  " + NormalStyle.Render(valuePreview))
	}
	if metaPreview != "" {
		b.WriteString("\n" + MutedStyle.Render("Metadata:") + "\n  " + NormalStyle.Render(metaPreview))
	}
	if m.addErr != "" {
		b.WriteString("\n\n" + ErrorStyle.Render(m.addErr))
	}
	return components.Indent(b.String(), 1)
}

// saveAdd handles save add.
func (m LogsModel) saveAdd() (LogsModel, tea.Cmd) {
	logType := strings.TrimSpace(m.addType)
	if logType == "" {
		m.addErr = "Type is required"
		return m, nil
	}
	timestamp, err := parseLogTimestamp(m.addTimestamp)
	if err != nil {
		m.addErr = err.Error()
		return m, nil
	}
	value, err := parseMetadataInput(m.addValue.Buffer)
	if err != nil {
		m.addErr = err.Error()
		return m, nil
	}
	meta, err := parseMetadataInput(m.addMeta.Buffer)
	if err != nil {
		m.addErr = err.Error()
		return m, nil
	}
	meta = mergeMetadataScopes(meta, m.addMeta.Scopes)
	tags := parseCommaSeparated(m.addTagStr)

	input := api.CreateLogInput{
		LogType:   logType,
		Status:    m.addStatus,
		Tags:      tags,
		Value:     value,
		Metadata:  meta,
		Timestamp: timestamp,
	}
	m.addSaving = true
	m.addErr = ""
	return m, func() tea.Msg {
		if _, err := m.client.CreateLog(input); err != nil {
			return errMsg{err}
		}
		return logCreatedMsg{}
	}
}

// resetAddForm handles reset add form.
func (m *LogsModel) resetAddForm() {
	m.addSaved = false
	m.addSaving = false
	m.addErr = ""
	m.addStatus = "active"
	m.addTagStr = ""
	m.addType = ""
	m.addTimestamp = ""
	m.addForm = nil
	m.addValue.Reset()
	m.addMeta.Reset()
}

// --- Edit View ---

func (m *LogsModel) startEdit() {
	if m.detail == nil {
		return
	}
	l := m.detail
	m.editStatus = l.Status
	if m.editStatus == "" {
		m.editStatus = "active"
	}
	m.editTagStr = strings.Join(l.Tags, ", ")
	m.editType = l.LogType
	m.editTimestamp = l.Timestamp.Format(time.RFC3339)
	m.editValue.Load(map[string]any(l.Value))
	m.editMeta.Load(map[string]any(l.Metadata))
	m.editSaving = false
	m.initEditForm()
}

// initEditForm initializes the huh edit form.
func (m *LogsModel) initEditForm() {
	m.editForm = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().Title("Status").Options(
				huh.NewOption("active", "active"),
				huh.NewOption("inactive", "inactive"),
			).Value(&m.editStatus),
			huh.NewInput().Title("Tags").Description("Comma-separated").Value(&m.editTagStr),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

// handleEditKeys handles handle edit keys.
func (m LogsModel) handleEditKeys(msg tea.KeyPressMsg) (LogsModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if isBack(msg) {
		m.view = logsViewDetail
		return m, nil
	}
	if m.editForm == nil {
		m.initEditForm()
		return m, m.editForm.Init()
	}
	var formCmd tea.Cmd
	_, formCmd = m.editForm.Update(msg)
	if m.editForm.State == huh.StateCompleted {
		return m.saveEdit()
	}
	if m.editForm.State == huh.StateAborted {
		m.view = logsViewDetail
		return m, nil
	}
	return m, formCmd
}

// renderEdit renders render edit.
func (m LogsModel) renderEdit() string {
	if m.editSaving {
		return components.Indent(MutedStyle.Render("Saving..."), 1)
	}
	if m.editForm == nil {
		return components.Indent(MutedStyle.Render("Initializing..."), 1)
	}
	var b strings.Builder
	b.WriteString(m.editForm.View())
	valuePreview := renderMetadataEditorPreview(m.editValue.Buffer, m.editValue.Scopes, m.width, 6)
	metaPreview := renderMetadataEditorPreview(m.editMeta.Buffer, m.editMeta.Scopes, m.width, 6)
	if valuePreview != "" {
		b.WriteString("\n" + MutedStyle.Render("Value:") + "\n  " + NormalStyle.Render(valuePreview))
	}
	if metaPreview != "" {
		b.WriteString("\n" + MutedStyle.Render("Metadata:") + "\n  " + NormalStyle.Render(metaPreview))
	}
	if m.errText != "" {
		b.WriteString("\n\n" + ErrorStyle.Render(m.errText))
	}
	return components.Indent(b.String(), 1)
}

// saveEdit handles save edit.
func (m LogsModel) saveEdit() (LogsModel, tea.Cmd) {
	value, err := parseMetadataInput(m.editValue.Buffer)
	if err != nil {
		m.errText = err.Error()
		return m, nil
	}
	meta, err := parseMetadataInput(m.editMeta.Buffer)
	if err != nil {
		m.errText = err.Error()
		return m, nil
	}
	meta = mergeMetadataScopes(meta, m.editMeta.Scopes)
	tags := parseCommaSeparated(m.editTagStr)

	input := api.UpdateLogInput{
		Status:   &m.editStatus,
		Tags:     &tags,
		Value:    value,
		Metadata: meta,
	}
	m.editSaving = true
	m.errText = ""
	return m, func() tea.Msg {
		if _, err := m.client.UpdateLog(m.detail.ID, input); err != nil {
			return errMsg{err}
		}
		return logUpdatedMsg{}
	}
}

// --- Data ---

func (m LogsModel) loadLogs() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryLogs(api.QueryParams{"status_category": "active"})
		if err != nil {
			return errMsg{err}
		}
		return logsLoadedMsg{items}
	}
}

// loadScopeOptions loads load scope options.
func (m LogsModel) loadScopeOptions() tea.Cmd {
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
		return logsScopesLoadedMsg{options: scopeNameList(names)}
	}
}

// applyLogSearch handles apply log search.
func (m *LogsModel) applyLogSearch() {
	query := strings.TrimSpace(strings.ToLower(m.searchBuf))
	if query == "" {
		m.items = m.allItems
	} else {
		filtered := make([]api.Log, 0, len(m.allItems))
		for _, l := range m.allItems {
			hay := strings.ToLower(strings.Join([]string{l.LogType, l.ID, l.Status}, " "))
			if strings.Contains(hay, query) {
				filtered = append(filtered, l)
			}
		}
		m.items = filtered
	}
	rows := make([]table.Row, len(m.items))
	for i, l := range m.items {
		rows[i] = table.Row{formatLogLine(l)}
	}
	m.dataTable.SetRows(rows)
	m.dataTable.SetCursor(0)
	m.updateSearchSuggest()
}

// updateSearchSuggest updates update search suggest.
func (m *LogsModel) updateSearchSuggest() {
	m.searchSuggest = ""
	query := strings.ToLower(strings.TrimSpace(m.searchBuf))
	if query == "" {
		return
	}
	for _, l := range m.allItems {
		if strings.HasPrefix(strings.ToLower(l.LogType), query) {
			m.searchSuggest = l.LogType
			return
		}
	}
}

// formatLogLine handles format log line.
func formatLogLine(l api.Log) string {
	label := components.SanitizeText(l.LogType)
	if label == "" {
		label = "log"
	}
	stamp := l.Timestamp.Format("2006-01-02")
	segments := []string{label, stamp}
	if l.Status != "" {
		segments = append(segments, components.SanitizeText(l.Status))
	}
	if preview := metadataPreview(map[string]any(l.Metadata), 40); preview != "" {
		segments = append(segments, preview)
	}
	return strings.Join(segments, " · ")
}

// parseLogTimestamp parses parse log timestamp.
func parseLogTimestamp(input string) (*time.Time, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("timestamp: use YYYY-MM-DD or RFC3339")
}

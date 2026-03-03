package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

const (
	logFieldType = iota
	logFieldTimestamp
	logFieldStatus
	logFieldTags
	logFieldValue
	logFieldMeta
	logFieldCount
)

const (
	logEditFieldStatus = iota
	logEditFieldTags
	logEditFieldValue
	logEditFieldMeta
	logEditFieldCount
)

var logStatusOptions = []string{"active", "inactive"}

// --- Logs Model ---

type LogsModel struct {
	client        *api.Client
	items         []api.Log
	allItems      []api.Log
	list          *components.List
	loading       bool
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

	// add
	addFields    []formField
	addFocus     int
	addStatusIdx int
	addTags      []string
	addTagBuf    string
	addType      string
	addTimestamp string
	addValue     MetadataEditor
	addMeta      MetadataEditor
	addSaving    bool
	addSaved     bool

	// edit
	editFocus     int
	editStatusIdx int
	editTags      []string
	editTagBuf    string
	editType      string
	editTimestamp string
	editValue     MetadataEditor
	editMeta      MetadataEditor
	editSaving    bool
}

// NewLogsModel builds the logs UI model.
func NewLogsModel(client *api.Client) LogsModel {
	return LogsModel{
		client: client,
		list:   components.NewList(12),
		view:   logsViewList,
		addFields: []formField{
			{label: "Type"},
			{label: "Timestamp"},
			{label: "Status"},
			{label: "Tags"},
			{label: "Value"},
			{label: "Metadata"},
		},
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
	m.addFocus = 0
	m.addStatusIdx = statusIndex(logStatusOptions, "active")
	m.addTags = nil
	m.addTagBuf = ""
	m.addType = ""
	m.addTimestamp = ""
	m.addValue.Reset()
	m.addMeta.Reset()
	m.addSaving = false
	m.addSaved = false
	m.editFocus = 0
	m.editStatusIdx = statusIndex(logStatusOptions, "active")
	m.editTags = nil
	m.editTagBuf = ""
	m.editType = ""
	m.editTimestamp = ""
	m.editValue.Reset()
	m.editMeta.Reset()
	m.editSaving = false
	return m.loadLogs()
}

// Update updates update.
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
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
		return m, m.loadLogs()
	case logUpdatedMsg:
		m.editSaving = false
		m.detail = nil
		m.view = logsViewList
		m.loading = true
		return m, m.loadLogs()
	case errMsg:
		m.loading = false
		m.addSaving = false
		m.editSaving = false
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyMsg:
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
func (m LogsModel) handleModeKeys(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
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
		return "  " + MutedStyle.Render("Loading logs...")
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

	cols := []components.TableColumn{
		{Header: "Type", Width: typeWidth, Align: lipgloss.Left},
		{Header: "Value", Width: valueWidth, Align: lipgloss.Left},
		{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.Log
	if idx := m.list.Selected(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}

	for i := range visible {
		absIdx := m.list.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.items) {
			continue
		}
		l := m.items[absIdx]

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

		if m.list.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(typ, typeWidth),
			components.ClampTextWidthEllipsis(value, valueWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			formatLocalTimeCompact(at),
		})
	}
	if m.modeFocus {
		activeRowRel = -1
	}

	countLine := fmt.Sprintf("%d total", len(m.items))
	if strings.TrimSpace(m.searchBuf) != "" {
		countLine = fmt.Sprintf("%s · search: %s", countLine, strings.TrimSpace(m.searchBuf))
		if m.searchSuggest != "" && !strings.EqualFold(strings.TrimSpace(m.searchBuf), strings.TrimSpace(m.searchSuggest)) {
			countLine = fmt.Sprintf("%s · next: %s", countLine, strings.TrimSpace(m.searchSuggest))
		}
	}
	countLine = MutedStyle.Render(countLine)

	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderLogPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
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
func (m LogsModel) handleListKeys(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
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
		if idx := m.list.Selected(); idx < len(m.items) {
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
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
			if ch == " " && m.searchBuf == "" {
				return m, nil
			}
			m.searchBuf += ch
			m.applyLogSearch()
		}
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m LogsModel) handleFilterInput(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
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
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
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

func (m LogsModel) handleDetailKeys(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
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

func (m LogsModel) handleAddKeys(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
	if m.addSaving {
		return m, nil
	}
	if m.addSaved {
		if isBack(msg) {
			m.resetAddForm()
		}
		return m, nil
	}
	if m.modeFocus {
		return m.handleModeKeys(msg)
	}
	if m.addFocus == logFieldStatus {
		switch {
		case isKey(msg, "left"):
			m.addStatusIdx = (m.addStatusIdx - 1 + len(logStatusOptions)) % len(logStatusOptions)
			return m, nil
		case isKey(msg, "right"), isSpace(msg):
			m.addStatusIdx = (m.addStatusIdx + 1) % len(logStatusOptions)
			return m, nil
		}
	}
	switch {
	case isDown(msg):
		m.addFocus = (m.addFocus + 1) % logFieldCount
	case isUp(msg):
		if m.addFocus == 0 {
			m.modeFocus = true
			return m, nil
		}
		m.addFocus = (m.addFocus - 1 + logFieldCount) % logFieldCount
	case isKey(msg, "ctrl+s"):
		return m.saveAdd()
	case isBack(msg):
		m.resetAddForm()
	case isKey(msg, "backspace", "delete"):
		switch m.addFocus {
		case logFieldTags:
			if len(m.addTagBuf) > 0 {
				m.addTagBuf = m.addTagBuf[:len(m.addTagBuf)-1]
			} else if len(m.addTags) > 0 {
				m.addTags = m.addTags[:len(m.addTags)-1]
			}
		case logFieldType:
			if len(m.addType) > 0 {
				m.addType = m.addType[:len(m.addType)-1]
			}
		case logFieldTimestamp:
			if len(m.addTimestamp) > 0 {
				m.addTimestamp = m.addTimestamp[:len(m.addTimestamp)-1]
			}
		default:
			return m, nil
		}
	default:
		switch m.addFocus {
		case logFieldTags:
			switch {
			case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
				m.commitAddTag()
			default:
				ch := msg.String()
				if len(ch) == 1 && ch != "," {
					m.addTagBuf += ch
				}
			}
		case logFieldType:
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				m.addType += ch
			}
		case logFieldTimestamp:
			ch := msg.String()
			if len(ch) == 1 || ch == " " || ch == ":" || ch == "-" || ch == "T" || ch == "Z" || ch == "+" {
				m.addTimestamp += ch
			}
		case logFieldValue:
			if isEnter(msg) {
				m.addValue.Active = true
			}
		case logFieldMeta:
			if isEnter(msg) {
				m.addMeta.Active = true
			}
		}
	}
	return m, nil
}

// renderAdd renders render add.
func (m LogsModel) renderAdd() string {
	var b strings.Builder
	for i, f := range m.addFields {
		label := MutedStyle.Render(f.label + ":")
		if i == m.addFocus {
			label = SelectedStyle.Render("  " + f.label + ":")
		} else {
			label = "  " + label
		}
		b.WriteString(label + "\n")

		switch i {
		case logFieldType:
			if m.addType == "" && i != m.addFocus {
				b.WriteString(NormalStyle.Render("  -"))
			} else if i == m.addFocus {
				b.WriteString(NormalStyle.Render("  " + m.addType + AccentStyle.Render("█")))
			} else {
				b.WriteString(NormalStyle.Render("  " + m.addType))
			}
		case logFieldTimestamp:
			if m.addTimestamp == "" && i != m.addFocus {
				b.WriteString(NormalStyle.Render("  -"))
			} else if i == m.addFocus {
				b.WriteString(NormalStyle.Render("  " + m.addTimestamp + AccentStyle.Render("█")))
			} else {
				b.WriteString(NormalStyle.Render("  " + m.addTimestamp))
			}
		case logFieldStatus:
			status := logStatusOptions[m.addStatusIdx]
			b.WriteString(NormalStyle.Render("  " + status))
		case logFieldTags:
			if i == m.addFocus {
				b.WriteString(NormalStyle.Render("  " + m.renderAddTags(true)))
			} else {
				b.WriteString(NormalStyle.Render("  " + m.renderAddTags(false)))
			}
		case logFieldValue:
			value := renderMetadataEditorPreview(m.addValue.Buffer, m.addValue.Scopes, m.width, 6)
			if strings.TrimSpace(value) == "" {
				value = "-"
			}
			b.WriteString(NormalStyle.Render("  " + value))
		case logFieldMeta:
			meta := renderMetadataEditorPreview(m.addMeta.Buffer, m.addMeta.Scopes, m.width, 6)
			if strings.TrimSpace(meta) == "" {
				meta = "-"
			}
			b.WriteString(NormalStyle.Render("  " + meta))
		}

		if i < len(m.addFields)-1 {
			b.WriteString("\n\n")
		}
	}
	if m.addErr != "" {
		b.WriteString("\n\n" + ErrorStyle.Render(m.addErr))
	}
	if m.addSaved {
		b.WriteString("\n\n" + SuccessStyle.Render("Saved."))
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
	status := logStatusOptions[m.addStatusIdx]
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

	input := api.CreateLogInput{
		LogType:   logType,
		Status:    status,
		Tags:      m.addTags,
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
	m.addFocus = 0
	m.addStatusIdx = statusIndex(logStatusOptions, "active")
	m.addTags = nil
	m.addTagBuf = ""
	m.addType = ""
	m.addTimestamp = ""
	m.addValue.Reset()
	m.addMeta.Reset()
}

// commitAddTag handles commit add tag.
func (m *LogsModel) commitAddTag() {
	raw := strings.TrimSpace(m.addTagBuf)
	if raw == "" {
		m.addTagBuf = ""
		return
	}
	tag := normalizeTag(raw)
	if tag == "" {
		m.addTagBuf = ""
		return
	}
	for _, t := range m.addTags {
		if t == tag {
			m.addTagBuf = ""
			return
		}
	}
	m.addTags = append(m.addTags, tag)
	m.addTagBuf = ""
}

// renderAddTags renders render add tags.
func (m LogsModel) renderAddTags(focused bool) string {
	if len(m.addTags) == 0 && m.addTagBuf == "" && !focused {
		return "-"
	}
	var b strings.Builder
	for i, t := range m.addTags {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(AccentStyle.Render("[" + t + "]"))
	}
	if focused {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		if m.addTagBuf != "" {
			b.WriteString(m.addTagBuf)
		}
		b.WriteString(AccentStyle.Render("█"))
	} else if m.addTagBuf != "" {
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(MutedStyle.Render(m.addTagBuf))
	}
	return b.String()
}

// --- Edit View ---

func (m LogsModel) startEdit() {
	if m.detail == nil {
		return
	}
	l := m.detail
	m.editFocus = 0
	m.editStatusIdx = statusIndex(logStatusOptions, l.Status)
	m.editTags = append([]string{}, l.Tags...)
	m.editTagBuf = ""
	m.editType = l.LogType
	m.editTimestamp = l.Timestamp.Format(time.RFC3339)
	m.editValue.Load(map[string]any(l.Value))
	m.editMeta.Load(map[string]any(l.Metadata))
	m.editSaving = false
}

// handleEditKeys handles handle edit keys.
func (m LogsModel) handleEditKeys(msg tea.KeyMsg) (LogsModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if m.editFocus == logEditFieldStatus {
		switch {
		case isKey(msg, "left"):
			m.editStatusIdx = (m.editStatusIdx - 1 + len(logStatusOptions)) % len(logStatusOptions)
			return m, nil
		case isKey(msg, "right"), isSpace(msg):
			m.editStatusIdx = (m.editStatusIdx + 1) % len(logStatusOptions)
			return m, nil
		}
	}
	switch {
	case isDown(msg):
		m.editFocus = (m.editFocus + 1) % logEditFieldCount
	case isUp(msg):
		if m.editFocus > 0 {
			m.editFocus = (m.editFocus - 1 + logEditFieldCount) % logEditFieldCount
		}
	case isBack(msg):
		m.view = logsViewDetail
	case isKey(msg, "ctrl+s"):
		return m.saveEdit()
	case isKey(msg, "backspace", "delete"):
		switch m.editFocus {
		case logEditFieldTags:
			if len(m.editTagBuf) > 0 {
				m.editTagBuf = m.editTagBuf[:len(m.editTagBuf)-1]
			} else if len(m.editTags) > 0 {
				m.editTags = m.editTags[:len(m.editTags)-1]
			}
		}
	default:
		switch m.editFocus {
		case logEditFieldTags:
			switch {
			case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
				m.commitEditTag()
			default:
				ch := msg.String()
				if len(ch) == 1 && ch != "," {
					m.editTagBuf += ch
				}
			}
		case logEditFieldValue:
			if isEnter(msg) {
				m.editValue.Active = true
			}
		case logEditFieldMeta:
			if isEnter(msg) {
				m.editMeta.Active = true
			}
		}
	}
	return m, nil
}

// renderEdit renders render edit.
func (m LogsModel) renderEdit() string {
	var b strings.Builder
	for i, f := range []string{"Status", "Tags", "Value", "Metadata"} {
		label := MutedStyle.Render(f + ":")
		if i == m.editFocus {
			label = SelectedStyle.Render("  " + f + ":")
		} else {
			label = "  " + label
		}
		b.WriteString(label + "\n")
		switch i {
		case logEditFieldStatus:
			b.WriteString(NormalStyle.Render("  " + logStatusOptions[m.editStatusIdx]))
		case logEditFieldTags:
			if i == m.editFocus {
				b.WriteString(NormalStyle.Render("  " + m.renderEditTags(true)))
			} else {
				b.WriteString(NormalStyle.Render("  " + m.renderEditTags(false)))
			}
		case logEditFieldValue:
			value := renderMetadataEditorPreview(m.editValue.Buffer, m.editValue.Scopes, m.width, 6)
			b.WriteString(NormalStyle.Render("  " + value))
		case logEditFieldMeta:
			meta := renderMetadataEditorPreview(m.editMeta.Buffer, m.editMeta.Scopes, m.width, 6)
			b.WriteString(NormalStyle.Render("  " + meta))
		}
		if i < logEditFieldCount-1 {
			b.WriteString("\n\n")
		}
	}
	return components.Indent(b.String(), 1)
}

// saveEdit handles save edit.
func (m LogsModel) saveEdit() (LogsModel, tea.Cmd) {
	status := logStatusOptions[m.editStatusIdx]
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

	input := api.UpdateLogInput{
		Status:   &status,
		Tags:     &m.editTags,
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

// commitEditTag handles commit edit tag.
func (m *LogsModel) commitEditTag() {
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

// renderEditTags renders render edit tags.
func (m LogsModel) renderEditTags(focused bool) string {
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
	labels := make([]string, len(m.items))
	for i, l := range m.items {
		labels[i] = formatLogLine(l)
	}
	m.list.SetItems(labels)
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

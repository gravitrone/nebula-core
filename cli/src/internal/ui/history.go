package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type historyLoadedMsg struct{ items []api.AuditEntry }
type historyScopesLoadedMsg struct{ items []api.AuditScope }
type historyActorsLoadedMsg struct{ items []api.AuditActor }
type historyRevertedMsg struct {
	entityID string
	auditID  string
}

type historyView int

const (
	historyViewList historyView = iota
	historyViewDetail
	historyViewScopes
	historyViewActors
)

type auditFilter struct {
	tableName string
	action    string
	actorType string
	actorID   string
	recordID  string
	scopeID   string
	actor     string
	terms     []string
}

type HistoryModel struct {
	client     *api.Client
	items      []api.AuditEntry
	dataTable  table.Model
	loading    bool
	spinner    spinner.Model
	width      int
	height     int
	view       historyView
	detail     *api.AuditEntry
	filtering  bool
	filterBuf  string
	filter     auditFilter
	errText    string
	scopes     []api.AuditScope
	actors     []api.AuditActor
	scopeTable table.Model
	actorTable table.Model
	reverting  bool

}

// NewHistoryModel builds the audit history UI model.
func NewHistoryModel(client *api.Client) HistoryModel {
	return HistoryModel{
		client:     client,
		spinner:    components.NewNebulaSpinner(),
		dataTable:  components.NewNebulaTable(nil, 10),
		scopeTable: components.NewNebulaTable(nil, 10),
		actorTable: components.NewNebulaTable(nil, 10),
		view:       historyViewList,
	}
}

// Init handles init.
func (m HistoryModel) Init() tea.Cmd {
	m.loading = true
	return tea.Batch(m.loadHistory(), m.spinner.Tick)
}

// Update updates update.
func (m HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case historyLoadedMsg:
		m.loading = false
		m.errText = ""
		m.items = m.applyLocalFilters(msg.items)
		rows := make([]table.Row, len(m.items))
		for i, entry := range m.items {
			rows[i] = table.Row{formatAuditLine(entry)}
		}
		m.dataTable.SetRows(rows)
		m.dataTable.SetCursor(0)
		return m, nil
	case historyScopesLoadedMsg:
		m.loading = false
		m.errText = ""
		m.scopes = msg.items
		rows := make([]table.Row, len(m.scopes))
		for i, scope := range m.scopes {
			rows[i] = table.Row{formatScopeLine(scope)}
		}
		m.scopeTable.SetRows(rows)
		m.scopeTable.SetCursor(0)
		return m, nil
	case historyActorsLoadedMsg:
		m.loading = false
		m.errText = ""
		m.actors = msg.items
		rows := make([]table.Row, len(m.actors))
		for i, actor := range m.actors {
			rows[i] = table.Row{formatActorLine(actor)}
		}
		m.actorTable.SetRows(rows)
		m.actorTable.SetCursor(0)
		return m, nil
	case historyRevertedMsg:
		m.reverting = false
		m.view = historyViewList
		m.detail = nil
		m.loading = true
		return m, tea.Batch(m.loadHistory(), m.spinner.Tick)
	case errMsg:
		m.loading = false
		m.reverting = false
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyPressMsg:
		if m.filtering {
			return m.handleFilterKeys(msg)
		}
		switch m.view {
		case historyViewList:
			return m.handleListKeys(msg)
		case historyViewDetail:
			if m.reverting {
				switch {
				case isKey(msg, "y"), isEnter(msg):
					return m.confirmRevert()
				case isKey(msg, "n"), isBack(msg):
					m.reverting = false
					return m, nil
				}
				return m, nil
			}
			if isBack(msg) {
				m.view = historyViewList
				m.detail = nil
				return m, nil
			}
			if isKey(msg, "r") && canRevertAuditEntry(m.detail) {
				m.reverting = true
				return m, nil
			}
		case historyViewScopes:
			return m.handleScopeKeys(msg)
		case historyViewActors:
			return m.handleActorKeys(msg)
		}
	}

	return m, nil
}

// View handles view.
func (m HistoryModel) View() string {
	if m.filtering {
		return components.Indent(components.InputDialog("Filter Audit Log", m.filterBuf), 1)
	}
	if m.loading {
		label := "Loading history..."
		switch m.view {
		case historyViewScopes:
			label = "Loading scopes..."
		case historyViewActors:
			label = "Loading actors..."
		}
		return "  " + m.spinner.View() + " " + MutedStyle.Render(label)
	}
	if m.errText != "" {
		return components.Indent(components.ErrorBox("Error", m.errText, m.width), 1)
	}
	if m.view == historyViewDetail && m.detail != nil {
		if m.reverting {
			return m.renderRevertConfirm(*m.detail)
		}
		return m.renderDetail(*m.detail)
	}
	if m.view == historyViewScopes {
		return m.renderScopes()
	}
	if m.view == historyViewActors {
		return m.renderActors()
	}
	return m.renderList()
}

// Hints returns the hint items for the current view state.
func (m HistoryModel) Hints() []components.HintItem {
	if m.filtering || m.loading || m.errText != "" {
		return nil
	}
	if m.view != historyViewList {
		return nil
	}
	return []components.HintItem{
		{Key: "1-9/0", Desc: "Tabs"},
		{Key: "/", Desc: "Command"},
		{Key: "?", Desc: "Help"},
		{Key: "q", Desc: "Quit"},
		{Key: "\u2191/\u2193", Desc: "Scroll"},
		{Key: "enter", Desc: "Diff"},
		{Key: "v", Desc: "Revert"},
	}
}

// canRevertAuditEntry handles can revert audit entry.
func canRevertAuditEntry(entry *api.AuditEntry) bool {
	if entry == nil {
		return false
	}
	if strings.TrimSpace(entry.RecordID) == "" || strings.TrimSpace(entry.ID) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(entry.TableName), "entities")
}

// renderRevertConfirm renders render revert confirm.
func (m HistoryModel) renderRevertConfirm(entry api.AuditEntry) string {
	summary := []components.TableRow{
		{Label: "Action", Value: "Revert entity to selected audit entry"},
		{Label: "Entity", Value: shortID(entry.RecordID)},
		{Label: "Audit Entry", Value: shortID(entry.ID)},
		{Label: "Changed At", Value: formatLocalTimeFull(entry.ChangedAt)},
	}
	diffs := buildAuditDiffRows(entry)
	return components.Indent(
		components.ConfirmPreviewDialog("Revert Entity", summary, diffs, m.width),
		1,
	)
}

// confirmRevert handles confirm revert.
func (m HistoryModel) confirmRevert() (HistoryModel, tea.Cmd) {
	if m.detail == nil {
		m.reverting = false
		return m, nil
	}
	entityID := strings.TrimSpace(m.detail.RecordID)
	auditID := strings.TrimSpace(m.detail.ID)
	if entityID == "" || auditID == "" {
		m.reverting = false
		return m, nil
	}
	m.reverting = false
	return m, func() tea.Msg {
		if _, err := m.client.RevertEntity(entityID, auditID); err != nil {
			return errMsg{err}
		}
		return historyRevertedMsg{entityID: entityID, auditID: auditID}
	}
}

// handleListKeys handles handle list keys.
func (m HistoryModel) handleListKeys(msg tea.KeyPressMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.dataTable.MoveDown(1)
	case isUp(msg):
		if m.dataTable.Cursor() <= 0 {
			return m, nil
		}
		m.dataTable.MoveUp(1)
	case isEnter(msg):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			entry := m.items[idx]
			m.detail = &entry
			m.view = historyViewDetail
		}
	case isKey(msg, "f"):
		m.filtering = true
	case isKey(msg, "s"):
		m.view = historyViewScopes
		m.loading = true
		return m, tea.Batch(m.loadScopes(), m.spinner.Tick)
	case isKey(msg, "a"):
		m.view = historyViewActors
		m.loading = true
		return m, tea.Batch(m.loadActors(), m.spinner.Tick)
	}
	return m, nil
}

// handleFilterKeys handles handle filter keys.
func (m HistoryModel) handleFilterKeys(msg tea.KeyPressMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
		m.filter = parseAuditFilter(m.filterBuf)
		m.loading = true
		return m, tea.Batch(m.loadHistory(), m.spinner.Tick)
	case isBack(msg):
		m.filtering = false
		m.filterBuf = ""
		m.filter = auditFilter{}
		m.loading = true
		return m, tea.Batch(m.loadHistory(), m.spinner.Tick)
	case isKey(msg, "backspace"):
		if len(m.filterBuf) > 0 {
			m.filterBuf = m.filterBuf[:len(m.filterBuf)-1]
		}
	default:
		if ch := keyText(msg); ch != "" {
			m.filterBuf += ch
		}
	}
	return m, nil
}

// handleScopeKeys handles handle scope keys.
func (m HistoryModel) handleScopeKeys(msg tea.KeyPressMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.scopeTable.MoveDown(1)
	case isUp(msg):
		if m.scopeTable.Cursor() <= 0 {
			return m, nil
		}
		m.scopeTable.MoveUp(1)
	case isEnter(msg):
		if idx := m.scopeTable.Cursor(); idx >= 0 && idx < len(m.scopes) {
			scope := m.scopes[idx]
			m.filter.scopeID = scope.ID
			m.view = historyViewList
			m.loading = true
			return m, tea.Batch(m.loadHistory(), m.spinner.Tick)
		}
	case isBack(msg):
		m.view = historyViewList
	}
	return m, nil
}

// handleActorKeys handles handle actor keys.
func (m HistoryModel) handleActorKeys(msg tea.KeyPressMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.actorTable.MoveDown(1)
	case isUp(msg):
		if m.actorTable.Cursor() <= 0 {
			return m, nil
		}
		m.actorTable.MoveUp(1)
	case isEnter(msg):
		if idx := m.actorTable.Cursor(); idx >= 0 && idx < len(m.actors) {
			actor := m.actors[idx]
			m.filter.actorType = actor.ActorType
			m.filter.actorID = actor.ActorID
			m.view = historyViewList
			m.loading = true
			return m, tea.Batch(m.loadHistory(), m.spinner.Tick)
		}
	case isBack(msg):
		m.view = historyViewList
	}
	return m, nil
}

// loadHistory loads load history.
func (m HistoryModel) loadHistory() tea.Cmd {
	filter := m.filter
	return func() tea.Msg {
		items, err := m.client.QueryAuditLogWithPagination(
			filter.tableName,
			filter.action,
			filter.actorType,
			filter.actorID,
			filter.recordID,
			filter.scopeID,
			50,
			0,
		)
		if err != nil {
			return errMsg{err}
		}
		return historyLoadedMsg{items: items}
	}
}

// loadScopes loads load scopes.
func (m HistoryModel) loadScopes() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListAuditScopes()
		if err != nil {
			return errMsg{err}
		}
		return historyScopesLoadedMsg{items: items}
	}
}

// loadActors loads load actors.
func (m HistoryModel) loadActors() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListAuditActors("")
		if err != nil {
			return errMsg{err}
		}
		return historyActorsLoadedMsg{items: items}
	}
}

// renderList renders render list.
func (m HistoryModel) renderList() string {
	contentWidth := components.BoxContentWidth(m.width)

	if len(m.items) == 0 {
		box := components.EmptyStateBox(
			"Audit Log",
			"No audit entries yet.",
			[]string{"Make changes to entities or context to generate audit entries", "Press f to filter by table or action"},
			m.width,
		)
		return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, box)
	}

	filterLine := formatAuditFilters(m.filter)

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap - components.TableBaseBorderWidth
	}

	numCols := 4
	availableCols := tableWidth - (numCols * 2)
	if availableCols < 30 {
		availableCols = 30
	}

	atWidth := compactTimeColumnWidth
	actionWidth := 6
	tableNameWidth := 17
	actorWidth := availableCols - (atWidth + actionWidth + tableNameWidth)
	if actorWidth < 14 {
		actorWidth = 14
		tableNameWidth = availableCols - (atWidth + actionWidth + actorWidth)
		if tableNameWidth < 10 {
			tableNameWidth = 10
		}
	}
	if actorWidth > 40 {
		actorWidth = 40
	}

	tableRows := make([]table.Row, len(m.items))
	for i, entry := range m.items {
		action := strings.TrimSpace(components.SanitizeOneLine(entry.Action))
		if action == "" {
			action = "update"
		}
		actor := strings.TrimSpace(components.SanitizeOneLine(formatAuditActor(entry)))
		if actor == "" {
			actor = "system"
		}
		tableName := strings.TrimSpace(components.SanitizeOneLine(entry.TableName))
		if tableName == "" {
			tableName = "-"
		}
		tableRows[i] = table.Row{
			formatLocalTimeCompact(entry.ChangedAt),
			components.ClampTextWidthEllipsis(strings.ToUpper(action), actionWidth),
			components.ClampTextWidthEllipsis(tableName, tableNameWidth),
			components.ClampTextWidthEllipsis(actor, actorWidth),
		}
	}

	m.dataTable.SetColumns([]table.Column{
		{Title: "At", Width: atWidth},
		{Title: "Action", Width: actionWidth},
		{Title: "Table", Width: tableNameWidth},
		{Title: "Actor", Width: actorWidth},
	})
	actualTableWidth := atWidth + actionWidth + tableNameWidth + actorWidth + (numCols * 2)
	m.dataTable.SetWidth(actualTableWidth)
	m.dataTable.SetRows(tableRows)

	countLine := ""
	if filterLine != "" {
		countLine = MutedStyle.Render(fmt.Sprintf("%d total · %s", len(m.items), filterLine))
	}

	tableView := components.TableBaseStyle.Render(m.dataTable.View())
	preview := ""
	var previewItem *api.AuditEntry
	if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}
	if previewItem != nil {
		content := m.renderAuditPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
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
	return components.Indent(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, result), 1)
}

// renderAuditPreview renders render audit preview.
func (m HistoryModel) renderAuditPreview(entry api.AuditEntry, width int) string {
	if width <= 0 {
		return ""
	}

	action := strings.TrimSpace(components.SanitizeOneLine(entry.Action))
	if action == "" {
		action = "update"
	}
	tableName := strings.TrimSpace(components.SanitizeOneLine(entry.TableName))
	if tableName == "" {
		tableName = "-"
	}
	actor := strings.TrimSpace(components.SanitizeOneLine(formatAuditActor(entry)))
	if actor == "" {
		actor = "system"
	}

	heading := strings.ToUpper(action) + " " + tableName

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(heading, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Table", tableName, width))
	lines = append(lines, renderPreviewRow("Action", strings.ToUpper(action), width))
	lines = append(lines, renderPreviewRow("Actor", actor, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeFull(entry.ChangedAt), width))
	if strings.TrimSpace(entry.RecordID) != "" {
		lines = append(lines, renderPreviewRow("Record", shortID(entry.RecordID), width))
	}
	if len(entry.ChangedFields) > 0 {
		lines = append(lines, renderPreviewRow("Fields", strings.Join(entry.ChangedFields, ", "), width))
	}
	if entry.ChangeReason != nil && strings.TrimSpace(*entry.ChangeReason) != "" {
		lines = append(lines, renderPreviewRow("Reason", strings.TrimSpace(*entry.ChangeReason), width))
	}

	return padPreviewLines(lines, width)
}

// renderScopes renders render scopes.
func (m HistoryModel) renderScopes() string {
	if len(m.scopes) == 0 {
		content := MutedStyle.Render("No scopes found.")
		return components.Indent(components.Box(content, m.width), 1)
	}

	contentWidth := components.BoxContentWidth(m.width)

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap - components.TableBaseBorderWidth
	}

	numCols := 4
	availableCols := tableWidth - (numCols * 2)
	if availableCols < 30 {
		availableCols = 30
	}

	agentsWidth := 6
	entitiesWidth := 8
	contextWidth := 10
	scopeWidth := availableCols - (agentsWidth + entitiesWidth + contextWidth)
	if scopeWidth < 12 {
		scopeWidth = 12
	}

	tableRows := make([]table.Row, len(m.scopes))
	for i, scope := range m.scopes {
		tableRows[i] = table.Row{
			components.ClampTextWidthEllipsis(components.SanitizeOneLine(scope.Name), scopeWidth),
			fmt.Sprintf("%d", scope.AgentCount),
			fmt.Sprintf("%d", scope.EntityCount),
			fmt.Sprintf("%d", scope.ContextCount),
		}
	}

	m.scopeTable.SetColumns([]table.Column{
		{Title: "Scope", Width: scopeWidth},
		{Title: "Agents", Width: agentsWidth},
		{Title: "Entities", Width: entitiesWidth},
		{Title: "Context", Width: contextWidth},
	})
	actualTableWidth := scopeWidth + agentsWidth + entitiesWidth + contextWidth + (numCols * 2)
	m.scopeTable.SetWidth(actualTableWidth)
	m.scopeTable.SetRows(tableRows)

	tableView := components.TableBaseStyle.Render(m.scopeTable.View())
	preview := ""
	var previewItem *api.AuditScope
	if idx := m.scopeTable.Cursor(); idx >= 0 && idx < len(m.scopes) {
		previewItem = &m.scopes[idx]
	}
	if previewItem != nil {
		content := m.renderScopePreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	return components.Indent(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, body), 1)
}

// renderScopePreview renders render scope preview.
func (m HistoryModel) renderScopePreview(scope api.AuditScope, width int) string {
	if width <= 0 {
		return ""
	}

	title := strings.TrimSpace(components.SanitizeOneLine(scope.Name))
	if title == "" {
		title = "scope"
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Agents", fmt.Sprintf("%d", scope.AgentCount), width))
	lines = append(lines, renderPreviewRow("Entities", fmt.Sprintf("%d", scope.EntityCount), width))
	lines = append(lines, renderPreviewRow("Context", fmt.Sprintf("%d", scope.ContextCount), width))
	if scope.Description != nil && strings.TrimSpace(*scope.Description) != "" {
		lines = append(lines, renderPreviewRow("Desc", strings.TrimSpace(*scope.Description), width))
	}

	return padPreviewLines(lines, width)
}

// renderActors renders render actors.
func (m HistoryModel) renderActors() string {
	if len(m.actors) == 0 {
		content := MutedStyle.Render("No actors found.")
		return components.Indent(components.Box(content, m.width), 1)
	}

	contentWidth := components.BoxContentWidth(m.width)

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap - components.TableBaseBorderWidth
	}

	numCols := 3
	availableCols := tableWidth - (numCols * 2)
	if availableCols < 30 {
		availableCols = 30
	}

	actionsWidth := 8
	lastWidth := 11
	actorWidth := availableCols - (actionsWidth + lastWidth)
	if actorWidth < 14 {
		actorWidth = 14
	}

	tableRows := make([]table.Row, len(m.actors))
	for i, actor := range m.actors {
		name := actorDisplayName(actor)
		display := formatActorDisplay(actor, name)
		tableRows[i] = table.Row{
			components.ClampTextWidthEllipsis(components.SanitizeOneLine(display), actorWidth),
			fmt.Sprintf("%d", actor.ActionCount),
			formatLocalTimeCompact(actor.LastSeen),
		}
	}

	m.actorTable.SetColumns([]table.Column{
		{Title: "Actor", Width: actorWidth},
		{Title: "Actions", Width: actionsWidth},
		{Title: "Last", Width: lastWidth},
	})
	actualTableWidth := actorWidth + actionsWidth + lastWidth + (numCols * 2)
	m.actorTable.SetWidth(actualTableWidth)
	m.actorTable.SetRows(tableRows)

	tableView := components.TableBaseStyle.Render(m.actorTable.View())
	preview := ""
	var previewItem *api.AuditActor
	if idx := m.actorTable.Cursor(); idx >= 0 && idx < len(m.actors) {
		previewItem = &m.actors[idx]
	}
	if previewItem != nil {
		content := m.renderActorPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	return components.Indent(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, body), 1)
}

// renderActorPreview renders render actor preview.
func (m HistoryModel) renderActorPreview(actor api.AuditActor, width int) string {
	if width <= 0 {
		return ""
	}

	name := actorDisplayName(actor)
	title := name

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Actor", formatActorRef(actor), width))
	lines = append(lines, renderPreviewRow("Actions", fmt.Sprintf("%d", actor.ActionCount), width))
	lines = append(lines, renderPreviewRow("Last", formatLocalTimeFull(actor.LastSeen), width))

	return padPreviewLines(lines, width)
}

// renderDetail renders render detail.
func (m HistoryModel) renderDetail(entry api.AuditEntry) string {
	when := formatLocalTimeFull(entry.ChangedAt)
	actor := formatAuditActor(entry)
	fields := ""
	if len(entry.ChangedFields) > 0 {
		fields = strings.Join(entry.ChangedFields, ", ")
	}

	// --- Info table ---
	infoRows := []table.Row{
		{"Table", entry.TableName},
		{"Action", entry.Action},
		{"Record", entry.RecordID},
		{"Actor", actor},
		{"At", when},
	}
	if fields != "" {
		infoRows = append(infoRows, table.Row{"Fields", fields})
	}
	if entry.ChangeReason != nil && *entry.ChangeReason != "" {
		infoRows = append(infoRows, table.Row{"Reason", *entry.ChangeReason})
	}

	fieldColWidth := 8
	valueColWidth := 50
	if m.width > 0 {
		avail := components.BoxContentWidth(m.width) - fieldColWidth - 4
		if avail > valueColWidth {
			valueColWidth = avail
		}
	}

	infoCols := []table.Column{
		{Title: "Field", Width: fieldColWidth},
		{Title: "Value", Width: valueColWidth},
	}

	sNoHL := table.DefaultStyles()
	sNoHL.Header = sNoHL.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ColorBorder).
		BorderBottom(true).
		Bold(false)
	sNoHL.Selected = lipgloss.NewStyle()

	infoTable := table.New(
		table.WithColumns(infoCols),
		table.WithRows(infoRows),
		table.WithHeight(len(infoRows)+1),
		table.WithStyles(sNoHL),
	)
	infoTable.Blur()
	section := components.TableBaseStyle.Render(infoTable.View())

	// --- Diff view ---
	diffRows := buildAuditDiffRows(entry)
	if len(diffRows) > 0 {
		diffW := m.width
		if diffW <= 0 {
			diffW = 80
		}
		diffLines := components.DiffRowsToLines(diffRows)
		diff := components.RenderDiffView(diffLines, components.BoxContentWidth(diffW))
		section = section + "\n\n" + diff
	}
	return components.Indent(section, 1)
}

// detailDiffChangeKind classifies a diff row as added/removed/updated/same.
func detailDiffChangeKind(from, to string) string {
	normalize := func(v string) string {
		v = strings.TrimSpace(v)
		if v == "" || v == "<nil>" || v == "-" || v == "--" {
			return "None"
		}
		return v
	}
	before := normalize(from)
	after := normalize(to)
	switch {
	case before == "None" && after != "None":
		return "added"
	case before != "None" && after == "None":
		return "removed"
	case before == after:
		return "same"
	default:
		return "updated"
	}
}

// parseAuditFilter parses parse audit filter.
func parseAuditFilter(input string) auditFilter {
	filter := auditFilter{}
	input = strings.TrimSpace(input)
	if input == "" {
		return filter
	}
	for _, token := range strings.Fields(input) {
		switch {
		case strings.HasPrefix(token, "table:"):
			filter.tableName = strings.TrimPrefix(token, "table:")
		case strings.HasPrefix(token, "action:"):
			filter.action = strings.TrimPrefix(token, "action:")
		case strings.HasPrefix(token, "actor_type:"):
			filter.actorType = strings.TrimPrefix(token, "actor_type:")
		case strings.HasPrefix(token, "actor_id:"):
			filter.actorID = strings.TrimPrefix(token, "actor_id:")
		case strings.HasPrefix(token, "record:"):
			filter.recordID = strings.TrimPrefix(token, "record:")
		case strings.HasPrefix(token, "record_id:"):
			filter.recordID = strings.TrimPrefix(token, "record_id:")
		case strings.HasPrefix(token, "scope:"):
			filter.scopeID = strings.TrimPrefix(token, "scope:")
		case strings.HasPrefix(token, "scope_id:"):
			filter.scopeID = strings.TrimPrefix(token, "scope_id:")
		case strings.HasPrefix(token, "actor:"):
			filter.actor = strings.ToLower(strings.TrimPrefix(token, "actor:"))
		default:
			filter.terms = append(filter.terms, strings.ToLower(token))
		}
	}
	return filter
}

// applyLocalFilters handles apply local filters.
func (m HistoryModel) applyLocalFilters(items []api.AuditEntry) []api.AuditEntry {
	filter := m.filter
	if filter.actor == "" && len(filter.terms) == 0 {
		return items
	}
	filtered := make([]api.AuditEntry, 0, len(items))
	for _, entry := range items {
		if filter.actor != "" {
			actor := strings.ToLower(formatAuditActor(entry))
			if !strings.Contains(actor, filter.actor) {
				continue
			}
		}
		if len(filter.terms) > 0 {
			haystack := strings.ToLower(fmt.Sprintf("%s %s %s", entry.TableName, entry.RecordID, formatAuditActor(entry)))
			matched := true
			for _, term := range filter.terms {
				if !strings.Contains(haystack, term) {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// formatAuditActor handles format audit actor.
func formatAuditActor(entry api.AuditEntry) string {
	if entry.ActorName != nil && *entry.ActorName != "" {
		return *entry.ActorName
	}
	if entry.ChangedByType != nil && entry.ChangedByID != nil {
		kind := normalizeActorType(*entry.ChangedByType)
		if strings.TrimSpace(*entry.ChangedByID) == "" {
			return kind
		}
		return fmt.Sprintf("%s:%s", kind, shortID(*entry.ChangedByID))
	}
	if entry.ChangedByType != nil {
		return normalizeActorType(*entry.ChangedByType)
	}
	return "system"
}

// formatAuditLine handles format audit line.
func formatAuditLine(entry api.AuditEntry) string {
	when := formatLocalTimeCompact(entry.ChangedAt)
	actor := formatAuditActor(entry)
	action := entry.Action
	if action == "" {
		action = "update"
	}
	return fmt.Sprintf("%s  %s  %s  %s", when, strings.ToUpper(action), entry.TableName, actor)
}

// formatScopeLine handles format scope line.
func formatScopeLine(scope api.AuditScope) string {
	desc := ""
	if scope.Description != nil && *scope.Description != "" {
		desc = " - " + *scope.Description
	}
	return fmt.Sprintf(
		"%s  agents:%d entities:%d context:%d%s",
		scope.Name,
		scope.AgentCount,
		scope.EntityCount,
		scope.ContextCount,
		desc,
	)
}

// formatActorLine handles format actor line.
func formatActorLine(actor api.AuditActor) string {
	name := actorDisplayName(actor)
	when := formatLocalTimeCompact(actor.LastSeen)
	return fmt.Sprintf(
		"%s  %s  actions:%d  last:%s",
		name,
		formatActorRef(actor),
		actor.ActionCount,
		when,
	)
}

// actorDisplayName handles actor display name.
func actorDisplayName(actor api.AuditActor) string {
	if actor.ActorName != nil {
		if name := strings.TrimSpace(*actor.ActorName); name != "" && !isUnknownLabel(name) {
			return name
		}
	}
	actorType := normalizeActorType(actor.ActorType)
	if actorType == "system" {
		if inferred := inferActorTypeFromID(actor.ActorID); inferred != "" {
			actorType = inferred
		}
	}
	if actorType == "" || actorType == "system" {
		return "system"
	}
	return actorType
}

// isUnknownLabel handles is unknown label.
func isUnknownLabel(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(value, ":")))
	switch normalized {
	case "", "unknown", "none", "null", "n/a":
		return true
	default:
		return false
	}
}

// formatActorRef handles format actor ref.
func formatActorRef(actor api.AuditActor) string {
	actorType := normalizeActorType(actor.ActorType)
	actorID := strings.TrimSpace(strings.TrimSuffix(actor.ActorID, ":"))
	if actorType == "system" {
		if inferred := inferActorTypeFromID(actorID); inferred != "" {
			actorType = inferred
		}
	}
	if actorID == "" {
		return actorType
	}
	if strings.HasPrefix(actorID, actorType+":") {
		actorID = strings.TrimSpace(strings.TrimPrefix(actorID, actorType+":"))
	}
	if actorID == "" {
		return actorType
	}
	return actorType + ":" + shortID(actorID)
}

// inferActorTypeFromID handles infer actor type from id.
func inferActorTypeFromID(actorID string) string {
	id := strings.TrimSpace(strings.TrimSuffix(actorID, ":"))
	if id == "" {
		return ""
	}
	if !strings.Contains(id, ":") {
		return ""
	}
	prefix := strings.TrimSpace(strings.SplitN(id, ":", 2)[0])
	if prefix == "" {
		return ""
	}
	return normalizeActorType(prefix)
}

// normalizeActorType handles normalize actor type.
func normalizeActorType(raw string) string {
	actorType := strings.TrimSpace(strings.TrimSuffix(raw, ":"))
	lower := strings.ToLower(actorType)
	switch lower {
	case "", "unknown", "none", "null", "system":
		return "system"
	}
	return actorType
}

// formatActorDisplay handles format actor display.
func formatActorDisplay(actor api.AuditActor, name string) string {
	ref := formatActorRef(actor)
	if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(ref)) {
		return name
	}
	return fmt.Sprintf("%s  %s", name, ref)
}

// formatAuditFilters handles format audit filters.
func formatAuditFilters(filter auditFilter) string {
	parts := []string{}
	if filter.tableName != "" {
		parts = append(parts, "table:"+filter.tableName)
	}
	if filter.action != "" {
		parts = append(parts, "action:"+filter.action)
	}
	if filter.actorType != "" {
		parts = append(parts, "actor_type:"+filter.actorType)
	}
	if filter.actorID != "" {
		parts = append(parts, "actor_id:"+shortID(filter.actorID))
	}
	if filter.recordID != "" {
		parts = append(parts, "record:"+shortID(filter.recordID))
	}
	if filter.scopeID != "" {
		parts = append(parts, "scope:"+shortID(filter.scopeID))
	}
	if filter.actor != "" {
		parts = append(parts, "actor:"+filter.actor)
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return "Filters: " + parts[0]
	}
	// Avoid lipgloss word-wrapping splitting filter tokens (for example "scope:...").
	return "Filters:\n  " + strings.Join(parts, "\n  ")
}

// parseAuditValuesMap parses a JSON text string into a map for diff display.
func parseAuditValuesMap(text string) map[string]any {
	if text == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		return nil
	}
	return m
}

// buildAuditDiffRows builds build audit diff rows.
func buildAuditDiffRows(entry api.AuditEntry) []components.DiffRow {
	oldMap := parseAuditValuesMap(entry.OldValues)
	newMap := parseAuditValuesMap(entry.NewValues)

	keys := make([]string, 0)
	seen := map[string]bool{}
	if len(entry.ChangedFields) > 0 {
		for _, k := range entry.ChangedFields {
			if k == "" {
				continue
			}
			seen[k] = true
			keys = append(keys, k)
		}
	} else {
		for k := range oldMap {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		for k := range newMap {
			if !seen[k] {
				keys = append(keys, k)
			}
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Strings(keys)
	rows := make([]components.DiffRow, 0, len(keys))
	for _, key := range keys {
		from := oldMap[key]
		to := newMap[key]
		if formatAuditValue(from) == formatAuditValue(to) {
			continue
		}
		rows = append(rows, components.DiffRow{
			Label: humanizeAuditField(key),
			From:  formatAuditValue(from),
			To:    formatAuditValue(to),
		})
	}
	return rows
}

// formatAuditValue handles format audit value.
func formatAuditValue(value any) string {
	if value == nil {
		return "None"
	}
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || trimmed == "<nil>" || trimmed == "-" || trimmed == "--" {
			return "None"
		}
		if parsed, ok := parseJSONStructuredString(trimmed); ok {
			return formatAuditValue(parsed)
		}
		trimmed = humanizeGoMapString(trimmed)
		return components.SanitizeText(trimmed)
	case time.Time:
		return formatLocalTimeFull(v)
	case map[string]any:
		lines := metadataLinesPlain(v, 0)
		if len(lines) == 0 {
			return "None"
		}
		return components.SanitizeText(strings.Join(lines, "\n"))
	case []any:
		lines := metadataListLinesPlain(v, 0)
		if len(lines) == 0 {
			return "None"
		}
		return components.SanitizeText(strings.Join(lines, "\n"))
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return components.SanitizeText(fmt.Sprintf("%v", v))
		}
		rendered := strings.TrimSpace(string(b))
		if rendered == "" || rendered == "null" || rendered == "\"\"" {
			return "None"
		}
		return components.SanitizeText(rendered)
	}
}

// humanizeAuditField handles humanize audit field.
func humanizeAuditField(raw string) string {
	key := strings.TrimSpace(raw)
	if key == "" {
		return ""
	}
	key = strings.ReplaceAll(key, "-", "_")
	parts := strings.Split(key, "_")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		if lower == "id" {
			out = append(out, "ID")
			continue
		}
		out = append(out, strings.ToUpper(lower[:1])+lower[1:])
	}
	if len(out) == 0 {
		return components.SanitizeOneLine(raw)
	}
	return components.SanitizeOneLine(strings.Join(out, " "))
}

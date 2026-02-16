package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type historyLoadedMsg struct{ items []api.AuditEntry }
type historyScopesLoadedMsg struct{ items []api.AuditScope }
type historyActorsLoadedMsg struct{ items []api.AuditActor }

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
	client    *api.Client
	items     []api.AuditEntry
	list      *components.List
	loading   bool
	width     int
	height    int
	view      historyView
	detail    *api.AuditEntry
	filtering bool
	filterBuf string
	filter    auditFilter
	errText   string
	scopes    []api.AuditScope
	actors    []api.AuditActor
	scopeList *components.List
	actorList *components.List
}

// NewHistoryModel builds the audit history UI model.
func NewHistoryModel(client *api.Client) HistoryModel {
	return HistoryModel{
		client:    client,
		list:      components.NewList(10),
		scopeList: components.NewList(10),
		actorList: components.NewList(10),
		view:      historyViewList,
	}
}

func (m HistoryModel) Init() tea.Cmd {
	m.loading = true
	return m.loadHistory()
}

func (m HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case historyLoadedMsg:
		m.loading = false
		m.errText = ""
		m.items = m.applyLocalFilters(msg.items)
		labels := make([]string, len(m.items))
		for i, entry := range m.items {
			labels[i] = formatAuditLine(entry)
		}
		m.list.SetItems(labels)
		return m, nil
	case historyScopesLoadedMsg:
		m.loading = false
		m.errText = ""
		m.scopes = msg.items
		labels := make([]string, len(m.scopes))
		for i, scope := range m.scopes {
			labels[i] = formatScopeLine(scope)
		}
		m.scopeList.SetItems(labels)
		return m, nil
	case historyActorsLoadedMsg:
		m.loading = false
		m.errText = ""
		m.actors = msg.items
		labels := make([]string, len(m.actors))
		for i, actor := range m.actors {
			labels[i] = formatActorLine(actor)
		}
		m.actorList.SetItems(labels)
		return m, nil
	case errMsg:
		m.loading = false
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyMsg:
		if m.filtering {
			return m.handleFilterKeys(msg)
		}
		switch m.view {
		case historyViewList:
			return m.handleListKeys(msg)
		case historyViewDetail:
			if isBack(msg) {
				m.view = historyViewList
				m.detail = nil
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

func (m HistoryModel) View() string {
	if m.filtering {
		return components.Indent(components.InputDialog("Filter Audit Log", m.filterBuf), 1)
	}
	if m.loading {
		label := "Loading history..."
		if m.view == historyViewScopes {
			label = "Loading scopes..."
		} else if m.view == historyViewActors {
			label = "Loading actors..."
		}
		return "  " + MutedStyle.Render(label)
	}
	if m.errText != "" {
		return components.Indent(components.ErrorBox("Error", m.errText, m.width), 1)
	}
	if m.view == historyViewDetail && m.detail != nil {
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

func (m HistoryModel) handleListKeys(msg tea.KeyMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.list.Down()
	case isUp(msg):
		m.list.Up()
	case isEnter(msg):
		if idx := m.list.Selected(); idx < len(m.items) {
			entry := m.items[idx]
			m.detail = &entry
			m.view = historyViewDetail
		}
	case isKey(msg, "f"):
		m.filtering = true
	case isKey(msg, "s"):
		m.view = historyViewScopes
		m.loading = true
		return m, m.loadScopes()
	case isKey(msg, "a"):
		m.view = historyViewActors
		m.loading = true
		return m, m.loadActors()
	}
	return m, nil
}

func (m HistoryModel) handleFilterKeys(msg tea.KeyMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
		m.filter = parseAuditFilter(m.filterBuf)
		m.loading = true
		return m, m.loadHistory()
	case isBack(msg):
		m.filtering = false
		m.filterBuf = ""
		m.filter = auditFilter{}
		m.loading = true
		return m, m.loadHistory()
	case msg.Type == tea.KeyBackspace:
		if len(m.filterBuf) > 0 {
			m.filterBuf = m.filterBuf[:len(m.filterBuf)-1]
		}
	case msg.Type == tea.KeyRunes:
		m.filterBuf += msg.String()
	}
	return m, nil
}

func (m HistoryModel) handleScopeKeys(msg tea.KeyMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.scopeList.Down()
	case isUp(msg):
		m.scopeList.Up()
	case isEnter(msg):
		if idx := m.scopeList.Selected(); idx < len(m.scopes) {
			scope := m.scopes[idx]
			m.filter.scopeID = scope.ID
			m.view = historyViewList
			m.loading = true
			return m, m.loadHistory()
		}
	case isBack(msg):
		m.view = historyViewList
	}
	return m, nil
}

func (m HistoryModel) handleActorKeys(msg tea.KeyMsg) (HistoryModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.actorList.Down()
	case isUp(msg):
		m.actorList.Up()
	case isEnter(msg):
		if idx := m.actorList.Selected(); idx < len(m.actors) {
			actor := m.actors[idx]
			m.filter.actorType = actor.ActorType
			m.filter.actorID = actor.ActorID
			m.view = historyViewList
			m.loading = true
			return m, m.loadHistory()
		}
	case isBack(msg):
		m.view = historyViewList
	}
	return m, nil
}

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

func (m HistoryModel) loadScopes() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListAuditScopes()
		if err != nil {
			return errMsg{err}
		}
		return historyScopesLoadedMsg{items: items}
	}
}

func (m HistoryModel) loadActors() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListAuditActors("")
		if err != nil {
			return errMsg{err}
		}
		return historyActorsLoadedMsg{items: items}
	}
}

func (m HistoryModel) renderList() string {
	if len(m.items) == 0 {
		content := MutedStyle.Render("No audit entries yet.")
		return components.Indent(components.Box(content, m.width), 1)
	}

	filterLine := formatAuditFilters(m.filter)

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

	atWidth := 11
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

	cols := []components.TableColumn{
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
		{Header: "Action", Width: actionWidth, Align: lipgloss.Left},
		{Header: "Table", Width: tableNameWidth, Align: lipgloss.Left},
		{Header: "Actor", Width: actorWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.AuditEntry
	if idx := m.list.Selected(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}

	for i := range visible {
		absIdx := m.list.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.items) {
			continue
		}
		entry := m.items[absIdx]
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
		at := entry.ChangedAt

		if m.list.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			formatLocalTimeCompact(at),
			components.ClampTextWidthEllipsis(strings.ToUpper(action), actionWidth),
			components.ClampTextWidthEllipsis(tableName, tableNameWidth),
			components.ClampTextWidthEllipsis(actor, actorWidth),
		})
	}

	countLine := MutedStyle.Render(fmt.Sprintf("%d total", len(m.items)))
	if filterLine != "" {
		filterLine = MutedStyle.Render(filterLine)
	}

	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderAuditPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	parts := []string{}
	if filterLine != "" {
		parts = append(parts, filterLine)
	}
	parts = append(parts, countLine, body)
	content := strings.Join(parts, "\n\n") + "\n"

	return components.Indent(components.TitledBox("History", content, m.width), 1)
}

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

func (m HistoryModel) renderScopes() string {
	if len(m.scopes) == 0 {
		content := MutedStyle.Render("No scopes found.")
		return components.Indent(components.Box(content, m.width), 1)
	}

	contentWidth := components.BoxContentWidth(m.width)
	visible := m.scopeList.Visible()

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

	agentsWidth := 6
	entitiesWidth := 8
	contextWidth := 10
	scopeWidth := availableCols - (agentsWidth + entitiesWidth + contextWidth)
	if scopeWidth < 12 {
		scopeWidth = 12
	}

	cols := []components.TableColumn{
		{Header: "Scope", Width: scopeWidth, Align: lipgloss.Left},
		{Header: "Agents", Width: agentsWidth, Align: lipgloss.Right},
		{Header: "Entities", Width: entitiesWidth, Align: lipgloss.Right},
		{Header: "Context", Width: contextWidth, Align: lipgloss.Right},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.AuditScope
	if idx := m.scopeList.Selected(); idx >= 0 && idx < len(m.scopes) {
		previewItem = &m.scopes[idx]
	}

	for i := range visible {
		absIdx := m.scopeList.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.scopes) {
			continue
		}
		scope := m.scopes[absIdx]

		if m.scopeList.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(components.SanitizeOneLine(scope.Name), scopeWidth),
			fmt.Sprintf("%d", scope.AgentCount),
			fmt.Sprintf("%d", scope.EntityCount),
			fmt.Sprintf("%d", scope.ContextCount),
		})
	}

	countLine := MutedStyle.Render(fmt.Sprintf("%d total", len(m.scopes)))
	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderScopePreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.Indent(components.TitledBox("Scopes", content, m.width), 1)
}

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

func (m HistoryModel) renderActors() string {
	if len(m.actors) == 0 {
		content := MutedStyle.Render("No actors found.")
		return components.Indent(components.Box(content, m.width), 1)
	}

	contentWidth := components.BoxContentWidth(m.width)
	visible := m.actorList.Visible()

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

	// 3 columns -> 2 separators.
	availableCols := tableWidth - (2 * sepWidth)
	if availableCols < 30 {
		availableCols = 30
	}

	actionsWidth := 8
	lastWidth := 11
	actorWidth := availableCols - (actionsWidth + lastWidth)
	if actorWidth < 14 {
		actorWidth = 14
	}

	cols := []components.TableColumn{
		{Header: "Actor", Width: actorWidth, Align: lipgloss.Left},
		{Header: "Actions", Width: actionsWidth, Align: lipgloss.Right},
		{Header: "Last", Width: lastWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.AuditActor
	if idx := m.actorList.Selected(); idx >= 0 && idx < len(m.actors) {
		previewItem = &m.actors[idx]
	}

	for i := range visible {
		absIdx := m.actorList.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.actors) {
			continue
		}
		actor := m.actors[absIdx]

		name := actorDisplayName(actor)
		display := formatActorDisplay(actor, name)

		if m.actorList.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(components.SanitizeOneLine(display), actorWidth),
			fmt.Sprintf("%d", actor.ActionCount),
			formatLocalTimeCompact(actor.LastSeen),
		})
	}

	countLine := MutedStyle.Render(fmt.Sprintf("%d total", len(m.actors)))
	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderActorPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.Indent(components.TitledBox("Actors", content, m.width), 1)
}

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

func (m HistoryModel) renderDetail(entry api.AuditEntry) string {
	when := formatLocalTimeFull(entry.ChangedAt)
	actor := formatAuditActor(entry)
	fields := ""
	if len(entry.ChangedFields) > 0 {
		fields = strings.Join(entry.ChangedFields, ", ")
	}
	rows := []components.TableRow{
		{Label: "Table", Value: entry.TableName},
		{Label: "Action", Value: entry.Action},
		{Label: "Record", Value: entry.RecordID},
		{Label: "Actor", Value: actor},
		{Label: "At", Value: when},
	}
	if fields != "" {
		rows = append(rows, components.TableRow{Label: "Fields", Value: fields})
	}
	if entry.ChangeReason != nil && *entry.ChangeReason != "" {
		rows = append(rows, components.TableRow{Label: "Reason", Value: *entry.ChangeReason})
	}
	section := components.Table("Audit Entry", rows, m.width)

	diffRows := buildAuditDiffRows(entry)
	if len(diffRows) > 0 {
		diff := components.DiffTable("Changes", diffRows, m.width)
		section = section + "\n\n" + diff
	}
	return components.Indent(section, 1)
}

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

func formatAuditActor(entry api.AuditEntry) string {
	if entry.ActorName != nil && *entry.ActorName != "" {
		return *entry.ActorName
	}
	if entry.ChangedByType != nil && entry.ChangedByID != nil {
		return fmt.Sprintf("%s:%s", *entry.ChangedByType, shortID(*entry.ChangedByID))
	}
	if entry.ChangedByType != nil {
		return *entry.ChangedByType
	}
	return "system"
}

func formatAuditLine(entry api.AuditEntry) string {
	when := formatLocalTimeCompact(entry.ChangedAt)
	actor := formatAuditActor(entry)
	action := entry.Action
	if action == "" {
		action = "update"
	}
	return fmt.Sprintf("%s  %s  %s  %s", when, strings.ToUpper(action), entry.TableName, actor)
}

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

func actorDisplayName(actor api.AuditActor) string {
	if actor.ActorName != nil {
		if name := strings.TrimSpace(*actor.ActorName); name != "" {
			return name
		}
	}
	actorType := strings.TrimSpace(actor.ActorType)
	if actorType == "" {
		return "system"
	}
	return actorType
}

func formatActorRef(actor api.AuditActor) string {
	actorType := strings.TrimSpace(actor.ActorType)
	if actorType == "" {
		actorType = "system"
	}
	actorID := strings.TrimSpace(actor.ActorID)
	if actorID == "" {
		return actorType
	}
	return actorType + ":" + shortID(actorID)
}

func formatActorDisplay(actor api.AuditActor, name string) string {
	ref := formatActorRef(actor)
	if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(ref)) {
		return name
	}
	return fmt.Sprintf("%s  %s", name, ref)
}

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

func buildAuditDiffRows(entry api.AuditEntry) []components.DiffRow {
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
		for k := range entry.OldData {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		for k := range entry.NewData {
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
		from := entry.OldData[key]
		to := entry.NewData[key]
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
		return components.SanitizeText(trimmed)
	case time.Time:
		return formatLocalTimeFull(v)
	case map[string]any, []any:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return components.SanitizeText(fmt.Sprintf("%v", v))
		}
		return components.SanitizeText(string(b))
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

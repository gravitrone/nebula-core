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

// --- Messages ---

type approvalsLoadedMsg struct{ items []api.Approval }
type approvalDoneMsg struct{ id string }
type approvalDiffLoadedMsg struct {
	id      string
	changes map[string]any
}

// --- Inbox Model ---

// InboxModel shows pending approval requests from agents.
type InboxModel struct {
	client        *api.Client
	items         []api.Approval
	dataTable     table.Model
	loading       bool
	spinner       spinner.Model
	detail           *api.Approval
	detailChangeMap  map[string]any
	filtering     bool
	filterBuf     string
	filtered      []int
	selected      map[string]bool
	confirming    bool
	confirmBulk   bool
	rejecting     bool
	rejectPreview bool
	rejectBuf     string
	grantEditing  bool
	grantApproval string
	grantScopes   string
	grantTrusted  bool
	bulkRejectIDs []string
	pendingLimit  int
	width         int
	height        int

}

// NewInboxModel builds the inbox UI model.
func NewInboxModel(client *api.Client) InboxModel {
	return InboxModel{
		client:       client,
		spinner:      components.NewNebulaSpinner(),
		dataTable:    components.NewNebulaTable(nil, 15),
		selected:     make(map[string]bool),
		pendingLimit: 500,
	}
}

// Init handles init.
func (m InboxModel) Init() tea.Cmd {
	m.loading = true
	return tea.Batch(m.loadApprovals, m.spinner.Tick)
}

// Update updates update.
func (m InboxModel) Update(msg tea.Msg) (InboxModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case approvalsLoadedMsg:
		m.loading = false
		m.items = msg.items
		m.applyFilter(true)
		return m, nil

	case approvalDoneMsg:
		m.detail = nil
		m.rejecting = false
		m.rejectPreview = false
		m.rejectBuf = ""
		m.bulkRejectIDs = nil
		m.confirming = false
		m.selected = make(map[string]bool)
		m.loading = true
		return m, tea.Batch(m.loadApprovals, m.spinner.Tick)

	case approvalDiffLoadedMsg:
		if m.detail != nil && m.detail.ID == msg.id {
			if m.detailChangeMap == nil {
				m.detailChangeMap = parseApprovalChangeDetails(m.detail.ChangeDetails)
				if m.detailChangeMap == nil {
					m.detailChangeMap = make(map[string]any)
				}
			}
			m.detailChangeMap["changes"] = msg.changes
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.confirming {
			switch {
			case isKey(msg, "y"), isEnter(msg):
				m.confirming = false
				return m.approveSelected()
			case isKey(msg, "n"), isBack(msg):
				m.confirming = false
				return m, nil
			}
			return m, nil
		}
		if m.rejectPreview {
			return m.handleRejectPreview(msg)
		}
		if m.grantEditing {
			return m.handleGrantInput(msg)
		}
		if m.filtering {
			return m.handleFilterInput(msg)
		}
		// Reject input mode
		if m.rejecting {
			return m.handleRejectInput(msg)
		}

		// Detail view
		if m.detail != nil {
			return m.handleDetailKeys(msg)
		}

		// List view
		switch {
		case isDown(msg):
			m.dataTable.MoveDown(1)
		case isUp(msg):
			m.dataTable.MoveUp(1)
		case isSpace(msg):
			m.toggleSelected()
		case isEnter(msg):
			if item, ok := m.selectedItem(); ok {
				m.detail = &item
				m.detailChangeMap = parseApprovalChangeDetails(item.ChangeDetails)
				return m, m.loadApprovalDiff(item.ID)
			}
		case isKey(msg, "a"):
			return m.beginApproveFlow()
		case isKey(msg, "A"):
			if m.selectedCount() == 0 {
				m.selectAllFiltered()
			}
			m.confirming = true
			return m, nil
		case isKey(msg, "r"):
			return m.startReject()
		case isKey(msg, "f"):
			m.filtering = true
		case isKey(msg, "b"):
			m.toggleSelectAll()
		case isBack(msg):
			if len(m.selected) > 0 {
				m.selected = make(map[string]bool)
			}
		}
	}
	return m, nil
}

// View handles view.
func (m InboxModel) View() string {
	if m.loading {
		return "  " + m.spinner.View() + " " + MutedStyle.Render("Loading approvals...")
	}

	if m.confirming {
		summary := m.approveSummaryRows()
		return components.Indent(components.ConfirmPreviewDialog("Approve Requests", summary, m.approveDiffRows(), m.width), 1)
	}

	if m.rejectPreview {
		summary := []components.TableRow{
			{Label: "Action", Value: "Reject"},
			{Label: "Requests", Value: fmt.Sprintf("%d", len(m.bulkRejectIDs))},
			{Label: "Notes", Value: formatAny(strings.TrimSpace(m.rejectBuf))},
		}
		diffs := []components.DiffRow{
			{Label: "Status", From: "Pending", To: "Rejected"},
			{Label: "Review Notes", From: "None", To: formatAny(strings.TrimSpace(m.rejectBuf))},
		}
		return components.Indent(components.ConfirmPreviewDialog("Reject Requests", summary, diffs, m.width), 1)
	}

	if m.grantEditing {
		return m.renderGrantEditor()
	}

	if m.rejecting && m.detail != nil {
		return components.Indent(components.InputDialog("Reject: Enter Review Notes", m.rejectBuf), 1)
	}

	if m.filtering {
		return components.Indent(components.InputDialog("Filter Approvals", m.filterBuf), 1)
	}

	if m.detail != nil {
		return m.renderDetail()
	}

	if len(m.items) == 0 {
		return components.Indent(components.EmptyStateBox(
			"Inbox",
			"No pending approvals.",
			[]string{"Switch tabs with 1-9/0", "Open command palette with /"},
			m.width,
		), 1)
	}

	if len(m.filtered) == 0 {
		return components.Indent(components.EmptyStateBox(
			"Inbox",
			"No approvals match the filter.",
			[]string{"Press f to update filter", "Press esc to clear"},
			m.width,
		), 1)
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

	actionWidth := 19
	whoWidth := 14
	atWidth := compactTimeColumnWidth

	titleWidth := availableCols - (actionWidth + whoWidth + atWidth)
	if titleWidth < 12 {
		titleWidth = 12
	}
	if titleWidth > 40 {
		titleWidth = 40
	}

	showCheckboxes := len(m.selected) > 0
	tableRows := make([]table.Row, 0, len(m.filtered))
	for i := range m.filtered {
		item, ok := m.itemAtFilteredIndex(i)
		if !ok {
			continue
		}

		fullTitle := approvalTitle(item)
		if showCheckboxes {
			checkbox := MutedStyle.Render("[ ]")
			if m.selected[item.ID] {
				checkbox = ErrorStyle.Render("[X]")
			}
			fullTitle = checkbox + " " + fullTitle
		}
		title := components.ClampTextWidthEllipsis(fullTitle, titleWidth)
		action := components.ClampTextWidthEllipsis(humanizeApprovalType(item.RequestType), actionWidth)
		who := components.ClampTextWidthEllipsis(approvalWhoLabel(item), whoWidth)
		when := formatLocalTimeCompact(item.CreatedAt)
		tableRows = append(tableRows, table.Row{title, action, who, when})
	}

	m.dataTable.SetColumns([]table.Column{
		{Title: "Title", Width: titleWidth},
		{Title: "Action", Width: actionWidth},
		{Title: "Who", Width: whoWidth},
		{Title: "At", Width: atWidth},
	})
	actualTableWidth := titleWidth + actionWidth + whoWidth + atWidth + cellPadding
	m.dataTable.SetWidth(actualTableWidth)
	m.dataTable.SetRows(tableRows)

	countLine := ""
	hasContext := m.filterBuf != "" || m.selectedCount() > 0
	if hasContext {
		parts := []string{fmt.Sprintf("%d pending", len(m.items))}
		if m.filterBuf != "" {
			parts = append(parts, fmt.Sprintf("filter: %s", m.filterBuf))
		}
		if count := m.selectedCount(); count > 0 {
			parts = append(parts, fmt.Sprintf("selected: %d", count))
		}
		countLine = MutedStyle.Render(strings.Join(parts, " · "))
	}

	tableView := components.TableBaseStyle.Render(m.dataTable.View())
	preview := ""
	var previewItem *api.Approval
	if item, ok := m.selectedItem(); ok {
		previewItem = &item
	}
	if previewItem != nil {
		previewContentWidth := previewBoxContentWidth(previewWidth)
		content := renderApprovalPreview(*previewItem, m.selected[previewItem.ID], previewContentWidth)
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
	centered := lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, result)
	return lipgloss.JoinVertical(lipgloss.Left, components.Indent(centered, 1), m.renderStatusHints())
}

// renderStatusHints builds the bottom status bar with keycap pill hints.
func (m InboxModel) renderStatusHints() string {
	hints := []string{
		components.Hint("1-9/0", "Tabs"),
		components.Hint("/", "Command"),
		components.Hint("?", "Help"),
		components.Hint("q", "Quit"),
		components.Hint("\u2191/\u2193", "Scroll"),
		components.Hint("enter", "Review"),
		components.Hint("a", "Approve"),
		components.Hint("r", "Reject"),
	}
	return components.StatusBar(hints, m.width)
}

// --- Helpers ---

func (m InboxModel) loadApprovals() tea.Msg {
	limit := m.pendingLimit
	if limit <= 0 {
		limit = 500
	}
	items, err := m.client.GetPendingApprovalsWithParams(limit, 0)
	if err != nil {
		return errMsg{err}
	}
	return approvalsLoadedMsg{items}
}

// SetPendingLimit sets set pending limit.
func (m *InboxModel) SetPendingLimit(limit int) {
	if limit <= 0 {
		limit = 500
	}
	if limit > 5000 {
		limit = 5000
	}
	m.pendingLimit = limit
}

// loadApprovalDiff loads load approval diff.
func (m InboxModel) loadApprovalDiff(id string) tea.Cmd {
	return func() tea.Msg {
		diff, err := m.client.GetApprovalDiff(id)
		if err != nil {
			return errMsg{err}
		}
		return approvalDiffLoadedMsg{id: id, changes: diff.Changes}
	}
}

// approveSelected handles approve selected.
func (m InboxModel) approveSelected() (InboxModel, tea.Cmd) {
	ids := m.selectedIDs()
	if len(ids) == 0 && m.detail != nil {
		ids = append(ids, m.detail.ID)
	}
	if len(ids) == 0 {
		if item, ok := m.selectedItem(); ok {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return m, nil
	}
	m.detail = nil
	return m, func() tea.Msg {
		for _, id := range ids {
			_, err := m.client.ApproveRequest(id)
			if err != nil {
				return errMsg{err}
			}
		}
		return approvalDoneMsg{""}
	}
}

// beginApproveFlow handles begin approve flow.
func (m InboxModel) beginApproveFlow() (InboxModel, tea.Cmd) {
	ids := m.selectedIDs()
	if len(ids) == 0 && m.detail != nil {
		ids = append(ids, m.detail.ID)
	}
	if len(ids) == 0 {
		if item, ok := m.selectedItem(); ok {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 1 {
		if approval, ok := m.findApprovalByID(ids[0]); ok && approval.RequestType == "register_agent" {
			m.grantEditing = true
			m.grantApproval = approval.ID
			m.grantScopes = strings.Join(requestedScopesFromApproval(approval), ",")
			m.grantTrusted = requestedRequiresApprovalFromApproval(approval)
			return m, nil
		}
	}
	m.confirming = true
	return m, nil
}

// handleDetailKeys handles handle detail keys.
func (m InboxModel) handleDetailKeys(msg tea.KeyPressMsg) (InboxModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.detail = nil
	case isKey(msg, "a"):
		return m.beginApproveFlow()
	case isKey(msg, "r"):
		m.rejecting = true
		m.rejectBuf = ""
	}
	return m, nil
}

// handleGrantInput handles handle grant input.
func (m InboxModel) handleGrantInput(msg tea.KeyPressMsg) (InboxModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.grantEditing = false
		m.grantApproval = ""
		m.grantScopes = ""
		m.grantTrusted = false
		return m, nil
	case isKey(msg, "t"):
		m.grantTrusted = !m.grantTrusted
		return m, nil
	case isEnter(msg):
		scopes := parseScopesCSV(m.grantScopes)
		if len(scopes) == 0 {
			return m, func() tea.Msg {
				return errMsg{fmt.Errorf("at least one scope is required")}
			}
		}
		trusted := m.grantTrusted
		approveID := m.grantApproval
		input := &api.ApproveRequestInput{
			GrantScopes:           scopes,
			GrantRequiresApproval: &trusted,
		}
		m.grantEditing = false
		m.grantApproval = ""
		m.grantScopes = ""
		m.grantTrusted = false
		m.detail = nil
		return m, func() tea.Msg {
			_, err := m.client.ApproveRequestWithInput(approveID, input)
			if err != nil {
				return errMsg{err}
			}
			return approvalDoneMsg{approveID}
		}
	case isKey(msg, "backspace"):
		if len(m.grantScopes) > 0 {
			m.grantScopes = m.grantScopes[:len(m.grantScopes)-1]
		}
		return m, nil
	default:
		s := keyText(msg)
		if s != "" {
			m.grantScopes += s
		}
	}
	return m, nil
}

// renderGrantEditor renders render grant editor.
func (m InboxModel) renderGrantEditor() string {
	mode := "true"
	if !m.grantTrusted {
		mode = "false"
	}
	lines := []string{
		"Approve register_agent with reviewer grants.",
		"",
		"Scopes (comma separated): " + m.grantScopes,
		"requires_approval: " + mode + " (press t to toggle)",
		"",
		MutedStyle.Render("enter submit  |  esc cancel"),
	}
	return components.Indent(
		components.TitledBox("Approve Agent Enrollment", strings.Join(lines, "\n"), m.width),
		1,
	)
}

// handleRejectInput handles handle reject input.
func (m InboxModel) handleRejectInput(msg tea.KeyPressMsg) (InboxModel, tea.Cmd) {
	if m.detail == nil {
		m.rejecting = false
		m.rejectBuf = ""
		m.bulkRejectIDs = nil
		return m, nil
	}
	switch {
	case isBack(msg):
		if len(m.bulkRejectIDs) > 0 {
			m.detail = nil
		}
		m.rejecting = false
		m.rejectBuf = ""
		m.bulkRejectIDs = nil
	case isEnter(msg):
		ids := m.bulkRejectIDs
		if len(ids) == 0 {
			ids = []string{m.detail.ID}
		}
		m.rejecting = false
		m.rejectPreview = true
		m.bulkRejectIDs = ids
		return m, nil
	case isKey(msg, "backspace"):
		if len(m.rejectBuf) > 0 {
			m.rejectBuf = m.rejectBuf[:len(m.rejectBuf)-1]
		}
	default:
		if ch := keyText(msg); ch != "" {
			m.rejectBuf += ch
		}
	}
	return m, nil
}

// renderDetail renders render detail.
func (m InboxModel) renderDetail() string {
	a := m.detail
	var sections []string

	// Approval info table
	rows := []components.TableRow{
		{Label: "ID", Value: a.ID},
		{Label: "Type", Value: a.RequestType},
		{Label: "Status", Value: a.Status},
		{Label: "Agent", Value: a.AgentName},
		{Label: "Requested By", Value: approvalRequestedBy(*a)},
		{Label: "Created", Value: formatLocalTimeFull(a.CreatedAt)},
	}
	if a.JobID != nil {
		rows = append(rows, components.TableRow{Label: "Job ID", Value: *a.JobID})
	}
	if a.Notes != nil && *a.Notes != "" {
		rows = append(rows, components.TableRow{Label: "Review Notes", Value: *a.Notes})
	}
	sections = append(sections, components.Table("Approval Request", rows, m.width))

	// Change details
	cd := m.detailChangeMap
	if cd == nil {
		cd = parseApprovalChangeDetails(a.ChangeDetails)
	}
	if len(cd) > 0 {
		var summaryRows []components.TableRow
		var diffRows []components.DiffRow
		metadata, hasMetadata := map[string]any(nil), false
		nested := make(map[string]map[string]any)

		keys := make([]string, 0, len(cd))
		for k := range cd {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := cd[k]
			if k == "changes" {
				// Diff object with from/to pairs
				if changesMap, ok := v.(map[string]any); ok {
					diffKeys := make([]string, 0, len(changesMap))
					for field := range changesMap {
						diffKeys = append(diffKeys, field)
					}
					sort.Strings(diffKeys)
					for _, field := range diffKeys {
						diff := changesMap[field]
						if diffObj, ok := diff.(map[string]any); ok {
							from := approvalDiffValue(cd, field, diffObj["from"])
							to := approvalDiffValue(cd, field, diffObj["to"])
							if from == to {
								continue
							}
							diffRows = append(diffRows, components.DiffRow{
								Label: detailLabel(field),
								From:  from,
								To:    to,
							})
						}
					}
				}
				continue
			}
			if val, ok := v.(map[string]any); ok {
				if strings.EqualFold(k, "metadata") {
					metadata = val
					hasMetadata = true
					continue
				}
				nested[k] = val
				continue
			}
			display := formatAny(v)
			switch strings.ToLower(k) {
			case "source_id":
				if label := approvalEndpointLabel(cd, "source"); label != "" {
					display = label
				}
			case "target_id":
				if label := approvalEndpointLabel(cd, "target"); label != "" {
					display = label
				}
			case "entity_ids":
				names := parseStringList(cd["entity_names"])
				if len(names) > 0 {
					display = strings.Join(names, ", ")
				} else {
					ids := parseStringList(v)
					if len(ids) > 0 {
						short := make([]string, 0, len(ids))
						for _, id := range ids {
							short = append(short, shortID(id))
						}
						display = strings.Join(short, ", ")
					}
				}
			}
			summaryRows = append(summaryRows, components.TableRow{
				Label: detailLabel(k),
				Value: display,
			})
		}

		if hasMetadata {
			sections = append(sections, components.MetadataTable(metadata, m.width))
		}

		if len(summaryRows) > 0 {
			sections = append(sections, components.Table("Change Details", summaryRows, m.width))
		}

		nestedKeys := make([]string, 0, len(nested))
		for k := range nested {
			nestedKeys = append(nestedKeys, k)
		}
		sort.Strings(nestedKeys)

		// Render each nested object as its own titled table.
		for _, k := range nestedKeys {
			obj := nested[k]
			var nestedRows []components.TableRow
			objKeys := make([]string, 0, len(obj))
			for sk := range obj {
				objKeys = append(objKeys, sk)
			}
			sort.Strings(objKeys)
			for _, sk := range objKeys {
				nestedRows = append(nestedRows, components.TableRow{
					Label: detailLabel(sk),
					Value: formatAny(obj[sk]),
				})
			}
			if len(nestedRows) > 0 {
				sections = append(sections, components.Table(detailLabel(k), nestedRows, m.width))
			}
		}

		// Diff table for update requests.
		if len(diffRows) > 0 {
			sections = append(sections, components.DiffTable("Changes", diffRows, m.width))
		}
	}

	return components.Indent(strings.Join(sections, "\n\n"), 1)
}

// formatAny handles format any.
func formatAny(v any) string {
	switch val := v.(type) {
	case map[string]any:
		lines := metadataLinesPlain(val, 0)
		if len(lines) == 0 {
			return "None"
		}
		return strings.Join(lines, "\n")
	case []string:
		if len(val) == 0 {
			return "None"
		}
		return components.SanitizeOneLine(strings.Join(val, ", "))
	case []any:
		if len(val) == 0 {
			return "None"
		}
		parts := make([]string, 0, len(val))
		for _, item := range val {
			rendered := formatAnyInline(item)
			if rendered == "" {
				continue
			}
			parts = append(parts, rendered)
		}
		if len(parts) == 0 {
			return "None"
		}
		return strings.Join(parts, ", ")
	case nil:
		return "None"
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" || s == "<nil>" || s == "-" || s == "--" {
			return "None"
		}
		return components.SanitizeOneLine(s)
	}
}

// formatAnyInline handles format any inline.
func formatAnyInline(v any) string {
	switch val := v.(type) {
	case nil:
		return ""
	case string:
		s := strings.TrimSpace(val)
		if s == "" || s == "<nil>" {
			return ""
		}
		return components.SanitizeOneLine(s)
	case map[string]any:
		encoded, err := json.Marshal(val)
		if err != nil {
			return components.SanitizeOneLine(fmt.Sprintf("%v", val))
		}
		return components.SanitizeOneLine(string(encoded))
	default:
		s := strings.TrimSpace(fmt.Sprintf("%v", val))
		if s == "" || s == "<nil>" || s == "-" || s == "--" {
			return ""
		}
		return components.SanitizeOneLine(s)
	}
}

// detailLabel handles detail label.
func detailLabel(raw string) string {
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
		switch lower {
		case "id":
			out = append(out, "ID")
		case "url":
			out = append(out, "URL")
		case "api":
			out = append(out, "API")
		case "mcp":
			out = append(out, "MCP")
		default:
			out = append(out, strings.ToUpper(lower[:1])+lower[1:])
		}
	}
	if len(out) == 0 {
		return components.SanitizeOneLine(raw)
	}
	return components.SanitizeOneLine(strings.Join(out, " "))
}

// formatApprovalLine handles format approval line.
func formatApprovalLine(a api.Approval) string {
	cd := parseApprovalChangeDetails(a.ChangeDetails)
	name := ""
	if n, ok := cd["name"]; ok {
		name = fmt.Sprintf(": %q", components.SanitizeText(fmt.Sprintf("%v", n)))
	}
	return fmt.Sprintf(
		"[%s] %s%s",
		components.SanitizeText(a.RequestType),
		components.SanitizeText(a.Status),
		name,
	)
}

// approvalTitle handles approval title.
func approvalTitle(a api.Approval) string {
	cd := parseApprovalChangeDetails(a.ChangeDetails)
	if v, ok := cd["name"]; ok {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s != "" {
			return components.SanitizeOneLine(s)
		}
	}
	if v, ok := cd["title"]; ok {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s != "" {
			return components.SanitizeOneLine(s)
		}
	}
	if v, ok := cd["entity_name"]; ok {
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s != "" {
			return components.SanitizeOneLine(s)
		}
	}

	reqType := strings.TrimSpace(components.SanitizeText(a.RequestType))

	// More descriptive fallbacks for request types that don't carry names/titles.
	switch reqType {
	case "create_relationship", "update_relationship":
		relType := strings.TrimSpace(fmt.Sprintf("%v", cd["relationship_type"]))
		relType = components.SanitizeOneLine(relType)
		src := approvalEndpointLabel(cd, "source")
		tgt := approvalEndpointLabel(cd, "target")
		if relType != "" && relType != "<nil>" {
			if src != "" && tgt != "" {
				return components.SanitizeOneLine(fmt.Sprintf("%s (%s -> %s)", relType, src, tgt))
			}
			return relType
		}
	case "bulk_update_entity_scopes", "bulk_update_entity_tags":
		entityNames := parseStringList(cd["entity_names"])
		if len(entityNames) == 1 {
			return components.SanitizeOneLine(fmt.Sprintf("%s (%s)", humanizeApprovalType(reqType), entityNames[0]))
		}
		if len(entityNames) > 1 {
			preview := strings.Join(entityNames[:min(2, len(entityNames))], ", ")
			if len(entityNames) > 2 {
				preview += fmt.Sprintf(" +%d", len(entityNames)-2)
			}
			return components.SanitizeOneLine(fmt.Sprintf("%s (%s)", humanizeApprovalType(reqType), preview))
		}
		entityIDs := parseStringList(cd["entity_ids"])
		if len(entityIDs) == 1 {
			return components.SanitizeOneLine(fmt.Sprintf("%s (%s)", humanizeApprovalType(reqType), shortID(entityIDs[0])))
		}
		if len(entityIDs) > 1 {
			return components.SanitizeOneLine(
				fmt.Sprintf("%s (%d entities)", humanizeApprovalType(reqType), len(entityIDs)),
			)
		}
	case "create_log", "update_log":
		logType := strings.TrimSpace(fmt.Sprintf("%v", cd["log_type"]))
		logType = components.SanitizeOneLine(logType)
		if logType != "" && logType != "<nil>" {
			return components.SanitizeOneLine("log: " + logType)
		}
	}

	// Default: make it human readable.
	if reqType != "" {
		return components.SanitizeOneLine(humanizeApprovalType(reqType))
	}
	return ""
}

// humanizeApprovalType handles humanize approval type.
func humanizeApprovalType(t string) string {
	t = strings.TrimSpace(components.SanitizeText(t))
	if t == "" {
		return ""
	}
	parts := strings.Split(strings.ToLower(t), "_")
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

// renderApprovalPreview renders render approval preview.
func renderApprovalPreview(a api.Approval, picked bool, width int) string {
	if width <= 0 {
		return ""
	}

	title := components.SanitizeOneLine(approvalTitle(a))
	action := components.SanitizeOneLine(humanizeApprovalType(a.RequestType))
	who := approvalWhoLabel(a)
	status := components.SanitizeOneLine(a.Status)
	when := formatLocalTimeCompact(a.CreatedAt)

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for i, part := range wrapPreviewText(title, width) {
		if i == 0 {
			lines = append(lines, SelectedStyle.Render(part))
			continue
		}
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Action", action, width))
	lines = append(lines, renderPreviewRow("Who", who, width))
	lines = append(lines, renderPreviewRow("At", when, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	if picked {
		lines = append(lines, renderPreviewRow("In batch", "yes", width))
	}

	cd := parseApprovalChangeDetails(a.ChangeDetails)
	if scopes := parseStringList(cd["scopes"]); len(scopes) > 0 {
		lines = append(lines, renderPreviewRow("Scopes", formatScopePreview(scopes), width))
	} else if scope := previewStringValue(cd, "scope"); scope != "" {
		lines = append(lines, renderPreviewRow("Scope", formatScopePreview([]string{scope}), width))
	}
	if tags := previewListValue(cd, "tags"); tags != "" {
		lines = append(lines, renderPreviewRow("Tags", tags, width))
	}
	if typ := previewStringValue(cd, "type"); typ != "" {
		lines = append(lines, renderPreviewRow("Type", typ, width))
	}
	if rel := previewStringValue(cd, "relationship_type"); rel != "" {
		lines = append(lines, renderPreviewRow("Rel", rel, width))
	}
	if src := approvalEndpointLabel(cd, "source"); src != "" {
		lines = append(lines, renderPreviewRow("From", src, width))
	}
	if tgt := approvalEndpointLabel(cd, "target"); tgt != "" {
		lines = append(lines, renderPreviewRow("To", tgt, width))
	}
	if ids := parseStringList(cd["entity_ids"]); len(ids) > 0 {
		display := make([]string, 0, len(ids))
		names := parseStringList(cd["entity_names"])
		if len(names) == len(ids) {
			display = append(display, names...)
		} else {
			for _, id := range ids {
				display = append(display, shortID(id))
			}
		}
		lines = append(lines, renderPreviewRow("Entities", strings.Join(display, ", "), width))
	}
	if logType := previewStringValue(cd, "log_type"); logType != "" {
		lines = append(lines, renderPreviewRow("Log", logType, width))
	}

	return padPreviewLines(lines, width)
}

// applyFilter handles apply filter.
func (m *InboxModel) applyFilter(resetSelection bool) {
	if resetSelection {
		m.selected = make(map[string]bool)
	}
	m.filtered = m.filtered[:0]
	filter := parseApprovalFilter(m.filterBuf)
	for i, a := range m.items {
		if matchesApprovalFilter(a, filter) {
			m.filtered = append(m.filtered, i)
		}
	}
	rows := make([]table.Row, len(m.filtered))
	for i, itemIdx := range m.filtered {
		if itemIdx >= 0 && itemIdx < len(m.items) {
			rows[i] = table.Row{formatApprovalLine(m.items[itemIdx])}
		}
	}
	m.dataTable.SetRows(rows)
	m.dataTable.SetCursor(0)
}

// selectedItem handles selected item.
func (m *InboxModel) selectedItem() (api.Approval, bool) {
	idx := m.dataTable.Cursor()
	return m.itemAtFilteredIndex(idx)
}

// selectAllFiltered handles select all filtered.
func (m *InboxModel) selectAllFiltered() {
	for _, itemIdx := range m.filtered {
		if itemIdx < 0 || itemIdx >= len(m.items) {
			continue
		}
		m.selected[m.items[itemIdx].ID] = true
	}
}

// toggleSelectAll handles toggle select all.
func (m *InboxModel) toggleSelectAll() {
	if len(m.filtered) == 0 {
		return
	}
	allSelected := true
	for _, itemIdx := range m.filtered {
		if itemIdx < 0 || itemIdx >= len(m.items) {
			continue
		}
		if !m.selected[m.items[itemIdx].ID] {
			allSelected = false
			break
		}
	}
	if allSelected {
		m.selected = make(map[string]bool)
		return
	}
	m.selectAllFiltered()
}

// itemAtFilteredIndex handles item at filtered index.
func (m *InboxModel) itemAtFilteredIndex(filteredIdx int) (api.Approval, bool) {
	if filteredIdx < 0 || filteredIdx >= len(m.filtered) {
		return api.Approval{}, false
	}
	itemIdx := m.filtered[filteredIdx]
	if itemIdx < 0 || itemIdx >= len(m.items) {
		return api.Approval{}, false
	}
	return m.items[itemIdx], true
}

// toggleSelected handles toggle selected.
func (m *InboxModel) toggleSelected() {
	item, ok := m.selectedItem()
	if !ok {
		return
	}
	if m.selected[item.ID] {
		delete(m.selected, item.ID)
		return
	}
	m.selected[item.ID] = true
}

// selectedIDs handles selected ids.
func (m *InboxModel) selectedIDs() []string {
	if len(m.selected) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m.selected))
	for _, item := range m.items {
		if m.selected[item.ID] {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// selectedCount handles selected count.
func (m *InboxModel) selectedCount() int {
	return len(m.selectedIDs())
}

// startReject handles start reject.
func (m *InboxModel) startReject() (InboxModel, tea.Cmd) {
	ids := m.selectedIDs()
	if len(ids) > 0 {
		m.bulkRejectIDs = ids
		m.rejecting = true
		m.rejectPreview = false
		m.rejectBuf = ""
		m.detail = &api.Approval{ID: ids[0]}
		return *m, nil
	}
	if item, ok := m.selectedItem(); ok {
		m.detail = &item
		m.rejecting = true
		m.rejectPreview = false
		m.rejectBuf = ""
		return *m, nil
	}
	return *m, nil
}

// handleRejectPreview handles handle reject preview.
func (m InboxModel) handleRejectPreview(msg tea.KeyPressMsg) (InboxModel, tea.Cmd) {
	switch {
	case isKey(msg, "y"), isEnter(msg):
		ids := append([]string(nil), m.bulkRejectIDs...)
		notes := m.rejectBuf
		m.rejectPreview = false
		m.rejectBuf = ""
		m.detail = nil
		m.bulkRejectIDs = nil
		return m, func() tea.Msg {
			for _, id := range ids {
				_, err := m.client.RejectRequest(id, notes)
				if err != nil {
					return errMsg{err}
				}
			}
			return approvalDoneMsg{""}
		}
	case isKey(msg, "n"), isBack(msg):
		m.rejectPreview = false
		m.rejecting = true
		return m, nil
	}
	return m, nil
}

// approveSummaryRows handles approve summary rows.
func (m InboxModel) approveSummaryRows() []components.TableRow {
	ids := m.selectedIDs()
	if len(ids) == 0 && m.detail != nil {
		ids = append(ids, m.detail.ID)
	}
	if len(ids) == 0 {
		if item, ok := m.selectedItem(); ok {
			ids = append(ids, item.ID)
		}
	}

	rows := []components.TableRow{
		{Label: "Action", Value: "approve"},
		{Label: "Requests", Value: fmt.Sprintf("%d", len(ids))},
	}
	if len(ids) == 1 {
		rows = append(rows, components.TableRow{Label: "Request ID", Value: ids[0]})
	}
	return rows
}

// findApprovalByID handles find approval by id.
func (m InboxModel) findApprovalByID(id string) (api.Approval, bool) {
	for _, item := range m.items {
		if item.ID == id {
			return item, true
		}
	}
	return api.Approval{}, false
}

// requestedScopesFromApproval handles requested scopes from approval.
func requestedScopesFromApproval(a api.Approval) []string {
	cd := parseApprovalChangeDetails(a.ChangeDetails)
	raw := cd["requested_scopes"]
	scopes := parseStringList(raw)
	if len(scopes) == 0 {
		scopes = []string{"public"}
	}
	return scopes
}

// requestedRequiresApprovalFromApproval handles requested requires approval from approval.
func requestedRequiresApprovalFromApproval(a api.Approval) bool {
	cd := parseApprovalChangeDetails(a.ChangeDetails)
	raw, ok := cd["requested_requires_approval"]
	if !ok {
		return true
	}
	if value, ok := raw.(bool); ok {
		return value
	}
	return true
}

// parseApprovalChangeDetails parses the change_details text field into a map.
func parseApprovalChangeDetails(text string) map[string]any {
	if text == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(text), &m); err != nil {
		return nil
	}
	return m
}

// parseStringList parses parse string list.
func parseStringList(raw any) []string {
	switch value := raw.(type) {
	case []string:
		return value
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	case string:
		return parseScopesCSV(value)
	default:
		return nil
	}
}

// approvalRequestedBy handles approval requested by.
func approvalRequestedBy(a api.Approval) string {
	requested := strings.TrimSpace(a.RequestedBy)
	requestedByName := strings.TrimSpace(components.SanitizeOneLine(a.RequestedByName))
	agent := strings.TrimSpace(components.SanitizeOneLine(a.AgentName))
	if requestedByName == "" {
		requestedByName = agent
	}
	if requested == "" {
		if requestedByName == "" {
			return "None"
		}
		return requestedByName
	}
	if requestedByName == "" {
		return shortID(requested)
	}
	return components.SanitizeOneLine(fmt.Sprintf("%s (%s)", requestedByName, shortID(requested)))
}

// approvalWhoLabel handles approval who label.
func approvalWhoLabel(a api.Approval) string {
	label := strings.TrimSpace(components.SanitizeOneLine(a.RequestedByName))
	if label != "" {
		return label
	}
	label = strings.TrimSpace(components.SanitizeOneLine(a.AgentName))
	if label != "" {
		return label
	}
	if strings.TrimSpace(a.RequestedBy) != "" {
		return shortID(a.RequestedBy)
	}
	return "system"
}

// approvalEndpointLabel handles approval endpoint label.
func approvalEndpointLabel(details map[string]any, prefix string) string {
	nameKeys := []string{
		prefix + "_name",
		prefix + "_title",
		prefix + "_label",
	}
	for _, key := range nameKeys {
		if value := previewStringValue(details, key); value != "" {
			return value
		}
	}
	idKey := prefix + "_id"
	if id := previewStringValue(details, idKey); id != "" {
		return shortID(id)
	}
	typeKey := prefix + "_type"
	if kind := previewStringValue(details, typeKey); kind != "" {
		return kind
	}
	return ""
}

// parseScopesCSV parses parse scopes csv.
func parseScopesCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope != "" {
			out = append(out, scope)
		}
	}
	return out
}

// approveDiffRows handles approve diff rows.
func (m InboxModel) approveDiffRows() []components.DiffRow {
	if m.detail == nil {
		return nil
	}
	cd := m.detailChangeMap
	if cd == nil {
		cd = parseApprovalChangeDetails(m.detail.ChangeDetails)
	}
	raw, ok := cd["changes"]
	if !ok {
		return nil
	}
	changesMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	rows := make([]components.DiffRow, 0, len(changesMap))
	for field, diff := range changesMap {
		diffObj, ok := diff.(map[string]any)
		if !ok {
			continue
		}
		from := approvalDiffValue(cd, field, diffObj["from"])
		to := approvalDiffValue(cd, field, diffObj["to"])
		if from == to {
			continue
		}
		rows = append(rows, components.DiffRow{
			Label: field,
			From:  from,
			To:    to,
		})
	}
	return rows
}

// approvalDiffValue handles approval diff value.
func approvalDiffValue(details map[string]any, field string, raw any) string {
	base := formatAny(raw)
	if base == "None" {
		return base
	}
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "source_id":
		if label := approvalEndpointLabel(details, "source"); label != "" {
			return label
		}
	case "target_id":
		if label := approvalEndpointLabel(details, "target"); label != "" {
			return label
		}
	case "entity_id":
		if label := previewStringValue(details, "entity_name"); label != "" {
			return label
		}
	case "entity_ids":
		if names := parseStringList(details["entity_names"]); len(names) > 0 {
			return strings.Join(names, ", ")
		}
	}
	return base
}

// handleFilterInput handles handle filter input.
func (m InboxModel) handleFilterInput(msg tea.KeyPressMsg) (InboxModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.filtering = false
		m.filterBuf = ""
		m.applyFilter(true)
	case isEnter(msg):
		m.filtering = false
		m.applyFilter(true)
	case isKey(msg, "backspace"):
		if len(m.filterBuf) > 0 {
			m.filterBuf = m.filterBuf[:len(m.filterBuf)-1]
			m.applyFilter(true)
		}
	default:
		if ch := keyText(msg); ch != "" {
			m.filterBuf += ch
			m.applyFilter(true)
		}
	}
	return m, nil
}

type approvalFilter struct {
	agent string
	req   string
	since *time.Time
	terms []string
}

// parseApprovalFilter parses parse approval filter.
func parseApprovalFilter(raw string) approvalFilter {
	filter := approvalFilter{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return filter
	}
	for _, token := range strings.Fields(raw) {
		switch {
		case strings.HasPrefix(token, "agent:"):
			filter.agent = strings.ToLower(strings.TrimPrefix(token, "agent:"))
		case strings.HasPrefix(token, "type:"):
			filter.req = strings.ToLower(strings.TrimPrefix(token, "type:"))
		case strings.HasPrefix(token, "since:"):
			val := strings.TrimPrefix(token, "since:")
			if t := parseFilterTime(val); t != nil {
				filter.since = t
			}
		default:
			filter.terms = append(filter.terms, strings.ToLower(token))
		}
	}
	return filter
}

// parseFilterTime parses parse filter time.
func parseFilterTime(value string) *time.Time {
	value = strings.TrimSpace(strings.ToLower(value))
	now := time.Now()
	switch value {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &start
	case "yesterday":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-24 * time.Hour)
		return &start
	}
	if strings.HasSuffix(value, "h") {
		if dur, err := time.ParseDuration(value); err == nil {
			t := now.Add(-dur)
			return &t
		}
	}
	if strings.HasSuffix(value, "d") {
		days := strings.TrimSuffix(value, "d")
		if n, err := time.ParseDuration(days + "h"); err == nil {
			t := now.Add(-24 * n)
			return &t
		}
	}
	if t, err := time.ParseInLocation("2006-01-02", value, now.Location()); err == nil {
		return &t
	}
	return nil
}

// matchesApprovalFilter handles matches approval filter.
func matchesApprovalFilter(a api.Approval, filter approvalFilter) bool {
	if filter.agent != "" && !strings.Contains(strings.ToLower(a.AgentName), filter.agent) {
		return false
	}
	if filter.req != "" && !strings.Contains(strings.ToLower(a.RequestType), filter.req) {
		return false
	}
	if filter.since != nil && a.CreatedAt.Before(*filter.since) {
		return false
	}
	if len(filter.terms) > 0 {
		search := strings.ToLower(formatApprovalLine(a))
		for _, term := range filter.terms {
			if !strings.Contains(search, term) {
				return false
			}
		}
	}
	return true
}

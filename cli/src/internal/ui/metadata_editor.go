package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

var copyMetadataEditorClipboard = copyTextToClipboard

type metadataEditorRow struct {
	path  string
	value string
}

type MetadataEditor struct {
	Active bool
	Buffer string
	Scopes []string

	scopeOptions   []string
	scopeIdx       int
	scopeSelecting bool

	rows     []metadataEditorRow
	list     *components.List
	selected map[int]bool

	entryMode    bool
	entryBuf     string
	entryEditIdx int

	inspectMode   bool
	inspectRowIdx int
	inspectOffset int

	notice string
}

// Open handles open.
func (m *MetadataEditor) Open(initial map[string]any) {
	m.Active = true
	m.Load(initial)
}

// Reset handles reset.
func (m *MetadataEditor) Reset() {
	m.Active = false
	m.Buffer = ""
	m.Scopes = nil
	m.scopeIdx = 0
	m.scopeSelecting = false
	m.rows = nil
	m.list = nil
	m.selected = nil
	m.entryMode = false
	m.entryBuf = ""
	m.entryEditIdx = -1
	m.inspectMode = false
	m.inspectRowIdx = 0
	m.inspectOffset = 0
	m.notice = ""
}

// Load loads load.
func (m *MetadataEditor) Load(initial map[string]any) {
	m.Scopes = extractMetadataScopes(initial)
	m.Buffer = metadataToInput(stripMetadataScopes(initial))
	m.rows = metadataEditorRowsFromMap(stripMetadataScopes(initial))
	m.selected = map[int]bool{}
	m.entryMode = false
	m.entryBuf = ""
	m.entryEditIdx = -1
	m.inspectMode = false
	m.inspectRowIdx = 0
	m.inspectOffset = 0
	m.notice = ""
	m.syncList()
}

// HandleKey handles handle key.
func (m *MetadataEditor) HandleKey(msg tea.KeyMsg) bool {
	if m.scopeSelecting {
		options := m.scopeOptions
		if len(options) == 0 {
			options = append([]string{}, m.Scopes...)
		}
		switch {
		case isKey(msg, "left"):
			if len(options) > 0 {
				m.scopeIdx = (m.scopeIdx - 1 + len(options)) % len(options)
			}
			return false
		case isKey(msg, "right"):
			if len(options) > 0 {
				m.scopeIdx = (m.scopeIdx + 1) % len(options)
			}
			return false
		case isSpace(msg):
			if len(options) > 0 {
				scope := options[m.scopeIdx]
				m.Scopes = toggleScope(m.Scopes, scope)
			}
			return false
		case isEnter(msg), isBack(msg):
			m.scopeSelecting = false
			return false
		}
	}
	if m.entryMode {
		switch {
		case isBack(msg):
			m.entryMode = false
			m.entryBuf = ""
			m.entryEditIdx = -1
		case isKey(msg, "backspace", "delete"):
			m.entryBuf = dropLastRune(m.entryBuf)
		case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
			m.entryBuf = ""
		case isEnter(msg):
			if err := m.commitEntry(); err != nil {
				m.notice = err.Error()
			}
		default:
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				m.entryBuf += ch
			}
		}
		return false
	}
	if m.inspectMode {
		switch {
		case isBack(msg):
			m.inspectMode = false
			m.inspectOffset = 0
		case isDown(msg):
			m.moveInspect(1)
		case isUp(msg):
			m.moveInspect(-1)
		case isEnter(msg):
			value := m.inspectValue()
			if strings.TrimSpace(value) == "" {
				value = "None"
			}
			if err := copyMetadataEditorClipboard(value); err != nil {
				m.notice = err.Error()
			} else {
				m.notice = "copied value."
			}
		}
		return false
	}
	m.syncList()
	switch {
	case isBack(msg):
		m.Active = false
		return true
	case isKey(msg, "s"):
		m.scopeSelecting = true
		return false
	case isDown(msg):
		if m.list != nil {
			m.list.Down()
		}
	case isUp(msg):
		if m.list != nil {
			m.list.Up()
		}
	case isSpace(msg):
		idx := m.selectedRowIndex()
		if idx >= 0 {
			m.toggleSelection(idx)
		}
	case isKey(msg, "b"):
		m.toggleSelectAll()
	case isKey(msg, "n"):
		m.entryMode = true
		m.entryBuf = ""
		m.entryEditIdx = -1
		m.notice = ""
	case isKey(msg, "e"):
		idx := m.selectedRowIndex()
		if idx >= 0 && idx < len(m.rows) {
			m.entryMode = true
			m.entryEditIdx = idx
			m.entryBuf = fmt.Sprintf("%s | %s", m.rows[idx].path, m.rows[idx].value)
			m.notice = ""
		}
	case isKey(msg, "d"):
		idx := m.selectedRowIndex()
		if idx >= 0 && idx < len(m.rows) {
			m.rows = append(m.rows[:idx], m.rows[idx+1:]...)
			m.rebuildBuffer()
			m.selected = map[int]bool{}
			m.syncList()
			m.notice = "row removed."
		}
	case isKey(msg, "c"):
		count, err := m.copySelectedValues()
		if err != nil {
			m.notice = err.Error()
		} else if count > 0 {
			m.notice = fmt.Sprintf("copied %d value(s).", count)
		}
	case isSpace(msg):
		// no-op; handled above
	case isEnter(msg):
		if idx := m.selectedRowIndex(); idx >= 0 {
			m.inspectMode = true
			m.inspectRowIdx = idx
			m.inspectOffset = 0
			m.notice = ""
		}
	}
	return false
}

// Render renders render.
func (m MetadataEditor) Render(width int) string {
	if m.entryMode {
		return m.renderEntryMode(width)
	}
	if m.inspectMode {
		return m.renderInspectMode(width)
	}
	return m.renderTableMode(width)
}

// renderTableMode renders render table mode.
func (m MetadataEditor) renderTableMode(width int) string {
	contentWidth := components.BoxContentWidth(width) - 4
	if contentWidth < 44 {
		contentWidth = 44
	}
	rows := m.rows
	if len(rows) == 0 {
		body := components.TitledBox("Metadata", MutedStyle.Render("No metadata rows. Press n to add one."), width)
		scopeBox := m.renderScopeBox(width)
		footer := MutedStyle.Render("n new · e edit · d delete · space select · b all · enter inspect · c copy values · s scopes · esc back")
		if m.notice != "" {
			footer += "\n" + MutedStyle.Render(m.notice)
		}
		return components.Indent(body+"\n\n"+scopeBox+"\n\n"+footer, 1)
	}

	if m.list == nil {
		list := components.NewList(metadataPanelPageSize(false))
		syncMetadataList(list, m.toDisplayRows(), metadataPanelPageSize(false))
		m.list = list
	}
	visible := m.list.Visible()
	selectedCount := 0
	for idx, selected := range m.selected {
		if !selected {
			continue
		}
		if idx < 0 || idx >= len(rows) {
			continue
		}
		selectedCount++
	}
	showSelectionColumn := selectedCount > 0
	columnBudget := contentWidth
	if showSelectionColumn {
		columnBudget -= 5
	}
	groupWidth, fieldWidth, valueWidth := metadataColumnWidths(columnBudget)
	columns := make([]components.TableColumn, 0, 4)
	if showSelectionColumn {
		columns = append(columns, components.TableColumn{Header: "Sel", Width: 4, Align: lipgloss.Left})
	}
	columns = append(columns,
		components.TableColumn{Header: "Group", Width: groupWidth, Align: lipgloss.Left},
		components.TableColumn{Header: "Field", Width: fieldWidth, Align: lipgloss.Left},
		components.TableColumn{Header: "Value", Width: valueWidth, Align: lipgloss.Left},
	)

	gridRows := make([][]string, 0, len(visible))
	activeRow := -1
	for relIdx := range visible {
		absIdx := m.list.RelToAbs(relIdx)
		if absIdx < 0 || absIdx >= len(rows) {
			continue
		}
		row := rows[absIdx]
		group, field := metadataGroupAndField(row.path)
		if showSelectionColumn && m.list.IsSelected(absIdx) {
			activeRow = len(gridRows)
		}
		cells := make([]string, 0, 4)
		if showSelectionColumn {
			mark := "[ ]"
			if m.selected != nil && m.selected[absIdx] {
				mark = "[X]"
			}
			cells = append(cells, mark)
		}
		cells = append(cells, group, field, row.value)
		gridRows = append(gridRows, cells)
	}
	table := components.TableGridWithActiveRow(columns, gridRows, contentWidth, activeRow)
	table = colorizeScopeBadges(table)

	start := m.list.Offset + 1
	end := m.list.Offset + len(gridRows)
	if start < 1 {
		start = 1
	}
	info := fmt.Sprintf("Rows %d-%d of %d", start, end, len(rows))
	if selectedCount > 0 {
		info += fmt.Sprintf(" · selected %d", selectedCount)
	}
	footer := "↑/↓ navigate · n new · e edit · d delete · space select · b all · enter inspect · c copy values · s scopes · esc back"
	if m.notice != "" {
		footer += "\n" + MutedStyle.Render(m.notice)
	}
	body := table + "\n\n" + MutedStyle.Render(info) + "\n" + MutedStyle.Render(footer)
	return components.Indent(components.TitledBox("Metadata", body, width)+"\n\n"+m.renderScopeBox(width), 1)
}

// renderEntryMode renders render entry mode.
func (m MetadataEditor) renderEntryMode(width int) string {
	title := "Add Metadata Row"
	if m.entryEditIdx >= 0 {
		title = "Edit Metadata Row"
	}
	hint := MutedStyle.Render("format: group | field | value (or path | value)\nexample: profile | timezone | europe/warsaw\nenter save · esc cancel")
	body := components.InputDialog(title, m.entryBuf) + "\n\n" + hint
	if strings.TrimSpace(m.notice) != "" {
		body += "\n" + ErrorStyle.Render(m.notice)
	}
	return components.Indent(body, 1)
}

// renderInspectMode renders render inspect mode.
func (m MetadataEditor) renderInspectMode(width int) string {
	lines := m.inspectLines()
	if len(lines) == 0 {
		return components.Indent(components.TitledBox("Metadata Value", MutedStyle.Render("No value"), width), 1)
	}
	page := m.inspectPageSize()
	start := m.inspectOffset
	if start < 0 {
		start = 0
	}
	if start > len(lines) {
		start = len(lines)
	}
	end := start + page
	if end > len(lines) {
		end = len(lines)
	}
	visible := append([]string{}, lines[start:end]...)
	if start > 0 && len(visible) > 0 {
		visible[0] = MutedStyle.Render("... ↑ more")
	}
	if end < len(lines) && len(visible) > 0 {
		visible[len(visible)-1] = MutedStyle.Render("... ↓ more")
	}
	info := MutedStyle.Render(fmt.Sprintf("Lines %d-%d of %d", start+1, end, len(lines)))
	hints := MutedStyle.Render("↑/↓ scroll · enter copy value · esc back")
	body := strings.Join(visible, "\n") + "\n\n" + info + "\n" + hints
	if m.notice != "" {
		body += "\n" + MutedStyle.Render(m.notice)
	}
	return components.Indent(components.TitledBox("Metadata Value", colorizeScopeBadges(body), width), 1)
}

// renderScopeBox renders render scope box.
func (m MetadataEditor) renderScopeBox(width int) string {
	var content strings.Builder
	content.WriteString(MutedStyle.Render("Scopes:"))
	content.WriteString("\n  ")
	if m.scopeSelecting {
		content.WriteString(renderScopeOptions(m.Scopes, m.scopeOptions, m.scopeIdx))
	} else {
		content.WriteString(renderScopePills(m.Scopes, true))
	}
	return components.TitledBox("Scopes", content.String(), width)
}

// SetScopeOptions sets set scope options.
func (m *MetadataEditor) SetScopeOptions(options []string) {
	m.scopeOptions = options
	if len(m.scopeOptions) == 0 {
		m.scopeIdx = 0
		return
	}
	if m.scopeIdx >= len(m.scopeOptions) {
		m.scopeIdx = 0
	}
}

// dropLastRune handles drop last rune.
func dropLastRune(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	return string(runes[:len(runes)-1])
}

// syncList handles sync list.
func (m *MetadataEditor) syncList() {
	if m.list == nil {
		m.list = components.NewList(metadataPanelPageSize(false))
	}
	if m.selected == nil {
		m.selected = map[int]bool{}
	}
	rows := m.toDisplayRows()
	syncMetadataList(m.list, rows, metadataPanelPageSize(false))
	for idx := range m.selected {
		if idx < 0 || idx >= len(rows) {
			delete(m.selected, idx)
		}
	}
}

// toDisplayRows handles to display rows.
func (m MetadataEditor) toDisplayRows() []metadataDisplayRow {
	rows := make([]metadataDisplayRow, 0, len(m.rows))
	for _, row := range m.rows {
		rows = append(rows, metadataDisplayRow{field: row.path, value: row.value})
	}
	return rows
}

// selectedRowIndex handles selected row index.
func (m MetadataEditor) selectedRowIndex() int {
	if m.list == nil {
		return -1
	}
	idx := m.list.Selected()
	if idx < 0 || idx >= len(m.rows) {
		return -1
	}
	return idx
}

// toggleSelection handles toggle selection.
func (m *MetadataEditor) toggleSelection(idx int) {
	if idx < 0 || idx >= len(m.rows) {
		return
	}
	if m.selected == nil {
		m.selected = map[int]bool{}
	}
	if m.selected[idx] {
		delete(m.selected, idx)
		return
	}
	m.selected[idx] = true
}

// toggleSelectAll handles toggle select all.
func (m *MetadataEditor) toggleSelectAll() {
	if len(m.rows) == 0 {
		return
	}
	if len(m.selected) == len(m.rows) {
		m.selected = map[int]bool{}
		return
	}
	all := make(map[int]bool, len(m.rows))
	for i := range m.rows {
		all[i] = true
	}
	m.selected = all
}

// commitEntry handles commit entry.
func (m *MetadataEditor) commitEntry() error {
	path, value, err := parseMetadataPipeLine(m.entryBuf, 1)
	if err != nil {
		return err
	}
	entry := metadataEditorRow{path: strings.TrimSpace(path), value: strings.TrimSpace(value)}
	if entry.path == "" {
		return fmt.Errorf("path is required")
	}
	if m.entryEditIdx >= 0 && m.entryEditIdx < len(m.rows) {
		m.rows[m.entryEditIdx] = entry
	} else {
		m.rows = append(m.rows, entry)
	}

	// Keep the most recent value for duplicate paths.
	seen := make(map[string]int, len(m.rows))
	cleaned := make([]metadataEditorRow, 0, len(m.rows))
	for _, row := range m.rows {
		pathKey := strings.ToLower(strings.TrimSpace(row.path))
		if prev, ok := seen[pathKey]; ok {
			cleaned[prev] = row
			continue
		}
		seen[pathKey] = len(cleaned)
		cleaned = append(cleaned, row)
	}
	m.rows = cleaned
	m.rebuildBuffer()
	m.entryMode = false
	m.entryBuf = ""
	m.entryEditIdx = -1
	m.notice = "row saved."
	m.syncList()
	return nil
}

// rebuildBuffer handles rebuild buffer.
func (m *MetadataEditor) rebuildBuffer() {
	if len(m.rows) == 0 {
		m.Buffer = ""
		return
	}
	root := map[string]any{}
	for _, row := range m.rows {
		val, err := parseMetadataValue(row.value, 1)
		if err != nil {
			val = row.value
		}
		_ = setMetadataPath(root, row.path, val, 1)
	}
	m.Buffer = metadataToInput(root)
}

// copySelectedValues handles copy selected values.
func (m MetadataEditor) copySelectedValues() (int, error) {
	if len(m.rows) == 0 {
		return 0, nil
	}
	indices := make([]int, 0, len(m.selected))
	for idx, selected := range m.selected {
		if selected {
			indices = append(indices, idx)
		}
	}
	sort.Ints(indices)
	if len(indices) == 0 {
		if idx := m.selectedRowIndex(); idx >= 0 {
			indices = append(indices, idx)
		}
	}
	if len(indices) == 0 {
		return 0, nil
	}
	values := make([]string, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(m.rows) {
			continue
		}
		value := strings.TrimSpace(m.rows[idx].value)
		if value == "" {
			value = "None"
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return 0, nil
	}
	if err := copyMetadataEditorClipboard(strings.Join(values, "\n")); err != nil {
		return 0, err
	}
	return len(values), nil
}

// moveInspect handles move inspect.
func (m *MetadataEditor) moveInspect(delta int) {
	lines := m.inspectLines()
	page := m.inspectPageSize()
	maxOffset := len(lines) - page
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.inspectOffset += delta
	if m.inspectOffset < 0 {
		m.inspectOffset = 0
	}
	if m.inspectOffset > maxOffset {
		m.inspectOffset = maxOffset
	}
}

// inspectPageSize handles inspect page size.
func (m MetadataEditor) inspectPageSize() int {
	page := 12
	if page < 6 {
		page = 6
	}
	if page > 20 {
		page = 20
	}
	return page
}

// inspectValue handles inspect value.
func (m MetadataEditor) inspectValue() string {
	if m.inspectRowIdx < 0 || m.inspectRowIdx >= len(m.rows) {
		return ""
	}
	return m.rows[m.inspectRowIdx].value
}

// inspectLines handles inspect lines.
func (m MetadataEditor) inspectLines() []string {
	if m.inspectRowIdx < 0 || m.inspectRowIdx >= len(m.rows) {
		return nil
	}
	row := m.rows[m.inspectRowIdx]
	group, field := metadataGroupAndField(row.path)
	width := 80
	lines := []string{
		renderPreviewRow("Group", group, width),
		renderPreviewRow("Field", field, width),
		"",
	}
	value := strings.TrimSpace(row.value)
	if value == "" {
		value = "None"
	}
	raw := strings.Split(value, "\n")
	for _, line := range raw {
		wrapped := wrapMetadataDisplayLine(line, width)
		if len(wrapped) == 0 {
			lines = append(lines, "")
			continue
		}
		lines = append(lines, wrapped...)
	}
	return lines
}

// metadataEditorRowsFromMap handles metadata editor rows from map.
func metadataEditorRowsFromMap(data map[string]any) []metadataEditorRow {
	if len(data) == 0 {
		return nil
	}
	display := metadataDisplayRows(data)
	rows := make([]metadataEditorRow, 0, len(display))
	for _, row := range display {
		rows = append(rows, metadataEditorRow{
			path:  row.field,
			value: row.value,
		})
	}
	return rows
}

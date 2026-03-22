package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type filesLoadedMsg struct{ items []api.File }
type fileCreatedMsg struct{}
type fileUpdatedMsg struct{}
type filesScopesLoadedMsg struct{ options []string }
type fileRelationshipsLoadedMsg struct {
	id            string
	relationships []api.Relationship
}

// --- Views ---

type filesView int

const (
	filesViewAdd filesView = iota
	filesViewList
	filesViewDetail
	filesViewEdit
)

const (
	fileFieldName = iota
	fileFieldPath
	fileFieldMime
	fileFieldSize
	fileFieldChecksum
	fileFieldStatus
	fileFieldTags
	fileFieldMeta
	fileFieldCount
)

var fileStatusOptions = []string{"active", "inactive"}

// --- Files Model ---

type FilesModel struct {
	client        *api.Client
	items         []api.File
	all           []api.File
	dataTable     table.Model
	loading       bool
	spinner       spinner.Model
	view          filesView
	modeFocus     bool
	filtering     bool
	searchBuf     string
	searchSuggest string
	detail        *api.File
	detailRels    []api.Relationship
	errText       string
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
	addName      string
	addPath      string
	addMime      string
	addSize      string
	addChecksum  string
	addMeta      MetadataEditor
	addSaving    bool
	addSaved     bool
	addErr       string

	// edit
	editFocus     int
	editStatusIdx int
	editTags      []string
	editTagBuf    string
	editName      string
	editPath      string
	editMime      string
	editSize      string
	editChecksum  string
	editMeta      MetadataEditor
	editSaving    bool
}

// NewFilesModel builds the files UI model.
func NewFilesModel(client *api.Client) FilesModel {
	return FilesModel{
		client:    client,
		spinner:   components.NewNebulaSpinner(),
		dataTable: components.NewNebulaTable(nil, 12),
		view:      filesViewList,
		addFields: []formField{
			{label: "Filename"},
			{label: "File Path"},
			{label: "MIME Type"},
			{label: "Size (bytes)"},
			{label: "Checksum"},
			{label: "Status"},
			{label: "Tags"},
			{label: "Metadata"},
		},
	}
}

// Init handles init.
func (m FilesModel) Init() tea.Cmd {
	m.loading = true
	m.view = filesViewList
	m.modeFocus = false
	m.filtering = false
	m.searchBuf = ""
	m.searchSuggest = ""
	m.detail = nil
	m.detailRels = nil
	m.errText = ""
	m.metaExpanded = false
	m.addFocus = 0
	m.addStatusIdx = statusIndex(fileStatusOptions, "active")
	m.addTags = nil
	m.addTagBuf = ""
	m.addName = ""
	m.addPath = ""
	m.addMime = ""
	m.addSize = ""
	m.addChecksum = ""
	m.addMeta.Reset()
	m.addSaving = false
	m.addSaved = false
	m.addErr = ""
	m.editFocus = 0
	m.editStatusIdx = statusIndex(fileStatusOptions, "active")
	m.editTags = nil
	m.editTagBuf = ""
	m.editName = ""
	m.editPath = ""
	m.editMime = ""
	m.editSize = ""
	m.editChecksum = ""
	m.editMeta.Reset()
	m.editSaving = false
	return tea.Batch(m.loadFiles(), m.spinner.Tick)
}

// Update updates update.
func (m FilesModel) Update(msg tea.Msg) (FilesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case filesLoadedMsg:
		m.loading = false
		m.all = msg.items
		m.applyFileSearch()
		return m, m.loadScopeOptions()
	case filesScopesLoadedMsg:
		m.scopeOptions = msg.options
		m.addMeta.SetScopeOptions(m.scopeOptions)
		m.editMeta.SetScopeOptions(m.scopeOptions)
		return m, nil
	case fileRelationshipsLoadedMsg:
		if m.detail != nil && m.detail.ID == msg.id {
			m.detailRels = msg.relationships
		}
		return m, nil
	case fileCreatedMsg:
		m.addSaving = false
		m.addSaved = true
		m.loading = true
		return m, tea.Batch(m.loadFiles(), m.spinner.Tick)
	case fileUpdatedMsg:
		m.editSaving = false
		m.detail = nil
		m.view = filesViewList
		m.loading = true
		return m, tea.Batch(m.loadFiles(), m.spinner.Tick)
	case errMsg:
		m.loading = false
		m.addSaving = false
		m.editSaving = false
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyPressMsg:
		if m.addMeta.Active {
			m.addMeta.HandleKey(msg)
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
		case filesViewAdd:
			return m.handleAddKeys(msg)
		case filesViewEdit:
			return m.handleEditKeys(msg)
		case filesViewDetail:
			return m.handleDetailKeys(msg)
		default:
			return m.handleListKeys(msg)
		}
	}
	return m, nil
}

// View handles view.
func (m FilesModel) View() string {
	if m.addMeta.Active {
		return m.addMeta.Render(m.width)
	}
	if m.editMeta.Active {
		return m.editMeta.Render(m.width)
	}
	if m.filtering && m.view == filesViewList {
		return components.Indent(components.InputDialog("Filter Files", m.searchBuf), 1)
	}
	modeLine := m.renderModeLine()
	var body string
	switch m.view {
	case filesViewAdd:
		body = m.renderAdd()
	case filesViewEdit:
		body = m.renderEdit()
	case filesViewDetail:
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

func (m FilesModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("Library")
	if m.view == filesViewAdd {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("Library")
	}
	if m.modeFocus {
		if m.view == filesViewAdd {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("Library")
		}
	}
	return add + " " + list
}

// handleModeKeys handles handle mode keys.
func (m FilesModel) handleModeKeys(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
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
func (m FilesModel) toggleMode() (FilesModel, tea.Cmd) {
	m.modeFocus = false
	if m.view == filesViewAdd {
		m.view = filesViewList
		return m, nil
	}
	m.view = filesViewAdd
	m.addSaved = false
	return m, nil
}

// --- List View ---

func (m FilesModel) renderList() string {
	if m.loading {
		return "  " + m.spinner.View() + " " + MutedStyle.Render("Loading files...")
	}
	if len(m.items) == 0 {
		return components.EmptyStateBox(
			"Files",
			"No files found.",
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
	sizeWidth := 10
	atWidth := compactTimeColumnWidth
	fileWidth := availableCols - (statusWidth + sizeWidth + atWidth)
	if fileWidth < 12 {
		fileWidth = 12
	}

	tableRows := make([]table.Row, len(m.items))
	for i, f := range m.items {
		status := strings.TrimSpace(components.SanitizeOneLine(f.Status))
		if status == "" {
			status = "-"
		}
		size := "-"
		if f.SizeBytes != nil {
			size = formatFileSize(*f.SizeBytes)
		}
		at := f.UpdatedAt
		if at.IsZero() {
			at = f.CreatedAt
		}
		tableRows[i] = table.Row{
			components.ClampTextWidthEllipsis(components.SanitizeOneLine(f.Filename), fileWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			components.ClampTextWidthEllipsis(size, sizeWidth),
			formatLocalTimeCompact(at),
		}
	}

	m.dataTable.SetColumns([]table.Column{
		{Title: "File", Width: fileWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "Size", Width: sizeWidth},
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
	var previewItem *api.File
	if !m.modeFocus {
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			previewItem = &m.items[idx]
		}
	}
	if previewItem != nil {
		content := m.renderFilePreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.TitledBox("Files", content, m.width)
}

// renderFilePreview renders render file preview.
func (m FilesModel) renderFilePreview(f api.File, width int) string {
	if width <= 0 {
		return ""
	}

	name := components.SanitizeOneLine(f.Filename)
	if strings.TrimSpace(name) == "" {
		name = "file"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(f.Status))
	if status == "" {
		status = "-"
	}
	size := "-"
	if f.SizeBytes != nil {
		size = formatFileSize(*f.SizeBytes)
	}
	at := f.UpdatedAt
	if at.IsZero() {
		at = f.CreatedAt
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(name, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("Size", size, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeCompact(at), width))
	if strings.TrimSpace(f.FilePath) != "" {
		lines = append(lines, renderPreviewRow("Path", f.FilePath, width))
	}
	if f.MimeType != nil && strings.TrimSpace(*f.MimeType) != "" {
		lines = append(lines, renderPreviewRow("MIME", strings.TrimSpace(*f.MimeType), width))
	}
	if f.Checksum != nil && strings.TrimSpace(*f.Checksum) != "" {
		lines = append(lines, renderPreviewRow("SHA", strings.TrimSpace(*f.Checksum), width))
	}
	if len(f.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(f.Tags, ", "), width))
	}
	if metaPreview := metadataPreview(map[string]any(f.Metadata), 80); metaPreview != "" {
		lines = append(lines, renderPreviewRow("Preview", metaPreview, width))
	}

	return padPreviewLines(lines, width)
}

// handleListKeys handles handle list keys.
func (m FilesModel) handleListKeys(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
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
			m.view = filesViewDetail
			return m, m.loadDetailRelationships(item.ID)
		}
	case isKey(msg, "f"):
		m.filtering = true
		return m, nil
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applyFileSearch()
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.applyFileSearch()
		}
	case isBack(msg):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.applyFileSearch()
		}
	case isKey(msg, "tab"):
		if m.searchSuggest != "" && !strings.EqualFold(strings.TrimSpace(m.searchBuf), strings.TrimSpace(m.searchSuggest)) {
			m.searchBuf = m.searchSuggest
			m.applyFileSearch()
		}
	default:
		ch := keyText(msg)
		if ch != "" {
			m.searchBuf += ch
			m.applyFileSearch()
		}
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m FilesModel) handleFilterInput(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
	case isBack(msg):
		m.filtering = false
		m.searchBuf = ""
		m.searchSuggest = ""
		m.applyFileSearch()
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applyFileSearch()
		}
	default:
		ch := keyText(msg)
		if ch != "" {
			if ch == " " && m.searchBuf == "" {
				return m, nil
			}
			m.searchBuf += ch
			m.applyFileSearch()
		}
	}
	return m, nil
}

// --- Detail View ---

func (m FilesModel) handleDetailKeys(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
	switch {
	case isUp(msg):
		m.modeFocus = true
	case isBack(msg):
		m.detail = nil
		m.detailRels = nil
		m.metaExpanded = false
		m.view = filesViewList
	case isKey(msg, "e"):
		m.startEdit()
		m.view = filesViewEdit
	case isKey(msg, "m"):
		m.metaExpanded = !m.metaExpanded
	}
	return m, nil
}

// renderDetail renders render detail.
func (m FilesModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}
	f := m.detail
	rows := []components.TableRow{
		{Label: "ID", Value: f.ID},
		{Label: "Filename", Value: f.Filename},
		{Label: "Path", Value: f.FilePath},
	}
	if f.MimeType != nil && *f.MimeType != "" {
		rows = append(rows, components.TableRow{Label: "MIME", Value: *f.MimeType})
	}
	if f.SizeBytes != nil {
		rows = append(rows, components.TableRow{Label: "Size", Value: formatFileSize(*f.SizeBytes)})
	}
	if f.Checksum != nil && *f.Checksum != "" {
		rows = append(rows, components.TableRow{Label: "Checksum", Value: *f.Checksum})
	}
	if f.Status != "" {
		rows = append(rows, components.TableRow{Label: "Status", Value: f.Status})
	}
	if len(f.Tags) > 0 {
		rows = append(rows, components.TableRow{Label: "Tags", Value: strings.Join(f.Tags, ", ")})
	}
	rows = append(rows, components.TableRow{Label: "Created", Value: formatLocalTimeFull(f.CreatedAt)})
	if !f.UpdatedAt.IsZero() {
		rows = append(rows, components.TableRow{Label: "Updated", Value: formatLocalTimeFull(f.UpdatedAt)})
	}

	sections := []string{components.Table("File", rows, m.width)}
	if len(f.Metadata) > 0 {
		sections = append(sections, renderMetadataBlock(map[string]any(f.Metadata), m.width, m.metaExpanded))
	}
	if len(m.detailRels) > 0 {
		sections = append(sections, renderRelationshipSummaryTable("file", f.ID, m.detailRels, 6, m.width))
	}
	return strings.Join(sections, "\n\n")
}

// loadDetailRelationships loads load detail relationships.
func (m FilesModel) loadDetailRelationships(fileID string) tea.Cmd {
	return func() tea.Msg {
		rels, err := m.client.GetRelationships("file", fileID)
		if err != nil {
			return fileRelationshipsLoadedMsg{id: fileID, relationships: nil}
		}
		return fileRelationshipsLoadedMsg{id: fileID, relationships: rels}
	}
}

// --- Add View ---

func (m FilesModel) handleAddKeys(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
	if m.addSaving {
		return m, nil
	}
	if m.addSaved {
		if isBack(msg) {
			m.resetAddForm()
		}
		return m, nil
	}
	if m.addMeta.Active {
		if m.addMeta.HandleKey(msg) {
			m.addMeta.Active = false
		}
		return m, nil
	}
	if m.modeFocus {
		return m.handleModeKeys(msg)
	}
	if m.addFocus == fileFieldStatus {
		switch {
		case isKey(msg, "left"):
			m.addStatusIdx = (m.addStatusIdx - 1 + len(fileStatusOptions)) % len(fileStatusOptions)
			return m, nil
		case isKey(msg, "right"), isSpace(msg):
			m.addStatusIdx = (m.addStatusIdx + 1) % len(fileStatusOptions)
			return m, nil
		}
	}
	switch {
	case isDown(msg):
		m.addFocus = (m.addFocus + 1) % fileFieldCount
	case isUp(msg):
		if m.addFocus == 0 {
			m.modeFocus = true
			return m, nil
		}
		m.addFocus = (m.addFocus - 1 + fileFieldCount) % fileFieldCount
	case isKey(msg, "ctrl+s"):
		return m.saveAdd()
	case isBack(msg):
		m.resetAddForm()
	case isKey(msg, "backspace", "delete"):
		switch m.addFocus {
		case fileFieldTags:
			if len(m.addTagBuf) > 0 {
				m.addTagBuf = m.addTagBuf[:len(m.addTagBuf)-1]
			} else if len(m.addTags) > 0 {
				m.addTags = m.addTags[:len(m.addTags)-1]
			}
		case fileFieldName:
			m.addName = dropLastRune(m.addName)
		case fileFieldPath:
			m.addPath = dropLastRune(m.addPath)
		case fileFieldMime:
			m.addMime = dropLastRune(m.addMime)
		case fileFieldSize:
			m.addSize = dropLastRune(m.addSize)
		case fileFieldChecksum:
			m.addChecksum = dropLastRune(m.addChecksum)
		default:
			return m, nil
		}
	default:
		switch m.addFocus {
		case fileFieldTags:
			switch {
			case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
				m.commitAddTag()
			default:
				ch := keyText(msg)
				if ch != "" && ch != "," {
					m.addTagBuf += ch
				}
			}
		case fileFieldName:
			appendChar(&m.addName, msg)
		case fileFieldPath:
			appendChar(&m.addPath, msg)
		case fileFieldMime:
			appendChar(&m.addMime, msg)
		case fileFieldSize:
			appendChar(&m.addSize, msg)
		case fileFieldChecksum:
			appendChar(&m.addChecksum, msg)
		case fileFieldMeta:
			if isEnter(msg) {
				m.addMeta.Active = true
			}
		}
	}
	return m, nil
}

// renderAdd renders render add.
func (m FilesModel) renderAdd() string {
	rows := make([][2]string, 0, len(m.addFields))
	for i := range m.addFields {
		label := m.addFields[i].label
		value := "-"
		switch i {
		case fileFieldName:
			value = formatFormValue(m.addName, i == m.addFocus)
		case fileFieldPath:
			value = formatFormValue(m.addPath, i == m.addFocus)
		case fileFieldMime:
			value = formatFormValue(m.addMime, i == m.addFocus)
		case fileFieldSize:
			value = formatFormValue(m.addSize, i == m.addFocus)
		case fileFieldChecksum:
			value = formatFormValue(m.addChecksum, i == m.addFocus)
		case fileFieldStatus:
			value = fileStatusOptions[m.addStatusIdx]
		case fileFieldTags:
			value = m.renderAddTags(i == m.addFocus)
		case fileFieldMeta:
			value = renderMetadataEditorPreview(m.addMeta.Buffer, m.addMeta.Scopes, m.width, 6)
		}
		rows = append(rows, [2]string{label, value})
	}
	body := renderFormGrid("Add File", rows, m.addFocus, m.width)
	if m.addErr != "" {
		body += "\n\n" + ErrorStyle.Render(m.addErr)
	}
	if m.addSaved {
		body += "\n\n" + SuccessStyle.Render("Saved.")
	}
	return body
}

// saveAdd handles save add.
func (m FilesModel) saveAdd() (FilesModel, tea.Cmd) {
	name := strings.TrimSpace(m.addName)
	if name == "" {
		m.addErr = "Filename is required"
		return m, nil
	}
	path := strings.TrimSpace(m.addPath)
	if path == "" {
		m.addErr = "File path is required"
		return m, nil
	}
	size, err := parseFileSize(m.addSize)
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

	input := api.CreateFileInput{
		Filename:  name,
		FilePath:  path,
		MimeType:  strings.TrimSpace(m.addMime),
		SizeBytes: size,
		Checksum:  strings.TrimSpace(m.addChecksum),
		Status:    fileStatusOptions[m.addStatusIdx],
		Tags:      m.addTags,
		Metadata:  meta,
	}
	m.addSaving = true
	m.addErr = ""
	return m, func() tea.Msg {
		if _, err := m.client.CreateFile(input); err != nil {
			return errMsg{err}
		}
		return fileCreatedMsg{}
	}
}

// resetAddForm handles reset add form.
func (m *FilesModel) resetAddForm() {
	m.addSaved = false
	m.addSaving = false
	m.addErr = ""
	m.addFocus = 0
	m.addStatusIdx = statusIndex(fileStatusOptions, "active")
	m.addTags = nil
	m.addTagBuf = ""
	m.addName = ""
	m.addPath = ""
	m.addMime = ""
	m.addSize = ""
	m.addChecksum = ""
	m.addMeta.Reset()
}

// commitAddTag handles commit add tag.
func (m *FilesModel) commitAddTag() {
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
func (m FilesModel) renderAddTags(focused bool) string {
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

func (m *FilesModel) startEdit() {
	if m.detail == nil {
		return
	}
	f := m.detail
	m.editFocus = 0
	m.editStatusIdx = statusIndex(fileStatusOptions, f.Status)
	m.editTags = append([]string{}, f.Tags...)
	m.editTagBuf = ""
	m.editName = f.Filename
	m.editPath = f.FilePath
	if f.MimeType != nil {
		m.editMime = *f.MimeType
	} else {
		m.editMime = ""
	}
	if f.SizeBytes != nil {
		m.editSize = fmt.Sprintf("%d", *f.SizeBytes)
	} else {
		m.editSize = ""
	}
	if f.Checksum != nil {
		m.editChecksum = *f.Checksum
	} else {
		m.editChecksum = ""
	}
	m.editMeta.Load(map[string]any(f.Metadata))
	m.editSaving = false
}

// handleEditKeys handles handle edit keys.
func (m FilesModel) handleEditKeys(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if m.editMeta.Active {
		if m.editMeta.HandleKey(msg) {
			m.editMeta.Active = false
		}
		return m, nil
	}
	if m.editFocus == fileFieldStatus {
		switch {
		case isKey(msg, "left"):
			m.editStatusIdx = (m.editStatusIdx - 1 + len(fileStatusOptions)) % len(fileStatusOptions)
			return m, nil
		case isKey(msg, "right"), isSpace(msg):
			m.editStatusIdx = (m.editStatusIdx + 1) % len(fileStatusOptions)
			return m, nil
		}
	}
	switch {
	case isDown(msg):
		m.editFocus = (m.editFocus + 1) % fileFieldCount
	case isUp(msg):
		if m.editFocus > 0 {
			m.editFocus = (m.editFocus - 1 + fileFieldCount) % fileFieldCount
		}
	case isBack(msg):
		m.view = filesViewDetail
	case isKey(msg, "ctrl+s"):
		return m.saveEdit()
	case isKey(msg, "backspace", "delete"):
		switch m.editFocus {
		case fileFieldTags:
			if len(m.editTagBuf) > 0 {
				m.editTagBuf = m.editTagBuf[:len(m.editTagBuf)-1]
			} else if len(m.editTags) > 0 {
				m.editTags = m.editTags[:len(m.editTags)-1]
			}
		case fileFieldName:
			m.editName = dropLastRune(m.editName)
		case fileFieldPath:
			m.editPath = dropLastRune(m.editPath)
		case fileFieldMime:
			m.editMime = dropLastRune(m.editMime)
		case fileFieldSize:
			m.editSize = dropLastRune(m.editSize)
		case fileFieldChecksum:
			m.editChecksum = dropLastRune(m.editChecksum)
		}
	default:
		switch m.editFocus {
		case fileFieldTags:
			switch {
			case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
				m.commitEditTag()
			default:
				ch := keyText(msg)
				if ch != "" && ch != "," {
					m.editTagBuf += ch
				}
			}
		case fileFieldName:
			appendChar(&m.editName, msg)
		case fileFieldPath:
			appendChar(&m.editPath, msg)
		case fileFieldMime:
			appendChar(&m.editMime, msg)
		case fileFieldSize:
			appendChar(&m.editSize, msg)
		case fileFieldChecksum:
			appendChar(&m.editChecksum, msg)
		case fileFieldMeta:
			if isEnter(msg) {
				m.editMeta.Active = true
			}
		}
	}
	return m, nil
}

// renderEdit renders render edit.
func (m FilesModel) renderEdit() string {
	fields := []string{"Filename", "File Path", "MIME Type", "Size (bytes)", "Checksum", "Status", "Tags", "Metadata"}
	rows := make([][2]string, 0, len(fields))
	for i, label := range fields {
		value := "-"
		switch i {
		case fileFieldName:
			value = formatFormValue(m.editName, i == m.editFocus)
		case fileFieldPath:
			value = formatFormValue(m.editPath, i == m.editFocus)
		case fileFieldMime:
			value = formatFormValue(m.editMime, i == m.editFocus)
		case fileFieldSize:
			value = formatFormValue(m.editSize, i == m.editFocus)
		case fileFieldChecksum:
			value = formatFormValue(m.editChecksum, i == m.editFocus)
		case fileFieldStatus:
			value = fileStatusOptions[m.editStatusIdx]
		case fileFieldTags:
			value = m.renderEditTags(i == m.editFocus)
		case fileFieldMeta:
			value = renderMetadataEditorPreview(m.editMeta.Buffer, m.editMeta.Scopes, m.width, 6)
		}
		rows = append(rows, [2]string{label, value})
	}
	return renderFormGrid("Edit File", rows, m.editFocus, m.width)
}

// saveEdit handles save edit.
func (m FilesModel) saveEdit() (FilesModel, tea.Cmd) {
	size, err := parseFileSize(m.editSize)
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

	status := fileStatusOptions[m.editStatusIdx]

	input := api.UpdateFileInput{
		Metadata: meta,
		Status:   &status,
		Tags:     &m.editTags,
	}
	if strings.TrimSpace(m.editName) != "" {
		input.Filename = stringPtr(strings.TrimSpace(m.editName))
	}
	if strings.TrimSpace(m.editPath) != "" {
		input.FilePath = stringPtr(strings.TrimSpace(m.editPath))
	}
	if strings.TrimSpace(m.editMime) != "" {
		input.MimeType = stringPtr(strings.TrimSpace(m.editMime))
	}
	if size != nil {
		input.SizeBytes = size
	}
	if strings.TrimSpace(m.editChecksum) != "" {
		input.Checksum = stringPtr(strings.TrimSpace(m.editChecksum))
	}

	m.editSaving = true
	m.errText = ""
	return m, func() tea.Msg {
		if _, err := m.client.UpdateFile(m.detail.ID, input); err != nil {
			return errMsg{err}
		}
		return fileUpdatedMsg{}
	}
}

// commitEditTag handles commit edit tag.
func (m *FilesModel) commitEditTag() {
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
func (m FilesModel) renderEditTags(focused bool) string {
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

func (m FilesModel) loadFiles() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryFiles(api.QueryParams{"status_category": "active"})
		if err != nil {
			return errMsg{err}
		}
		return filesLoadedMsg{items}
	}
}

// loadScopeOptions loads load scope options.
func (m FilesModel) loadScopeOptions() tea.Cmd {
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
		return filesScopesLoadedMsg{options: scopeNameList(names)}
	}
}

// applyFileSearch handles apply file search.
func (m *FilesModel) applyFileSearch() {
	query := strings.TrimSpace(strings.ToLower(m.searchBuf))
	if query == "" {
		m.items = m.all
	} else {
		filtered := make([]api.File, 0, len(m.all))
		for _, f := range m.all {
			hay := strings.ToLower(strings.Join([]string{f.Filename, f.FilePath, f.ID, derefString(f.MimeType)}, " "))
			if strings.Contains(hay, query) {
				filtered = append(filtered, f)
			}
		}
		m.items = filtered
	}
	rows := make([]table.Row, len(m.items))
	for i, f := range m.items {
		rows[i] = table.Row{formatFileLine(f)}
	}
	m.dataTable.SetRows(rows)
	m.dataTable.SetCursor(0)
	m.updateSearchSuggest()
}

// updateSearchSuggest updates update search suggest.
func (m *FilesModel) updateSearchSuggest() {
	m.searchSuggest = ""
	query := strings.ToLower(strings.TrimSpace(m.searchBuf))
	if query == "" {
		return
	}
	for _, f := range m.all {
		if strings.HasPrefix(strings.ToLower(f.Filename), query) {
			m.searchSuggest = f.Filename
			return
		}
	}
}

// formatFileLine handles format file line.
func formatFileLine(f api.File) string {
	name := components.SanitizeText(f.Filename)
	if name == "" {
		name = "file"
	}
	segments := []string{name}
	if f.MimeType != nil && *f.MimeType != "" {
		segments = append(segments, components.SanitizeText(*f.MimeType))
	}
	if f.SizeBytes != nil {
		segments = append(segments, formatFileSize(*f.SizeBytes))
	}
	if f.Status != "" {
		segments = append(segments, components.SanitizeText(f.Status))
	}
	if preview := metadataPreview(map[string]any(f.Metadata), 40); preview != "" {
		segments = append(segments, preview)
	}
	return strings.Join(segments, " · ")
}

// parseFileSize parses parse file size.
func parseFileSize(raw string) (*int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return nil, fmt.Errorf("size: enter a non-negative integer")
	}
	return &parsed, nil
}

// formatFileSize handles format file size.
func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	kb := float64(size) / 1024
	if kb < 1024 {
		return fmt.Sprintf("%.1f KB", kb)
	}
	mb := kb / 1024
	if mb < 1024 {
		return fmt.Sprintf("%.1f MB", mb)
	}
	gb := mb / 1024
	return fmt.Sprintf("%.1f GB", gb)
}

// appendChar handles append char.
func appendChar(target *string, msg tea.KeyPressMsg) {
	ch := keyText(msg)
	if ch != "" {
		*target += ch
	}
}

// formatFormValue handles format form value.
func formatFormValue(value string, focused bool) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" && !focused {
		return "-"
	}
	if focused {
		return strings.TrimRight(value, "\n") + AccentStyle.Render("█")
	}
	return value
}

// derefString handles deref string.
func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textarea"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type filesLoadedMsg struct{ items []api.File }
type fileCreatedMsg struct{}
type fileUpdatedMsg struct{}
type fileNotesSavedMsg struct{}
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

	// add (huh form)
	addForm     *huh.Form
	addStatus   string
	addTagStr   string
	addName     string
	addPath     string
	addMime     string
	addSize     string
	addChecksum string
	addMeta     MetadataEditor
	addSaving   bool
	addSaved    bool
	addErr      string

	// edit (huh form)
	editForm     *huh.Form
	editStatus   string
	editTagStr   string
	editName     string
	editPath     string
	editMime     string
	editSize     string
	editChecksum string
	editMeta     MetadataEditor
	editSaving   bool

	// inline notes editing (split-pane)
	notesEditing  bool
	notesTextarea textarea.Model
	notesDirty    bool

}

// NewFilesModel builds the files UI model.
func NewFilesModel(client *api.Client) FilesModel {
	return FilesModel{
		client:    client,
		spinner:   components.NewNebulaSpinner(),
		dataTable: components.NewNebulaTable(nil, 12),
		view:      filesViewList,
		addStatus: "active",
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
	m.addStatus = "active"
	m.addTagStr = ""
	m.addForm = nil
	m.addName = ""
	m.addPath = ""
	m.addMime = ""
	m.addSize = ""
	m.addChecksum = ""
	m.addMeta.Reset()
	m.addSaving = false
	m.addSaved = false
	m.addErr = ""
	m.editStatus = "active"
	m.editTagStr = ""
	m.editForm = nil
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
	case fileNotesSavedMsg:
		m.notesEditing = false
		m.notesDirty = false
		m.loading = true
		return m, tea.Batch(m.loadFiles(), m.spinner.Tick)
	case errMsg:
		m.loading = false
		m.addSaving = false
		m.editSaving = false
		m.notesEditing = false
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyPressMsg:
		if m.notesEditing {
			return m.handleNotesEditKeys(msg)
		}
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
	if m.view == filesViewList {
		return lipgloss.JoinVertical(lipgloss.Left, components.Indent(body, 1), m.renderStatusHints())
	}
	return components.Indent(body, 1)
}

// renderStatusHints builds the bottom status bar with keycap pill hints.
func (m FilesModel) renderStatusHints() string {
	if m.notesEditing {
		hints := []string{
			components.Hint("esc", "Cancel"),
			components.Hint("ctrl+s", "Save"),
		}
		return components.StatusBar(hints, m.width)
	}
	hints := []string{
		components.Hint("1-9/0", "Tabs"),
		components.Hint("/", "Command"),
		components.Hint("?", "Help"),
		components.Hint("q", "Quit"),
		components.Hint("\u2191/\u2193", "Scroll"),
		components.Hint("enter", "View"),
		components.Hint("a", "Add"),
		components.Hint("e", "Edit"),
	}
	return components.StatusBar(hints, m.width)
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
	if m.notesEditing {
		m.notesTextarea.SetWidth(previewWidth - 4)
		m.notesTextarea.SetHeight(10)
		preview = m.notesTextarea.View()
	} else {
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
	if f.Notes != "" {
		lines = append(lines, renderPreviewRow("Notes", truncateString(f.Notes, 80), width))
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
	case isKey(msg, "e"):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			item := m.items[idx]
			m.notesEditing = true
			m.notesDirty = false
			m.notesTextarea = components.NewNebulaTextarea(36, 10)
			m.notesTextarea.SetValue(item.Notes)
			m.notesTextarea.Focus()
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

// --- Inline Notes Edit ---

// handleNotesEditKeys routes keys to the textarea when inline notes editing is active.
func (m FilesModel) handleNotesEditKeys(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
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
func (m FilesModel) saveInlineNotes() (FilesModel, tea.Cmd) {
	if idx := m.dataTable.Cursor(); idx < 0 || idx >= len(m.items) {
		m.notesEditing = false
		return m, nil
	}
	item := m.items[m.dataTable.Cursor()]
	notes := m.notesTextarea.Value()
	return m, func() tea.Msg {
		input := api.UpdateFileInput{Notes: notes}
		_, err := m.client.UpdateFile(item.ID, input)
		if err != nil {
			return errMsg{err}
		}
		return fileNotesSavedMsg{}
	}
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
	if f.Notes != "" {
		sections = append(sections, components.TitledBox("Notes", f.Notes, m.width))
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

// initAddForm initializes the huh add form.
func (m *FilesModel) initAddForm() {
	m.addForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Filename").Value(&m.addName),
			huh.NewInput().Title("URI / Path").Value(&m.addPath),
			huh.NewInput().Title("MIME Type").Value(&m.addMime),
			huh.NewInput().Title("Tags").Description("Comma-separated").Value(&m.addTagStr),
			huh.NewSelect[string]().Title("Status").Options(
				huh.NewOption("active", "active"),
				huh.NewOption("inactive", "inactive"),
			).Value(&m.addStatus),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

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
func (m FilesModel) renderAdd() string {
	if m.addSaving {
		return components.Indent(MutedStyle.Render("Saving..."), 1)
	}
	if m.addSaved {
		var b strings.Builder
		b.WriteString(SuccessStyle.Render("File saved!"))
		b.WriteString("\n\n" + MutedStyle.Render("Press Esc to add another."))
		return components.Indent(b.String(), 1)
	}
	if m.addForm == nil {
		return components.Indent(MutedStyle.Render("Initializing..."), 1)
	}
	var b strings.Builder
	b.WriteString(m.addForm.View())
	if m.addMeta.Buffer != "" {
		b.WriteString("\n" + MutedStyle.Render("Notes:") + "\n  " + NormalStyle.Render(m.addMeta.Buffer))
	}
	if m.addErr != "" {
		b.WriteString("\n\n" + ErrorStyle.Render(m.addErr))
	}
	return components.Indent(b.String(), 1)
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
	tags := parseCommaSeparated(m.addTagStr)

	input := api.CreateFileInput{
		Filename:  name,
		FilePath:  path,
		MimeType:  strings.TrimSpace(m.addMime),
		SizeBytes: size,
		Checksum:  strings.TrimSpace(m.addChecksum),
		Status:    m.addStatus,
		Tags:      tags,
		Notes:     m.addMeta.Buffer,
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
	m.addStatus = "active"
	m.addTagStr = ""
	m.addForm = nil
	m.addName = ""
	m.addPath = ""
	m.addMime = ""
	m.addSize = ""
	m.addChecksum = ""
	m.addMeta.Reset()
}

// --- Edit View ---

func (m *FilesModel) startEdit() {
	if m.detail == nil {
		return
	}
	f := m.detail
	m.editStatus = f.Status
	if m.editStatus == "" {
		m.editStatus = "active"
	}
	m.editTagStr = strings.Join(f.Tags, ", ")
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
	m.editMeta.Buffer = f.Notes
	m.editSaving = false
	m.initEditForm()
}

// initEditForm initializes the huh edit form.
func (m *FilesModel) initEditForm() {
	m.editForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Filename").Value(&m.editName),
			huh.NewInput().Title("URI / Path").Value(&m.editPath),
			huh.NewInput().Title("MIME Type").Value(&m.editMime),
			huh.NewInput().Title("Tags").Description("Comma-separated").Value(&m.editTagStr),
			huh.NewSelect[string]().Title("Status").Options(
				huh.NewOption("active", "active"),
				huh.NewOption("inactive", "inactive"),
			).Value(&m.editStatus),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

// handleEditKeys handles handle edit keys.
func (m FilesModel) handleEditKeys(msg tea.KeyPressMsg) (FilesModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if isBack(msg) {
		m.view = filesViewDetail
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
		m.view = filesViewDetail
		return m, nil
	}
	return m, formCmd
}

// renderEdit renders render edit.
func (m FilesModel) renderEdit() string {
	if m.editSaving {
		return components.Indent(MutedStyle.Render("Saving..."), 1)
	}
	if m.editForm == nil {
		return components.Indent(MutedStyle.Render("Initializing..."), 1)
	}
	var b strings.Builder
	b.WriteString(m.editForm.View())
	if m.editMeta.Buffer != "" {
		b.WriteString("\n" + MutedStyle.Render("Notes:") + "\n  " + NormalStyle.Render(m.editMeta.Buffer))
	}
	if m.errText != "" {
		b.WriteString("\n\n" + ErrorStyle.Render(m.errText))
	}
	return components.Indent(b.String(), 1)
}

// saveEdit handles save edit.
func (m FilesModel) saveEdit() (FilesModel, tea.Cmd) {
	size, err := parseFileSize(m.editSize)
	if err != nil {
		m.errText = err.Error()
		return m, nil
	}
	tags := parseCommaSeparated(m.editTagStr)

	input := api.UpdateFileInput{
		Notes:  m.editMeta.Buffer,
		Status: &m.editStatus,
		Tags:   &tags,
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
	if f.Notes != "" {
		segments = append(segments, truncateString(f.Notes, 40))
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

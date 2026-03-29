package ui

import (
	"fmt"
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

type contextSavedMsg struct{}
type contextNotesSavedMsg struct{}
type contextLinkResultsMsg struct{ items []api.Entity }
type contextListLoadedMsg struct{ items []api.Context }
type contextScopesLoadedMsg struct{ names map[string]string }
type contextDetailLoadedMsg struct {
	item          api.Context
	relationships []api.Relationship
}
type contextUpdatedMsg struct{ item api.Context }

// --- Constants ---

// formField is a simple label+value pair used by protocols, files, and logs.
type formField struct {
	label string
	value string
}

var contextTypes = []string{
	"note",
	"video",
	"article",
	"paper",
	"tool",
	"course",
	"thread",
}

type contextView int

const (
	contextViewAdd contextView = iota
	contextViewList
	contextViewDetail
	contextViewEdit
)

// --- Context Model ---

// ContextModel handles adding context items manually.
type ContextModel struct {
	client      *api.Client
	scopeOptions []string
	modeFocus   bool
	saved       bool
	saving      bool
	view        contextView
	errText     string

	// add form (huh)
	addForm    *huh.Form
	addTitle   string
	addURL     string
	addType    string
	addTagStr  string
	addScopeStr string
	addNotes   string

	// edit form (huh)
	editForm    *huh.Form
	editTitle   string
	editURL     string
	editType    string
	editStatus  string
	editTagStr  string
	editScopeStr string
	editNotes   string
	editSaving  bool

	// link search
	linkSearching bool
	linkLoading   bool
	linkQuery     string
	linkResults   []api.Entity
	linkTable     table.Model
	linkEntities  []api.Entity

	// list
	dataTable   table.Model
	allItems    []api.Context
	items       []api.Context
	filtering   bool
	filterBuf   string
	loadingList bool
	spinner     spinner.Model

	// detail
	detail              *api.Context
	detailRelationships []api.Relationship
	contentExpanded     bool
	sourcePathExpanded  bool

	scopeNames map[string]string
	width      int
	height     int

	// inline content editing (split-pane)
	notesEditing  bool
	notesTextarea textarea.Model
	notesDirty    bool

}

// NewContextModel builds the context UI model.
func NewContextModel(client *api.Client) ContextModel {
	return ContextModel{
		client:      client,
		spinner:     components.NewNebulaSpinner(),
		linkTable:   components.NewNebulaTable(nil, 6),
		dataTable:   components.NewNebulaTable(nil, 10),
		view:        contextViewList,
		loadingList: true,
		addType:     "note",
		editType:    "note",
	}
}

// initAddForm creates a new huh form for the add context flow.
func (m *ContextModel) initAddForm() {
	typeOptions := make([]huh.Option[string], len(contextTypes))
	for i, t := range contextTypes {
		typeOptions[i] = huh.NewOption(t, t)
	}
	scopeOptions := make([]huh.Option[string], len(m.scopeOptions))
	for i, s := range m.scopeOptions {
		scopeOptions[i] = huh.NewOption(s, s)
	}
	m.addForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Title").Value(&m.addTitle),
			huh.NewInput().Title("URL").Value(&m.addURL),
			huh.NewSelect[string]().Title("Type").Options(typeOptions...).Value(&m.addType),
			huh.NewInput().Title("Tags (comma-separated)").Value(&m.addTagStr),
			huh.NewInput().Title("Scopes (comma-separated)").Value(&m.addScopeStr),
			huh.NewInput().Title("Notes").Value(&m.addNotes),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

// initEditForm creates a new huh form for the edit context flow.
func (m *ContextModel) initEditForm() {
	typeOptions := make([]huh.Option[string], len(contextTypes))
	for i, t := range contextTypes {
		typeOptions[i] = huh.NewOption(t, t)
	}
	m.editForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Title").Value(&m.editTitle),
			huh.NewInput().Title("URL").Value(&m.editURL),
			huh.NewSelect[string]().Title("Type").Options(typeOptions...).Value(&m.editType),
			huh.NewSelect[string]().Title("Status").Options(
				huh.NewOption("active", "active"),
				huh.NewOption("inactive", "inactive"),
			).Value(&m.editStatus),
			huh.NewInput().Title("Tags (comma-separated)").Value(&m.editTagStr),
			huh.NewInput().Title("Scopes (comma-separated)").Value(&m.editScopeStr),
			huh.NewInput().Title("Notes").Value(&m.editNotes),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

// resetAddForm resets the add form state.
func (m *ContextModel) resetAddForm() {
	m.saved = false
	m.errText = ""
	m.addTitle = ""
	m.addURL = ""
	m.addType = "note"
	m.addTagStr = ""
	m.addScopeStr = ""
	m.addNotes = ""
	m.addForm = nil
	m.linkSearching = false
	m.linkLoading = false
	m.linkQuery = ""
	m.linkResults = nil
	m.linkEntities = nil
	m.linkTable.SetRows(nil)
	m.linkTable.SetCursor(0)
}

// Init handles init.
func (m ContextModel) Init() tea.Cmd {
	m.saved = false
	m.errText = ""
	m.modeFocus = false
	m.view = contextViewList
	m.linkSearching = false
	m.linkLoading = false
	m.linkQuery = ""
	m.linkResults = nil
	m.linkEntities = nil
	m.allItems = nil
	m.filtering = false
	m.filterBuf = ""
	m.detail = nil
	m.loadingList = true
	m.editSaving = false
	m.contentExpanded = false
	m.sourcePathExpanded = false
	if m.scopeNames == nil {
		m.scopeNames = map[string]string{}
	}
	m.linkTable.SetRows(nil)
	m.linkTable.SetCursor(0)
	m.dataTable.SetRows(nil)
	m.dataTable.SetCursor(0)
	m.addTitle = ""
	m.addURL = ""
	m.addType = "note"
	m.addTagStr = ""
	m.addScopeStr = ""
	m.addNotes = ""
	m.addForm = nil
	return tea.Batch(m.loadContextList(), m.loadScopeNames(), m.spinner.Tick)
}

// Update updates update.
func (m ContextModel) Update(msg tea.Msg) (ContextModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case contextSavedMsg:
		m.saving = false
		m.saved = true
		return m, nil

	case contextNotesSavedMsg:
		m.notesEditing = false
		m.notesDirty = false
		m.loadingList = true
		return m, tea.Batch(m.loadContextList(), m.spinner.Tick)

	case errMsg:
		m.saving = false
		m.editSaving = false
		m.notesEditing = false
		m.errText = msg.err.Error()
		return m, nil
	case contextLinkResultsMsg:
		m.linkLoading = false
		m.linkResults = msg.items
		rows := make([]table.Row, len(msg.items))
		for i, e := range msg.items {
			rows[i] = table.Row{formatEntityLine(e)}
		}
		m.linkTable.SetRows(rows)
		m.linkTable.SetCursor(0)
		return m, nil

	case contextListLoadedMsg:
		m.loadingList = false
		m.allItems = append([]api.Context{}, msg.items...)
		m.applyContextFilter()
		return m, nil
	case contextScopesLoadedMsg:
		if m.scopeNames == nil {
			m.scopeNames = map[string]string{}
		}
		for id, name := range msg.names {
			m.scopeNames[id] = name
		}
		m.scopeOptions = scopeNameList(m.scopeNames)
		return m, nil
	case contextDetailLoadedMsg:
		m.detail = &msg.item
		m.detailRelationships = msg.relationships
		return m, nil
	case contextUpdatedMsg:
		m.editSaving = false
		m.detail = &msg.item
		m.view = contextViewDetail
		return m, nil

	case tea.KeyPressMsg:
		if m.notesEditing {
			return m.handleNotesEditKeys(msg)
		}
		if m.view == contextViewList {
			return m.handleListKeys(msg)
		}
		if m.view == contextViewEdit {
			return m.handleEditKeys(msg)
		}
		if m.view == contextViewDetail {
			return m.handleDetailKeys(msg)
		}
		if m.saved {
			if isBack(msg) {
				m.resetAddForm()
				m.initAddForm()
				cmd := m.addForm.Init()
				return m, cmd
			}
			return m, nil
		}
		if m.linkSearching {
			return m.handleLinkSearch(msg)
		}
		if m.modeFocus {
			return m.handleModeKeys(msg)
		}
		return m.handleAddKeys(msg)

	default:
		// Forward non-key messages to active huh forms.
		if m.view == contextViewAdd && m.addForm != nil && !m.saving && !m.saved {
			_, cmd := m.addForm.Update(msg)
			return m, cmd
		}
		if m.view == contextViewEdit && m.editForm != nil && !m.editSaving {
			_, cmd := m.editForm.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// handleAddKeys handles key input in the add view.
func (m ContextModel) handleAddKeys(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
	if m.saving {
		return m, nil
	}

	// Let link search capture keys first if active.
	if m.linkSearching {
		return m.handleLinkSearch(msg)
	}

	if m.addForm == nil {
		m.initAddForm()
		cmd := m.addForm.Init()
		return m, cmd
	}

	_, cmd := m.addForm.Update(msg)

	switch m.addForm.State {
	case huh.StateCompleted:
		return m.save()
	case huh.StateAborted:
		m.resetAddForm()
		m.initAddForm()
		cmd = m.addForm.Init()
		return m, cmd
	}

	return m, cmd
}

// handleEditKeys handles key input in the edit view.
func (m ContextModel) handleEditKeys(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if m.modeFocus {
		return m.handleModeKeys(msg)
	}

	if m.editForm == nil {
		m.initEditForm()
		cmd := m.editForm.Init()
		return m, cmd
	}

	_, cmd := m.editForm.Update(msg)

	switch m.editForm.State {
	case huh.StateCompleted:
		return m.saveEdit()
	case huh.StateAborted:
		m.view = contextViewDetail
		m.editForm = nil
		return m, nil
	}

	return m, cmd
}

// View handles view.
func (m ContextModel) View() string {
	if m.saving {
		return "  " + MutedStyle.Render("Saving...")
	}

	if m.saved {
		return components.Indent(components.RenderCompactBox(SuccessStyle.Render("Context saved! Press Esc to add another.")), 1)
	}

	if m.linkSearching {
		return m.renderLinkSearch()
	}
	if m.filtering && m.view == contextViewList {
		return components.Indent(components.InputDialog("Filter Context", m.filterBuf), 1)
	}

	modeLine := m.renderModeLine()
	var body string
	switch m.view {
	case contextViewAdd:
		body = m.renderAdd()
	case contextViewDetail:
		body = m.renderDetail()
	case contextViewEdit:
		body = m.renderEdit()
	default:
		body = m.renderList()
	}
	if modeLine != "" {
		body = components.CenterLine(modeLine, m.width) + "\n\n" + body
	}
	return components.Indent(body, 1)
}

// Hints returns the hint items for the current view state.
func (m ContextModel) Hints() []components.HintItem {
	if m.filtering || m.linkSearching {
		return nil
	}
	if m.notesEditing {
		return []components.HintItem{
			{Key: "esc", Desc: "Cancel"},
			{Key: "ctrl+s", Desc: "Save"},
		}
	}
	if m.view != contextViewList {
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
		{Key: "l", Desc: "Link"},
	}
}

// renderAdd renders the add context form.
func (m ContextModel) renderAdd() string {
	if m.addForm == nil {
		compactWidth := 60 + 6
		if m.width > 0 && m.width < compactWidth {
			compactWidth = m.width
		}
		return components.TitledBox("Add Context", MutedStyle.Render("  Initializing..."), compactWidth)
	}

	content := m.addForm.View()

	linked := m.renderLinkedEntities()
	if linked != "" {
		content += "\n\n" + MutedStyle.Render("  Entities: ") + NormalStyle.Render(linked)
	}

	if m.errText != "" {
		content += "\n\n" + components.ErrorBox("Error", m.errText, m.width)
	}

	return components.TitledBox("Add Context", content, m.width)
}

// renderEdit renders the edit context form.
func (m ContextModel) renderEdit() string {
	if m.editSaving {
		return components.TitledBox("Edit Context", MutedStyle.Render("  Saving..."), m.width)
	}
	if m.editForm == nil {
		return components.TitledBox("Edit Context", MutedStyle.Render("  Initializing..."), m.width)
	}
	content := m.editForm.View()
	if m.errText != "" {
		content += "\n\n" + components.ErrorBox("Error", m.errText, m.width)
	}
	return components.TitledBox("Edit Context", content, m.width)
}

// renderModeLine renders render mode line.
func (m ContextModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("Library")
	if m.view == contextViewAdd {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("Library")
	}
	if m.modeFocus {
		if m.view == contextViewAdd {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("Library")
		}
	}
	return add + " " + list
}

// handleModeKeys handles handle mode keys.
func (m ContextModel) handleModeKeys(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
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
func (m ContextModel) toggleMode() (ContextModel, tea.Cmd) {
	m.modeFocus = false
	m.detail = nil
	m.contentExpanded = false
	m.sourcePathExpanded = false
	if m.view == contextViewAdd {
		m.view = contextViewList
		m.loadingList = true
		return m, tea.Batch(m.loadContextList(), m.spinner.Tick)
	}
	if m.view == contextViewDetail || m.view == contextViewEdit {
		m.view = contextViewList
		return m, nil
	}
	// list -> add
	m.view = contextViewAdd
	if m.addForm == nil {
		m.initAddForm()
		cmd := m.addForm.Init()
		return m, cmd
	}
	return m, nil
}

// handleListKeys handles handle list keys.
func (m ContextModel) handleListKeys(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
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
	case isEnter(msg):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			item := m.items[idx]
			itemID := strings.TrimSpace(item.ID)
			if itemID == "" {
				return m, func() tea.Msg {
					return errMsg{fmt.Errorf("selected context is missing id")}
				}
			}
			m.detail = &item
			m.view = contextViewDetail
			return m, m.loadContextDetail(itemID)
		}
	case isKey(msg, "e"):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			item := m.items[idx]
			m.notesEditing = true
			m.notesDirty = false
			m.notesTextarea = components.NewNebulaTextarea(36, 10)
			content := ""
			if item.Content != nil {
				content = *item.Content
			}
			m.notesTextarea.SetValue(content)
			m.notesTextarea.Focus()
		}
	case isKey(msg, "f"):
		m.filtering = true
		return m, nil
	case isBack(msg):
		m.view = contextViewAdd
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m ContextModel) handleFilterInput(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
	case isBack(msg):
		m.filtering = false
		m.filterBuf = ""
		m.applyContextFilter()
	case isKey(msg, "backspace", "delete"):
		if len(m.filterBuf) > 0 {
			m.filterBuf = m.filterBuf[:len(m.filterBuf)-1]
			m.applyContextFilter()
		}
	default:
		ch := keyText(msg)
		if ch != "" {
			if ch == " " && m.filterBuf == "" {
				return m, nil
			}
			m.filterBuf += ch
			m.applyContextFilter()
		}
	}
	return m, nil
}

// --- Inline Content Edit ---

// handleNotesEditKeys routes keys to the textarea when inline content editing is active.
func (m ContextModel) handleNotesEditKeys(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
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
func (m ContextModel) saveInlineNotes() (ContextModel, tea.Cmd) {
	if idx := m.dataTable.Cursor(); idx < 0 || idx >= len(m.items) {
		m.notesEditing = false
		return m, nil
	}
	item := m.items[m.dataTable.Cursor()]
	content := m.notesTextarea.Value()
	return m, func() tea.Msg {
		input := api.UpdateContextInput{Content: &content}
		_, err := m.client.UpdateContext(item.ID, input)
		if err != nil {
			return errMsg{err}
		}
		return contextNotesSavedMsg{}
	}
}

// handleDetailKeys handles handle detail keys.
func (m ContextModel) handleDetailKeys(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
	switch {
	case isUp(msg):
		m.modeFocus = true
	case isBack(msg):
		m.detail = nil
		m.detailRelationships = nil
		m.contentExpanded = false
		m.sourcePathExpanded = false
		m.view = contextViewList
	case isKey(msg, "e"):
		m.startEdit()
		m.view = contextViewEdit
	case isKey(msg, "c"):
		m.contentExpanded = !m.contentExpanded
	case isKey(msg, "v"):
		m.sourcePathExpanded = !m.sourcePathExpanded
	}
	return m, nil
}

// renderList renders render list.
func (m ContextModel) renderList() string {
	contentWidth := components.BoxContentWidth(m.width)
	if m.loadingList {
		box := components.RenderCompactBox(m.spinner.View() + " " + MutedStyle.Render("Loading context..."))
		return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, box)
	}

	if len(m.items) == 0 {
		box := components.EmptyStateBox(
			"Context",
			"No context found.",
			[]string{"Press tab to switch Add/Library", "Press / for command palette"},
			m.width,
		)
		return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, box)
	}

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

	typeWidth := 10
	statusWidth := 11
	atWidth := compactTimeColumnWidth
	titleWidth := availableCols - (typeWidth + statusWidth + atWidth)
	if titleWidth < 12 {
		titleWidth = 12
	}
	if titleWidth > 40 {
		titleWidth = 40
	}

	tableRows := make([]table.Row, len(m.items))
	for i, k := range m.items {
		title := components.ClampTextWidthEllipsis(components.SanitizeOneLine(contextTitle(k)), titleWidth)
		typ := strings.TrimSpace(components.SanitizeOneLine(k.SourceType))
		if typ == "" {
			typ = "note"
		}
		status := strings.TrimSpace(components.SanitizeOneLine(k.Status))
		if status == "" {
			status = "-"
		}
		at := k.UpdatedAt
		if at.IsZero() {
			at = k.CreatedAt
		}
		when := formatLocalTimeCompact(at)

		tableRows[i] = table.Row{
			title,
			components.ClampTextWidthEllipsis(typ, typeWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			when,
		}
	}

	m.dataTable.SetColumns([]table.Column{
		{Title: "Title", Width: titleWidth},
		{Title: "Type", Width: typeWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "At", Width: atWidth},
	})
	actualTableWidth := titleWidth + typeWidth + statusWidth + atWidth + cellPadding
	m.dataTable.SetWidth(actualTableWidth)
	m.dataTable.SetRows(tableRows)

	countLine := ""
	if query := strings.TrimSpace(m.filterBuf); query != "" {
		countLine = MutedStyle.Render(fmt.Sprintf("%d total · filter: %s", len(m.items), query))
	}

	tableView := components.TableBaseStyle.Render(m.dataTable.View())
	preview := ""
	if m.notesEditing {
		m.notesTextarea.SetWidth(previewWidth - 4)
		m.notesTextarea.SetHeight(10)
		preview = m.notesTextarea.View()
	} else {
		var previewItem *api.Context
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			previewItem = &m.items[idx]
		}
		if previewItem != nil {
			content := m.renderContextPreview(*previewItem, previewBoxContentWidth(previewWidth))
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

// renderDetail renders render detail.
func (m ContextModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}

	k := m.detail
	infoRows := []components.InfoTableRow{
		{Key: "ID", Value: k.ID},
		{Key: "Title", Value: contextTitle(*k)},
	}
	if k.SourceType != "" {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Type", Value: k.SourceType})
	}
	if k.Status != "" {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Status", Value: k.Status})
	}
	if k.URL != nil && strings.TrimSpace(*k.URL) != "" {
		infoRows = append(infoRows, components.InfoTableRow{Key: "URL", Value: *k.URL})
	}
	if len(k.PrivacyScopeIDs) > 0 {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Scopes", Value: m.formatContextScopes(k.PrivacyScopeIDs)})
	}
	if len(k.Tags) > 0 {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Tags", Value: strings.Join(k.Tags, ", ")})
	}
	infoRows = append(infoRows, components.InfoTableRow{Key: "Created", Value: formatLocalTimeFull(k.CreatedAt)})
	if !k.UpdatedAt.IsZero() {
		infoRows = append(infoRows, components.InfoTableRow{Key: "Updated", Value: formatLocalTimeFull(k.UpdatedAt)})
	}
	if k.SourcePath != nil && strings.TrimSpace(*k.SourcePath) != "" {
		path := *k.SourcePath
		if !m.sourcePathExpanded {
			path = truncateString(path, 60)
		}
		infoRows = append(infoRows, components.InfoTableRow{Key: "Source Path", Value: path})
	}

	sections := []string{components.RenderInfoTable(infoRows, m.width)}
	if k.Content != nil && strings.TrimSpace(*k.Content) != "" {
		content := strings.TrimSpace(components.SanitizeText(*k.Content))
		if !m.contentExpanded {
			content = truncateString(content, 220)
		}
		if m.contentExpanded {
			content = strings.TrimSpace(components.RenderMarkdown(content, m.width-6))
		}
		sections = append(sections, components.TitledBox("Content", content, m.width))
	}
	if len(m.detailRelationships) > 0 {
		sections = append(sections, renderRelationshipSummaryTable("context", k.ID, m.detailRelationships, 6, m.width))
	}

	return strings.Join(sections, "\n\n")
}

// renderContextPreview renders render context preview.
func (m ContextModel) renderContextPreview(k api.Context, width int) string {
	if width <= 0 {
		return ""
	}

	title := components.SanitizeOneLine(contextTitle(k))
	typ := strings.TrimSpace(components.SanitizeOneLine(k.SourceType))
	if typ == "" {
		typ = "note"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(k.Status))
	if status == "" {
		status = "-"
	}
	at := k.UpdatedAt
	if at.IsZero() {
		at = k.CreatedAt
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Type", typ, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeCompact(at), width))

	if k.URL != nil && strings.TrimSpace(*k.URL) != "" {
		lines = append(lines, renderPreviewRow("URL", strings.TrimSpace(*k.URL), width))
	}
	if len(k.PrivacyScopeIDs) > 0 {
		lines = append(lines, renderPreviewRow("Scopes", m.formatContextScopes(k.PrivacyScopeIDs), width))
	}
	if len(k.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(k.Tags, ", "), width))
	}
	if m.detail != nil && m.detail.ID == k.ID && len(m.detailRelationships) > 0 {
		lines = append(
			lines,
			renderPreviewRow("Links", fmt.Sprintf("%d", len(m.detailRelationships)), width),
		)
	}

	snippet := ""
	if k.Content != nil {
		snippet = truncateString(strings.TrimSpace(components.SanitizeText(*k.Content)), 80)
	} else if k.URL != nil {
		snippet = truncateString(strings.TrimSpace(components.SanitizeText(*k.URL)), 80)
	}
	if strings.TrimSpace(snippet) != "" {
		lines = append(lines, renderPreviewRow("Preview", strings.TrimSpace(snippet), width))
	}

	return padPreviewLines(lines, width)
}

// startEdit populates edit form fields from the current detail.
func (m *ContextModel) startEdit() {
	if m.detail == nil {
		return
	}
	k := m.detail
	m.editTitle = contextTitle(*k)
	m.editURL = ""
	if k.URL != nil {
		m.editURL = *k.URL
	}
	m.editNotes = ""
	if k.Content != nil {
		m.editNotes = *k.Content
	}
	m.editType = k.SourceType
	if m.editType == "" {
		m.editType = "note"
	}
	m.editStatus = k.Status
	if m.editStatus == "" {
		m.editStatus = "active"
	}
	m.editTagStr = strings.Join(k.Tags, ", ")
	m.editScopeStr = strings.Join(m.scopeNamesFromIDs(k.PrivacyScopeIDs), ", ")
	m.editSaving = false
	m.editForm = nil
	m.initEditForm()
}

// saveEdit handles save edit.
func (m ContextModel) saveEdit() (ContextModel, tea.Cmd) {
	if m.detail == nil {
		return m, nil
	}

	title := strings.TrimSpace(m.editTitle)
	url := strings.TrimSpace(m.editURL)
	content := strings.TrimSpace(m.editNotes)
	sourceType := m.editType
	if sourceType == "" {
		sourceType = "note"
	}
	status := m.editStatus
	if status == "" {
		status = "active"
	}

	tags := parseCommaSeparated(m.editTagStr)
	for i, t := range tags {
		tags[i] = normalizeTag(t)
	}
	tags = dedup(tags)

	scopes := parseCommaSeparated(m.editScopeStr)
	for i, s := range scopes {
		scopes[i] = normalizeScope(s)
	}
	scopes = normalizeScopeList(scopes)

	input := api.UpdateContextInput{
		Title:      &title,
		URL:        &url,
		SourceType: &sourceType,
		Content:    &content,
		Status:     &status,
		Tags:       &tags,
		Scopes:     &scopes,
	}

	m.editSaving = true
	return m, func() tea.Msg {
		updated, err := m.client.UpdateContext(m.detail.ID, input)
		if err != nil {
			return errMsg{err}
		}
		return contextUpdatedMsg{item: *updated}
	}
}

// --- Helpers ---

func (m ContextModel) loadContextList() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryContext(api.QueryParams{})
		if err != nil {
			return errMsg{err}
		}
		return contextListLoadedMsg{items: items}
	}
}

// applyContextFilter handles apply context filter.
func (m *ContextModel) applyContextFilter() {
	query := strings.ToLower(strings.TrimSpace(m.filterBuf))
	if query == "" {
		m.items = append([]api.Context{}, m.allItems...)
	} else {
		filtered := make([]api.Context, 0, len(m.allItems))
		for _, item := range m.allItems {
			title := strings.ToLower(strings.TrimSpace(contextTitle(item)))
			typ := strings.ToLower(strings.TrimSpace(item.SourceType))
			status := strings.ToLower(strings.TrimSpace(item.Status))
			tags := strings.ToLower(strings.Join(item.Tags, " "))
			content := ""
			if item.Content != nil {
				content = strings.ToLower(strings.TrimSpace(*item.Content))
			}
			url := ""
			if item.URL != nil {
				url = strings.ToLower(strings.TrimSpace(*item.URL))
			}
			if strings.Contains(title, query) ||
				strings.Contains(typ, query) ||
				strings.Contains(status, query) ||
				strings.Contains(tags, query) ||
				strings.Contains(content, query) ||
				strings.Contains(url, query) {
				filtered = append(filtered, item)
			}
		}
		m.items = filtered
	}
	rows := make([]table.Row, len(m.items))
	for i, item := range m.items {
		rows[i] = table.Row{formatContextLine(item)}
	}
	m.dataTable.SetRows(rows)
	m.dataTable.SetCursor(0)
}

// loadContextDetail loads load context detail.
func (m ContextModel) loadContextDetail(id string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(id) == "" {
			return errMsg{fmt.Errorf("context id is required")}
		}
		item, err := m.client.GetContext(id)
		if err != nil {
			return errMsg{err}
		}
		rels, relErr := m.client.GetRelationships("context", id)
		if relErr != nil {
			rels = nil
		}
		return contextDetailLoadedMsg{item: *item, relationships: rels}
	}
}

// formatContextLine handles format context line.
func formatContextLine(k api.Context) string {
	t := components.SanitizeText(k.SourceType)
	if t == "" {
		t = "note"
	}
	name := truncateContextName(components.SanitizeText(contextTitle(k)), maxContextNameLen)
	line := fmt.Sprintf("%s %s", name, TypeBadgeStyle.Render(components.SanitizeText(t)))
	if status := strings.TrimSpace(components.SanitizeText(k.Status)); status != "" {
		line = fmt.Sprintf("%s · %s", line, status)
	}
	preview := ""
	if k.Content != nil {
		preview = truncateString(strings.TrimSpace(components.SanitizeText(*k.Content)), 40)
	} else if k.URL != nil {
		preview = truncateString(strings.TrimSpace(components.SanitizeText(*k.URL)), 40)
	}
	if preview != "" {
		line = fmt.Sprintf("%s · %s", line, preview)
	}
	return line
}

// contextTitle handles context title.
func contextTitle(k api.Context) string {
	title := strings.TrimSpace(k.Title)
	if title != "" {
		return title
	}
	title = strings.TrimSpace(k.Name)
	if title != "" {
		return title
	}
	return "(untitled)"
}

const maxContextNameLen = 80

// truncateContextName handles truncate context name.
func truncateContextName(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

// loadScopeNames loads load scope names.
func (m ContextModel) loadScopeNames() tea.Cmd {
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
		return contextScopesLoadedMsg{names: names}
	}
}

// formatContextScopes handles format context scopes.
func (m ContextModel) formatContextScopes(ids []string) string {
	if len(ids) == 0 {
		return "-"
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := m.scopeNames[id]; ok && name != "" {
			names = append(names, name)
		} else {
			names = append(names, id)
		}
	}
	return formatScopePreview(names)
}

// scopeNamesFromIDs handles scope names from ids.
func (m ContextModel) scopeNamesFromIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := m.scopeNames[id]; ok && name != "" {
			names = append(names, name)
		} else {
			names = append(names, id)
		}
	}
	return names
}

// save handles save.
func (m ContextModel) save() (ContextModel, tea.Cmd) {
	title := strings.TrimSpace(m.addTitle)
	if title == "" {
		m.errText = "Title is required"
		return m, nil
	}

	url := strings.TrimSpace(m.addURL)
	sourceType := m.addType
	if sourceType == "" {
		sourceType = "note"
	}
	notes := strings.TrimSpace(m.addNotes)

	tags := parseCommaSeparated(m.addTagStr)
	for i, t := range tags {
		tags[i] = normalizeTag(t)
	}
	tags = dedup(tags)

	scopes := parseCommaSeparated(m.addScopeStr)
	for i, s := range scopes {
		scopes[i] = normalizeScope(s)
	}
	scopes = normalizeScopeList(scopes)
	if len(scopes) == 0 {
		scopes = []string{"private"}
	}

	input := api.CreateContextInput{
		Title:      title,
		URL:        url,
		SourceType: sourceType,
		Content:    notes,
		Scopes:     scopes,
		Tags:       tags,
	}

	linkIDs := make([]string, 0, len(m.linkEntities))
	for _, e := range m.linkEntities {
		linkIDs = append(linkIDs, e.ID)
	}

	m.saving = true
	return m, func() tea.Msg {
		created, err := m.client.CreateContext(input)
		if err != nil {
			return errMsg{err}
		}
		for _, id := range linkIDs {
			if err := m.client.LinkContext(created.ID, api.LinkContextInput{
				OwnerType: "entity",
				OwnerID:   id,
			}); err != nil {
				return errMsg{err}
			}
		}
		return contextSavedMsg{}
	}
}

// renderLinkedEntities renders the list of linked entities.
func (m *ContextModel) renderLinkedEntities() string {
	if len(m.linkEntities) == 0 {
		return ""
	}
	var b strings.Builder
	for i, e := range m.linkEntities {
		if i > 0 {
			b.WriteString(" ")
		}
		label := e.Name
		if label == "" {
			label = shortID(e.ID)
		}
		b.WriteString(AccentStyle.Render("[" + label + "]"))
	}
	return b.String()
}

// startLinkSearch handles start link search.
func (m *ContextModel) startLinkSearch() {
	m.linkSearching = true
	m.linkLoading = false
	m.linkQuery = ""
	m.linkResults = nil
	m.linkTable.SetRows(nil)
	m.linkTable.SetCursor(0)
}

// handleLinkSearch handles handle link search.
func (m ContextModel) handleLinkSearch(msg tea.KeyPressMsg) (ContextModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.linkSearching = false
		m.linkLoading = false
		m.linkQuery = ""
		m.linkResults = nil
		m.linkTable.SetRows(nil)
		m.linkTable.SetCursor(0)
	case isDown(msg):
		m.linkTable.MoveDown(1)
	case isUp(msg):
		m.linkTable.MoveUp(1)
	case isEnter(msg):
		if idx := m.linkTable.Cursor(); idx >= 0 && idx < len(m.linkResults) {
			m.addLinkedEntity(m.linkResults[idx])
		}
		m.linkSearching = false
		m.linkLoading = false
		m.linkQuery = ""
		m.linkResults = nil
		m.linkTable.SetRows(nil)
		m.linkTable.SetCursor(0)
	case isKey(msg, "backspace"):
		if len(m.linkQuery) > 0 {
			m.linkQuery = m.linkQuery[:len(m.linkQuery)-1]
			return m, m.updateLinkSearch()
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.linkQuery != "" {
			m.linkQuery = ""
			m.linkResults = nil
			m.linkTable.SetRows(nil)
			m.linkTable.SetCursor(0)
			return m, nil
		}
	default:
		if ch := keyText(msg); ch != "" {
			if len(m.linkResults) > 0 {
				m.linkResults = nil
				m.linkTable.SetRows(nil)
				m.linkTable.SetCursor(0)
			}
			m.linkQuery += ch
			return m, m.updateLinkSearch()
		}
	}
	return m, nil
}

// renderLinkSearch renders render link search.
func (m ContextModel) renderLinkSearch() string {
	var b strings.Builder
	b.WriteString(MetaKeyStyle.Render("Search") + MetaPunctStyle.Render(": ") + SelectedStyle.Render(components.SanitizeText(m.linkQuery)))
	b.WriteString(AccentStyle.Render("█"))
	b.WriteString("\n\n")
	if m.linkLoading {
		b.WriteString(MutedStyle.Render("Searching..."))
	} else if strings.TrimSpace(m.linkQuery) == "" {
		b.WriteString(MutedStyle.Render("Type to search."))
	} else if len(m.linkResults) == 0 {
		b.WriteString(MutedStyle.Render("No matches."))
	} else {
		contentWidth := components.BoxContentWidth(m.width)

		previewWidth := preferredPreviewWidth(contentWidth)

		gap := 3
		tableWidth := contentWidth
		sideBySide := contentWidth >= minSideBySideContentWidth
		if sideBySide {
			tableWidth = contentWidth - previewWidth - gap
		}

		// Each table cell has Padding(0,1) = 2 chars. 3 columns = 6 chars of padding.
		cellPadding := 3 * 2
		availableCols := tableWidth - cellPadding
		if availableCols < 30 {
			availableCols = 30
		}

		typeWidth := 14
		statusWidth := 11
		nameWidth := availableCols - (typeWidth + statusWidth)
		if nameWidth < 16 {
			nameWidth = 16
			typeWidth = availableCols - (nameWidth + statusWidth)
			if typeWidth < 12 {
				typeWidth = 12
			}
		}

		tableRows := make([]table.Row, len(m.linkResults))
		for i, e := range m.linkResults {
			name := strings.TrimSpace(components.SanitizeOneLine(e.Name))
			if name == "" {
				name = "entity"
			}
			typ := strings.TrimSpace(components.SanitizeOneLine(e.Type))
			if typ == "" {
				typ = "entity"
			}
			status := strings.TrimSpace(components.SanitizeOneLine(e.Status))
			if status == "" {
				status = "-"
			}

			tableRows[i] = table.Row{
				components.ClampTextWidthEllipsis(name, nameWidth),
				components.ClampTextWidthEllipsis(typ, typeWidth),
				components.ClampTextWidthEllipsis(status, statusWidth),
			}
		}

		m.linkTable.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Type", Width: typeWidth},
			{Title: "Status", Width: statusWidth},
		})
		actualTableWidth := nameWidth + typeWidth + statusWidth + cellPadding
		m.linkTable.SetWidth(actualTableWidth)
		m.linkTable.SetRows(tableRows)

		countLine := MutedStyle.Render(fmt.Sprintf("%d results", len(m.linkResults)))
		tableView := m.linkTable.View()
		preview := ""
		var previewItem *api.Entity
		if idx := m.linkTable.Cursor(); idx >= 0 && idx < len(m.linkResults) {
			previewItem = &m.linkResults[idx]
		}
		if previewItem != nil {
			content := m.renderLinkEntityPreview(*previewItem, previewBoxContentWidth(previewWidth))
			preview = renderPreviewBox(content, previewWidth)
		}

		body := tableView
		if sideBySide && preview != "" {
			body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
		} else if preview != "" {
			body = tableView + "\n\n" + preview
		}

		b.WriteString(countLine)
		b.WriteString("\n\n")
		b.WriteString(body)
	}
	return components.Indent(components.TitledBox("Link Entity", b.String(), m.width), 1)
}

// renderLinkEntityPreview renders render link entity preview.
func (m ContextModel) renderLinkEntityPreview(e api.Entity, width int) string {
	if width <= 0 {
		return ""
	}

	name := strings.TrimSpace(components.SanitizeOneLine(e.Name))
	if name == "" {
		name = "entity"
	}
	typ := strings.TrimSpace(components.SanitizeOneLine(e.Type))
	if typ == "" {
		typ = "entity"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(e.Status))
	if status == "" {
		status = "-"
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(name, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Type", typ, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	if len(e.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(e.Tags, ", "), width))
	}

	return padPreviewLines(lines, width)
}

// searchLinkEntities handles search link entities.
func (m ContextModel) searchLinkEntities(query string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryEntities(api.QueryParams{"search_text": query})
		if err != nil {
			return errMsg{err}
		}
		return contextLinkResultsMsg{items: items}
	}
}

// updateLinkSearch updates update link search.
func (m *ContextModel) updateLinkSearch() tea.Cmd {
	query := strings.TrimSpace(m.linkQuery)
	if query == "" {
		m.linkLoading = false
		m.linkResults = nil
		m.linkTable.SetRows(nil)
		m.linkTable.SetCursor(0)
		return nil
	}
	m.linkLoading = true
	return m.searchLinkEntities(query)
}

// addLinkedEntity handles add linked entity.
func (m *ContextModel) addLinkedEntity(entity api.Entity) {
	for _, e := range m.linkEntities {
		if e.ID == entity.ID {
			return
		}
	}
	m.linkEntities = append(m.linkEntities, entity)
}

// normalizeTag handles normalize tag.
func normalizeTag(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.Join(strings.Fields(s), "-")
	return s
}

// normalizeScope handles normalize scope.
func normalizeScope(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), "-")
	return s
}

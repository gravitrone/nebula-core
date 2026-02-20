package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type jobsLoadedMsg struct{ items []api.Job }
type jobStatusUpdatedMsg struct{}
type subtaskCreatedMsg struct{}
type jobCreatedMsg struct{}
type jobsScopesLoadedMsg struct{ options []string }
type jobRelationshipsLoadedMsg struct {
	id            string
	relationships []api.Relationship
}

type jobsView int

const (
	jobsViewAdd jobsView = iota
	jobsViewList
	jobsViewDetail
	jobsViewEdit
)

const (
	jobFieldTitle = iota
	jobFieldDescription
	jobFieldStatus
	jobFieldPriority
	jobFieldMetadata
	jobFieldCount
)

const (
	jobEditFieldStatus = iota
	jobEditFieldDescription
	jobEditFieldPriority
	jobEditFieldMetadata
	jobEditFieldCount
)

var jobStatusOptions = []string{"pending", "active", "completed", "failed"}
var jobPriorityOptions = []string{"", "low", "medium", "high"}

// --- Jobs Model ---

type JobsModel struct {
	client          *api.Client
	allItems        []api.Job
	items           []api.Job
	list            *components.List
	selected        map[string]bool
	loading         bool
	detail          *api.Job
	detailRels      []api.Relationship
	filtering       bool
	searchBuf       string
	searchSuggest   string
	view            jobsView
	modeFocus       bool
	changingSt      bool
	statusBuf       string
	statusTargets   []string
	creatingSubtask bool
	subtaskBuf      string
	metaExpanded    bool
	width           int
	height          int
	scopeOptions    []string

	// add
	addFields      []formField
	addFocus       int
	addStatusIdx   int
	addPriorityIdx int
	addMeta        MetadataEditor
	addSaving      bool
	addSaved       bool
	addErr         string

	// edit
	editFocus       int
	editStatusIdx   int
	editPriorityIdx int
	editDesc        string
	editMeta        MetadataEditor
	editSaving      bool
}

// NewJobsModel builds the jobs UI model.
func NewJobsModel(client *api.Client) JobsModel {
	return JobsModel{
		client:   client,
		list:     components.NewList(15),
		selected: map[string]bool{},
		view:     jobsViewList,
		addFields: []formField{
			{label: "Title"},
			{label: "Description"},
			{label: "Status"},
			{label: "Priority"},
			{label: "Metadata"},
		},
	}
}

func (m JobsModel) Init() tea.Cmd {
	m.loading = true
	m.view = jobsViewList
	m.modeFocus = false
	m.metaExpanded = false
	m.addFocus = 0
	m.addStatusIdx = statusIndex(jobStatusOptions, "pending")
	m.addPriorityIdx = statusIndex(jobPriorityOptions, "")
	m.addMeta.Reset()
	m.addSaving = false
	m.addSaved = false
	m.addErr = ""
	m.filtering = false
	m.searchBuf = ""
	m.searchSuggest = ""
	m.selected = map[string]bool{}
	m.statusTargets = nil
	m.detailRels = nil
	return m.loadJobs
}

func (m JobsModel) Update(msg tea.Msg) (JobsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case jobsLoadedMsg:
		m.loading = false
		m.allItems = msg.items
		m.applyJobSearch()
		return m, m.loadScopeOptions()
	case jobsScopesLoadedMsg:
		m.scopeOptions = msg.options
		m.addMeta.SetScopeOptions(m.scopeOptions)
		m.editMeta.SetScopeOptions(m.scopeOptions)
		return m, nil
	case jobRelationshipsLoadedMsg:
		if m.detail != nil && m.detail.ID == msg.id {
			m.detailRels = msg.relationships
		}
		return m, nil

	case jobStatusUpdatedMsg:
		m.detail = nil
		m.changingSt = false
		m.statusBuf = ""
		m.statusTargets = nil
		return m, m.loadJobs
	case subtaskCreatedMsg:
		m.detail = nil
		m.creatingSubtask = false
		m.subtaskBuf = ""
		return m, m.loadJobs
	case jobCreatedMsg:
		m.addSaving = false
		m.addSaved = true
		m.loading = true
		return m, m.loadJobs
	case errMsg:
		m.loading = false
		m.addSaving = false
		m.editSaving = false
		m.changingSt = false
		m.statusTargets = nil
		m.creatingSubtask = false
		m.addErr = msg.err.Error()
		return m, nil

	case tea.KeyMsg:
		if m.addMeta.Active {
			m.addMeta.HandleKey(msg)
			return m, nil
		}
		if m.editMeta.Active {
			m.editMeta.HandleKey(msg)
			return m, nil
		}
		if m.creatingSubtask {
			return m.handleSubtaskInput(msg)
		}
		if m.changingSt {
			return m.handleStatusInput(msg)
		}
		if m.modeFocus {
			return m.handleModeKeys(msg)
		}
		switch m.view {
		case jobsViewAdd:
			return m.handleAddKeys(msg)
		case jobsViewEdit:
			return m.handleEditKeys(msg)
		case jobsViewDetail:
			return m.handleDetailKeys(msg)
		default:
			return m.handleListKeys(msg)
		}
	}
	return m, nil
}

func (m JobsModel) View() string {
	if m.addMeta.Active {
		return m.addMeta.Render(m.width)
	}
	if m.editMeta.Active {
		return m.editMeta.Render(m.width)
	}
	if m.creatingSubtask && m.detail != nil {
		return components.Indent(components.InputDialog("New Subtask Title", m.subtaskBuf), 1)
	}

	if m.changingSt {
		return components.Indent(components.InputDialog("New Status (pending/active/completed/failed)", m.statusBuf), 1)
	}
	if m.filtering && m.view == jobsViewList {
		return components.Indent(components.InputDialog("Filter Jobs", m.searchBuf), 1)
	}
	modeLine := m.renderModeLine()
	var body string
	switch m.view {
	case jobsViewAdd:
		body = m.renderAdd()
	case jobsViewEdit:
		body = m.renderEdit()
	case jobsViewDetail:
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

func (m JobsModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("List")
	if m.view == jobsViewAdd {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("List")
	}
	if m.modeFocus {
		if m.view == jobsViewAdd {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("List")
		}
	}
	return add + " " + list
}

func (m JobsModel) handleModeKeys(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
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

func (m JobsModel) toggleMode() (JobsModel, tea.Cmd) {
	m.modeFocus = false
	m.detail = nil
	m.metaExpanded = false
	if m.view == jobsViewAdd {
		m.view = jobsViewList
		m.loading = true
		return m, m.loadJobs
	}
	m.view = jobsViewAdd
	return m, nil
}

// --- List ---

func (m JobsModel) renderList() string {
	if m.loading {
		return MutedStyle.Render("Loading jobs...")
	}
	if len(m.items) == 0 {
		return components.EmptyStateBox(
			"Jobs",
			"No jobs found.",
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

	statusWidth := 12
	prioWidth := 10
	atWidth := compactTimeColumnWidth
	titleWidth := availableCols - (statusWidth + prioWidth + atWidth)
	if titleWidth < 12 {
		titleWidth = 12
	}
	cols := []components.TableColumn{
		{Header: "Title", Width: titleWidth, Align: lipgloss.Left},
		{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		{Header: "Priority", Width: prioWidth, Align: lipgloss.Left},
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.Job
	if idx := m.list.Selected(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}

	for i := range visible {
		absIdx := m.list.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.items) {
			continue
		}
		j := m.items[absIdx]

		status := strings.TrimSpace(components.SanitizeOneLine(j.Status))
		if status == "" {
			status = "-"
		}
		priority := "-"
		if j.Priority != nil && strings.TrimSpace(*j.Priority) != "" {
			priority = strings.TrimSpace(components.SanitizeOneLine(*j.Priority))
		}
		at := j.UpdatedAt
		if at.IsZero() {
			at = j.CreatedAt
		}

		if m.list.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		titleValue := components.SanitizeOneLine(j.Title)
		if len(m.selected) > 0 {
			if m.selected[j.ID] {
				titleValue = "[X] " + titleValue
			} else {
				titleValue = "[ ] " + titleValue
			}
		}

		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(titleValue, titleWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			components.ClampTextWidthEllipsis(priority, prioWidth),
			formatLocalTimeCompact(at),
		})
	}
	if m.modeFocus {
		activeRowRel = -1
	}
	title := "Jobs"
	countLine := fmt.Sprintf("%d total", len(m.items))
	if selected := m.selectedCount(); selected > 0 {
		countLine = fmt.Sprintf("%s · selected: %d", countLine, selected)
	}
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
		content := m.renderJobPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.TitledBox(title, content, m.width)
}

func (m JobsModel) renderJobPreview(j api.Job, width int) string {
	if width <= 0 {
		return ""
	}

	title := components.SanitizeOneLine(j.Title)
	status := strings.TrimSpace(components.SanitizeOneLine(j.Status))
	if status == "" {
		status = "-"
	}
	priority := "-"
	if j.Priority != nil && strings.TrimSpace(*j.Priority) != "" {
		priority = strings.TrimSpace(components.SanitizeOneLine(*j.Priority))
	}
	at := j.UpdatedAt
	if at.IsZero() {
		at = j.CreatedAt
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")
	lines = append(lines, MetaKeyStyle.Render("ID"))
	for _, part := range wrapPreviewText(components.SanitizeOneLine(j.ID), width) {
		lines = append(lines, MetaValueStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("Priority", priority, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeCompact(at), width))
	if m.detail != nil && m.detail.ID == j.ID && len(m.detailRels) > 0 {
		lines = append(lines, renderPreviewRow("Links", fmt.Sprintf("%d", len(m.detailRels)), width))
	}

	if j.Description != nil && strings.TrimSpace(*j.Description) != "" {
		desc := truncateString(strings.TrimSpace(components.SanitizeText(*j.Description)), 120)
		lines = append(lines, renderPreviewRow("Notes", desc, width))
	}
	if metaPreview := metadataPreview(map[string]any(j.Metadata), 80); metaPreview != "" {
		lines = append(lines, renderPreviewRow("Preview", metaPreview, width))
	}

	return padPreviewLines(lines, width)
}

func (m JobsModel) handleListKeys(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
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
	case isEnter(msg):
		if idx := m.list.Selected(); idx < len(m.items) {
			item := m.items[idx]
			m.detail = &item
			m.detailRels = nil
			m.view = jobsViewDetail
			return m, m.loadDetailRelationships(item.ID)
		}
	case isSpace(msg):
		m.toggleSelected()
	case isKey(msg, "b"):
		m.toggleSelectAll()
	case isKey(msg, "f"):
		m.filtering = true
		return m, nil
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applyJobSearch()
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.applyJobSearch()
		}
	case isBack(msg):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.applyJobSearch()
		}
	case isKey(msg, "tab"):
		if m.searchSuggest != "" && !strings.EqualFold(strings.TrimSpace(m.searchBuf), strings.TrimSpace(m.searchSuggest)) {
			m.searchBuf = m.searchSuggest
			m.applyJobSearch()
		}
	case isKey(msg, "s"):
		targets := m.selectedIDs()
		if len(targets) > 0 {
			m.changingSt = true
			m.statusBuf = ""
			m.statusTargets = targets
			return m, nil
		}
		if idx := m.list.Selected(); idx < len(m.items) {
			item := m.items[idx]
			m.detail = &item
			m.view = jobsViewDetail
			m.changingSt = true
			m.statusBuf = ""
			m.statusTargets = []string{item.ID}
		}
	default:
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
			if ch == " " && m.searchBuf == "" {
				return m, nil
			}
			m.searchBuf += ch
			m.applyJobSearch()
		}
	}
	return m, nil
}

func (m JobsModel) handleFilterInput(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
	case isBack(msg):
		m.filtering = false
		m.searchBuf = ""
		m.searchSuggest = ""
		m.applyJobSearch()
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.applyJobSearch()
		}
	default:
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
			if ch == " " && m.searchBuf == "" {
				return m, nil
			}
			m.searchBuf += ch
			m.applyJobSearch()
		}
	}
	return m, nil
}

// --- Add ---

func (m JobsModel) handleAddKeys(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
	if m.addSaving {
		return m, nil
	}
	switch {
	case isDown(msg):
		m.addFocus = (m.addFocus + 1) % jobFieldCount
	case isUp(msg):
		if m.addFocus == 0 {
			m.modeFocus = true
			return m, nil
		}
		m.addFocus = (m.addFocus - 1 + jobFieldCount) % jobFieldCount
	case isKey(msg, "ctrl+s"):
		return m.saveAdd()
	case isBack(msg):
		m.resetAddForm()
	case isKey(msg, "backspace"):
		if m.addFocus == jobFieldMetadata {
			return m, nil
		}
		if m.addFocus == jobFieldTitle || m.addFocus == jobFieldDescription {
			f := &m.addFields[m.addFocus]
			if len(f.value) > 0 {
				f.value = f.value[:len(f.value)-1]
			}
		}
	default:
		switch m.addFocus {
		case jobFieldStatus:
			switch {
			case isKey(msg, "left"):
				m.addStatusIdx = (m.addStatusIdx - 1 + len(jobStatusOptions)) % len(jobStatusOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.addStatusIdx = (m.addStatusIdx + 1) % len(jobStatusOptions)
			}
		case jobFieldPriority:
			switch {
			case isKey(msg, "left"):
				m.addPriorityIdx = (m.addPriorityIdx - 1 + len(jobPriorityOptions)) % len(jobPriorityOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.addPriorityIdx = (m.addPriorityIdx + 1) % len(jobPriorityOptions)
			}
		case jobFieldMetadata:
			if isEnter(msg) {
				m.addMeta.Active = true
			}
		default:
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				m.addFields[m.addFocus].value += ch
			}
		}
	}
	return m, nil
}

func (m JobsModel) renderAdd() string {
	if m.addSaving {
		return MutedStyle.Render("Saving...")
	}
	if m.addSaved {
		return components.Box(SuccessStyle.Render("Job saved! Press Esc to add another."), m.width)
	}

	var b strings.Builder
	for i, f := range m.addFields {
		label := f.label
		switch i {
		case jobFieldStatus:
			status := jobStatusOptions[m.addStatusIdx]
			if i == m.addFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + status))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + status))
			}
		case jobFieldPriority:
			priority := jobPriorityOptions[m.addPriorityIdx]
			if priority == "" {
				priority = "-"
			}
			if i == m.addFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + priority))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + priority))
			}
		case jobFieldMetadata:
			if i == m.addFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
			}
			b.WriteString("\n")
			meta := renderMetadataEditorPreview(m.addMeta.Buffer, m.addMeta.Scopes, m.width, 6)
			if strings.TrimSpace(meta) == "" {
				meta = "-"
			}
			b.WriteString(NormalStyle.Render("  " + meta))
		default:
			if i == m.addFocus {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + f.value))
				b.WriteString(AccentStyle.Render("█"))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				val := f.value
				if val == "" {
					val = "-"
				}
				b.WriteString(NormalStyle.Render("  " + val))
			}
		}

		if i < jobFieldCount-1 {
			b.WriteString("\n\n")
		}
	}

	if m.addErr != "" {
		b.WriteString("\n\n")
		b.WriteString(components.ErrorBox("Error", m.addErr, m.width))
	}
	return components.TitledBox("Add Job", b.String(), m.width)
}

func (m JobsModel) saveAdd() (JobsModel, tea.Cmd) {
	title := strings.TrimSpace(m.addFields[jobFieldTitle].value)
	if title == "" {
		m.addErr = "Title is required"
		return m, nil
	}
	desc := strings.TrimSpace(m.addFields[jobFieldDescription].value)
	status := jobStatusOptions[m.addStatusIdx]
	priority := strings.TrimSpace(jobPriorityOptions[m.addPriorityIdx])

	meta, err := parseMetadataInput(m.addMeta.Buffer)
	if err != nil {
		m.addErr = err.Error()
		return m, nil
	}
	meta = mergeMetadataScopes(meta, m.addMeta.Scopes)

	input := api.CreateJobInput{
		Title:       title,
		Description: desc,
		Status:      status,
		Priority:    priority,
		Metadata:    meta,
	}

	m.addSaving = true
	return m, func() tea.Msg {
		if _, err := m.client.CreateJob(input); err != nil {
			return errMsg{err}
		}
		return jobCreatedMsg{}
	}
}

func (m *JobsModel) resetAddForm() {
	m.addSaved = false
	m.addSaving = false
	m.addErr = ""
	m.addFocus = 0
	m.addStatusIdx = statusIndex(jobStatusOptions, "pending")
	m.addPriorityIdx = statusIndex(jobPriorityOptions, "")
	m.addMeta.Reset()
	for i := range m.addFields {
		m.addFields[i].value = ""
	}
}

// --- Edit ---

func (m *JobsModel) startEdit() {
	if m.detail == nil {
		return
	}
	m.editFocus = jobEditFieldStatus
	m.editStatusIdx = statusIndex(jobStatusOptions, m.detail.Status)
	m.editPriorityIdx = statusIndex(jobPriorityOptions, valueOrEmpty(m.detail.Priority))
	m.editDesc = valueOrEmpty(m.detail.Description)
	m.editMeta.Reset()
	m.editMeta.Load(map[string]any(m.detail.Metadata))
	m.editSaving = false
}

func (m JobsModel) handleEditKeys(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	switch {
	case isDown(msg):
		m.editFocus = (m.editFocus + 1) % jobEditFieldCount
	case isUp(msg):
		if m.editFocus > 0 {
			m.editFocus = (m.editFocus - 1 + jobEditFieldCount) % jobEditFieldCount
		}
	case isBack(msg):
		m.view = jobsViewDetail
	case isKey(msg, "ctrl+s"):
		return m.saveEdit()
	default:
		switch m.editFocus {
		case jobEditFieldStatus:
			switch {
			case isKey(msg, "left"):
				m.editStatusIdx = (m.editStatusIdx - 1 + len(jobStatusOptions)) % len(jobStatusOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.editStatusIdx = (m.editStatusIdx + 1) % len(jobStatusOptions)
			}
		case jobEditFieldPriority:
			switch {
			case isKey(msg, "left"):
				m.editPriorityIdx = (m.editPriorityIdx - 1 + len(jobPriorityOptions)) % len(jobPriorityOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.editPriorityIdx = (m.editPriorityIdx + 1) % len(jobPriorityOptions)
			}
		case jobEditFieldMetadata:
			if isEnter(msg) {
				m.editMeta.Active = true
			}
		case jobEditFieldDescription:
			switch {
			case isKey(msg, "backspace"):
				m.editDesc = dropLastRune(m.editDesc)
			default:
				ch := msg.String()
				if len(ch) == 1 || ch == " " {
					m.editDesc += ch
				}
			}
		}
	}
	return m, nil
}

func (m JobsModel) renderEdit() string {
	var b strings.Builder

	// Status
	status := jobStatusOptions[m.editStatusIdx]
	if m.editFocus == jobEditFieldStatus {
		b.WriteString(SelectedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	} else {
		b.WriteString(MutedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	}

	b.WriteString("\n\n")

	// Description
	if m.editFocus == jobEditFieldDescription {
		b.WriteString(SelectedStyle.Render("  Description:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + m.editDesc))
		b.WriteString(AccentStyle.Render("█"))
	} else {
		b.WriteString(MutedStyle.Render("  Description:"))
		b.WriteString("\n")
		val := m.editDesc
		if val == "" {
			val = "-"
		}
		b.WriteString(NormalStyle.Render("  " + val))
	}

	b.WriteString("\n\n")

	// Priority
	priority := jobPriorityOptions[m.editPriorityIdx]
	if priority == "" {
		priority = "-"
	}
	if m.editFocus == jobEditFieldPriority {
		b.WriteString(SelectedStyle.Render("  Priority:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + priority))
	} else {
		b.WriteString(MutedStyle.Render("  Priority:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + priority))
	}

	b.WriteString("\n\n")

	// Metadata
	if m.editFocus == jobEditFieldMetadata {
		b.WriteString(SelectedStyle.Render("  Metadata:"))
	} else {
		b.WriteString(MutedStyle.Render("  Metadata:"))
	}
	b.WriteString("\n")
	meta := renderMetadataEditorPreview(m.editMeta.Buffer, m.editMeta.Scopes, m.width, 6)
	if strings.TrimSpace(meta) == "" {
		meta = "-"
	}
	b.WriteString(NormalStyle.Render("  " + meta))

	if m.editSaving {
		b.WriteString("\n\n" + MutedStyle.Render("Saving..."))
	}

	return components.TitledBox("Edit Job", b.String(), m.width)
}

func (m JobsModel) saveEdit() (JobsModel, tea.Cmd) {
	if m.detail == nil {
		return m, nil
	}
	status := jobStatusOptions[m.editStatusIdx]
	priority := strings.TrimSpace(jobPriorityOptions[m.editPriorityIdx])
	desc := strings.TrimSpace(m.editDesc)
	meta, err := parseMetadataInput(m.editMeta.Buffer)
	if err != nil {
		m.addErr = err.Error()
		return m, nil
	}
	meta = mergeMetadataScopes(meta, m.editMeta.Scopes)

	input := api.UpdateJobInput{
		Status:      &status,
		Priority:    &priority,
		Description: &desc,
		Metadata:    meta,
	}

	m.editSaving = true
	return m, func() tea.Msg {
		if _, err := m.client.UpdateJob(m.detail.ID, input); err != nil {
			return errMsg{err}
		}
		return jobStatusUpdatedMsg{}
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// --- Helpers ---

func (m JobsModel) loadJobs() tea.Msg {
	items, err := m.client.QueryJobs(nil)
	if err != nil {
		return errMsg{err}
	}
	return jobsLoadedMsg{items}
}

func (m JobsModel) loadScopeOptions() tea.Cmd {
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
		return jobsScopesLoadedMsg{options: scopeNameList(names)}
	}
}

func (m *JobsModel) applyJobSearch() {
	query := strings.TrimSpace(strings.ToLower(m.searchBuf))
	if query == "" {
		m.items = m.allItems
	} else {
		filtered := make([]api.Job, 0, len(m.allItems))
		for _, j := range m.allItems {
			line := strings.ToLower(j.Title + " " + j.Status + " " + j.ID)
			if strings.Contains(line, query) {
				filtered = append(filtered, j)
			}
		}
		m.items = filtered
	}
	labels := make([]string, len(m.items))
	for i, j := range m.items {
		labels[i] = formatJobLine(j)
	}
	m.list.SetItems(labels)
	m.retainSelection()
	m.updateSearchSuggest()
}

func (m *JobsModel) retainSelection() {
	if len(m.selected) == 0 {
		return
	}
	visible := make(map[string]struct{}, len(m.allItems))
	for _, item := range m.allItems {
		visible[item.ID] = struct{}{}
	}
	next := make(map[string]bool, len(m.selected))
	for id := range m.selected {
		if _, ok := visible[id]; ok {
			next[id] = true
		}
	}
	m.selected = next
}

func (m *JobsModel) toggleSelected() {
	idx := m.list.Selected()
	if idx < 0 || idx >= len(m.items) {
		return
	}
	id := strings.TrimSpace(m.items[idx].ID)
	if id == "" {
		return
	}
	if m.selected[id] {
		delete(m.selected, id)
		return
	}
	m.selected[id] = true
}

func (m *JobsModel) toggleSelectAll() {
	if len(m.items) == 0 {
		return
	}
	if len(m.selected) == len(m.items) {
		m.selected = map[string]bool{}
		return
	}
	selected := make(map[string]bool, len(m.items))
	for _, item := range m.items {
		selected[item.ID] = true
	}
	m.selected = selected
}

func (m JobsModel) selectedIDs() []string {
	if len(m.selected) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m.selected))
	for _, item := range m.items {
		if m.selected[item.ID] {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) > 0 {
		return ids
	}
	for id := range m.selected {
		ids = append(ids, id)
	}
	return ids
}

func (m JobsModel) selectedCount() int {
	return len(m.selectedIDs())
}

func (m *JobsModel) updateSearchSuggest() {
	m.searchSuggest = ""
	query := strings.ToLower(strings.TrimSpace(m.searchBuf))
	if query == "" {
		return
	}
	for _, j := range m.allItems {
		name := strings.ToLower(strings.TrimSpace(j.Title))
		if strings.HasPrefix(name, query) {
			m.searchSuggest = j.Title
			return
		}
	}
}

func (m JobsModel) handleDetailKeys(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
	switch {
	case isUp(msg):
		m.modeFocus = true
	case isBack(msg):
		m.detail = nil
		m.detailRels = nil
		m.metaExpanded = false
		m.view = jobsViewList
	case isKey(msg, "s"):
		m.changingSt = true
		m.statusBuf = ""
		m.statusTargets = []string{m.detail.ID}
	case isKey(msg, "n"):
		m.creatingSubtask = true
		m.subtaskBuf = ""
	case isKey(msg, "e"):
		m.startEdit()
		m.view = jobsViewEdit
	case isKey(msg, "m"):
		m.metaExpanded = !m.metaExpanded
	}
	return m, nil
}

func (m JobsModel) handleStatusInput(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.changingSt = false
		m.statusBuf = ""
		m.statusTargets = nil
	case isEnter(msg):
		ids := append([]string(nil), m.statusTargets...)
		if len(ids) == 0 && m.detail != nil {
			ids = []string{m.detail.ID}
		}
		status := strings.TrimSpace(m.statusBuf)
		if len(ids) == 0 || status == "" {
			m.changingSt = false
			m.statusBuf = ""
			m.statusTargets = nil
			return m, nil
		}
		m.changingSt = false
		m.statusBuf = ""
		m.statusTargets = nil
		m.selected = map[string]bool{}
		return m, func() tea.Msg {
			for _, id := range ids {
				if _, err := m.client.UpdateJobStatus(id, status); err != nil {
					return errMsg{err}
				}
			}
			return jobStatusUpdatedMsg{}
		}
	case isKey(msg, "backspace"):
		if len(m.statusBuf) > 0 {
			m.statusBuf = m.statusBuf[:len(m.statusBuf)-1]
		}
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.statusBuf += msg.String()
		}
	}
	return m, nil
}

func (m JobsModel) handleSubtaskInput(msg tea.KeyMsg) (JobsModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.creatingSubtask = false
		m.subtaskBuf = ""
	case isEnter(msg):
		title := strings.TrimSpace(m.subtaskBuf)
		if title == "" {
			return m, nil
		}
		id := m.detail.ID
		m.creatingSubtask = false
		m.subtaskBuf = ""
		return m, func() tea.Msg {
			_, err := m.client.CreateSubtask(id, map[string]string{"title": title})
			if err != nil {
				return errMsg{err}
			}
			return subtaskCreatedMsg{}
		}
	case isKey(msg, "backspace"):
		if len(m.subtaskBuf) > 0 {
			m.subtaskBuf = m.subtaskBuf[:len(m.subtaskBuf)-1]
		}
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.subtaskBuf += msg.String()
		}
	}
	return m, nil
}

func (m JobsModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}
	j := m.detail
	var sections []string

	rows := []components.TableRow{
		{Label: "ID", Value: j.ID},
		{Label: "Title", Value: j.Title},
		{Label: "Status", Value: j.Status},
	}
	if j.Priority != nil && strings.TrimSpace(*j.Priority) != "" {
		rows = append(rows, components.TableRow{Label: "Priority", Value: *j.Priority})
	}
	rows = append(rows, components.TableRow{Label: "Created", Value: formatLocalTimeFull(j.CreatedAt)})
	if !j.UpdatedAt.IsZero() {
		rows = append(rows, components.TableRow{Label: "Updated", Value: formatLocalTimeFull(j.UpdatedAt)})
	}
	sections = append(sections, components.Table("Job", rows, m.width))

	if j.Description != nil && strings.TrimSpace(*j.Description) != "" {
		sections = append(
			sections,
			components.TitledBox(
				"Description",
				NormalStyle.Render(components.SanitizeText(*j.Description)),
				m.width,
			),
		)
	}

	if len(j.Metadata) > 0 {
		metaTable := renderMetadataBlock(map[string]any(j.Metadata), m.width, m.metaExpanded)
		if metaTable != "" {
			sections = append(sections, metaTable)
		}
	}
	if len(m.detailRels) > 0 {
		sections = append(sections, components.Table("Relationships", relationshipSummaryRows("job", j.ID, m.detailRels, 6), m.width))
	}

	return strings.Join(sections, "\n\n")
}

func (m JobsModel) loadDetailRelationships(jobID string) tea.Cmd {
	return func() tea.Msg {
		rels, err := m.client.GetRelationships("job", jobID)
		if err != nil {
			return jobRelationshipsLoadedMsg{id: jobID, relationships: nil}
		}
		return jobRelationshipsLoadedMsg{id: jobID, relationships: rels}
	}
}

func formatJobLine(j api.Job) string {
	p := ""
	if j.Priority != nil {
		p = fmt.Sprintf(" · %s", components.SanitizeText(*j.Priority))
	}
	line := fmt.Sprintf(
		"%s · %s%s",
		components.SanitizeText(j.Title),
		components.SanitizeText(j.Status),
		p,
	)
	if preview := metadataPreview(map[string]any(j.Metadata), 40); preview != "" {
		line = fmt.Sprintf("%s · %s", line, preview)
	}
	return line
}

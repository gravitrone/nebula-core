package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

var importExportMarshalIndent = json.MarshalIndent

type importExportMode int

const (
	importMode importExportMode = iota
	exportMode
)

type importExportStep int

const (
	stepResource importExportStep = iota
	stepFormat
	stepPath
	stepRunning
	stepResult
)

type importExportResource struct {
	label string
	value string
}

type importExportDoneMsg struct {
	summary string
	details []string
}

type importExportErrorMsg struct {
	err error
}

type ImportExportModel struct {
	client *api.Client

	mode      importExportMode
	step      importExportStep
	resources []importExportResource
	formats   []string

	resourceIndex int
	formatIndex   int
	path          string
	summary       string
	details       []string
	errText       string
	closed        bool

	width  int
	height int
}

// NewImportExportModel handles new import export model.
func NewImportExportModel(client *api.Client) ImportExportModel {
	return ImportExportModel{
		client:  client,
		formats: []string{"json", "csv"},
	}
}

// Start handles start.
func (m *ImportExportModel) Start(mode importExportMode) {
	m.mode = mode
	m.step = stepResource
	m.resourceIndex = 0
	m.formatIndex = 0
	m.path = ""
	m.summary = ""
	m.details = nil
	m.errText = ""
	m.closed = false
	m.resources = importExportResourcesForMode(mode)
}

// Update updates update.
func (m ImportExportModel) Update(msg tea.Msg) (ImportExportModel, tea.Cmd) {
	switch msg := msg.(type) {
	case importExportDoneMsg:
		m.step = stepResult
		m.summary = msg.summary
		m.details = msg.details
		return m, nil
	case importExportErrorMsg:
		m.step = stepResult
		m.errText = msg.err.Error()
		return m, nil
	case tea.KeyMsg:
		switch m.step {
		case stepResource:
			return m.handleResourceKeys(msg)
		case stepFormat:
			return m.handleFormatKeys(msg)
		case stepPath:
			return m.handlePathKeys(msg)
		case stepResult:
			if isBack(msg) || isEnter(msg) {
				m.closed = true
			}
		}
	}
	return m, nil
}

// View handles view.
func (m ImportExportModel) View() string {
	switch m.step {
	case stepResource:
		title := "Choose resource"
		return components.TitledBox(title, m.renderOptions(m.resources, m.resourceIndex), m.width)
	case stepFormat:
		title := "Choose format"
		return components.TitledBox(title, m.renderFormatOptions(), m.width)
	case stepPath:
		title := "Enter file path"
		if m.mode == exportMode {
			title = "Export file path"
		}
		return components.InputDialog(title, m.path)
	case stepRunning:
		label := "Importing..."
		if m.mode == exportMode {
			label = "Exporting..."
		}
		return components.Indent(components.Box(MutedStyle.Render(label), m.width), 1)
	case stepResult:
		if m.errText != "" {
			return components.Indent(components.ErrorBox("Import/Export Failed", m.errText, m.width), 1)
		}
		body := m.summary
		if len(m.details) > 0 {
			body = body + "\n\n" + strings.Join(m.details, "\n")
		}
		return components.Indent(components.TitledBox("Import/Export", body, m.width), 1)
	default:
		return ""
	}
}

// renderOptions renders render options.
func (m ImportExportModel) renderOptions(options []importExportResource, index int) string {
	contentWidth := components.BoxContentWidth(m.width)
	if contentWidth < 10 {
		contentWidth = 10
	}

	cols := []components.TableColumn{
		{Header: "Option", Width: contentWidth, Align: lipgloss.Left},
	}

	rows := make([][]string, 0, len(options))
	for _, opt := range options {
		label := strings.TrimSpace(components.SanitizeOneLine(opt.label))
		if label == "" {
			label = "-"
		}
		rows = append(rows, []string{components.ClampTextWidthEllipsis(label, contentWidth)})
	}

	table := components.TableGridWithActiveRow(cols, rows, contentWidth, index)
	return table + "\n\n" + MutedStyle.Render("enter: select | esc: cancel")
}

// renderFormatOptions renders render format options.
func (m ImportExportModel) renderFormatOptions() string {
	contentWidth := components.BoxContentWidth(m.width)
	if contentWidth < 10 {
		contentWidth = 10
	}

	cols := []components.TableColumn{
		{Header: "Format", Width: contentWidth, Align: lipgloss.Left},
	}

	rows := make([][]string, 0, len(m.formats))
	for _, format := range m.formats {
		label := strings.TrimSpace(components.SanitizeOneLine(strings.ToUpper(format)))
		if label == "" {
			label = "-"
		}
		rows = append(rows, []string{components.ClampTextWidthEllipsis(label, contentWidth)})
	}

	table := components.TableGridWithActiveRow(cols, rows, contentWidth, m.formatIndex)
	return table + "\n\n" + MutedStyle.Render("enter: select | esc: back")
}

// handleResourceKeys handles handle resource keys.
func (m ImportExportModel) handleResourceKeys(msg tea.KeyMsg) (ImportExportModel, tea.Cmd) {
	switch {
	case isDown(msg):
		if m.resourceIndex < len(m.resources)-1 {
			m.resourceIndex++
		}
	case isUp(msg):
		if m.resourceIndex > 0 {
			m.resourceIndex--
		}
	case isEnter(msg):
		m.step = stepFormat
	case isBack(msg):
		m.closed = true
	}
	return m, nil
}

// handleFormatKeys handles handle format keys.
func (m ImportExportModel) handleFormatKeys(msg tea.KeyMsg) (ImportExportModel, tea.Cmd) {
	switch {
	case isDown(msg):
		if m.formatIndex < len(m.formats)-1 {
			m.formatIndex++
		}
	case isUp(msg):
		if m.formatIndex > 0 {
			m.formatIndex--
		}
	case isEnter(msg):
		m.step = stepPath
	case isBack(msg):
		m.step = stepResource
	}
	return m, nil
}

// handlePathKeys handles handle path keys.
func (m ImportExportModel) handlePathKeys(msg tea.KeyMsg) (ImportExportModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.step = stepFormat
	case isEnter(msg):
		if strings.TrimSpace(m.path) == "" {
			return m, nil
		}
		m.step = stepRunning
		return m, m.run()
	case msg.Type == tea.KeyBackspace:
		if len(m.path) > 0 {
			m.path = m.path[:len(m.path)-1]
		}
	case msg.Type == tea.KeyRunes:
		m.path += msg.String()
	}
	return m, nil
}

// run runs run.
func (m ImportExportModel) run() tea.Cmd {
	mode := m.mode
	resource := m.resources[m.resourceIndex].value
	format := m.formats[m.formatIndex]
	path := m.path
	client := m.client

	return func() tea.Msg {
		if mode == importMode {
			return runImport(client, resource, format, path)
		}
		return runExport(client, resource, format, path)
	}
}

// runImport runs run import.
func runImport(client *api.Client, resource, format, path string) tea.Msg {
	data, err := os.ReadFile(path)
	if err != nil {
		return importExportErrorMsg{err: err}
	}
	payload := api.BulkImportRequest{
		Format: format,
		Data:   string(data),
	}
	var result *api.BulkImportResult
	switch resource {
	case "entities":
		result, err = client.ImportEntities(payload)
	case "context":
		result, err = client.ImportContext(payload)
	case "relationships":
		result, err = client.ImportRelationships(payload)
	case "jobs":
		result, err = client.ImportJobs(payload)
	default:
		return importExportErrorMsg{err: fmt.Errorf("unknown import resource")}
	}
	if err != nil {
		return importExportErrorMsg{err: err}
	}
	summary := fmt.Sprintf("Created %d, Failed %d", result.Created, result.Failed)
	details := []string{}
	if len(result.Errors) > 0 {
		for i, entry := range result.Errors {
			if i >= 5 {
				break
			}
			details = append(details, fmt.Sprintf("Row %d: %s", entry.Row, entry.Error))
		}
		if len(result.Errors) > 5 {
			details = append(details, fmt.Sprintf("...and %d more errors", len(result.Errors)-5))
		}
	}
	return importExportDoneMsg{summary: summary, details: details}
}

// runExport runs run export.
func runExport(client *api.Client, resource, format, path string) tea.Msg {
	params := api.QueryParams{
		"format": format,
	}
	var result *api.ExportResult
	var err error
	switch resource {
	case "entities":
		result, err = client.ExportEntities(params)
	case "context":
		result, err = client.ExportContextItems(params)
	case "relationships":
		result, err = client.ExportRelationships(params)
	case "jobs":
		result, err = client.ExportJobs(params)
	case "snapshot":
		result, err = client.ExportContext(params)
	default:
		return importExportErrorMsg{err: fmt.Errorf("unknown export resource")}
	}
	if err != nil {
		return importExportErrorMsg{err: err}
	}
	content := result.Content
	if result.Format == "json" {
		payload, err := importExportMarshalIndent(result.Items, "", "  ")
		if err != nil {
			return importExportErrorMsg{err: err}
		}
		content = string(payload)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return importExportErrorMsg{err: err}
	}
	summary := fmt.Sprintf("Exported %d %s to %s", result.Count, resource, path)
	return importExportDoneMsg{summary: summary}
}

// importExportResourcesForMode handles import export resources for mode.
func importExportResourcesForMode(mode importExportMode) []importExportResource {
	if mode == importMode {
		return []importExportResource{
			{label: "Entities", value: "entities"},
			{label: "Context", value: "context"},
			{label: "Relationships", value: "relationships"},
			{label: "Jobs", value: "jobs"},
		}
	}
	return []importExportResource{
		{label: "Entities", value: "entities"},
		{label: "Context", value: "context"},
		{label: "Relationships", value: "relationships"},
		{label: "Jobs", value: "jobs"},
		{label: "Snapshot", value: "snapshot"},
	}
}

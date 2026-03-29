package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type taxonomyLoadedMsg struct {
	kind  string
	items []api.TaxonomyEntry
}

type taxonomyActionDoneMsg struct{}

type taxonomyPromptMode int

const (
	taxPromptNone taxonomyPromptMode = iota
	taxPromptCreateName
	taxPromptCreateDescription
	taxPromptEditName
	taxPromptEditDescription
	taxPromptFilter
)

var taxonomyKinds = []struct {
	Label string
	Path  string
}{
	{Label: "Scopes", Path: "scopes"},
	{Label: "Entity Types", Path: "entity-types"},
	{Label: "Relationship Types", Path: "relationship-types"},
	{Label: "Log Types", Path: "log-types"},
}

// taxonomyKindPath handles taxonomy kind path.
func (m ProfileModel) taxonomyKindPath() string {
	if m.taxKind < 0 || m.taxKind >= len(taxonomyKinds) {
		return taxonomyKinds[0].Path
	}
	return taxonomyKinds[m.taxKind].Path
}

// loadTaxonomy loads load taxonomy.
func (m ProfileModel) loadTaxonomy() tea.Msg {
	kind := m.taxonomyKindPath()
	items, err := m.client.ListTaxonomy(kind, m.taxIncludeInactive, m.taxSearch, 200, 0)
	if err != nil {
		return errMsg{err}
	}
	return taxonomyLoadedMsg{kind: kind, items: items}
}

// setTaxonomyItems sets set taxonomy items.
func (m *ProfileModel) setTaxonomyItems(items []api.TaxonomyEntry) {
	m.taxItems = items
	rows := make([]table.Row, len(items))
	for i, item := range items {
		rows[i] = table.Row{formatTaxonomyLine(item)}
	}
	m.taxList.SetRows(rows)
	m.taxList.SetCursor(0)
}

// formatTaxonomyLine handles format taxonomy line.
func formatTaxonomyLine(item api.TaxonomyEntry) string {
	name := components.SanitizeOneLine(item.Name)
	parts := []string{name}
	if item.IsBuiltin {
		parts = append(parts, TypeBadgeStyle.Render("builtin"))
	}
	if !item.IsActive {
		parts = append(parts, WarningStyle.Render("inactive"))
	}
	if item.Description != nil && strings.TrimSpace(*item.Description) != "" {
		parts = append(parts, MutedStyle.Render(components.SanitizeOneLine(*item.Description)))
	}
	return strings.Join(parts, "  ")
}

// selectedTaxonomy handles selected taxonomy.
func (m ProfileModel) selectedTaxonomy() *api.TaxonomyEntry {
	idx := m.taxList.Cursor()
	if idx < 0 || idx >= len(m.taxItems) {
		return nil
	}
	item := m.taxItems[idx]
	return &item
}

// openTaxPrompt handles open tax prompt.
func (m *ProfileModel) openTaxPrompt(mode taxonomyPromptMode, defaultValue string) {
	m.taxPromptMode = mode
	m.taxPromptInput.SetValue(defaultValue)
	m.taxPromptInput.Focus()
}

// taxonomyPromptTitle handles taxonomy prompt title.
func (m ProfileModel) taxonomyPromptTitle() string {
	switch m.taxPromptMode {
	case taxPromptCreateName:
		return "New Taxonomy Name"
	case taxPromptCreateDescription:
		return "New Taxonomy Description (optional)"
	case taxPromptEditName:
		return "Edit Taxonomy Name"
	case taxPromptEditDescription:
		return "Edit Taxonomy Description (optional)"
	case taxPromptFilter:
		return "Taxonomy Filter"
	default:
		return "Taxonomy"
	}
}

// handleTaxonomyPrompt handles handle taxonomy prompt.
func (m ProfileModel) handleTaxonomyPrompt(msg tea.KeyPressMsg) (ProfileModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.taxPromptMode = taxPromptNone
		m.taxPromptInput.Reset()
		m.taxPromptInput.Blur()
		m.taxPendingName = ""
		m.taxPendingDesc = ""
		m.taxEditID = ""
		return m, nil
	case isEnter(msg):
		return m.submitTaxonomyPrompt()
	default:
		var cmd tea.Cmd
		m.taxPromptInput, cmd = m.taxPromptInput.Update(msg)
		return m, cmd
	}
}

// submitTaxonomyPrompt handles submit taxonomy prompt.
func (m ProfileModel) submitTaxonomyPrompt() (ProfileModel, tea.Cmd) {
	switch m.taxPromptMode {
	case taxPromptCreateName:
		name := strings.TrimSpace(m.taxPromptInput.Value())
		if name == "" {
			m.taxPromptMode = taxPromptNone
			m.taxPromptInput.Reset()
			return m, func() tea.Msg { return errMsg{fmt.Errorf("taxonomy name required")} }
		}
		m.taxPendingName = name
		m.openTaxPrompt(taxPromptCreateDescription, "")
		return m, nil
	case taxPromptCreateDescription:
		desc := strings.TrimSpace(m.taxPromptInput.Value())
		input := api.CreateTaxonomyInput{
			Name:        m.taxPendingName,
			Description: desc,
		}
		kind := m.taxonomyKindPath()
		m.taxPromptMode = taxPromptNone
		m.taxPromptInput.Reset()
		m.taxPendingName = ""
		m.taxPendingDesc = ""
		m.taxLoading = true
		return m, func() tea.Msg {
			if _, err := m.client.CreateTaxonomy(kind, input); err != nil {
				return errMsg{err}
			}
			return taxonomyActionDoneMsg{}
		}
	case taxPromptEditName:
		name := strings.TrimSpace(m.taxPromptInput.Value())
		if name == "" {
			m.taxPromptMode = taxPromptNone
			m.taxPromptInput.Reset()
			return m, func() tea.Msg { return errMsg{fmt.Errorf("taxonomy name required")} }
		}
		m.taxPendingName = name
		m.openTaxPrompt(taxPromptEditDescription, m.taxPendingDesc)
		return m, nil
	case taxPromptEditDescription:
		name := m.taxPendingName
		desc := strings.TrimSpace(m.taxPromptInput.Value())
		id := m.taxEditID
		kind := m.taxonomyKindPath()
		m.taxPromptMode = taxPromptNone
		m.taxPromptInput.Reset()
		m.taxPendingName = ""
		m.taxPendingDesc = ""
		m.taxEditID = ""
		m.taxLoading = true
		return m, func() tea.Msg {
			_, err := m.client.UpdateTaxonomy(kind, id, api.UpdateTaxonomyInput{
				Name:        &name,
				Description: &desc,
			})
			if err != nil {
				return errMsg{err}
			}
			return taxonomyActionDoneMsg{}
		}
	case taxPromptFilter:
		m.taxSearch = strings.TrimSpace(m.taxPromptInput.Value())
		m.taxPromptMode = taxPromptNone
		m.taxPromptInput.Reset()
		m.taxLoading = true
		return m, m.loadTaxonomy
	default:
		return m, nil
	}
}

// taxonomyArchiveSelected handles taxonomy archive selected.
func (m ProfileModel) taxonomyArchiveSelected() (ProfileModel, tea.Cmd) {
	item := m.selectedTaxonomy()
	if item == nil {
		return m, nil
	}
	kind := m.taxonomyKindPath()
	m.taxLoading = true
	return m, func() tea.Msg {
		if _, err := m.client.ArchiveTaxonomy(kind, item.ID); err != nil {
			return errMsg{err}
		}
		return taxonomyActionDoneMsg{}
	}
}

// taxonomyActivateSelected handles taxonomy activate selected.
func (m ProfileModel) taxonomyActivateSelected() (ProfileModel, tea.Cmd) {
	item := m.selectedTaxonomy()
	if item == nil {
		return m, nil
	}
	kind := m.taxonomyKindPath()
	m.taxLoading = true
	return m, func() tea.Msg {
		if _, err := m.client.ActivateTaxonomy(kind, item.ID); err != nil {
			return errMsg{err}
		}
		return taxonomyActionDoneMsg{}
	}
}

// renderTaxonomy renders render taxonomy.
func (m ProfileModel) renderTaxonomy() string {
	var b strings.Builder

	kindTabs := make([]string, 0, len(taxonomyKinds))
	for i, kind := range taxonomyKinds {
		if i == m.taxKind {
			kindTabs = append(kindTabs, SelectedStyle.Render(kind.Label))
		} else {
			kindTabs = append(kindTabs, MutedStyle.Render(kind.Label))
		}
	}
	b.WriteString(components.CenterLine(strings.Join(kindTabs, "   "), m.width))
	b.WriteString("\n\n")

	if m.taxPromptMode != taxPromptNone {
		return b.String() + components.Indent(
			components.TextInputDialog(m.taxonomyPromptTitle(), m.taxPromptInput.View()),
			1,
		)
	}

	if m.taxLoading {
		return b.String() + components.Indent(
			components.Box(MutedStyle.Render("Loading taxonomy..."), m.width),
			1,
		)
	}

	if len(m.taxItems) == 0 {
		return b.String() + components.Indent(
			components.Box(MutedStyle.Render("No taxonomy rows found."), m.width),
			1,
		)
	}

	contentWidth := components.BoxContentWidth(m.width)

	filterText := m.taxSearch
	if filterText == "" {
		filterText = "-"
	}
	info := fmt.Sprintf(
		"%d rows  ·  include inactive: %t  ·  filter: %s",
		len(m.taxItems),
		m.taxIncludeInactive,
		filterText,
	)

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap
	}

	// 3 columns, 2 padding chars each = 6 padding total.
	availableCols := tableWidth - (3 * 2)
	if availableCols < 30 {
		availableCols = 30
	}

	flagsWidth := 14
	descWidth := 30
	nameWidth := availableCols - (flagsWidth + descWidth)
	if nameWidth < 14 {
		nameWidth = 14
		descWidth = availableCols - (nameWidth + flagsWidth)
		if descWidth < 14 {
			descWidth = 14
		}
	}

	tableRows := make([]table.Row, 0, len(m.taxItems))
	for _, item := range m.taxItems {
		name := strings.TrimSpace(components.SanitizeOneLine(item.Name))
		if name == "" {
			name = "-"
		}
		flags := []string{}
		if item.IsBuiltin {
			flags = append(flags, "builtin")
		}
		if !item.IsActive {
			flags = append(flags, "inactive")
		}
		flagText := "-"
		if len(flags) > 0 {
			flagText = strings.Join(flags, ", ")
		}

		desc := "-"
		if item.Description != nil && strings.TrimSpace(*item.Description) != "" {
			desc = strings.TrimSpace(*item.Description)
		}

		tableRows = append(tableRows, table.Row{
			components.ClampTextWidthEllipsis(name, nameWidth),
			components.ClampTextWidthEllipsis(flagText, flagsWidth),
			components.ClampTextWidthEllipsis(components.SanitizeOneLine(desc), descWidth),
		})
	}

	m.taxList.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Flags", Width: flagsWidth},
		{Title: "Description", Width: descWidth},
	})
	m.taxList.SetWidth(tableWidth)
	m.taxList.SetRows(tableRows)

	var previewItem *api.TaxonomyEntry
	if !m.sectionFocus {
		if idx := m.taxList.Cursor(); idx >= 0 && idx < len(m.taxItems) {
			previewItem = &m.taxItems[idx]
		}
	}

	tableView := components.TableBaseStyle.Render(m.taxList.View())
	preview := ""
	if previewItem != nil {
		content := m.renderTaxonomyPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	content := MutedStyle.Render(info) + "\n\n" + body + "\n"
	title := fmt.Sprintf("%s Taxonomy", taxonomyKinds[m.taxKind].Label)
	return b.String() + components.Indent(components.TitledBox(title, content, m.width), 1)
}

// renderTaxonomyPreview renders render taxonomy preview.
func (m ProfileModel) renderTaxonomyPreview(item api.TaxonomyEntry, width int) string {
	if width <= 0 {
		return ""
	}

	title := strings.TrimSpace(components.SanitizeOneLine(item.Name))
	if title == "" {
		title = "taxonomy"
	}

	status := "active"
	if !item.IsActive {
		status = "inactive"
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Status", status, width))
	if item.IsBuiltin {
		lines = append(lines, renderPreviewRow("Builtin", "true", width))
	}
	lines = append(lines, renderPreviewRow("ID", shortID(item.ID), width))
	if item.Description != nil && strings.TrimSpace(*item.Description) != "" {
		lines = append(lines, renderPreviewRow("Desc", strings.TrimSpace(*item.Description), width))
	}
	if item.IsSymmetric != nil {
		lines = append(lines, renderPreviewRow("Symmetric", fmt.Sprintf("%t", *item.IsSymmetric), width))
	}
	if item.Notes != "" {
		lines = append(lines, renderPreviewRow("Notes", truncateString(item.Notes, 80), width))
	}

	return padPreviewLines(lines, width)
}

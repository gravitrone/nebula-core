package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type searchResultsMsg struct {
	query    string
	mode     string
	entities []api.Entity
	context  []api.Context
	jobs     []api.Job
	semantic []api.SemanticSearchResult
}

type searchSelectionMsg struct {
	kind    string
	entity  *api.Entity
	context *api.Context
	job     *api.Job
	rel     *api.Relationship
	log     *api.Log
	file    *api.File
	proto   *api.Protocol
}

type searchEntry struct {
	kind    string
	id      string
	label   string
	desc    string
	entity  *api.Entity
	context *api.Context
	job     *api.Job
	rel     *api.Relationship
	log     *api.Log
	file    *api.File
	proto   *api.Protocol
}

type SearchModel struct {
	client    *api.Client
	textInput textinput.Model
	mode      string
	loading   bool
	spinner   spinner.Model
	dataTable table.Model
	items     []searchEntry
	width     int

}

const (
	searchModeText     = "text"
	searchModeSemantic = "semantic"
)

// NewSearchModel builds the search UI model.
func NewSearchModel(client *api.Client) SearchModel {
	ti := components.NewNebulaTextInput("Search...")
	ti.Prompt = ""
	ti.Focus()
	return SearchModel{
		client:    client,
		textInput: ti,
		spinner:   components.NewNebulaSpinner(),
		mode:      searchModeText,
		dataTable: components.NewNebulaTable(nil, 12),
	}
}

// Init handles init.
func (m SearchModel) Init() tea.Cmd {
	return m.textInput.Focus()
}

// Update updates update.
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case searchResultsMsg:
		if strings.TrimSpace(msg.query) != strings.TrimSpace(m.textInput.Value()) {
			return m, nil
		}
		if msg.mode != m.mode {
			return m, nil
		}
		m.loading = false
		if m.mode == searchModeSemantic {
			m.items = buildSemanticEntries(msg.semantic)
		} else {
			m.items = buildSearchEntries(msg.query, msg.entities, msg.context, msg.jobs)
		}
		rows := make([]table.Row, len(m.items))
		for i, item := range m.items {
			rows[i] = table.Row{fmt.Sprintf(
				"%s  %s",
				components.SanitizeText(item.label),
				MutedStyle.Render(components.SanitizeText(item.desc)),
			)}
		}
		m.dataTable.SetRows(rows)
		m.dataTable.SetCursor(0)
		return m, nil
	case tea.KeyPressMsg:
		switch {
		case isBack(msg):
			if m.textInput.Value() != "" {
				m.textInput.Reset()
				m.items = nil
				m.dataTable.SetRows(nil)
				m.dataTable.SetCursor(0)
				m.loading = false
				return m, nil
			}
		case isDown(msg):
			m.dataTable.MoveDown(1)
			return m, nil
		case isUp(msg):
			m.dataTable.MoveUp(1)
			return m, nil
		case isKey(msg, "tab"):
			if m.mode == searchModeText {
				m.mode = searchModeSemantic
			} else {
				m.mode = searchModeText
			}
			if strings.TrimSpace(m.textInput.Value()) == "" {
				m.loading = false
				m.items = nil
				m.dataTable.SetRows(nil)
				m.dataTable.SetCursor(0)
				return m, nil
			}
			if cmd := m.search(m.textInput.Value()); cmd != nil {
				return m, tea.Batch(cmd, m.spinner.Tick)
			}
			return m, nil
		case isEnter(msg):
			if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
				entry := m.items[idx]
				return m, m.emitSelection(entry)
			}
			return m, nil
		}

		// Delegate all other key handling to the textinput.
		prevValue := m.textInput.Value()
		var tiCmd tea.Cmd
		m.textInput, tiCmd = m.textInput.Update(msg)
		newValue := m.textInput.Value()

		// Reject leading spaces.
		if strings.TrimSpace(prevValue) == "" && strings.TrimSpace(newValue) == "" && newValue != prevValue {
			m.textInput.Reset()
			return m, tiCmd
		}

		if newValue != prevValue {
			if cmd := m.search(newValue); cmd != nil {
				return m, tea.Batch(tiCmd, cmd, m.spinner.Tick)
			}
		}
		return m, tiCmd
	}

	// Forward non-key messages (e.g. cursor blink) to the textinput.
	var tiCmd tea.Cmd
	m.textInput, tiCmd = m.textInput.Update(msg)
	return m, tiCmd
}

// View handles view.
func (m SearchModel) View() string {
	var b strings.Builder
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Mode: %s (tab to toggle)", m.mode)))
	b.WriteString("\n\n")
	b.WriteString(MetaKeyStyle.Render("Query") + MetaPunctStyle.Render(": ") + m.textInput.View())
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + " " + MutedStyle.Render("Searching..."))
	} else if strings.TrimSpace(m.textInput.Value()) == "" {
		b.WriteString(MutedStyle.Render("Type to search."))
	} else if len(m.items) == 0 {
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

		kindWidth := 10
		infoWidth := 28
		titleWidth := availableCols - (kindWidth + infoWidth)
		if titleWidth < 16 {
			titleWidth = 16
			infoWidth = availableCols - (titleWidth + kindWidth)
			if infoWidth < 14 {
				infoWidth = 14
			}
		}
		if titleWidth > 40 {
			titleWidth = 40
		}

		tableRows := make([]table.Row, len(m.items))
		for i, entry := range m.items {
			kind := strings.TrimSpace(components.SanitizeOneLine(entry.kind))
			if kind == "" {
				kind = "-"
			}
			title := strings.TrimSpace(components.SanitizeOneLine(entry.label))
			if title == "" {
				title = "-"
			}
			info := strings.TrimSpace(components.SanitizeOneLine(entry.desc))
			if info == "" {
				info = "-"
			}

			tableRows[i] = table.Row{
				components.ClampTextWidthEllipsis(title, titleWidth),
				components.ClampTextWidthEllipsis(kind, kindWidth),
				components.ClampTextWidthEllipsis(info, infoWidth),
			}
		}

		m.dataTable.SetColumns([]table.Column{
			{Title: "Title", Width: titleWidth},
			{Title: "Kind", Width: kindWidth},
			{Title: "Info", Width: infoWidth},
		})
		actualTableWidth := titleWidth + kindWidth + infoWidth + cellPadding
		m.dataTable.SetWidth(actualTableWidth)
		m.dataTable.SetRows(tableRows)

		countLine := ""
		if query := strings.TrimSpace(m.textInput.Value()); query != "" {
			countLine = MutedStyle.Render(fmt.Sprintf("%d results · search: %s", len(m.items), query))
		}
		tableView := components.TableBaseStyle.Render(m.dataTable.View())
		preview := ""
		var previewItem *searchEntry
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			previewItem = &m.items[idx]
		}
		if previewItem != nil {
			content := m.renderSearchPreview(*previewItem, previewBoxContentWidth(previewWidth))
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
		b.WriteString(lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, result))
	}

	return lipgloss.JoinVertical(lipgloss.Left, components.Indent(b.String(), 1), m.renderStatusHints())
}

// renderStatusHints builds the bottom status bar with keycap pill hints.
func (m SearchModel) renderStatusHints() string {
	hints := []string{
		components.Hint("1-9/0", "Tabs"),
		components.Hint("/", "Search"),
		components.Hint("q", "Quit"),
		components.Hint("enter", "Select"),
		components.Hint("tab", "Switch"),
	}
	return components.StatusBar(hints, m.width)
}

// renderSearchPreview renders render search preview.
func (m SearchModel) renderSearchPreview(entry searchEntry, width int) string {
	if width <= 0 {
		return ""
	}

	title := strings.TrimSpace(components.SanitizeOneLine(entry.label))
	if title == "" {
		title = "result"
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(title, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	kind := strings.TrimSpace(components.SanitizeOneLine(entry.kind))
	if kind == "" {
		kind = "-"
	}
	lines = append(lines, renderPreviewRow("Kind", kind, width))
	if strings.TrimSpace(entry.id) != "" {
		lines = append(lines, renderPreviewRow("ID", shortID(entry.id), width))
	}
	if strings.TrimSpace(entry.desc) != "" {
		lines = append(lines, renderPreviewRow("Info", entry.desc, width))
	}

	if entry.entity != nil {
		typ := strings.TrimSpace(components.SanitizeOneLine(entry.entity.Type))
		if typ != "" {
			lines = append(lines, renderPreviewRow("Type", typ, width))
		}
		status := strings.TrimSpace(components.SanitizeOneLine(entry.entity.Status))
		if status != "" {
			lines = append(lines, renderPreviewRow("Status", status, width))
		}
		if len(entry.entity.Tags) > 0 {
			lines = append(lines, renderPreviewRow("Tags", strings.Join(entry.entity.Tags, ", "), width))
		}
	} else if entry.context != nil {
		src := strings.TrimSpace(components.SanitizeOneLine(entry.context.SourceType))
		if src != "" {
			lines = append(lines, renderPreviewRow("Source", src, width))
		}
		status := strings.TrimSpace(components.SanitizeOneLine(entry.context.Status))
		if status != "" {
			lines = append(lines, renderPreviewRow("Status", status, width))
		}
		if entry.context.URL != nil && strings.TrimSpace(*entry.context.URL) != "" {
			lines = append(lines, renderPreviewRow("URL", strings.TrimSpace(*entry.context.URL), width))
		}
		if len(entry.context.Tags) > 0 {
			lines = append(lines, renderPreviewRow("Tags", strings.Join(entry.context.Tags, ", "), width))
		}
		snippet := ""
		if entry.context.Content != nil {
			snippet = truncateString(
				strings.TrimSpace(components.SanitizeText(*entry.context.Content)),
				80,
			)
		} else if entry.context.URL != nil {
			snippet = truncateString(
				strings.TrimSpace(components.SanitizeText(*entry.context.URL)),
				80,
			)
		}
		if strings.TrimSpace(snippet) != "" {
			lines = append(lines, renderPreviewRow("Preview", strings.TrimSpace(snippet), width))
		}
	} else if entry.job != nil {
		status := strings.TrimSpace(components.SanitizeOneLine(entry.job.Status))
		if status != "" {
			lines = append(lines, renderPreviewRow("Status", status, width))
		}
		if entry.job.Priority != nil && strings.TrimSpace(*entry.job.Priority) != "" {
			lines = append(lines, renderPreviewRow("Priority", strings.TrimSpace(*entry.job.Priority), width))
		}
		if entry.job.Description != nil {
			desc := truncateString(
				strings.TrimSpace(components.SanitizeText(*entry.job.Description)),
				80,
			)
			if strings.TrimSpace(desc) != "" {
				lines = append(lines, renderPreviewRow("Description", strings.TrimSpace(desc), width))
			}
		}
	}

	return padPreviewLines(lines, width)
}

// search handles search.
func (m *SearchModel) search(query string) tea.Cmd {
	q := strings.TrimSpace(query)
	if q == "" {
		m.loading = false
		m.items = nil
		m.dataTable.SetRows(nil)
		m.dataTable.SetCursor(0)
		return nil
	}
	m.loading = true
	mode := m.mode
	return func() tea.Msg {
		if mode == searchModeSemantic {
			results, err := m.client.SemanticSearch(q, []string{"entity", "context"}, 20)
			if err != nil {
				return errMsg{err}
			}
			return searchResultsMsg{
				query:    q,
				mode:     mode,
				semantic: results,
			}
		}
		entities, err := m.client.QueryEntities(api.QueryParams{
			"search_text": q,
			"limit":       "20",
		})
		if err != nil {
			return errMsg{err}
		}
		context, err := m.client.QueryContext(api.QueryParams{
			"search_text": q,
			"limit":       "20",
		})
		if err != nil {
			return errMsg{err}
		}
		jobs, err := m.client.QueryJobs(api.QueryParams{
			"search_text": q,
			"limit":       "20",
		})
		if err != nil {
			return errMsg{err}
		}
		return searchResultsMsg{
			query:    q,
			mode:     mode,
			entities: filterEntitiesByQuery(entities, q),
			context:  filterContextByQuery(context, q),
			jobs:     filterJobsByQuery(jobs, q),
		}
	}
}

// emitSelection handles emit selection.
func (m SearchModel) emitSelection(entry searchEntry) tea.Cmd {
	return func() tea.Msg {
		switch entry.kind {
		case "entity":
			if entry.entity == nil {
				item, err := m.client.GetEntity(entry.id)
				if err != nil {
					return errMsg{err}
				}
				return searchSelectionMsg{kind: entry.kind, entity: item}
			}
		case "context":
			if entry.context == nil {
				item, err := m.client.GetContext(entry.id)
				if err != nil {
					return errMsg{err}
				}
				return searchSelectionMsg{kind: entry.kind, context: item}
			}
		case "job":
			if entry.job == nil {
				item, err := m.client.GetJob(entry.id)
				if err != nil {
					return errMsg{err}
				}
				return searchSelectionMsg{kind: entry.kind, job: item}
			}
		case "relationship":
			// Relationships are loaded from query results in search mode.
			// Keep this branch non-fetching to avoid depending on a single-item API.
		case "log":
			if entry.log == nil {
				item, err := m.client.GetLog(entry.id)
				if err != nil {
					return errMsg{err}
				}
				return searchSelectionMsg{kind: entry.kind, log: item}
			}
		case "file":
			if entry.file == nil {
				item, err := m.client.GetFile(entry.id)
				if err != nil {
					return errMsg{err}
				}
				return searchSelectionMsg{kind: entry.kind, file: item}
			}
		case "protocol":
			if entry.proto == nil {
				item, err := m.client.GetProtocol(entry.id)
				if err != nil {
					return errMsg{err}
				}
				return searchSelectionMsg{kind: entry.kind, proto: item}
			}
		}
		return searchSelectionMsg{
			kind:    entry.kind,
			entity:  entry.entity,
			context: entry.context,
			job:     entry.job,
			rel:     entry.rel,
			log:     entry.log,
			file:    entry.file,
			proto:   entry.proto,
		}
	}
}

// buildSemanticEntries builds build semantic entries.
func buildSemanticEntries(items []api.SemanticSearchResult) []searchEntry {
	out := make([]searchEntry, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = item.ID
		}
		descParts := []string{
			fmt.Sprintf("%.2f", item.Score),
		}
		if strings.TrimSpace(item.Subtitle) != "" {
			descParts = append(descParts, item.Subtitle)
		}
		if strings.TrimSpace(item.Snippet) != "" {
			descParts = append(descParts, item.Snippet)
		}
		out = append(out, searchEntry{
			kind:  item.Kind,
			id:    item.ID,
			label: components.SanitizeText(title),
			desc:  components.SanitizeText(strings.Join(descParts, " · ")),
		})
	}
	return out
}

// buildSearchEntries builds build search entries.
func buildSearchEntries(query string, entities []api.Entity, context []api.Context, jobs []api.Job) []searchEntry {
	filteredEntities := filterEntitiesByQuery(entities, query)
	filteredContext := filterContextByQuery(context, query)
	filteredJobs := filterJobsByQuery(jobs, query)

	items := make([]searchEntry, 0, len(filteredEntities)+len(filteredContext)+len(filteredJobs))
	for _, e := range filteredEntities {
		kind := "entity"
		descType := e.Type
		if descType == "" {
			descType = "entity"
		}
		entity := e
		items = append(items, searchEntry{
			kind:   kind,
			id:     e.ID,
			label:  components.SanitizeText(e.Name),
			desc:   components.SanitizeText(fmt.Sprintf("%s · %s", descType, shortID(e.ID))),
			entity: &entity,
		})
	}
	for _, k := range filteredContext {
		kind := "context"
		descType := k.SourceType
		if descType == "" {
			descType = "context"
		}
		contextItem := k
		items = append(items, searchEntry{
			kind:    kind,
			id:      k.ID,
			label:   components.SanitizeText(k.Name),
			desc:    components.SanitizeText(fmt.Sprintf("%s · %s", descType, shortID(k.ID))),
			context: &contextItem,
		})
	}
	for _, j := range filteredJobs {
		kind := "job"
		desc := j.Status
		if desc == "" {
			desc = "job"
		}
		job := j
		items = append(items, searchEntry{
			kind:  kind,
			id:    j.ID,
			label: components.SanitizeText(j.Title),
			desc:  components.SanitizeText(fmt.Sprintf("%s · %s", desc, shortID(j.ID))),
			job:   &job,
		})
	}
	return items
}

// buildPaletteSearchEntries builds build palette search entries.
func buildPaletteSearchEntries(
	query string,
	entities []api.Entity,
	context []api.Context,
	jobs []api.Job,
	rels []api.Relationship,
	logs []api.Log,
	files []api.File,
	protos []api.Protocol,
) []searchEntry {
	items := buildSearchEntries(query, entities, context, jobs)

	for _, r := range filterRelationshipsByQuery(rels, query) {
		rel := r
		edge := strings.TrimSpace(components.SanitizeText(fmt.Sprintf("%s -> %s", r.SourceName, r.TargetName)))
		if edge == "->" {
			edge = components.SanitizeText(fmt.Sprintf("%s -> %s", shortID(r.SourceID), shortID(r.TargetID)))
		}
		label := components.SanitizeText(strings.TrimSpace(fmt.Sprintf("%s (%s)", r.Type, edge)))
		if strings.TrimSpace(label) == "" || strings.HasPrefix(label, "(") {
			label = components.SanitizeText(edge)
		}
		desc := r.Status
		if desc == "" {
			desc = "relationship"
		}
		items = append(items, searchEntry{
			kind:  "relationship",
			id:    r.ID,
			label: label,
			desc:  components.SanitizeText(fmt.Sprintf("%s · %s", desc, shortID(r.ID))),
			rel:   &rel,
		})
	}

	for _, l := range filterLogsByQuery(logs, query) {
		log := l
		label := strings.TrimSpace(components.SanitizeText("log: " + l.LogType))
		if label == "" || label == "log:" {
			label = "log"
		}
		desc := l.Status
		if desc == "" {
			desc = "log"
		}
		items = append(items, searchEntry{
			kind:  "log",
			id:    l.ID,
			label: label,
			desc:  components.SanitizeText(fmt.Sprintf("%s · %s", desc, shortID(l.ID))),
			log:   &log,
		})
	}

	for _, f := range filterFilesByQuery(files, query) {
		file := f
		label := strings.TrimSpace(components.SanitizeText(f.Filename))
		if label == "" {
			label = shortID(f.ID)
		}
		desc := "file"
		if mime := fileMimeType(f); mime != "" {
			desc = mime
		}
		items = append(items, searchEntry{
			kind:  "file",
			id:    f.ID,
			label: label,
			desc:  components.SanitizeText(fmt.Sprintf("%s · %s", desc, shortID(f.ID))),
			file:  &file,
		})
	}

	for _, p := range filterProtocolsByQuery(protos, query) {
		protocol := p
		label := strings.TrimSpace(components.SanitizeText(p.Title))
		if label == "" {
			label = strings.TrimSpace(components.SanitizeText(p.Name))
		}
		if label == "" {
			label = shortID(p.ID)
		}
		desc := "protocol"
		if kind := protocolType(p); kind != "" {
			desc = kind
		}
		items = append(items, searchEntry{
			kind:  "protocol",
			id:    p.ID,
			label: label,
			desc:  components.SanitizeText(fmt.Sprintf("%s · %s", desc, shortID(p.ID))),
			proto: &protocol,
		})
	}

	return items
}

// filterEntitiesByQuery handles filter entities by query.
func filterEntitiesByQuery(items []api.Entity, query string) []api.Entity {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.Entity, 0, len(items))
	for _, e := range items {
		name, typ := normalizeEntityNameType(e.Name, e.Type)
		haystack := strings.ToLower(strings.Join([]string{name, typ, e.ID}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, e)
		}
	}
	return out
}

// filterContextByQuery handles filter context by query.
func filterContextByQuery(items []api.Context, query string) []api.Context {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.Context, 0, len(items))
	for _, k := range items {
		if strings.Contains(strings.ToLower(k.Name), q) || strings.Contains(strings.ToLower(k.ID), q) {
			out = append(out, k)
		}
	}
	return out
}

// filterJobsByQuery handles filter jobs by query.
func filterJobsByQuery(items []api.Job, query string) []api.Job {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.Job, 0, len(items))
	for _, j := range items {
		if strings.Contains(strings.ToLower(j.Title), q) || strings.Contains(strings.ToLower(j.ID), q) {
			out = append(out, j)
		}
	}
	return out
}

// filterRelationshipsByQuery handles filter relationships by query.
func filterRelationshipsByQuery(items []api.Relationship, query string) []api.Relationship {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.Relationship, 0, len(items))
	for _, r := range items {
		haystack := strings.ToLower(strings.Join([]string{
			r.Type, r.Status, r.SourceName, r.TargetName, r.SourceID, r.TargetID, r.ID,
		}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, r)
		}
	}
	return out
}

// filterLogsByQuery handles filter logs by query.
func filterLogsByQuery(items []api.Log, query string) []api.Log {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.Log, 0, len(items))
	for _, l := range items {
		haystack := strings.ToLower(strings.Join([]string{
			l.ID, l.LogType, l.Status, l.Content,
		}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, l)
		}
	}
	return out
}

// filterFilesByQuery handles filter files by query.
func filterFilesByQuery(items []api.File, query string) []api.File {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.File, 0, len(items))
	for _, f := range items {
		mime := fileMimeType(f)
		haystack := strings.ToLower(strings.Join([]string{
			f.ID, f.Filename, f.URI, f.FilePath, mime, f.Status,
		}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, f)
		}
	}
	return out
}

// filterProtocolsByQuery handles filter protocols by query.
func filterProtocolsByQuery(items []api.Protocol, query string) []api.Protocol {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.Protocol, 0, len(items))
	for _, p := range items {
		kind := protocolType(p)
		haystack := strings.ToLower(strings.Join([]string{
			p.ID, p.Name, p.Title, kind, p.Status,
		}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, p)
		}
	}
	return out
}

// fileMimeType handles file mime type.
func fileMimeType(file api.File) string {
	if file.MimeType == nil {
		return ""
	}
	return strings.TrimSpace(*file.MimeType)
}

// protocolType handles protocol type.
func protocolType(protocol api.Protocol) string {
	if protocol.ProtocolType == nil {
		return ""
	}
	return strings.TrimSpace(*protocol.ProtocolType)
}

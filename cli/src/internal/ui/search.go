package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	client  *api.Client
	query   string
	mode    string
	loading bool
	list    *components.List
	items   []searchEntry
	width   int
	height  int
}

const (
	searchModeText     = "text"
	searchModeSemantic = "semantic"
)

// NewSearchModel builds the search UI model.
func NewSearchModel(client *api.Client) SearchModel {
	return SearchModel{
		client: client,
		mode:   searchModeText,
		list:   components.NewList(12),
	}
}

func (m SearchModel) Init() tea.Cmd {
	return nil
}

func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case searchResultsMsg:
		if strings.TrimSpace(msg.query) != strings.TrimSpace(m.query) {
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
		labels := make([]string, len(m.items))
		for i, item := range m.items {
			labels[i] = fmt.Sprintf(
				"%s  %s",
				components.SanitizeText(item.label),
				MutedStyle.Render(components.SanitizeText(item.desc)),
			)
		}
		m.list.SetItems(labels)
		return m, nil
	case tea.KeyMsg:
		switch {
		case isBack(msg):
			if m.query != "" {
				m.query = ""
				m.items = nil
				m.list.SetItems(nil)
				m.loading = false
				return m, nil
			}
		case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
			if m.query != "" {
				m.query = ""
				m.items = nil
				m.list.SetItems(nil)
				m.loading = false
				return m, nil
			}
		case isKey(msg, "backspace", "delete"):
			if len(m.query) > 0 {
				m.query = m.query[:len(m.query)-1]
				return m, m.search(m.query)
			}
		case isDown(msg):
			m.list.Down()
		case isUp(msg):
			m.list.Up()
		case isKey(msg, "tab"):
			if m.mode == searchModeText {
				m.mode = searchModeSemantic
			} else {
				m.mode = searchModeText
			}
			if strings.TrimSpace(m.query) == "" {
				m.loading = false
				m.items = nil
				m.list.SetItems(nil)
				return m, nil
			}
			return m, m.search(m.query)
		case isEnter(msg):
			if idx := m.list.Selected(); idx < len(m.items) {
				entry := m.items[idx]
				return m, m.emitSelection(entry)
			}
		default:
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
				if ch == " " && m.query == "" {
					return m, nil
				}
				m.query += ch
				return m, m.search(m.query)
			}
		}
	}
	return m, nil
}

func (m SearchModel) View() string {
	var b strings.Builder
	b.WriteString(MutedStyle.Render(fmt.Sprintf("Mode: %s (tab to toggle)", m.mode)))
	b.WriteString("\n\n")
	query := components.SanitizeText(m.query)
	queryWidth := components.BoxContentWidth(m.width) - 8
	if queryWidth < 10 {
		queryWidth = 10
	}
	query = components.ClampTextWidthEllipsis(query, queryWidth)
	b.WriteString(MetaKeyStyle.Render("Query") + MetaPunctStyle.Render(": ") + SelectedStyle.Render(query))
	b.WriteString(AccentStyle.Render("█"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(MutedStyle.Render("Searching..."))
	} else if strings.TrimSpace(m.query) == "" {
		b.WriteString(MutedStyle.Render("Type to search."))
	} else if len(m.items) == 0 {
		b.WriteString(MutedStyle.Render("No matches."))
	} else {
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
		if br := lipgloss.RoundedBorder().Left; br != "" {
			sepWidth = lipgloss.Width(br)
		}

		// 3 columns -> 2 separators.
		availableCols := tableWidth - (2 * sepWidth)
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

		cols := []components.TableColumn{
			{Header: "Title", Width: titleWidth, Align: lipgloss.Left},
			{Header: "Kind", Width: kindWidth, Align: lipgloss.Left},
			{Header: "Info", Width: infoWidth, Align: lipgloss.Left},
		}

		tableRows := make([][]string, 0, len(visible))
		activeRowRel := -1
		var previewItem *searchEntry
		if idx := m.list.Selected(); idx >= 0 && idx < len(m.items) {
			previewItem = &m.items[idx]
		}

		for i := range visible {
			absIdx := m.list.RelToAbs(i)
			if absIdx < 0 || absIdx >= len(m.items) {
				continue
			}
			entry := m.items[absIdx]

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

			if m.list.IsSelected(absIdx) {
				activeRowRel = len(tableRows)
			}

			tableRows = append(tableRows, []string{
				components.ClampTextWidthEllipsis(title, titleWidth),
				components.ClampTextWidthEllipsis(kind, kindWidth),
				components.ClampTextWidthEllipsis(info, infoWidth),
			})
		}

		countLine := MutedStyle.Render(fmt.Sprintf("%d results", len(m.items)))
		table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
		preview := ""
		if previewItem != nil {
			content := m.renderSearchPreview(*previewItem, previewBoxContentWidth(previewWidth))
			preview = renderPreviewBox(content, previewWidth)
		}

		body := table
		if sideBySide && preview != "" {
			body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
		} else if preview != "" {
			body = table + "\n\n" + preview
		}

		b.WriteString(countLine)
		b.WriteString("\n\n")
		b.WriteString(body)
	}

	return components.Indent(components.TitledBox("Search", b.String(), m.width), 1)
}

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
		if snippet := previewStringValue(entry.context.Metadata, "snippet"); snippet != "" {
			lines = append(lines, renderPreviewRow("Snippet", snippet, width))
		}
	} else if entry.job != nil {
		status := strings.TrimSpace(components.SanitizeOneLine(entry.job.Status))
		if status != "" {
			lines = append(lines, renderPreviewRow("Status", status, width))
		}
		if entry.job.Priority != nil && strings.TrimSpace(*entry.job.Priority) != "" {
			lines = append(lines, renderPreviewRow("Priority", strings.TrimSpace(*entry.job.Priority), width))
		}
		if metaPreview := metadataPreview(map[string]any(entry.job.Metadata), 80); metaPreview != "" {
			lines = append(lines, renderPreviewRow("Meta", metaPreview, width))
		}
	}

	return padPreviewLines(lines, width)
}

func (m *SearchModel) search(query string) tea.Cmd {
	q := strings.TrimSpace(query)
	if q == "" {
		m.loading = false
		m.items = nil
		m.list.SetItems(nil)
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
		if strings.TrimSpace(label) == "" || strings.HasPrefix(label, " (") {
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

func filterLogsByQuery(items []api.Log, query string) []api.Log {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]api.Log, 0, len(items))
	for _, l := range items {
		haystack := strings.ToLower(strings.Join([]string{
			l.ID, l.LogType, l.Status, fmt.Sprintf("%v", l.Value),
		}, " "))
		if strings.Contains(haystack, q) {
			out = append(out, l)
		}
	}
	return out
}

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

func fileMimeType(file api.File) string {
	if file.MimeType == nil {
		return ""
	}
	return strings.TrimSpace(*file.MimeType)
}

func protocolType(protocol api.Protocol) string {
	if protocol.ProtocolType == nil {
		return ""
	}
	return strings.TrimSpace(*protocol.ProtocolType)
}

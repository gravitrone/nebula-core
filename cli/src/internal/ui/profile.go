package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type keysLoadedMsg struct{ items []api.APIKey }
type agentsLoadedMsg struct{ items []api.Agent }
type keyCreatedMsg struct{ resp *api.CreateKeyResponse }
type keyRevokedMsg struct{}
type agentUpdatedMsg struct{}
type apiKeySavedMsg struct{}
type pendingLimitSavedMsg struct{ limit int }

// --- Profile Model ---

type ProfileModel struct {
	client *api.Client
	config *config.Config

	section      int // 0 = keys, 1 = agents, 2 = taxonomy
	sectionFocus bool

	keys        []api.APIKey
	keyList     *components.List
	agents      []api.Agent
	agentList   *components.List
	agentDetail *api.Agent

	loading          bool
	creating         bool
	createBuf        string
	createdKey       string
	editAPIKey       bool
	apiKeyBuf        string
	editPendingLimit bool
	pendingLimitBuf  string

	taxKind            int
	taxIncludeInactive bool
	taxSearch          string
	taxLoading         bool
	taxItems           []api.TaxonomyEntry
	taxList            *components.List
	taxPromptMode      taxonomyPromptMode
	taxPromptBuf       string
	taxPendingName     string
	taxPendingDesc     string
	taxEditID          string

	width  int
	height int
}

// NewProfileModel builds the profile UI model.
func NewProfileModel(client *api.Client, cfg *config.Config) ProfileModel {
	return ProfileModel{
		client:    client,
		config:    cfg,
		keyList:   components.NewList(10),
		agentList: components.NewList(10),
		taxList:   components.NewList(12),
	}
}

func (m ProfileModel) Init() tea.Cmd {
	m.loading = true
	m.taxLoading = true
	m.agentDetail = nil
	return tea.Batch(m.loadKeys, m.loadAgents, m.loadTaxonomy)
}

func (m ProfileModel) Update(msg tea.Msg) (ProfileModel, tea.Cmd) {
	switch msg := msg.(type) {
	case keysLoadedMsg:
		m.keys = msg.items
		labels := make([]string, len(msg.items))
		for i, k := range msg.items {
			labels[i] = formatKeyLine(k)
		}
		m.keyList.SetItems(labels)
		m.loading = false
		return m, nil

	case agentsLoadedMsg:
		m.agents = msg.items
		labels := make([]string, len(msg.items))
		for i, a := range msg.items {
			labels[i] = formatAgentLine(a)
		}
		m.agentList.SetItems(labels)
		return m, nil

	case keyCreatedMsg:
		m.creating = false
		m.createBuf = ""
		m.createdKey = msg.resp.APIKey
		return m, m.loadKeys

	case keyRevokedMsg:
		return m, m.loadKeys

	case agentUpdatedMsg:
		return m, m.loadAgents

	case apiKeySavedMsg:
		m.editAPIKey = false
		m.apiKeyBuf = ""
		return m, nil
	case pendingLimitSavedMsg:
		m.editPendingLimit = false
		m.pendingLimitBuf = ""
		return m, nil

	case taxonomyLoadedMsg:
		if msg.kind != m.taxonomyKindPath() {
			return m, nil
		}
		m.setTaxonomyItems(msg.items)
		m.taxLoading = false
		m.loading = false
		return m, nil

	case taxonomyActionDoneMsg:
		m.taxLoading = true
		return m, m.loadTaxonomy

	case tea.KeyMsg:
		if m.taxPromptMode != taxPromptNone {
			return m.handleTaxonomyPrompt(msg)
		}
		if m.editPendingLimit {
			return m.handlePendingLimitInput(msg)
		}
		if m.editAPIKey {
			return m.handleAPIKeyInput(msg)
		}
		if m.creating {
			return m.handleCreateInput(msg)
		}

		if m.agentDetail != nil {
			return m.handleAgentDetailKeys(msg)
		}

		if m.createdKey != "" {
			if isBack(msg) || isEnter(msg) {
				m.createdKey = ""
			}
			return m, nil
		}

		if m.sectionFocus {
			switch {
			case isKey(msg, "left"):
				m.section = (m.section - 1 + 3) % 3
			case isKey(msg, "right"):
				m.section = (m.section + 1) % 3
			case isDown(msg), isEnter(msg), isSpace(msg):
				m.sectionFocus = false
			}
			return m, nil
		}

		switch {
		case isKey(msg, "left"):
			m.section = (m.section - 1 + 3) % 3
		case isKey(msg, "right"):
			m.section = (m.section + 1) % 3
		case isDown(msg):
			if m.section == 2 {
				m.taxList.Down()
			} else {
				m.activeList().Down()
			}
		case isUp(msg):
			if m.section == 2 {
				if m.taxList == nil || m.taxList.Selected() <= 0 {
					m.sectionFocus = true
				} else {
					m.taxList.Up()
				}
			} else {
				list := m.activeList()
				if list == nil || list.Selected() <= 0 {
					m.sectionFocus = true
				} else {
					list.Up()
				}
			}
		case isKey(msg, "n"):
			m.sectionFocus = false
			if m.section == 0 {
				m.creating = true
				m.createBuf = ""
			} else if m.section == 2 {
				m.openTaxPrompt(taxPromptCreateName, "")
			}
		case isKey(msg, "k"):
			m.sectionFocus = false
			m.editAPIKey = true
			m.apiKeyBuf = m.config.APIKey
		case isKey(msg, "p"):
			m.sectionFocus = false
			m.editPendingLimit = true
			limit := 500
			if m.config != nil && m.config.PendingLimit > 0 {
				limit = m.config.PendingLimit
			}
			m.pendingLimitBuf = fmt.Sprintf("%d", limit)
		case isKey(msg, "r"):
			if m.section == 0 {
				return m.revokeSelected()
			}
		case isKey(msg, "t"):
			if m.section == 1 {
				return m.toggleTrust()
			}
		case isEnter(msg):
			m.sectionFocus = false
			if m.section == 1 {
				if idx := m.agentList.Selected(); idx < len(m.agents) {
					agent := m.agents[idx]
					m.agentDetail = &agent
				}
			} else if m.section == 2 {
				item := m.selectedTaxonomy()
				if item != nil {
					desc := ""
					if item.Description != nil {
						desc = *item.Description
					}
					m.taxEditID = item.ID
					m.taxPendingDesc = desc
					m.openTaxPrompt(taxPromptEditName, item.Name)
				}
			}
		case isKey(msg, "e"):
			if m.section == 2 {
				item := m.selectedTaxonomy()
				if item != nil {
					desc := ""
					if item.Description != nil {
						desc = *item.Description
					}
					m.taxEditID = item.ID
					m.taxPendingDesc = desc
					m.openTaxPrompt(taxPromptEditName, item.Name)
				}
			}
		case isKey(msg, "d"):
			if m.section == 2 {
				return m.taxonomyArchiveSelected()
			}
		case isKey(msg, "a"):
			if m.section == 2 {
				return m.taxonomyActivateSelected()
			}
		case isKey(msg, "f"):
			if m.section == 2 {
				m.openTaxPrompt(taxPromptFilter, m.taxSearch)
			}
		case isKey(msg, "i"):
			if m.section == 2 {
				m.taxIncludeInactive = !m.taxIncludeInactive
				m.taxLoading = true
				return m, m.loadTaxonomy
			}
		case isKey(msg, "["):
			if m.section == 2 {
				m.taxKind = (m.taxKind - 1 + len(taxonomyKinds)) % len(taxonomyKinds)
				m.taxLoading = true
				return m, m.loadTaxonomy
			}
		case isKey(msg, "]"), isKey(msg, "tab"):
			if m.section == 2 {
				m.taxKind = (m.taxKind + 1) % len(taxonomyKinds)
				m.taxLoading = true
				return m, m.loadTaxonomy
			}
		}
	}
	return m, nil
}

func (m ProfileModel) View() string {
	if m.loading {
		return "  " + MutedStyle.Render("Loading profile...")
	}

	if m.editAPIKey {
		return components.Indent(components.InputDialog("Set API Key", m.apiKeyBuf), 1)
	}
	if m.editPendingLimit {
		return components.Indent(components.InputDialog("Pending Queue Limit", m.pendingLimitBuf), 1)
	}

	if m.creating {
		return components.Indent(components.InputDialog("New Key Name", m.createBuf), 1)
	}

	if m.createdKey != "" {
		return components.Indent(components.ConfirmDialog("Key Created",
			fmt.Sprintf("Save this key, it won't be shown again:\n\n%s\n\nPress Enter to continue.", m.createdKey)), 1)
	}

	if m.agentDetail != nil {
		return m.renderAgentDetail()
	}

	var b strings.Builder

	// User info table
	b.WriteString(components.Indent(components.Table("Settings", []components.TableRow{
		{Label: "User", Value: m.config.Username},
		{Label: "API Key", Value: maskedAPIKey(m.config.APIKey)},
		{Label: "Pending Queue", Value: fmt.Sprintf("%d", m.config.PendingLimit)},
	}, m.width), 1))
	b.WriteString("\n\n")

	// Section tabs
	keysLabel := "API Keys"
	agentsLabel := "Agents"
	taxonomyLabel := "Taxonomy"
	var tabs string
	if m.section == 0 {
		active := TabActiveStyle
		if m.sectionFocus {
			active = TabFocusStyle
		}
		tabs = active.Render(keysLabel) +
			" " + TabInactiveStyle.Render(agentsLabel) +
			" " + TabInactiveStyle.Render(taxonomyLabel)
	} else if m.section == 1 {
		active := TabActiveStyle
		if m.sectionFocus {
			active = TabFocusStyle
		}
		tabs = TabInactiveStyle.Render(keysLabel) +
			" " + active.Render(agentsLabel) +
			" " + TabInactiveStyle.Render(taxonomyLabel)
	} else {
		active := TabActiveStyle
		if m.sectionFocus {
			active = TabFocusStyle
		}
		tabs = TabInactiveStyle.Render(keysLabel) +
			" " + TabInactiveStyle.Render(agentsLabel) +
			" " + active.Render(taxonomyLabel)
	}
	b.WriteString(components.CenterLine(tabs, m.width))
	b.WriteString("\n\n")

	if m.section == 0 {
		b.WriteString(m.renderKeys())
	} else if m.section == 1 {
		b.WriteString(m.renderAgents())
	} else {
		b.WriteString(m.renderTaxonomy())
	}

	return b.String()
}

// --- Helpers ---

func (m *ProfileModel) activeList() *components.List {
	if m.section == 0 {
		return m.keyList
	}
	return m.agentList
}

func (m ProfileModel) loadKeys() tea.Msg {
	items, err := m.client.ListAllKeys()
	if err != nil {
		return errMsg{err}
	}
	return keysLoadedMsg{items}
}

func (m ProfileModel) loadAgents() tea.Msg {
	items, err := m.client.ListAgents("")
	if err != nil {
		return errMsg{err}
	}
	return agentsLoadedMsg{items}
}

func (m ProfileModel) handleCreateInput(msg tea.KeyMsg) (ProfileModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.creating = false
		m.createBuf = ""
	case isEnter(msg):
		name := m.createBuf
		m.creating = false
		m.createBuf = ""
		return m, func() tea.Msg {
			resp, err := m.client.CreateKey(name)
			if err != nil {
				return errMsg{err}
			}
			return keyCreatedMsg{resp}
		}
	case isKey(msg, "backspace"):
		if len(m.createBuf) > 0 {
			m.createBuf = m.createBuf[:len(m.createBuf)-1]
		}
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.createBuf += msg.String()
		}
	}
	return m, nil
}

func (m ProfileModel) handleAPIKeyInput(msg tea.KeyMsg) (ProfileModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.editAPIKey = false
		m.apiKeyBuf = ""
		return m, nil
	case isEnter(msg):
		key := strings.TrimSpace(m.apiKeyBuf)
		if key == "" {
			return m, func() tea.Msg {
				return errMsg{fmt.Errorf("api key cannot be empty")}
			}
		}
		return m, func() tea.Msg {
			m.config.APIKey = key
			if err := m.config.Save(); err != nil {
				return errMsg{err}
			}
			if m.client != nil {
				m.client.SetAPIKey(key)
			}
			return apiKeySavedMsg{}
		}
	case isKey(msg, "backspace", "delete"):
		if len(m.apiKeyBuf) > 0 {
			m.apiKeyBuf = m.apiKeyBuf[:len(m.apiKeyBuf)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
			m.apiKeyBuf += ch
		}
	}
	return m, nil
}

func (m ProfileModel) handlePendingLimitInput(msg tea.KeyMsg) (ProfileModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.editPendingLimit = false
		m.pendingLimitBuf = ""
		return m, nil
	case isEnter(msg):
		raw := strings.TrimSpace(m.pendingLimitBuf)
		if raw == "" {
			return m, func() tea.Msg { return errMsg{fmt.Errorf("pending limit cannot be empty")} }
		}
		limit, err := parsePositiveInt(raw)
		if err != nil {
			return m, func() tea.Msg { return errMsg{err} }
		}
		if limit > 5000 {
			limit = 5000
		}
		return m, func() tea.Msg {
			m.config.PendingLimit = limit
			if err := m.config.Save(); err != nil {
				return errMsg{err}
			}
			return pendingLimitSavedMsg{limit: limit}
		}
	case isKey(msg, "backspace", "delete"):
		if len(m.pendingLimitBuf) > 0 {
			m.pendingLimitBuf = m.pendingLimitBuf[:len(m.pendingLimitBuf)-1]
		}
	default:
		ch := msg.String()
		if len(ch) == 1 && ch[0] >= '0' && ch[0] <= '9' {
			m.pendingLimitBuf += ch
		}
	}
	return m, nil
}

func parsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("pending limit must be a number")
	}
	if n <= 0 {
		return 0, fmt.Errorf("pending limit must be greater than zero")
	}
	return n, nil
}

func (m ProfileModel) revokeSelected() (ProfileModel, tea.Cmd) {
	if idx := m.keyList.Selected(); idx < len(m.keys) {
		id := m.keys[idx].ID
		return m, func() tea.Msg {
			err := m.client.RevokeKey(id)
			if err != nil {
				return errMsg{err}
			}
			return keyRevokedMsg{}
		}
	}
	return m, nil
}

func (m ProfileModel) toggleTrust() (ProfileModel, tea.Cmd) {
	if idx := m.agentList.Selected(); idx < len(m.agents) {
		agent := m.agents[idx]
		newVal := !agent.RequiresApproval
		return m, func() tea.Msg {
			_, err := m.client.UpdateAgent(agent.ID, api.UpdateAgentInput{
				RequiresApproval: &newVal,
			})
			if err != nil {
				return errMsg{err}
			}
			return agentUpdatedMsg{}
		}
	}
	return m, nil
}

func (m ProfileModel) renderKeys() string {
	if len(m.keys) == 0 {
		return components.Indent(components.Box(MutedStyle.Render("No API keys."), m.width), 1)
	}

	contentWidth := components.BoxContentWidth(m.width)
	visible := m.keyList.Visible()

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

	prefixWidth := 12
	atWidth := compactTimeColumnWidth
	ownerWidth := 18
	nameWidth := availableCols - (prefixWidth + ownerWidth + atWidth)
	if nameWidth < 14 {
		nameWidth = 14
		ownerWidth = availableCols - (prefixWidth + nameWidth + atWidth)
		if ownerWidth < 12 {
			ownerWidth = 12
		}
	}

	cols := []components.TableColumn{
		{Header: "Prefix", Width: prefixWidth, Align: lipgloss.Left},
		{Header: "Name", Width: nameWidth, Align: lipgloss.Left},
		{Header: "Owner", Width: ownerWidth, Align: lipgloss.Left},
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.APIKey
	if idx := m.keyList.Selected(); idx >= 0 && idx < len(m.keys) {
		previewItem = &m.keys[idx]
	}

	for i := range visible {
		absIdx := m.keyList.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.keys) {
			continue
		}
		k := m.keys[absIdx]

		prefix := strings.TrimSpace(components.SanitizeOneLine(k.KeyPrefix + "..."))
		if prefix == "..." || prefix == "" {
			prefix = "-"
		}
		name := strings.TrimSpace(components.SanitizeOneLine(k.Name))
		if name == "" {
			name = "-"
		}
		owner := "-"
		if k.EntityName != nil && strings.TrimSpace(*k.EntityName) != "" {
			owner = strings.TrimSpace(*k.EntityName)
		} else if k.AgentName != nil && strings.TrimSpace(*k.AgentName) != "" {
			owner = "agent:" + strings.TrimSpace(*k.AgentName)
		}
		owner = components.SanitizeOneLine(owner)
		at := k.CreatedAt.Format("01-02")

		if m.section == 0 && m.keyList.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(prefix, prefixWidth),
			components.ClampTextWidthEllipsis(name, nameWidth),
			components.ClampTextWidthEllipsis(owner, ownerWidth),
			at,
		})
	}
	if m.sectionFocus {
		activeRowRel = -1
	}

	title := "API Keys"
	countLine := MutedStyle.Render(fmt.Sprintf("%d keys", len(m.keys)))
	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderKeyPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.Indent(components.TitledBox(title, content, m.width), 1)
}

func (m ProfileModel) renderKeyPreview(k api.APIKey, width int) string {
	if width <= 0 {
		return ""
	}

	name := strings.TrimSpace(components.SanitizeOneLine(k.Name))
	if name == "" {
		name = "key"
	}

	owner := "-"
	if k.EntityName != nil && strings.TrimSpace(*k.EntityName) != "" {
		owner = strings.TrimSpace(*k.EntityName)
	} else if k.AgentName != nil && strings.TrimSpace(*k.AgentName) != "" {
		owner = "agent:" + strings.TrimSpace(*k.AgentName)
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(name, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Prefix", k.KeyPrefix+"...", width))
	lines = append(lines, renderPreviewRow("Owner", owner, width))
	lines = append(lines, renderPreviewRow("Created", formatLocalTimeFull(k.CreatedAt), width))
	if k.LastUsedAt != nil {
		lines = append(lines, renderPreviewRow("Last Used", formatLocalTimeFull(*k.LastUsedAt), width))
	}
	if k.ExpiresAt != nil {
		lines = append(lines, renderPreviewRow("Expires", formatLocalTimeFull(*k.ExpiresAt), width))
	}

	return padPreviewLines(lines, width)
}

func (m ProfileModel) renderAgents() string {
	if len(m.agents) == 0 {
		return components.Indent(components.Box(MutedStyle.Render("No agents registered."), m.width), 1)
	}

	contentWidth := components.BoxContentWidth(m.width)
	visible := m.agentList.Visible()

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

	statusWidth := 11
	trustWidth := 10
	scopesWidth := 22
	nameWidth := availableCols - (statusWidth + trustWidth + scopesWidth)
	if nameWidth < 14 {
		nameWidth = 14
		scopesWidth = availableCols - (nameWidth + statusWidth + trustWidth)
		if scopesWidth < 14 {
			scopesWidth = 14
		}
	}

	cols := []components.TableColumn{
		{Header: "Name", Width: nameWidth, Align: lipgloss.Left},
		{Header: "Trust", Width: trustWidth, Align: lipgloss.Left},
		{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		{Header: "Scopes", Width: scopesWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.Agent
	if idx := m.agentList.Selected(); idx >= 0 && idx < len(m.agents) {
		previewItem = &m.agents[idx]
	}

	for i := range visible {
		absIdx := m.agentList.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.agents) {
			continue
		}
		a := m.agents[absIdx]

		name := strings.TrimSpace(components.SanitizeOneLine(a.Name))
		if name == "" {
			name = "agent"
		}
		status := strings.TrimSpace(components.SanitizeOneLine(a.Status))
		if status == "" {
			status = "-"
		}
		trust := "untrusted"
		if !a.RequiresApproval {
			trust = "trusted"
		}
		scopes := "-"
		if len(a.Scopes) > 0 {
			scopes = strings.Join(a.Scopes, ", ")
		}

		if m.section == 1 && m.agentList.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(name, nameWidth),
			components.ClampTextWidthEllipsis(trust, trustWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			components.ClampTextWidthEllipsis(components.SanitizeOneLine(scopes), scopesWidth),
		})
	}
	if m.sectionFocus {
		activeRowRel = -1
	}

	title := "Agents"
	countLine := MutedStyle.Render(fmt.Sprintf("%d agents", len(m.agents)))
	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderAgentPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := table
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = table + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.Indent(components.TitledBox(title, content, m.width), 1)
}

func (m ProfileModel) renderAgentPreview(a api.Agent, width int) string {
	if width <= 0 {
		return ""
	}

	name := strings.TrimSpace(components.SanitizeOneLine(a.Name))
	if name == "" {
		name = "agent"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(a.Status))
	if status == "" {
		status = "-"
	}
	trust := "untrusted"
	if !a.RequiresApproval {
		trust = "trusted"
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(name, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Trust", trust, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	if len(a.Scopes) > 0 {
		lines = append(lines, renderPreviewRow("Scopes", formatScopePreview(a.Scopes), width))
	}
	if len(a.Capabilities) > 0 {
		lines = append(lines, renderPreviewRow("Caps", strings.Join(a.Capabilities, ", "), width))
	}
	if a.Description != nil && strings.TrimSpace(*a.Description) != "" {
		lines = append(lines, renderPreviewRow("Desc", strings.TrimSpace(*a.Description), width))
	}

	return padPreviewLines(lines, width)
}

func (m ProfileModel) renderAgentDetail() string {
	if m.agentDetail == nil {
		return ""
	}
	agent := m.agentDetail
	trust := "trusted"
	if agent.RequiresApproval {
		trust = "untrusted"
	}
	scopes := "-"
	if len(agent.Scopes) > 0 {
		scopes = strings.Join(agent.Scopes, ", ")
	}
	caps := "-"
	if len(agent.Capabilities) > 0 {
		caps = strings.Join(agent.Capabilities, ", ")
	}
	rows := []components.TableRow{
		{Label: "ID", Value: agent.ID},
		{Label: "Name", Value: agent.Name},
		{Label: "Status", Value: agent.Status},
		{Label: "Trust", Value: trust},
		{Label: "Scopes", Value: scopes},
		{Label: "Capabilities", Value: caps},
		{Label: "Created", Value: formatLocalTimeFull(agent.CreatedAt)},
		{Label: "Updated", Value: formatLocalTimeFull(agent.UpdatedAt)},
	}
	if agent.Description != nil && strings.TrimSpace(*agent.Description) != "" {
		rows = append(rows, components.TableRow{Label: "Description", Value: strings.TrimSpace(*agent.Description)})
	}
	return components.Indent(components.Table("Agent Details", rows, m.width), 1)
}

func (m ProfileModel) handleAgentDetailKeys(msg tea.KeyMsg) (ProfileModel, tea.Cmd) {
	if isBack(msg) || isEnter(msg) {
		m.agentDetail = nil
	}
	return m, nil
}

func formatKeyLine(k api.APIKey) string {
	prefix := components.SanitizeOneLine(k.KeyPrefix + "...")
	owner := "-"
	if k.EntityName != nil {
		owner = *k.EntityName
	} else if k.AgentName != nil {
		owner = "agent: " + *k.AgentName
	}
	name := components.SanitizeOneLine(k.Name)
	owner = components.SanitizeOneLine(owner)
	return fmt.Sprintf("%-12s  %-20s  %-5s  %s", prefix, name, k.CreatedAt.Format("01/02"), owner)
}

func formatAgentLine(a api.Agent) string {
	trust := "untrusted"
	if !a.RequiresApproval {
		trust = "trusted"
	}
	status := components.SanitizeOneLine(a.Status)
	name := components.SanitizeOneLine(a.Name)
	return fmt.Sprintf("[%s] %s (%s)", status, name, trust)
}

func maskedAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "-"
	}
	if len(key) <= 10 {
		return strings.Repeat("*", len(key))
	}
	return fmt.Sprintf("%s...%s", key[:6], key[len(key)-4:])
}

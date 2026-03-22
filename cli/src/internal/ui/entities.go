package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	huh "charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Messages ---

type entitiesLoadedMsg struct{ items []api.Entity }
type relationshipsLoadedMsg struct{ items []api.Relationship }
type entityDetailRelationshipsLoadedMsg struct {
	id    string
	items []api.Relationship
}
type entityContextLoadedMsg struct {
	id    string
	items []api.Context
}
type entityUpdatedMsg struct{ entity api.Entity }
type entityCreatedMsg struct{ entity api.Entity }
type relationshipUpdatedMsg struct{ rel api.Relationship }
type relationshipCreatedMsg struct{ rel api.Relationship }
type relateResultsMsg struct{ items []api.Entity }
type entityHistoryLoadedMsg struct{ items []api.AuditEntry }
type entityRevertedMsg struct{ entity api.Entity }
type entityBulkUpdatedMsg struct{}
type entityScopesLoadedMsg struct{ names map[string]string }

// --- View States ---

type entitiesView int

const (
	entitiesViewAdd entitiesView = iota
	entitiesViewList
	entitiesViewSearch
	entitiesViewDetail
	entitiesViewEdit
	entitiesViewConfirm
	entitiesViewRelationships
	entitiesViewRelateSearch
	entitiesViewRelateSelect
	entitiesViewRelateType
	entitiesViewRelEdit
	entitiesViewHistory
)

const (
	relEditFieldStatus = iota
	relEditFieldProperties
	relEditFieldCount
)

var relationshipStatusOptions = []string{"active", "inactive"}

type bulkTarget int

const (
	bulkTargetTags bulkTarget = iota
	bulkTargetScopes
)

type entitiesFilterFacet int

const (
	entitiesFilterFacetType entitiesFilterFacet = iota
	entitiesFilterFacetStatus
	entitiesFilterFacetScope
	entitiesFilterFacetCount
)

type bulkInput struct {
	op     string
	values []string
}

// --- Entities Model ---

type EntitiesModel struct {
	client         *api.Client
	items          []api.Entity
	allItems       []api.Entity
	dataTable      table.Model
	loading        bool
	spinner        spinner.Model
	view           entitiesView
	modeFocus      bool
	filtering      bool
	searchBuf      string
	searchSuggest  string
	filterFacet    entitiesFilterFacet
	filterCursor   [entitiesFilterFacetCount]int
	filterTypes    map[string]bool
	filterStatus   map[string]bool
	filterScopes   map[string]bool
	filterTypeSet  []string
	filterStatSet  []string
	filterScopeSet []string
	width          int
	height         int

	detail           *api.Entity
	detailRels       []api.Relationship
	errText          string
	detailContext    []api.Context
	contextLoading   bool
	contextLinking   bool
	contextLinkBuf   string
	contextCreating  bool
	contextCreateBuf string

	// add
	addForm     *huh.Form
	addName     string
	addType     string
	addStatus   string
	addTagStr   string
	addScopeStr string
	addSaving   bool
	addSaved    bool

	// edit
	editForm       *huh.Form
	editTagStr     string
	editStatus     string
	editScopeStr   string
	editScopesDirty bool
	editSaving     bool

	// confirm
	confirmKind    string
	confirmReturn  entitiesView
	confirmRelID   string
	confirmAuditID string
	confirmAudit   *api.AuditEntry

	// relationships
	rels       []api.Relationship
	relTable   table.Model
	relLoading bool

	scopeNames   map[string]string
	scopeOptions []string

	// history
	history        []api.AuditEntry
	historyTable   table.Model
	historyLoading bool

	// relate flow
	relateQuery   string
	relateResults []api.Entity
	relateTable   table.Model
	relateTarget  *api.Entity
	relateType    string
	relateLoading bool

	// relationship edit
	relEditFocus     int
	relEditStatusIdx int
	relEditBuf       string
	relEditID        string

	// bulk operations
	bulkSelected map[string]bool
	bulkPrompt   string
	bulkBuf      string
	bulkRunning  bool
	bulkTarget   bulkTarget
}

// NewEntitiesModel builds the entities UI model.
func NewEntitiesModel(client *api.Client) EntitiesModel {
	return EntitiesModel{
		client:    client,
		spinner:   components.NewNebulaSpinner(),
		dataTable: components.NewNebulaTable(nil, 15),
		addStatus: "active",
		relTable:     components.NewNebulaTable(nil, 8),
		relateTable:  components.NewNebulaTable(nil, 8),
		historyTable: components.NewNebulaTable(nil, 8),
		view:         entitiesViewList,
		bulkSelected: map[string]bool{},
		scopeNames:   map[string]string{},
		filterTypes:  map[string]bool{},
		filterStatus: map[string]bool{},
		filterScopes: map[string]bool{},
	}
}

// Init handles init.
func (m EntitiesModel) Init() tea.Cmd {
	m.loading = true
	m.view = entitiesViewList
	m.modeFocus = false
	m.filtering = false
	m.searchBuf = ""
	m.searchSuggest = ""
	m.filterFacet = entitiesFilterFacetType
	m.filterCursor = [entitiesFilterFacetCount]int{}
	m.filterTypes = map[string]bool{}
	m.filterStatus = map[string]bool{}
	m.filterScopes = map[string]bool{}
	m.filterTypeSet = nil
	m.filterStatSet = nil
	m.filterScopeSet = nil
	m.detailRels = nil
	m.detailContext = nil
	m.contextLoading = false
	m.contextLinking = false
	m.contextLinkBuf = ""
	m.contextCreating = false
	m.contextCreateBuf = ""
	m.addForm = nil
	m.addName = ""
	m.addType = ""
	m.addStatus = "active"
	m.addTagStr = ""
	m.addScopeStr = ""
	m.addSaving = false
	m.addSaved = false
	return tea.Batch(
		m.loadEntities(""),
		m.loadScopeNames(),
		m.spinner.Tick,
	)
}

// Update updates update.
func (m EntitiesModel) Update(msg tea.Msg) (EntitiesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case entitiesLoadedMsg:
		m.loading = false
		m.allItems = msg.items
		m.refreshFilterSets()
		m.applyEntityFilters()
		m.updateSearchSuggest()
		if m.view == entitiesViewSearch {
			m.view = entitiesViewList
		}
		return m, nil

	case relationshipsLoadedMsg:
		m.relLoading = false
		m.rels = msg.items
		rows := make([]table.Row, len(msg.items))
		for i, r := range msg.items {
			rows[i] = table.Row{m.formatRelationshipLine(r)}
		}
		m.relTable.SetRows(rows)
		m.relTable.SetCursor(0)
		return m, nil
	case entityDetailRelationshipsLoadedMsg:
		if m.detail != nil && m.detail.ID == msg.id {
			m.detailRels = msg.items
		}
		return m, nil
	case entityContextLoadedMsg:
		if m.detail != nil && m.detail.ID == msg.id {
			m.contextLoading = false
			m.detailContext = msg.items
		}
		return m, nil

	case relateResultsMsg:
		m.relateLoading = false
		m.relateResults = msg.items
		rows := make([]table.Row, len(msg.items))
		for i, e := range msg.items {
			rows[i] = table.Row{formatEntityLine(e)}
		}
		m.relateTable.SetRows(rows)
		m.relateTable.SetCursor(0)
		m.view = entitiesViewRelateSelect
		return m, nil

	case entityUpdatedMsg:
		m.editSaving = false
		m.applyEntityUpdate(msg.entity)
		m.view = entitiesViewDetail
		return m, nil
	case entityCreatedMsg:
		m.addSaving = false
		m.addSaved = true
		m.loading = true
		return m, tea.Batch(m.loadEntities(""), m.spinner.Tick)

	case relationshipUpdatedMsg:
		m.relLoading = true
		return m, m.loadRelationships()

	case relationshipCreatedMsg:
		m.relLoading = true
		return m, m.loadRelationships()

	case entityHistoryLoadedMsg:
		m.historyLoading = false
		m.history = msg.items
		rows := make([]table.Row, len(msg.items))
		for i, entry := range msg.items {
			rows[i] = table.Row{formatHistoryLine(entry)}
		}
		m.historyTable.SetRows(rows)
		m.historyTable.SetCursor(0)
		return m, nil

	case entityRevertedMsg:
		m.editSaving = false
		m.applyEntityUpdate(msg.entity)
		m.view = entitiesViewDetail
		return m, nil

	case entityBulkUpdatedMsg:
		m.bulkRunning = false
		m.clearBulkSelection()
		m.loading = true
		return m, tea.Batch(m.loadEntities(strings.TrimSpace(m.searchBuf)), m.spinner.Tick)
	case entityScopesLoadedMsg:
		if m.scopeNames == nil {
			m.scopeNames = map[string]string{}
		}
		for id, name := range msg.names {
			m.scopeNames[id] = name
		}
		m.scopeOptions = scopeNameList(m.scopeNames)
		m.refreshFilterSets()
		m.applyEntityFilters()
		return m, nil

	case errMsg:
		m.loading = false
		m.relLoading = false
		m.relateLoading = false
		m.historyLoading = false
		m.editSaving = false
		m.addSaving = false
		m.bulkRunning = false
		m.contextLoading = false
		m.contextLinking = false
		m.contextCreating = false
		m.errText = msg.err.Error()
		return m, nil

	case tea.KeyPressMsg:
		switch m.view {
		case entitiesViewAdd:
			return m.handleAddKeys(msg)
		case entitiesViewSearch:
			return m.handleSearchInput(msg)
		case entitiesViewDetail:
			return m.handleDetailKeys(msg)
		case entitiesViewEdit:
			return m.handleEditKeys(msg)
		case entitiesViewConfirm:
			return m.handleConfirmKeys(msg)
		case entitiesViewRelationships:
			return m.handleRelationshipsKeys(msg)
		case entitiesViewRelateSearch, entitiesViewRelateSelect, entitiesViewRelateType:
			return m.handleRelateKeys(msg)
		case entitiesViewRelEdit:
			return m.handleRelEditKeys(msg)
		case entitiesViewHistory:
			return m.handleHistoryKeys(msg)
		default:
			return m.handleListKeys(msg)
		}

	default:
		// Forward non-key messages to active huh forms (cursor blinks, etc).
		if m.view == entitiesViewAdd && m.addForm != nil && !m.addSaving && !m.addSaved {
			_, cmd := m.addForm.Update(msg)
			return m, cmd
		}
		if m.view == entitiesViewEdit && m.editForm != nil && !m.editSaving {
			_, cmd := m.editForm.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// View handles view.
func (m EntitiesModel) View() string {
	if m.view == entitiesViewList && m.bulkPrompt != "" {
		return components.Indent(components.InputDialog(m.bulkPrompt, m.bulkBuf), 1)
	}
	if m.view == entitiesViewList && m.filtering {
		return components.Indent(m.renderFilterPicker(), 1)
	}
	if m.view == entitiesViewAdd {
		if m.addSaving {
			return "  " + MutedStyle.Render("Saving...")
		}
		if m.addSaved {
			return components.Indent(components.Box(SuccessStyle.Render("Entity saved! Press Esc to add another."), m.width), 1)
		}
		body := m.renderAdd()
		modeLine := m.renderModeLine()
		if modeLine != "" {
			body = components.CenterLine(modeLine, m.width) + "\n\n" + body
		}
		if m.errText != "" {
			body += "\n\n" + components.ErrorBox("Error", m.errText, m.width)
		}
		return components.Indent(body, 1)
	}
	switch m.view {
	case entitiesViewSearch:
		return components.Indent(components.InputDialog("Search Entities", m.searchBuf), 1)
	case entitiesViewEdit:
		body := m.renderEdit()
		if m.errText != "" {
			body += "\n\n" + components.ErrorBox("Error", m.errText, m.width)
		}
		return body
	case entitiesViewConfirm:
		return m.renderConfirm()
	case entitiesViewRelationships:
		return m.renderRelationships()
	case entitiesViewRelateSearch, entitiesViewRelateSelect, entitiesViewRelateType:
		return m.renderRelate()
	case entitiesViewRelEdit:
		return m.renderRelEdit()
	case entitiesViewDetail:
		if m.contextLinking {
			return components.Indent(components.InputDialog("Link context id", m.contextLinkBuf), 1)
		}
		if m.contextCreating {
			return components.Indent(components.InputDialog("New context title", m.contextCreateBuf), 1)
		}
		body := m.renderDetail()
		modeLine := m.renderModeLine()
		if modeLine != "" {
			body = components.CenterLine(modeLine, m.width) + "\n\n" + body
		}
		return components.Indent(body, 1)
	case entitiesViewHistory:
		return m.renderHistory()
	default:
		body := m.renderList()
		modeLine := m.renderModeLine()
		if modeLine != "" {
			body = components.CenterLine(modeLine, m.width) + "\n\n" + body
		}
		return components.Indent(body, 1)
	}
}

// --- List View ---

func (m EntitiesModel) handleListKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	if m.bulkPrompt != "" {
		return m.handleBulkPromptKeys(msg)
	}
	if m.filtering {
		return m.handleFilterInput(msg)
	}
	if m.modeFocus {
		return m.handleModeKeys(msg)
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
	case isSpace(msg):
		if m.searchBuf == "" {
			m.toggleBulkSelection(m.dataTable.Cursor())
			return m, nil
		}
		m.searchBuf += " "
		m.loading = true
		return m, tea.Batch(m.loadEntities(strings.TrimSpace(m.searchBuf)), m.spinner.Tick)
	case isEnter(msg):
		if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
			item := m.items[idx]
			m.detail = &item
			m.detailRels = nil
			m.detailContext = nil
			m.contextLoading = true
			m.view = entitiesViewDetail
			return m, tea.Batch(
				m.loadEntityDetailRelationships(item.ID),
				m.loadEntityContext(item.ID),
			)
		}
	case isKey(msg, "f"):
		m.filtering = true
		m.refreshFilterSets()
		return m, nil
	case isKey(msg, "tab"):
		if m.searchSuggest != "" && strings.TrimSpace(m.searchBuf) != strings.TrimSpace(m.searchSuggest) {
			m.searchBuf = m.searchSuggest
			m.loading = true
			return m, tea.Batch(m.loadEntities(strings.TrimSpace(m.searchBuf)), m.spinner.Tick)
		}
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.loading = true
			return m, tea.Batch(m.loadEntities(strings.TrimSpace(m.searchBuf)), m.spinner.Tick)
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.loading = true
			return m, tea.Batch(m.loadEntities(""), m.spinner.Tick)
		}
	case isBack(msg):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.loading = true
			return m, tea.Batch(m.loadEntities(""), m.spinner.Tick)
		}
	case isKey(msg, "t"):
		if m.bulkCount() > 0 {
			m.bulkPrompt = "Bulk Tags (add:tag1,tag2)"
			m.bulkBuf = ""
			m.bulkTarget = bulkTargetTags
			return m, nil
		}
	case isKey(msg, "p"):
		if m.bulkCount() > 0 {
			m.bulkPrompt = "Bulk Scopes (add:scope1,scope2)"
			m.bulkBuf = ""
			m.bulkTarget = bulkTargetScopes
			return m, nil
		}
	case isKey(msg, "c"):
		if m.bulkCount() > 0 {
			m.clearBulkSelection()
			return m, nil
		}
	default:
		ch := keyText(msg)
		if ch != "" {
			m.searchBuf += ch
			m.loading = true
			return m, tea.Batch(m.loadEntities(strings.TrimSpace(m.searchBuf)), m.spinner.Tick)
		}
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m EntitiesModel) handleFilterInput(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isEnter(msg):
		m.filtering = false
	case isBack(msg):
		m.filtering = false
		if m.hasActiveEntityFilters() {
			m.clearEntityFilters()
			m.applyEntityFilters()
		}
	case isKey(msg, "left"):
		m.filterFacet = (m.filterFacet - 1 + entitiesFilterFacetCount) % entitiesFilterFacetCount
	case isKey(msg, "right"):
		m.filterFacet = (m.filterFacet + 1) % entitiesFilterFacetCount
	case isDown(msg):
		options := m.filterOptionsForFacet(m.filterFacet)
		if len(options) > 0 {
			m.filterCursor[m.filterFacet] = (m.filterCursor[m.filterFacet] + 1) % len(options)
		}
	case isUp(msg):
		options := m.filterOptionsForFacet(m.filterFacet)
		if len(options) > 0 {
			m.filterCursor[m.filterFacet] = (m.filterCursor[m.filterFacet] - 1 + len(options)) % len(options)
		}
	case isSpace(msg):
		options := m.filterOptionsForFacet(m.filterFacet)
		if len(options) > 0 {
			idx := m.filterCursor[m.filterFacet]
			if idx >= 0 && idx < len(options) {
				key := options[idx]
				selected := m.filterMapForFacet(m.filterFacet)
				if selected[key] {
					delete(selected, key)
				} else {
					selected[key] = true
				}
				m.applyEntityFilters()
			}
		}
	case isKey(msg, "b"):
		options := m.filterOptionsForFacet(m.filterFacet)
		selected := m.filterMapForFacet(m.filterFacet)
		if len(options) > 0 {
			if len(selected) == len(options) {
				for _, option := range options {
					delete(selected, option)
				}
			} else {
				for _, option := range options {
					selected[option] = true
				}
			}
			m.applyEntityFilters()
		}
	case isKey(msg, "c"):
		m.clearEntityFilters()
		m.applyEntityFilters()
	default:
		return m, nil
	}
	return m, nil
}

// refreshFilterSets handles refresh filter sets.
func (m *EntitiesModel) refreshFilterSets() {
	typeSeen := map[string]struct{}{}
	statusSeen := map[string]struct{}{}
	scopeSeen := map[string]struct{}{}

	for _, item := range m.allItems {
		typ := normalizeScope(item.Type)
		if typ != "" {
			typeSeen[typ] = struct{}{}
		}
		status := normalizeScope(item.Status)
		if status != "" {
			statusSeen[status] = struct{}{}
		}
		for _, scope := range m.scopeNamesFromIDs(item.PrivacyScopeIDs) {
			scopeName := normalizeScope(scope)
			if scopeName != "" {
				scopeSeen[scopeName] = struct{}{}
			}
		}
	}

	m.filterTypeSet = sortedFilterKeys(typeSeen)
	m.filterStatSet = sortedFilterKeys(statusSeen)
	m.filterScopeSet = sortedFilterKeys(scopeSeen)
	m.filterTypes = retainEntityFilterSelection(m.filterTypes, m.filterTypeSet)
	m.filterStatus = retainEntityFilterSelection(m.filterStatus, m.filterStatSet)
	m.filterScopes = retainEntityFilterSelection(m.filterScopes, m.filterScopeSet)

	for facet := entitiesFilterFacetType; facet < entitiesFilterFacetCount; facet++ {
		options := m.filterOptionsForFacet(facet)
		if len(options) == 0 {
			m.filterCursor[facet] = 0
			continue
		}
		if m.filterCursor[facet] < 0 {
			m.filterCursor[facet] = 0
		}
		if m.filterCursor[facet] >= len(options) {
			m.filterCursor[facet] = len(options) - 1
		}
	}
}

// retainEntityFilterSelection handles retain entity filter selection.
func retainEntityFilterSelection(current map[string]bool, options []string) map[string]bool {
	if current == nil {
		current = map[string]bool{}
	}
	allowed := map[string]struct{}{}
	for _, option := range options {
		allowed[option] = struct{}{}
	}
	next := map[string]bool{}
	for key, selected := range current {
		if !selected {
			continue
		}
		if _, ok := allowed[key]; ok {
			next[key] = true
		}
	}
	return next
}

// sortedFilterKeys handles sorted filter keys.
func sortedFilterKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// filterOptionsForFacet handles filter options for facet.
func (m EntitiesModel) filterOptionsForFacet(facet entitiesFilterFacet) []string {
	switch facet {
	case entitiesFilterFacetType:
		return m.filterTypeSet
	case entitiesFilterFacetStatus:
		return m.filterStatSet
	case entitiesFilterFacetScope:
		return m.filterScopeSet
	default:
		return nil
	}
}

// filterMapForFacet handles filter map for facet.
func (m *EntitiesModel) filterMapForFacet(facet entitiesFilterFacet) map[string]bool {
	switch facet {
	case entitiesFilterFacetType:
		if m.filterTypes == nil {
			m.filterTypes = map[string]bool{}
		}
		return m.filterTypes
	case entitiesFilterFacetStatus:
		if m.filterStatus == nil {
			m.filterStatus = map[string]bool{}
		}
		return m.filterStatus
	case entitiesFilterFacetScope:
		if m.filterScopes == nil {
			m.filterScopes = map[string]bool{}
		}
		return m.filterScopes
	default:
		return map[string]bool{}
	}
}

// clearEntityFilters handles clear entity filters.
func (m *EntitiesModel) clearEntityFilters() {
	m.filterTypes = map[string]bool{}
	m.filterStatus = map[string]bool{}
	m.filterScopes = map[string]bool{}
}

// hasActiveEntityFilters handles has active entity filters.
func (m EntitiesModel) hasActiveEntityFilters() bool {
	return len(m.filterTypes) > 0 || len(m.filterStatus) > 0 || len(m.filterScopes) > 0
}

// applyEntityFilters handles apply entity filters.
func (m *EntitiesModel) applyEntityFilters() {
	filtered := make([]api.Entity, 0, len(m.allItems))
	for _, item := range m.allItems {
		if !m.matchesEntityFilters(item) {
			continue
		}
		filtered = append(filtered, item)
	}
	m.items = filtered
	rows := make([]table.Row, len(filtered))
	for i, e := range filtered {
		rows[i] = table.Row{formatEntityLine(e)}
	}
	m.dataTable.SetRows(rows)
	m.dataTable.SetCursor(0)
	m.pruneBulkSelection(filtered)
	m.updateSearchSuggest()
}

// pruneBulkSelection handles prune bulk selection.
func (m *EntitiesModel) pruneBulkSelection(items []api.Entity) {
	if len(m.bulkSelected) == 0 {
		return
	}
	visible := map[string]struct{}{}
	for _, item := range items {
		if item.ID != "" {
			visible[item.ID] = struct{}{}
		}
	}
	for id := range m.bulkSelected {
		if _, ok := visible[id]; !ok {
			delete(m.bulkSelected, id)
		}
	}
}

// matchesEntityFilters handles matches entity filters.
func (m EntitiesModel) matchesEntityFilters(item api.Entity) bool {
	if len(m.filterTypes) > 0 {
		typ := normalizeScope(item.Type)
		if typ == "" || !m.filterTypes[typ] {
			return false
		}
	}
	if len(m.filterStatus) > 0 {
		status := normalizeScope(item.Status)
		if status == "" || !m.filterStatus[status] {
			return false
		}
	}
	if len(m.filterScopes) > 0 {
		matched := false
		for _, scope := range m.scopeNamesFromIDs(item.PrivacyScopeIDs) {
			scopeName := normalizeScope(scope)
			if scopeName == "" {
				continue
			}
			if m.filterScopes[scopeName] {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// renderFilterPicker renders render filter picker.
func (m EntitiesModel) renderFilterPicker() string {
	labels := []string{"Type", "Status", "Scope"}
	facetTabs := make([]string, len(labels))
	for idx, label := range labels {
		tab := TabInactiveStyle.Render(label)
		selectedCount := len(m.filterMapForFacet(entitiesFilterFacet(idx)))
		if selectedCount > 0 {
			tab = TabActiveStyle.Render(fmt.Sprintf("%s (%d)", label, selectedCount))
		}
		if m.filterFacet == entitiesFilterFacet(idx) {
			tab = TabFocusStyle.Render(label)
			if selectedCount > 0 {
				tab = TabFocusStyle.Render(fmt.Sprintf("%s (%d)", label, selectedCount))
			}
		}
		facetTabs[idx] = tab
	}

	options := m.filterOptionsForFacet(m.filterFacet)
	selected := m.filterMapForFacet(m.filterFacet)
	rows := make([][]string, 0, len(options))
	activeRow := -1
	for i, option := range options {
		marker := "[ ]"
		if selected[option] {
			marker = "[X]"
		}
		rows = append(rows, []string{marker, option})
		if i == m.filterCursor[m.filterFacet] {
			activeRow = i
		}
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "No values in current list"})
	}

	boxWidth := components.BoxContentWidth(m.width)
	if boxWidth < 48 {
		boxWidth = 48
	}
	tableWidth := boxWidth - 2
	optionWidth := tableWidth - 6
	cols := []components.TableColumn{
		{Header: "Sel", Width: 4, Align: lipgloss.Left},
		{Header: "Value", Width: optionWidth, Align: lipgloss.Left},
	}
	table := components.TableGridWithActiveRow(cols, rows, tableWidth, activeRow)

	summary := "No active filters"
	if m.hasActiveEntityFilters() {
		var parts []string
		if len(m.filterTypes) > 0 {
			parts = append(parts, fmt.Sprintf("type=%d", len(m.filterTypes)))
		}
		if len(m.filterStatus) > 0 {
			parts = append(parts, fmt.Sprintf("status=%d", len(m.filterStatus)))
		}
		if len(m.filterScopes) > 0 {
			parts = append(parts, fmt.Sprintf("scope=%d", len(m.filterScopes)))
		}
		summary = "Active: " + strings.Join(parts, ", ")
	}

	content := strings.Join([]string{
		strings.Join(facetTabs, " "),
		"",
		MutedStyle.Render(summary),
		"",
		table,
		"",
		MutedStyle.Render("left/right facet · up/down option · space toggle · b toggle all · c clear · enter apply · esc clear"),
	}, "\n")
	return components.TitledBox("Filter Entities", content, m.width)
}

// --- Mode Line ---

func (m EntitiesModel) renderModeLine() string {
	add := TabInactiveStyle.Render("Add")
	list := TabInactiveStyle.Render("Library")
	if m.view == entitiesViewAdd {
		add = TabActiveStyle.Render("Add")
	} else {
		list = TabActiveStyle.Render("Library")
	}
	if m.modeFocus {
		if m.view == entitiesViewAdd {
			add = TabFocusStyle.Render("Add")
		} else {
			list = TabFocusStyle.Render("Library")
		}
	}
	return add + " " + list
}

// handleModeKeys handles handle mode keys.
func (m EntitiesModel) handleModeKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
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
func (m EntitiesModel) toggleMode() (EntitiesModel, tea.Cmd) {
	m.modeFocus = false
	if m.view == entitiesViewAdd {
		m.view = entitiesViewList
		return m, nil
	}
	m.view = entitiesViewAdd
	m.addSaved = false
	m.initAddForm()
	cmd := m.addForm.Init()
	return m, cmd
}

// --- Add View ---

func (m EntitiesModel) handleAddKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	if m.addSaving {
		return m, nil
	}
	if m.addSaved {
		if isBack(msg) {
			m.resetAddForm()
		}
		return m, nil
	}
	if m.modeFocus {
		return m.handleModeKeys(msg)
	}

	if m.addForm == nil {
		m.initAddForm()
		cmd := m.addForm.Init()
		return m, cmd
	}

	_, cmd := m.addForm.Update(msg)

	switch m.addForm.State {
	case huh.StateCompleted:
		return m.saveAdd()
	case huh.StateAborted:
		m.resetAddForm()
		return m, nil
	}

	return m, cmd
}

// renderAdd renders the add entity form.
func (m EntitiesModel) renderAdd() string {
	if m.addForm == nil {
		return components.TitledBox("Add Entity", MutedStyle.Render("  Initializing..."), m.width)
	}
	return components.TitledBox("Add Entity", m.addForm.View(), m.width)
}

// saveAdd handles save add.
func (m EntitiesModel) saveAdd() (EntitiesModel, tea.Cmd) {
	name := strings.TrimSpace(m.addName)
	if name == "" {
		m.errText = "Name is required"
		return m, nil
	}
	typ := strings.TrimSpace(m.addType)
	if typ == "" {
		m.errText = "Type is required"
		return m, nil
	}

	status := m.addStatus
	if status == "" {
		status = "active"
	}

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

	input := api.CreateEntityInput{
		Scopes: scopes,
		Name:   name,
		Type:   typ,
		Status: status,
		Tags:   tags,
	}

	m.addSaving = true
	return m, func() tea.Msg {
		created, err := m.client.CreateEntity(input)
		if err != nil {
			return errMsg{err}
		}
		return entityCreatedMsg{entity: *created}
	}
}

// resetAddForm handles reset add form.
func (m *EntitiesModel) resetAddForm() {
	m.addSaved = false
	m.errText = ""
	m.modeFocus = false
	m.addName = ""
	m.addType = ""
	m.addStatus = "active"
	m.addTagStr = ""
	m.addScopeStr = ""
	m.initAddForm()
}

// initAddForm creates a new huh form for the add entity flow.
func (m *EntitiesModel) initAddForm() {
	m.addForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Name").Value(&m.addName),
			huh.NewInput().Title("Type").Value(&m.addType),
			huh.NewSelect[string]().Title("Status").Options(
				huh.NewOption("active", "active"),
				huh.NewOption("inactive", "inactive"),
			).Value(&m.addStatus),
			huh.NewInput().Title("Tags (comma-separated)").Value(&m.addTagStr),
			huh.NewInput().Title("Scopes (comma-separated)").Value(&m.addScopeStr),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

// parseCommaSeparated splits a comma-separated string into trimmed non-empty parts.
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// dedup removes duplicate strings from a slice preserving order.
func dedup(ss []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, s := range ss {
		if s != "" && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// renderList renders render list.
func (m EntitiesModel) renderList() string {
	if m.loading {
		return "  " + m.spinner.View() + " " + MutedStyle.Render("Loading entities...")
	}

	if len(m.items) == 0 {
		return components.EmptyStateBox(
			"Entities",
			"No entities found.",
			[]string{"Type to live-search", "Press tab to switch Add/Library", "Press / for command palette"},
			m.width,
		)
	}

	contentWidth := components.BoxContentWidth(m.width)
	showCheckboxes := m.bulkCount() > 0

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

	typeWidth := 12
	statusWidth := 11
	atWidth := compactTimeColumnWidth
	nameWidth := availableCols - (typeWidth + statusWidth + atWidth)
	if nameWidth < 12 {
		nameWidth = 12
	}

	// Build rows from current items for the table.
	tableRows := make([]table.Row, len(m.items))
	for i, e := range m.items {
		name, typ := normalizeEntityNameType(components.SanitizeText(e.Name), components.SanitizeText(e.Type))
		name = components.SanitizeOneLine(name)
		typ = components.SanitizeOneLine(typ)
		if typ == "" {
			typ = "?"
		}
		status := strings.TrimSpace(components.SanitizeOneLine(e.Status))
		if status == "" {
			status = "-"
		}
		at := e.UpdatedAt
		if at.IsZero() {
			at = e.CreatedAt
		}

		displayName := name
		if showCheckboxes {
			checkbox := "[ ]"
			if m.isBulkSelected(i) {
				checkbox = "[X]"
			}
			displayName = checkbox + " " + displayName
		}

		tableRows[i] = table.Row{
			components.ClampTextWidthEllipsis(displayName, nameWidth),
			components.ClampTextWidthEllipsis(typ, typeWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			formatLocalTimeCompact(at),
		}
	}

	m.dataTable.SetColumns([]table.Column{
		{Title: "Name", Width: nameWidth},
		{Title: "Type", Width: typeWidth},
		{Title: "Status", Width: statusWidth},
		{Title: "At", Width: atWidth},
	})
	m.dataTable.SetWidth(tableWidth)
	m.dataTable.SetRows(tableRows)

	title := "Entities"
	countLine := fmt.Sprintf("%d total", len(m.items))
	if selected := m.bulkCount(); selected > 0 {
		countLine = fmt.Sprintf("%s · selected: %d", countLine, selected)
	}
	if strings.TrimSpace(m.searchBuf) != "" {
		query := strings.TrimSpace(m.searchBuf)
		countLine = fmt.Sprintf("%s · search: %s", countLine, query)
		if m.searchSuggest != "" && !strings.EqualFold(query, strings.TrimSpace(m.searchSuggest)) {
			countLine = fmt.Sprintf("%s · next: %s", countLine, strings.TrimSpace(m.searchSuggest))
		}
	}
	if m.hasActiveEntityFilters() {
		countLine = fmt.Sprintf("%s · filters active", countLine)
	}
	countLine = MutedStyle.Render(countLine)

	tableView := m.dataTable.View()
	preview := ""
	var previewItem *api.Entity
	if idx := m.dataTable.Cursor(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}
	if previewItem != nil {
		content := m.renderEntityPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.TitledBox(title, content, m.width)
}

// renderEntityPreview renders render entity preview.
func (m EntitiesModel) renderEntityPreview(e api.Entity, width int) string {
	if width <= 0 {
		return ""
	}

	name, typ := normalizeEntityNameType(components.SanitizeText(e.Name), components.SanitizeText(e.Type))
	name = components.SanitizeOneLine(name)
	typ = components.SanitizeOneLine(typ)
	if typ == "" {
		typ = "?"
	}
	status := strings.TrimSpace(components.SanitizeOneLine(e.Status))
	if status == "" {
		status = "-"
	}

	at := e.UpdatedAt
	if at.IsZero() {
		at = e.CreatedAt
	}

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(name, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Type", typ, width))
	lines = append(lines, renderPreviewRow("Status", status, width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeCompact(at), width))

	if len(e.PrivacyScopeIDs) > 0 {
		lines = append(lines, renderPreviewRow("Scopes", m.formatEntityScopes(e.PrivacyScopeIDs), width))
	}
	if len(e.Tags) > 0 {
		lines = append(lines, renderPreviewRow("Tags", strings.Join(e.Tags, ", "), width))
	}
	if m.detail != nil && m.detail.ID == e.ID && len(m.detailRels) > 0 {
		lines = append(lines, renderPreviewRow("Links", fmt.Sprintf("%d", len(m.detailRels)), width))
	}
	if m.detail != nil && m.detail.ID == e.ID && len(m.detailContext) > 0 {
		lines = append(lines, renderPreviewRow("Context", fmt.Sprintf("%d", len(m.detailContext)), width))
	}

	return padPreviewLines(lines, width)
}

// updateSearchSuggest updates update search suggest.
func (m *EntitiesModel) updateSearchSuggest() {
	m.searchSuggest = ""
	query := strings.ToLower(strings.TrimSpace(m.searchBuf))
	if query == "" {
		return
	}
	pool := m.items
	if len(m.allItems) > 0 {
		pool = m.allItems
	}
	for _, e := range pool {
		name, _ := normalizeEntityNameType(e.Name, e.Type)
		if strings.HasPrefix(strings.ToLower(name), query) {
			m.searchSuggest = name
			return
		}
	}
}

// toggleBulkSelection handles toggle bulk selection.
func (m *EntitiesModel) toggleBulkSelection(absIdx int) {
	if absIdx < 0 || absIdx >= len(m.items) {
		return
	}
	id := m.items[absIdx].ID
	if id == "" {
		return
	}
	if m.bulkSelected[id] {
		delete(m.bulkSelected, id)
		return
	}
	m.bulkSelected[id] = true
}

// clearBulkSelection handles clear bulk selection.
func (m *EntitiesModel) clearBulkSelection() {
	m.bulkSelected = map[string]bool{}
}

// bulkCount handles bulk count.
func (m *EntitiesModel) bulkCount() int {
	return len(m.bulkSelected)
}

// isBulkSelected handles is bulk selected.
func (m *EntitiesModel) isBulkSelected(absIdx int) bool {
	if absIdx < 0 || absIdx >= len(m.items) {
		return false
	}
	id := m.items[absIdx].ID
	if id == "" {
		return false
	}
	return m.bulkSelected[id]
}

// bulkSelectedIDs handles bulk selected ids.
func (m *EntitiesModel) bulkSelectedIDs() []string {
	if len(m.bulkSelected) == 0 {
		return nil
	}
	ids := make([]string, 0, len(m.bulkSelected))
	for id := range m.bulkSelected {
		ids = append(ids, id)
	}
	return ids
}

// handleBulkPromptKeys handles handle bulk prompt keys.
func (m EntitiesModel) handleBulkPromptKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.bulkPrompt = ""
		m.bulkBuf = ""
		return m, nil
	case isKey(msg, "enter"):
		spec, err := parseBulkInput(m.bulkBuf)
		if err != nil {
			return m, func() tea.Msg { return errMsg{err} }
		}
		m.bulkPrompt = ""
		m.bulkBuf = ""
		m.bulkRunning = true
		switch m.bulkTarget {
		case bulkTargetScopes:
			return m, m.bulkUpdateScopes(spec)
		default:
			return m, m.bulkUpdateTags(spec)
		}
	case isKey(msg, "backspace", "delete"):
		if len(m.bulkBuf) > 0 {
			m.bulkBuf = m.bulkBuf[:len(m.bulkBuf)-1]
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		m.bulkBuf = ""
	default:
		ch := keyText(msg)
		if ch != "" {
			m.bulkBuf += ch
		}
	}
	return m, nil
}

// bulkUpdateTags handles bulk update tags.
func (m EntitiesModel) bulkUpdateTags(spec bulkInput) tea.Cmd {
	ids := m.bulkSelectedIDs()
	if len(ids) == 0 {
		m.bulkRunning = false
		return nil
	}
	tags := normalizeBulkTags(spec.values)
	if spec.op != "set" && len(tags) == 0 {
		m.bulkRunning = false
		return func() tea.Msg { return errMsg{fmt.Errorf("no valid tags provided")} }
	}
	input := api.BulkUpdateEntityTagsInput{
		EntityIDs: ids,
		Tags:      tags,
		Op:        spec.op,
	}
	return func() tea.Msg {
		_, err := m.client.BulkUpdateEntityTags(input)
		if err != nil {
			return errMsg{err}
		}
		return entityBulkUpdatedMsg{}
	}
}

// bulkUpdateScopes handles bulk update scopes.
func (m EntitiesModel) bulkUpdateScopes(spec bulkInput) tea.Cmd {
	ids := m.bulkSelectedIDs()
	if len(ids) == 0 {
		m.bulkRunning = false
		return nil
	}
	scopes := normalizeBulkScopes(spec.values)
	if spec.op != "set" && len(scopes) == 0 {
		m.bulkRunning = false
		return func() tea.Msg { return errMsg{fmt.Errorf("no valid scopes provided")} }
	}
	input := api.BulkUpdateEntityScopesInput{
		EntityIDs: ids,
		Scopes:    scopes,
		Op:        spec.op,
	}
	return func() tea.Msg {
		_, err := m.client.BulkUpdateEntityScopes(input)
		if err != nil {
			return errMsg{err}
		}
		return entityBulkUpdatedMsg{}
	}
}

// --- Search ---

func (m EntitiesModel) handleSearchInput(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.view = entitiesViewList
		m.searchBuf = ""
	case isEnter(msg):
		query := strings.TrimSpace(m.searchBuf)
		m.searchBuf = ""
		m.loading = true
		m.view = entitiesViewList
		return m, tea.Batch(m.loadEntities(query), m.spinner.Tick)
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
		}
	default:
		if ch := keyText(msg); ch != "" {
			m.searchBuf += ch
		}
	}
	return m, nil
}

// --- Detail ---

func (m EntitiesModel) handleDetailKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	if m.contextLinking || m.contextCreating {
		return m.handleContextPromptKeys(msg)
	}

	switch {
	case isBack(msg):
		m.detail = nil
		m.detailRels = nil
		m.detailContext = nil
		m.contextLoading = false
		m.contextLinking = false
		m.contextLinkBuf = ""
		m.contextCreating = false
		m.contextCreateBuf = ""
		m.view = entitiesViewList
	case isKey(msg, "e"):
		m.startEdit()
		m.view = entitiesViewEdit
		if m.editForm != nil {
			cmd := m.editForm.Init()
			return m, cmd
		}
	case isKey(msg, "r"):
		m.view = entitiesViewRelationships
		m.relLoading = true
		return m, m.loadRelationships()
	case isKey(msg, "h"):
		m.view = entitiesViewHistory
		m.historyLoading = true
		return m, m.loadHistory()
	case isKey(msg, "a"):
		m.contextCreating = true
		m.contextCreateBuf = ""
	case isKey(msg, "l"):
		m.contextLinking = true
		m.contextLinkBuf = ""
	case isKey(msg, "d"):
		m.confirmKind = "entity-archive"
		m.confirmReturn = entitiesViewDetail
		m.view = entitiesViewConfirm
	}
	return m, nil
}

func (m EntitiesModel) handleContextPromptKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.contextLinking = false
		m.contextLinkBuf = ""
		m.contextCreating = false
		m.contextCreateBuf = ""
		return m, nil
	case isKey(msg, "enter"):
		if m.contextLinking {
			value := strings.TrimSpace(m.contextLinkBuf)
			if value == "" {
				return m, func() tea.Msg { return errMsg{fmt.Errorf("context id is required")} }
			}
			m.contextLinking = false
			m.contextLinkBuf = ""
			m.contextLoading = true
			return m, m.linkContextToEntity(value)
		}
		if m.contextCreating {
			title := strings.TrimSpace(m.contextCreateBuf)
			if title == "" {
				return m, func() tea.Msg { return errMsg{fmt.Errorf("context title is required")} }
			}
			m.contextCreating = false
			m.contextCreateBuf = ""
			m.contextLoading = true
			return m, m.createContextForEntity(title)
		}
	case isKey(msg, "backspace", "delete"):
		if m.contextLinking && len(m.contextLinkBuf) > 0 {
			m.contextLinkBuf = m.contextLinkBuf[:len(m.contextLinkBuf)-1]
		}
		if m.contextCreating && len(m.contextCreateBuf) > 0 {
			m.contextCreateBuf = m.contextCreateBuf[:len(m.contextCreateBuf)-1]
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.contextLinking {
			m.contextLinkBuf = ""
		}
		if m.contextCreating {
			m.contextCreateBuf = ""
		}
	default:
		ch := keyText(msg)
		if ch != "" {
			if m.contextLinking {
				m.contextLinkBuf += ch
			}
			if m.contextCreating {
				m.contextCreateBuf += ch
			}
		}
	}
	return m, nil
}

// renderDetail renders render detail.
func (m EntitiesModel) renderDetail() string {
	if m.detail == nil {
		return m.renderList()
	}

	e := m.detail
	rows := []components.TableRow{
		{Label: "ID", Value: e.ID},
		{Label: "Name", Value: e.Name},
	}
	if e.Type != "" {
		rows = append(rows, components.TableRow{Label: "Type", Value: e.Type})
	}
	if e.Status != "" {
		rows = append(rows, components.TableRow{Label: "Status", Value: e.Status})
	}
	if len(e.Tags) > 0 {
		rows = append(rows, components.TableRow{Label: "Tags", Value: strings.Join(e.Tags, ", ")})
	}
	if len(e.PrivacyScopeIDs) > 0 {
		rows = append(rows, components.TableRow{Label: "Scopes", Value: m.formatEntityScopes(e.PrivacyScopeIDs)})
	}
	rows = append(rows, components.TableRow{Label: "Created", Value: formatLocalTimeFull(e.CreatedAt)})
	if !e.UpdatedAt.IsZero() {
		rows = append(rows, components.TableRow{Label: "Updated", Value: formatLocalTimeFull(e.UpdatedAt)})
	}
	if e.SourcePath != nil && *e.SourcePath != "" {
		rows = append(rows, components.TableRow{Label: "Source Path", Value: *e.SourcePath})
	}

	sections := []string{components.Table("Entity", rows, m.width)}
	if m.contextLoading {
		sections = append(sections, components.TitledBox("Context Items", MutedStyle.Render("Loading..."), m.width))
	} else {
		sections = append(sections, renderContextSummaryTable(m.detailContext, 6, m.width))
	}
	if len(m.detailRels) > 0 {
		sections = append(sections, renderRelationshipSummaryTable("entity", e.ID, m.detailRels, 8, m.width))
	}

	return strings.Join(sections, "\n\n")
}

// --- History ---

func (m EntitiesModel) loadHistory() tea.Cmd {
	if m.detail == nil {
		return nil
	}
	entityID := m.detail.ID
	return func() tea.Msg {
		items, err := m.client.GetEntityHistory(entityID, 50, 0)
		if err != nil {
			return errMsg{err}
		}
		return entityHistoryLoadedMsg{items: items}
	}
}

// loadScopeNames loads load scope names.
func (m EntitiesModel) loadScopeNames() tea.Cmd {
	return func() tea.Msg {
		scopes, err := m.client.ListAuditScopes()
		if err != nil {
			return errMsg{err}
		}
		names := map[string]string{}
		for _, scope := range scopes {
			names[scope.ID] = scope.Name
		}
		return entityScopesLoadedMsg{names: names}
	}
}

// handleHistoryKeys handles handle history keys.
func (m EntitiesModel) handleHistoryKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.view = entitiesViewDetail
	case isDown(msg):
		m.historyTable.MoveDown(1)
	case isUp(msg):
		m.historyTable.MoveUp(1)
	case isEnter(msg):
		if idx := m.historyTable.Cursor(); idx >= 0 && idx < len(m.history) {
			entry := m.history[idx]
			m.confirmKind = "entity-revert"
			m.confirmAuditID = entry.ID
			m.confirmReturn = entitiesViewDetail
			m.view = entitiesViewConfirm
		}
	}
	return m, nil
}

// renderHistory renders render history.
func (m EntitiesModel) renderHistory() string {
	if m.historyLoading {
		return "  " + MutedStyle.Render("Loading history...")
	}
	if len(m.history) == 0 {
		content := MutedStyle.Render("No history entries yet.")
		return components.Indent(components.Box(content, m.width), 1)
	}
	title := "History"
	if m.detail != nil {
		title = fmt.Sprintf("History - %s", components.SanitizeOneLine(m.detail.Name))
	}

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

	atWidth := compactTimeColumnWidth
	actionWidth := 10
	fieldsWidth := availableCols - (atWidth + actionWidth)
	if fieldsWidth < 14 {
		fieldsWidth = 14
		actionWidth = availableCols - (atWidth + fieldsWidth)
		if actionWidth < 8 {
			actionWidth = 8
		}
	}

	// Build rows from history entries.
	tableRows := make([]table.Row, len(m.history))
	for i, entry := range m.history {
		action := strings.TrimSpace(components.SanitizeOneLine(entry.Action))
		if action == "" {
			action = "update"
		}
		fields := "-"
		if n := len(entry.ChangedFields); n > 0 {
			fields = fmt.Sprintf("%d fields", n)
		}
		tableRows[i] = table.Row{
			formatLocalTimeCompact(entry.ChangedAt),
			components.ClampTextWidthEllipsis(strings.ToUpper(action), actionWidth),
			components.ClampTextWidthEllipsis(fields, fieldsWidth),
		}
	}

	m.historyTable.SetColumns([]table.Column{
		{Title: "At", Width: atWidth},
		{Title: "Action", Width: actionWidth},
		{Title: "Fields", Width: fieldsWidth},
	})
	m.historyTable.SetWidth(tableWidth)
	m.historyTable.SetRows(tableRows)

	countLine := MutedStyle.Render(fmt.Sprintf("%d entries", len(m.history)))
	tableView := m.historyTable.View()
	preview := ""
	var previewItem *api.AuditEntry
	if idx := m.historyTable.Cursor(); idx >= 0 && idx < len(m.history) {
		previewItem = &m.history[idx]
	}
	if previewItem != nil {
		content := m.renderEntityHistoryPreview(*previewItem, previewBoxContentWidth(previewWidth))
		preview = renderPreviewBox(content, previewWidth)
	}

	body := tableView
	if sideBySide && preview != "" {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
	} else if preview != "" {
		body = tableView + "\n\n" + preview
	}

	content := countLine + "\n\n" + body + "\n"
	return components.Indent(components.TitledBox(title, content, m.width), 1)
}

// renderEntityHistoryPreview renders render entity history preview.
func (m EntitiesModel) renderEntityHistoryPreview(entry api.AuditEntry, width int) string {
	if width <= 0 {
		return ""
	}

	action := strings.TrimSpace(components.SanitizeOneLine(entry.Action))
	if action == "" {
		action = "update"
	}
	heading := strings.ToUpper(action) + " @ " + formatLocalTimeFull(entry.ChangedAt)

	var lines []string
	lines = append(lines, MetaKeyStyle.Render("Selected"))
	for _, part := range wrapPreviewText(heading, width) {
		lines = append(lines, SelectedStyle.Render(part))
	}
	lines = append(lines, "")

	lines = append(lines, renderPreviewRow("Action", strings.ToUpper(action), width))
	lines = append(lines, renderPreviewRow("At", formatLocalTimeFull(entry.ChangedAt), width))
	if len(entry.ChangedFields) > 0 {
		lines = append(lines, renderPreviewRow("Fields", strings.Join(entry.ChangedFields, ", "), width))
	}
	if entry.ChangeReason != nil && strings.TrimSpace(*entry.ChangeReason) != "" {
		lines = append(lines, renderPreviewRow("Reason", strings.TrimSpace(*entry.ChangeReason), width))
	}

	return padPreviewLines(lines, width)
}

// --- Edit Entity ---

func (m *EntitiesModel) startEdit() {
	if m.detail == nil {
		return
	}
	m.editTagStr = strings.Join(m.detail.Tags, ", ")
	m.editStatus = m.detail.Status
	if m.editStatus == "" {
		m.editStatus = "active"
	}
	m.editScopeStr = strings.Join(m.scopeNamesFromIDs(m.detail.PrivacyScopeIDs), ", ")
	m.editScopesDirty = false
	m.editSaving = false
	m.initEditForm()
}

// handleEditKeys handles edit key events by forwarding to the huh form.
func (m EntitiesModel) handleEditKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
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
		m.view = entitiesViewDetail
		return m, nil
	}

	return m, cmd
}

// renderEdit renders the edit entity form.
func (m EntitiesModel) renderEdit() string {
	if m.detail == nil {
		return m.renderList()
	}

	var b strings.Builder
	b.WriteString(MutedStyle.Render("Entity: " + components.SanitizeOneLine(m.detail.Name)))
	b.WriteString("\n\n")

	if m.editForm != nil {
		b.WriteString(m.editForm.View())
	}

	if m.editSaving {
		b.WriteString("\n\n" + MutedStyle.Render("Saving..."))
	}

	return components.Indent(components.TitledBox("Edit Entity", b.String(), m.width), 1)
}

// saveEdit handles save edit.
func (m EntitiesModel) saveEdit() (EntitiesModel, tea.Cmd) {
	if m.detail == nil {
		return m, nil
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

	input := api.UpdateEntityInput{
		Status: &status,
		Tags:   &tags,
	}

	// Determine if scopes changed from original.
	newScopes := parseCommaSeparated(m.editScopeStr)
	for i, s := range newScopes {
		newScopes[i] = normalizeScope(s)
	}
	origScopes := m.scopeNamesFromIDs(m.detail.PrivacyScopeIDs)
	m.editScopesDirty = !stringSlicesEqual(newScopes, origScopes)

	m.editSaving = true
	return m, func() tea.Msg {
		updated, err := m.client.UpdateEntity(m.detail.ID, input)
		if err != nil {
			return errMsg{err}
		}
		if m.editScopesDirty {
			scopeInput := api.BulkUpdateEntityScopesInput{
				EntityIDs: []string{m.detail.ID},
				Scopes:    normalizeBulkScopes(newScopes),
				Op:        "set",
			}
			if _, err := m.client.BulkUpdateEntityScopes(scopeInput); err != nil {
				return errMsg{err}
			}
			updated, err = m.client.GetEntity(m.detail.ID)
			if err != nil {
				return errMsg{err}
			}
		}
		return entityUpdatedMsg{entity: *updated}
	}
}

// initEditForm creates a new huh form for the edit entity flow.
func (m *EntitiesModel) initEditForm() {
	m.editForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Tags (comma-separated)").Value(&m.editTagStr),
			huh.NewSelect[string]().Title("Status").Options(
				huh.NewOption("active", "active"),
				huh.NewOption("inactive", "inactive"),
			).Value(&m.editStatus),
			huh.NewInput().Title("Scopes (comma-separated)").Value(&m.editScopeStr),
		),
	).WithTheme(huh.ThemeFunc(huh.ThemeDracula)).WithWidth(60)
}

// stringSlicesEqual reports whether two string slices contain the same elements.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- Confirm ---

func (m EntitiesModel) handleConfirmKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isKey(msg, "y"), isEnter(msg):
		switch m.confirmKind {
		case "entity-archive":
			if m.detail == nil {
				m.view = m.confirmReturn
				m.resetConfirmState()
				return m, nil
			}
			status := "inactive"
			input := api.UpdateEntityInput{Status: &status}
			m.view = m.confirmReturn
			m.resetConfirmState()
			return m, func() tea.Msg {
				updated, err := m.client.UpdateEntity(m.detail.ID, input)
				if err != nil {
					return errMsg{err}
				}
				return entityUpdatedMsg{entity: *updated}
			}
		case "entity-revert":
			if m.detail == nil || m.confirmAuditID == "" {
				m.view = m.confirmReturn
				m.resetConfirmState()
				return m, nil
			}
			entityID := m.detail.ID
			auditID := m.confirmAuditID
			m.view = m.confirmReturn
			m.resetConfirmState()
			return m, func() tea.Msg {
				updated, err := m.client.RevertEntity(entityID, auditID)
				if err != nil {
					return errMsg{err}
				}
				return entityRevertedMsg{entity: *updated}
			}
		case "rel-archive":
			if m.confirmRelID == "" {
				m.view = m.confirmReturn
				m.resetConfirmState()
				return m, nil
			}
			status := "inactive"
			input := api.UpdateRelationshipInput{Status: &status}
			m.view = m.confirmReturn
			m.resetConfirmState()
			return m, func() tea.Msg {
				updated, err := m.client.UpdateRelationship(m.confirmRelID, input)
				if err != nil {
					return errMsg{err}
				}
				return relationshipUpdatedMsg{rel: *updated}
			}
		}
	case isKey(msg, "n"), isBack(msg):
		m.view = m.confirmReturn
		m.resetConfirmState()
	}
	return m, nil
}

// renderConfirm renders render confirm.
func (m EntitiesModel) renderConfirm() string {
	title := "Confirm"
	var summary []components.TableRow
	var diffs []components.DiffRow

	switch m.confirmKind {
	case "entity-archive":
		title = "Archive Entity"
		if m.detail != nil {
			summary = append(summary,
				components.TableRow{Label: "Entity", Value: m.detail.Name},
				components.TableRow{Label: "ID", Value: m.detail.ID},
			)
			diffs = append(diffs, components.DiffRow{
				Label: "status",
				From:  firstNonEmpty(m.detail.Status, "active"),
				To:    "inactive",
			})
		}
	case "entity-revert":
		title = "Revert Entity"
		if m.detail != nil {
			summary = append(summary,
				components.TableRow{Label: "Entity", Value: m.detail.Name},
				components.TableRow{Label: "ID", Value: m.detail.ID},
				components.TableRow{Label: "Audit ID", Value: m.confirmAuditID},
			)
		}
		if m.confirmAudit != nil {
			summary = append(summary, components.TableRow{
				Label: "Changed At",
				Value: formatLocalTimeFull(m.confirmAudit.ChangedAt),
			})
			diffs = append(diffs, buildAuditDiffRows(*m.confirmAudit)...)
		}
	case "rel-archive":
		title = "Archive Relationship"
		if rel := m.selectedRelationshipByID(m.confirmRelID); rel != nil {
			summary = append(summary,
				components.TableRow{Label: "Relationship", Value: rel.Type},
				components.TableRow{Label: "ID", Value: rel.ID},
				components.TableRow{Label: "Source", Value: m.relationshipNodeLabel(rel.SourceName, rel.SourceID, rel.SourceType)},
				components.TableRow{Label: "Target", Value: m.relationshipNodeLabel(rel.TargetName, rel.TargetID, rel.TargetType)},
			)
			diffs = append(diffs, components.DiffRow{
				Label: "status",
				From:  firstNonEmpty(rel.Status, "active"),
				To:    "inactive",
			})
		}
	}

	return components.Indent(components.ConfirmPreviewDialog(title, summary, diffs, m.width), 1)
}

// resetConfirmState handles reset confirm state.
func (m *EntitiesModel) resetConfirmState() {
	m.confirmKind = ""
	m.confirmRelID = ""
	m.confirmAuditID = ""
	m.confirmAudit = nil
}

// selectedRelationshipByID handles selected relationship by id.
func (m EntitiesModel) selectedRelationshipByID(id string) *api.Relationship {
	if id == "" {
		return nil
	}
	for i := range m.rels {
		if m.rels[i].ID == id {
			rel := m.rels[i]
			return &rel
		}
	}
	return nil
}

// relationshipNodeLabel handles relationship node label.
func (m EntitiesModel) relationshipNodeLabel(name, id, typ string) string {
	label := strings.TrimSpace(name)
	if label == "" {
		label = shortID(id)
	}
	if strings.TrimSpace(typ) == "" {
		return label
	}
	return fmt.Sprintf("%s (%s)", label, typ)
}

// firstNonEmpty handles first non empty.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "-"
}

// --- Relationships ---

func (m EntitiesModel) handleRelationshipsKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.view = entitiesViewDetail
	case isDown(msg):
		m.relTable.MoveDown(1)
	case isUp(msg):
		m.relTable.MoveUp(1)
	case isKey(msg, "n"):
		m.startRelate()
		m.view = entitiesViewRelateSearch
	case isKey(msg, "e"):
		if m.selectedRelationship() != nil {
			m.startRelEdit()
			m.view = entitiesViewRelEdit
		}
	case isKey(msg, "d"):
		if rel := m.selectedRelationship(); rel != nil {
			m.confirmKind = "rel-archive"
			m.confirmRelID = rel.ID
			m.confirmReturn = entitiesViewRelationships
			m.view = entitiesViewConfirm
		}
	}
	return m, nil
}

// renderRelationships renders render relationships.
func (m EntitiesModel) renderRelationships() string {
	if m.relLoading {
		return "  " + MutedStyle.Render("Loading relationships...")
	}

	if len(m.rels) == 0 {
		content := MutedStyle.Render("No relationships yet.")
		return components.Indent(components.Box(content, m.width), 1)
	}

	idx := m.relTable.Cursor()
	if idx < 0 || idx >= len(m.rels) {
		idx = 0
	}
	rel := m.rels[idx]
	direction, other := m.relationshipDirection(rel)

	rows := []components.TableRow{
		{Label: "Index", Value: fmt.Sprintf("%d of %d", idx+1, len(m.rels))},
		{Label: "Type", Value: rel.Type},
		{Label: "Status", Value: rel.Status},
		{Label: "Direction", Value: direction},
		{Label: "Other", Value: other},
		{Label: "Created", Value: formatLocalTimeFull(rel.CreatedAt)},
	}

	sections := []string{components.Table("Relationship", rows, m.width)}
	if len(rel.Properties) > 0 {
		props := renderMetadataBlock(map[string]any(rel.Properties), m.width, true)
		if props != "" {
			sections = append(sections, props)
		}
	}
	return components.Indent(strings.Join(sections, "\n\n"), 1)
}

// --- Relate Flow ---

func (m *EntitiesModel) startRelate() {
	m.relateQuery = ""
	m.relateResults = nil
	m.relateTable.SetRows([]table.Row{})
	m.relateTable.SetCursor(0)
	m.relateTarget = nil
	m.relateType = ""
	m.relateLoading = false
}

// handleRelateKeys handles handle relate keys.
func (m EntitiesModel) handleRelateKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch m.view {
	case entitiesViewRelateSearch:
		switch {
		case isBack(msg):
			m.view = entitiesViewRelationships
		case isEnter(msg):
			query := strings.TrimSpace(m.relateQuery)
			if query == "" {
				return m, nil
			}
			m.relateLoading = true
			m.view = entitiesViewRelateSelect
			return m, m.loadRelateResults(query)
		case isKey(msg, "backspace"):
			if len(m.relateQuery) > 0 {
				m.relateQuery = m.relateQuery[:len(m.relateQuery)-1]
			}
		default:
			if ch := keyText(msg); ch != "" {
				m.relateQuery += ch
			}
		}
	case entitiesViewRelateSelect:
		switch {
		case isBack(msg):
			m.view = entitiesViewRelateSearch
		case isDown(msg):
			m.relateTable.MoveDown(1)
		case isUp(msg):
			m.relateTable.MoveUp(1)
		case isEnter(msg):
			if idx := m.relateTable.Cursor(); idx >= 0 && idx < len(m.relateResults) {
				item := m.relateResults[idx]
				m.relateTarget = &item
				m.relateType = ""
				m.view = entitiesViewRelateType
			}
		}
	case entitiesViewRelateType:
		switch {
		case isBack(msg):
			m.view = entitiesViewRelateSelect
		case isEnter(msg):
			if m.relateTarget == nil {
				return m, nil
			}
			kind := strings.TrimSpace(m.relateType)
			if kind == "" {
				return m, nil
			}
			m.view = entitiesViewRelationships
			m.relLoading = true
			return m, m.createRelationship(*m.detail, *m.relateTarget, kind)
		case isKey(msg, "backspace"):
			if len(m.relateType) > 0 {
				m.relateType = m.relateType[:len(m.relateType)-1]
			}
		default:
			if ch := keyText(msg); ch != "" {
				m.relateType += ch
			}
		}
	}
	return m, nil
}

// renderRelate renders render relate.
func (m EntitiesModel) renderRelate() string {
	switch m.view {
	case entitiesViewRelateSearch:
		return components.Indent(components.InputDialog("Search Entity", m.relateQuery), 1)
	case entitiesViewRelateSelect:
		if m.relateLoading {
			return "  " + MutedStyle.Render("Searching...")
		}
		if len(m.relateResults) == 0 {
			content := MutedStyle.Render("No matches. Press Esc to go back.")
			return components.Indent(components.Box(content, m.width), 1)
		}

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

		typeWidth := 14
		statusWidth := 11
		nameWidth := availableCols - (typeWidth + statusWidth)

		// Build rows from relate results.
		tableRows := make([]table.Row, len(m.relateResults))
		for i, e := range m.relateResults {
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

		m.relateTable.SetColumns([]table.Column{
			{Title: "Name", Width: nameWidth},
			{Title: "Type", Width: typeWidth},
			{Title: "Status", Width: statusWidth},
		})
		m.relateTable.SetWidth(tableWidth)
		m.relateTable.SetRows(tableRows)

		countLine := MutedStyle.Render(fmt.Sprintf("%d results", len(m.relateResults)))
		tableView := m.relateTable.View()
		preview := ""
		var previewItem *api.Entity
		if idx := m.relateTable.Cursor(); idx >= 0 && idx < len(m.relateResults) {
			previewItem = &m.relateResults[idx]
		}
		if previewItem != nil {
			content := m.renderRelateEntityPreview(*previewItem, previewBoxContentWidth(previewWidth))
			preview = renderPreviewBox(content, previewWidth)
		}

		body := tableView
		if sideBySide && preview != "" {
			body = lipgloss.JoinHorizontal(lipgloss.Top, tableView, strings.Repeat(" ", gap), preview)
		} else if preview != "" {
			body = tableView + "\n\n" + preview
		}

		content := countLine + "\n\n" + body + "\n"
		return components.Indent(components.TitledBox("Select Entity", content, m.width), 1)
	case entitiesViewRelateType:
		return components.Indent(components.InputDialog("Relationship Type", m.relateType), 1)
	}
	return ""
}

// renderRelateEntityPreview renders render relate entity preview.
func (m EntitiesModel) renderRelateEntityPreview(e api.Entity, width int) string {
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

// --- Relationship Edit ---

func (m *EntitiesModel) startRelEdit() {
	rel := m.selectedRelationship()
	if rel == nil {
		return
	}
	m.relEditID = rel.ID
	m.relEditFocus = relEditFieldStatus
	m.relEditStatusIdx = statusIndex(relationshipStatusOptions, rel.Status)
	m.relEditBuf = compactJSON(map[string]any(rel.Properties))
}

// handleRelEditKeys handles handle rel edit keys.
func (m EntitiesModel) handleRelEditKeys(msg tea.KeyPressMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.relEditFocus = (m.relEditFocus + 1) % relEditFieldCount
	case isUp(msg):
		if m.relEditFocus > 0 {
			m.relEditFocus = (m.relEditFocus - 1 + relEditFieldCount) % relEditFieldCount
		}
	case isBack(msg):
		m.view = entitiesViewRelationships
	case isKey(msg, "ctrl+s"):
		return m.saveRelEdit()
	case isKey(msg, "backspace"):
		if m.relEditFocus == relEditFieldProperties && len(m.relEditBuf) > 0 {
			m.relEditBuf = m.relEditBuf[:len(m.relEditBuf)-1]
		}
	default:
		switch m.relEditFocus {
		case relEditFieldStatus:
			switch {
			case isKey(msg, "left"):
				m.relEditStatusIdx = (m.relEditStatusIdx - 1 + len(relationshipStatusOptions)) % len(relationshipStatusOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.relEditStatusIdx = (m.relEditStatusIdx + 1) % len(relationshipStatusOptions)
			}
		case relEditFieldProperties:
			ch := keyText(msg)
			if ch != "" {
				m.relEditBuf += ch
			}
		}
	}
	return m, nil
}

// renderRelEdit renders render rel edit.
func (m EntitiesModel) renderRelEdit() string {
	status := relationshipStatusOptions[m.relEditStatusIdx]
	var b strings.Builder

	if m.relEditFocus == relEditFieldStatus {
		b.WriteString(SelectedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	} else {
		b.WriteString(MutedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	}

	b.WriteString("\n\n")

	if m.relEditFocus == relEditFieldProperties {
		b.WriteString(SelectedStyle.Render("  Properties (JSON):"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + m.relEditBuf))
		b.WriteString(AccentStyle.Render("█"))
	} else {
		b.WriteString(MutedStyle.Render("  Properties (JSON):"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + m.relEditBuf))
	}

	return components.Indent(components.TitledBox("Edit Relationship", b.String(), m.width), 1)
}

// saveRelEdit handles save rel edit.
func (m EntitiesModel) saveRelEdit() (EntitiesModel, tea.Cmd) {
	status := relationshipStatusOptions[m.relEditStatusIdx]
	input := api.UpdateRelationshipInput{Status: &status}
	if strings.TrimSpace(m.relEditBuf) != "" {
		props, err := parseJSONMap(m.relEditBuf)
		if err != nil {
			m.errText = err.Error()
			return m, nil
		}
		input.Properties = props
	}

	m.view = entitiesViewRelationships
	return m, func() tea.Msg {
		updated, err := m.client.UpdateRelationship(m.relEditID, input)
		if err != nil {
			return errMsg{err}
		}
		return relationshipUpdatedMsg{rel: *updated}
	}
}

// --- Helpers ---

func (m EntitiesModel) loadEntities(search string) func() tea.Msg {
	return func() tea.Msg {
		params := api.QueryParams{}
		if search != "" {
			params["search_text"] = search
		}
		items, err := m.client.QueryEntities(params)
		if err != nil {
			return errMsg{err}
		}
		return entitiesLoadedMsg{items}
	}
}

// loadEntityDetailRelationships loads load entity detail relationships.
func (m EntitiesModel) loadEntityDetailRelationships(entityID string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.GetRelationships("entity", entityID)
		if err != nil {
			return entityDetailRelationshipsLoadedMsg{id: entityID, items: nil}
		}
		return entityDetailRelationshipsLoadedMsg{id: entityID, items: items}
	}
}

// loadEntityContext loads context items linked to entity.
func (m EntitiesModel) loadEntityContext(entityID string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListContextByOwner("entity", entityID, api.QueryParams{
			"limit":  "50",
			"offset": "0",
		})
		if err != nil {
			return entityContextLoadedMsg{id: entityID, items: nil}
		}
		return entityContextLoadedMsg{id: entityID, items: items}
	}
}

func (m EntitiesModel) linkContextToEntity(contextID string) tea.Cmd {
	return func() tea.Msg {
		if m.detail == nil {
			return errMsg{fmt.Errorf("no entity selected")}
		}
		ownerID := m.detail.ID
		if err := m.client.LinkContext(contextID, api.LinkContextInput{
			OwnerType: "entity",
			OwnerID:   ownerID,
		}); err != nil {
			return errMsg{err}
		}
		items, err := m.client.ListContextByOwner("entity", ownerID, api.QueryParams{
			"limit":  "50",
			"offset": "0",
		})
		if err != nil {
			return errMsg{err}
		}
		return entityContextLoadedMsg{id: ownerID, items: items}
	}
}

func (m EntitiesModel) createContextForEntity(title string) tea.Cmd {
	return func() tea.Msg {
		if m.detail == nil {
			return errMsg{fmt.Errorf("no entity selected")}
		}
		input := api.CreateContextInput{
			Title:      title,
			SourceType: "note",
			Scopes:     m.contextScopesForEntity(),
			Tags:       []string{},
		}
		created, err := m.client.CreateContext(input)
		if err != nil {
			return errMsg{err}
		}
		if err := m.client.LinkContext(created.ID, api.LinkContextInput{
			OwnerType: "entity",
			OwnerID:   m.detail.ID,
		}); err != nil {
			return errMsg{err}
		}
		items, err := m.client.ListContextByOwner("entity", m.detail.ID, api.QueryParams{
			"limit":  "50",
			"offset": "0",
		})
		if err != nil {
			return errMsg{err}
		}
		return entityContextLoadedMsg{id: m.detail.ID, items: items}
	}
}

func (m EntitiesModel) contextScopesForEntity() []string {
	if m.detail == nil {
		return []string{"private"}
	}
	if len(m.detail.PrivacyScopeIDs) == 0 {
		return []string{"private"}
	}
	scopes := make([]string, 0, len(m.detail.PrivacyScopeIDs))
	for _, id := range m.detail.PrivacyScopeIDs {
		if name, ok := m.scopeNames[id]; ok && strings.TrimSpace(name) != "" {
			scopes = append(scopes, name)
		}
	}
	if len(scopes) == 0 {
		return []string{"private"}
	}
	return scopes
}

// loadRelationships loads load relationships.
func (m EntitiesModel) loadRelationships() tea.Cmd {
	return func() tea.Msg {
		if m.detail == nil {
			return relationshipsLoadedMsg{items: nil}
		}
		items, err := m.client.GetRelationships("entity", m.detail.ID)
		if err != nil {
			return errMsg{err}
		}
		return relationshipsLoadedMsg{items: items}
	}
}

// loadRelateResults loads load relate results.
func (m EntitiesModel) loadRelateResults(query string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.QueryEntities(api.QueryParams{"search_text": query})
		if err != nil {
			return errMsg{err}
		}
		return relateResultsMsg{items: items}
	}
}

// createRelationship creates create relationship.
func (m EntitiesModel) createRelationship(source api.Entity, target api.Entity, relType string) tea.Cmd {
	return func() tea.Msg {
		input := api.CreateRelationshipInput{
			SourceType: "entity",
			SourceID:   source.ID,
			TargetType: "entity",
			TargetID:   target.ID,
			Type:       relType,
		}
		created, err := m.client.CreateRelationship(input)
		if err != nil {
			return errMsg{err}
		}
		return relationshipCreatedMsg{rel: *created}
	}
}

// applyEntityUpdate handles apply entity update.
func (m *EntitiesModel) applyEntityUpdate(updated api.Entity) {
	m.detail = &updated
	for i := range m.items {
		if m.items[i].ID == updated.ID {
			m.items[i] = updated
			break
		}
	}
	// Also update allItems so filters stay consistent.
	for i := range m.allItems {
		if m.allItems[i].ID == updated.ID {
			m.allItems[i] = updated
			break
		}
	}
	m.applyEntityFilters()
}

// selectedRelationship handles selected relationship.
func (m EntitiesModel) selectedRelationship() *api.Relationship {
	if len(m.rels) == 0 {
		return nil
	}
	idx := m.relTable.Cursor()
	if idx < 0 || idx >= len(m.rels) {
		return nil
	}
	return &m.rels[idx]
}

// relationshipDirection handles relationship direction.
func (m EntitiesModel) relationshipDirection(rel api.Relationship) (string, string) {
	if m.detail == nil {
		return "", relationshipLabel(rel.TargetID, rel.TargetName)
	}
	if rel.SourceID == m.detail.ID {
		return "outgoing", relationshipLabel(rel.TargetID, rel.TargetName)
	}
	return "incoming", relationshipLabel(rel.SourceID, rel.SourceName)
}

// formatRelationshipLine handles format relationship line.
func (m EntitiesModel) formatRelationshipLine(rel api.Relationship) string {
	direction, other := m.relationshipDirection(rel)
	label := rel.Type
	if label == "" {
		label = "relationship"
	}
	if direction != "" {
		return fmt.Sprintf("%s (%s -> %s)", label, direction, other)
	}
	return fmt.Sprintf("%s (%s)", label, other)
}

// statusIndex handles status index.
func statusIndex(options []string, value string) int {
	for i, opt := range options {
		if opt == value {
			return i
		}
	}
	return 0
}

// compactJSON handles compact json.
func compactJSON(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
}

// parseJSONMap parses parse jsonmap.
func parseJSONMap(input string) (map[string]any, error) {
	var data map[string]any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return data, nil
}

// shortID handles short id.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// formatEntityScopes handles format entity scopes.
func (m EntitiesModel) formatEntityScopes(ids []string) string {
	if len(ids) == 0 {
		return "-"
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := m.scopeNames[id]; ok && name != "" {
			names = append(names, name)
		} else {
			names = append(names, shortID(id))
		}
	}
	return formatScopePreview(names)
}

// scopeNamesFromIDs handles scope names from ids.
func (m EntitiesModel) scopeNamesFromIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := m.scopeNames[id]; ok && name != "" {
			out = append(out, name)
		} else {
			out = append(out, id)
		}
	}
	return out
}

// formatEntityLine handles format entity line.
func formatEntityLine(e api.Entity) string {
	return formatEntityLineWidth(e, maxEntityLineLen)
}

// formatEntityLineWidth handles format entity line width.
func formatEntityLineWidth(e api.Entity, maxWidth int) string {
	name, t := normalizeEntityNameType(
		components.SanitizeText(e.Name),
		components.SanitizeText(e.Type),
	)
	if t == "" {
		t = "?"
	}
	lineWidth := maxWidth
	if lineWidth <= 0 || lineWidth > maxEntityLineLen {
		lineWidth = maxEntityLineLen
	}
	header := formatEntityHeader(name, strings.ToLower(components.SanitizeText(t)), lineWidth)
	segments := []string{header}
	if status := strings.TrimSpace(components.SanitizeText(e.Status)); status != "" {
		segments = append(segments, status)
	}
	if tagPreview := previewTags(e.Tags, 2); tagPreview != "" {
		segments = append(segments, tagPreview)
	}
	return joinEntitySegments(segments, lineWidth)
}

// previewTags handles preview tags.
func previewTags(tags []string, max int) string {
	if len(tags) == 0 || max <= 0 {
		return ""
	}
	cleaned := make([]string, len(tags))
	for i, tag := range tags {
		cleaned[i] = components.SanitizeText(tag)
	}
	if len(tags) <= max {
		return strings.Join(cleaned, ", ")
	}
	head := strings.Join(cleaned[:max], ", ")
	return fmt.Sprintf("%s +%d", head, len(cleaned)-max)
}

// formatHistoryLine handles format history line.
func formatHistoryLine(entry api.AuditEntry) string {
	action := components.SanitizeText(entry.Action)
	if action == "" {
		action = "update"
	}
	when := formatLocalTimeFull(entry.ChangedAt)
	fieldCount := len(entry.ChangedFields)
	fields := ""
	if fieldCount > 0 {
		fields = fmt.Sprintf(" (%d fields)", fieldCount)
	}
	return fmt.Sprintf("%s %s%s", when, action, fields)
}

// relationshipLabel handles relationship label.
func relationshipLabel(id, name string) string {
	if strings.TrimSpace(name) != "" {
		return components.SanitizeText(name)
	}
	return shortID(id)
}

// formatEntityHeader handles format entity header.
func formatEntityHeader(name string, typ string, maxWidth int) string {
	name = truncateString(components.SanitizeText(name), maxEntityNameLen)
	if strings.TrimSpace(typ) == "" {
		typ = "?"
	}
	badge := TypeBadgeStyle.Render(components.SanitizeText(typ))
	header := fmt.Sprintf("%s %s", name, badge)
	if maxWidth <= 0 || lipgloss.Width(header) <= maxWidth {
		return header
	}
	badgeWidth := lipgloss.Width(" " + badge)
	available := maxWidth - badgeWidth
	if available < 4 {
		available = 4
	}
	trimmed := truncateString(name, available)
	return fmt.Sprintf("%s %s", trimmed, badge)
}

// joinEntitySegments handles join entity segments.
func joinEntitySegments(segments []string, maxWidth int) string {
	if len(segments) == 0 {
		return ""
	}
	line := strings.Join(segments, " · ")
	if maxWidth <= 0 || lipgloss.Width(line) <= maxWidth {
		return line
	}
	for len(segments) > 1 && lipgloss.Width(line) > maxWidth {
		segments = segments[:len(segments)-1]
		line = strings.Join(segments, " · ")
	}
	return line
}

// normalizeEntityNameType handles normalize entity name type.
func normalizeEntityNameType(name, typ string) (string, string) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return name, strings.TrimSpace(typ)
	}
	if strings.HasPrefix(trimmed, "[") {
		if idx := strings.Index(trimmed, "]"); idx > 1 {
			prefix := strings.TrimSpace(trimmed[1:idx])
			if prefix != "" && (typ == "" || strings.EqualFold(prefix, typ)) {
				if typ == "" {
					typ = prefix
				}
				trimmed = strings.TrimSpace(trimmed[idx+1:])
			}
		}
	}
	if typ != "" {
		typ = strings.ToLower(strings.TrimSpace(typ))
	}
	return trimmed, typ
}

// parseBulkInput parses parse bulk input.
func parseBulkInput(raw string) (bulkInput, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return bulkInput{}, fmt.Errorf("enter values like add:tag1,tag2")
	}

	op := "add"
	lower := strings.ToLower(input)
	switch {
	case strings.HasPrefix(lower, "add:"):
		op = "add"
		input = input[4:]
	case strings.HasPrefix(lower, "remove:"):
		op = "remove"
		input = input[7:]
	case strings.HasPrefix(lower, "set:"):
		op = "set"
		input = input[4:]
	case strings.HasPrefix(input, "+"):
		op = "add"
		input = input[1:]
	case strings.HasPrefix(input, "-"):
		op = "remove"
		input = input[1:]
	case strings.HasPrefix(input, "="):
		op = "set"
		input = input[1:]
	}

	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ' '
	})
	values := make([]string, 0, len(parts))
	for _, p := range parts {
		val := strings.TrimSpace(p)
		if val == "" {
			continue
		}
		values = append(values, val)
	}

	if op != "set" && len(values) == 0 {
		return bulkInput{}, fmt.Errorf("no values provided")
	}

	return bulkInput{op: op, values: values}, nil
}

// normalizeBulkTags handles normalize bulk tags.
func normalizeBulkTags(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		tag := normalizeTag(v)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

// normalizeBulkScopes handles normalize bulk scopes.
func normalizeBulkScopes(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, v := range values {
		scope := strings.ToLower(strings.TrimSpace(v))
		scope = strings.TrimPrefix(scope, "#")
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	return out
}

const maxEntityNameLen = 80
const maxEntityLineLen = 128

// truncateString handles truncate string.
func truncateString(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

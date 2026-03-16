package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	addFieldName = iota
	addFieldType
	addFieldStatus
	addFieldTags
	addFieldScopes
	addFieldCount
)

const (
	editFieldTags = iota
	editFieldStatus
	editFieldScopes
	editFieldCount
)

const (
	relEditFieldStatus = iota
	relEditFieldProperties
	relEditFieldCount
)

var entityStatusOptions = []string{"active", "inactive"}
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
	list           *components.List
	loading        bool
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
	addFields         []formField
	addFocus          int
	addStatusIdx      int
	addTags           []string
	addTagBuf         string
	addScopes         []string
	addScopeBuf       string
	addScopeIdx       int
	addScopeSelecting bool
	addSaving         bool
	addSaved          bool

	// edit
	editFocus          int
	editTags           []string
	editTagBuf         string
	editStatusIdx      int
	editScopes         []string
	editScopeBuf       string
	editScopeIdx       int
	editScopeSelecting bool
	editScopesDirty    bool
	editSaving         bool

	// confirm
	confirmKind    string
	confirmReturn  entitiesView
	confirmRelID   string
	confirmAuditID string
	confirmAudit   *api.AuditEntry

	// relationships
	rels       []api.Relationship
	relList    *components.List
	relLoading bool

	scopeNames   map[string]string
	scopeOptions []string

	// history
	history        []api.AuditEntry
	historyList    *components.List
	historyLoading bool

	// relate flow
	relateQuery   string
	relateResults []api.Entity
	relateList    *components.List
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
		client: client,
		list:   components.NewList(15),
		addFields: []formField{
			{label: "Name"},
			{label: "Type"},
			{label: "Status"},
			{label: "Tags"},
			{label: "Scopes"},
		},
		relList:      components.NewList(8),
		relateList:   components.NewList(8),
		historyList:  components.NewList(8),
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
	m.addFocus = 0
	m.addStatusIdx = statusIndex(entityStatusOptions, "active")
	m.addTags = nil
	m.addTagBuf = ""
	m.addScopes = nil
	m.addScopeBuf = ""
	m.addScopeIdx = 0
	m.addScopeSelecting = false
	m.addSaving = false
	m.addSaved = false
	return tea.Batch(
		m.loadEntities(""),
		m.loadScopeNames(),
	)
}

// Update updates update.
func (m EntitiesModel) Update(msg tea.Msg) (EntitiesModel, tea.Cmd) {
	switch msg := msg.(type) {
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
		labels := make([]string, len(msg.items))
		for i, r := range msg.items {
			labels[i] = m.formatRelationshipLine(r)
		}
		m.relList.SetItems(labels)
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
		labels := make([]string, len(msg.items))
		for i, e := range msg.items {
			labels[i] = formatEntityLine(e)
		}
		m.relateList.SetItems(labels)
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
		return m, m.loadEntities("")

	case relationshipUpdatedMsg:
		m.relLoading = true
		return m, m.loadRelationships()

	case relationshipCreatedMsg:
		m.relLoading = true
		return m, m.loadRelationships()

	case entityHistoryLoadedMsg:
		m.historyLoading = false
		m.history = msg.items
		labels := make([]string, len(msg.items))
		for i, entry := range msg.items {
			labels[i] = formatHistoryLine(entry)
		}
		m.historyList.SetItems(labels)
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
		return m, m.loadEntities(strings.TrimSpace(m.searchBuf))
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

	case tea.KeyMsg:
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
		return components.Indent(body, 1)
	}
	switch m.view {
	case entitiesViewSearch:
		return components.Indent(components.InputDialog("Search Entities", m.searchBuf), 1)
	case entitiesViewEdit:
		return m.renderEdit()
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

func (m EntitiesModel) handleListKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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
		m.list.Down()
	case isUp(msg):
		if m.list.Selected() == 0 {
			m.modeFocus = true
		} else {
			m.list.Up()
		}
	case isSpace(msg):
		if m.searchBuf == "" {
			m.toggleBulkSelection(m.list.Selected())
			return m, nil
		}
		m.searchBuf += " "
		m.loading = true
		return m, m.loadEntities(strings.TrimSpace(m.searchBuf))
	case isEnter(msg):
		if idx := m.list.Selected(); idx < len(m.items) {
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
			return m, m.loadEntities(strings.TrimSpace(m.searchBuf))
		}
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
			m.loading = true
			return m, m.loadEntities(strings.TrimSpace(m.searchBuf))
		}
	case isKey(msg, "cmd+backspace", "cmd+delete", "ctrl+u"):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.loading = true
			return m, m.loadEntities("")
		}
	case isBack(msg):
		if m.searchBuf != "" {
			m.searchBuf = ""
			m.searchSuggest = ""
			m.loading = true
			return m, m.loadEntities("")
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
		ch := msg.String()
		if len(ch) == 1 {
			m.searchBuf += ch
			m.loading = true
			return m, m.loadEntities(strings.TrimSpace(m.searchBuf))
		}
	}
	return m, nil
}

// handleFilterInput handles handle filter input.
func (m EntitiesModel) handleFilterInput(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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
	labels := make([]string, len(filtered))
	for i, e := range filtered {
		labels[i] = formatEntityLine(e)
	}
	m.list.SetItems(labels)
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
func (m EntitiesModel) handleModeKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isDown(msg):
		m.modeFocus = false
		if m.view == entitiesViewAdd {
			m.addFocus = 0
		}
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
	return m, nil
}

// --- Add View ---

func (m EntitiesModel) handleAddKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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

	if m.addFocus == addFieldStatus {
		switch {
		case isKey(msg, "left"):
			m.addStatusIdx = (m.addStatusIdx - 1 + len(entityStatusOptions)) % len(entityStatusOptions)
			return m, nil
		case isKey(msg, "right"), isSpace(msg):
			m.addStatusIdx = (m.addStatusIdx + 1) % len(entityStatusOptions)
			return m, nil
		}
	}
	if m.addFocus == addFieldScopes && m.addScopeSelecting {
		switch {
		case isKey(msg, "left"):
			if len(m.scopeOptions) > 0 {
				m.addScopeIdx = (m.addScopeIdx - 1 + len(m.scopeOptions)) % len(m.scopeOptions)
			}
			return m, nil
		case isKey(msg, "right"):
			if len(m.scopeOptions) > 0 {
				m.addScopeIdx = (m.addScopeIdx + 1) % len(m.scopeOptions)
			}
			return m, nil
		case isSpace(msg):
			if len(m.scopeOptions) > 0 {
				scope := m.scopeOptions[m.addScopeIdx]
				m.addScopes = toggleScope(m.addScopes, scope)
			}
			return m, nil
		case isEnter(msg), isBack(msg):
			m.addScopeSelecting = false
			return m, nil
		}
	}

	switch {
	case isDown(msg):
		m.addScopeSelecting = false
		m.addFocus = (m.addFocus + 1) % addFieldCount
	case isUp(msg):
		if m.addFocus == 0 {
			m.addScopeSelecting = false
			m.modeFocus = true
			return m, nil
		}
		m.addScopeSelecting = false
		m.addFocus = (m.addFocus - 1 + addFieldCount) % addFieldCount
	case isKey(msg, "ctrl+s"):
		return m.saveAdd()
	case isBack(msg):
		m.resetAddForm()
	case isKey(msg, "backspace", "delete"):
		switch m.addFocus {
		case addFieldTags:
			if len(m.addTagBuf) > 0 {
				m.addTagBuf = m.addTagBuf[:len(m.addTagBuf)-1]
			} else if len(m.addTags) > 0 {
				m.addTags = m.addTags[:len(m.addTags)-1]
			}
		case addFieldScopes:
			if len(m.addScopes) > 0 {
				m.addScopes = m.addScopes[:len(m.addScopes)-1]
			}
		default:
			if m.addFocus < len(m.addFields) {
				f := &m.addFields[m.addFocus]
				if len(f.value) > 0 {
					f.value = f.value[:len(f.value)-1]
				}
			}
		}
	default:
		switch m.addFocus {
		case addFieldTags:
			switch {
			case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
				m.commitAddTag()
			default:
				ch := msg.String()
				if len(ch) == 1 && ch != "," {
					m.addTagBuf += ch
				}
			}
		case addFieldScopes:
			if isSpace(msg) {
				m.addScopeSelecting = true
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

// renderAdd renders render add.
func (m EntitiesModel) renderAdd() string {
	var b strings.Builder
	for i, f := range m.addFields {
		label := f.label
		switch i {
		case addFieldStatus:
			status := entityStatusOptions[m.addStatusIdx]
			if m.addFocus == i {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + status))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + status))
			}
		case addFieldTags:
			if m.addFocus == i {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderAddTags(true)))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderAddTags(false)))
			}
		case addFieldScopes:
			if m.addFocus == i && m.addScopeSelecting {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + renderScopeOptions(m.addScopes, m.scopeOptions, m.addScopeIdx)))
			} else if m.addFocus == i {
				b.WriteString(SelectedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderAddScopes(true)))
			} else {
				b.WriteString(MutedStyle.Render("  " + label + ":"))
				b.WriteString("\n")
				b.WriteString(NormalStyle.Render("  " + m.renderAddScopes(false)))
			}
		default:
			if m.addFocus == i {
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
		if i < addFieldCount-1 {
			b.WriteString("\n\n")
		}
	}

	if m.errText != "" {
		b.WriteString("\n\n")
		b.WriteString(components.ErrorBox("Error", m.errText, m.width))
	}

	return components.TitledBox("Add Entity", b.String(), m.width)
}

// saveAdd handles save add.
func (m EntitiesModel) saveAdd() (EntitiesModel, tea.Cmd) {
	name := strings.TrimSpace(m.addFields[addFieldName].value)
	if name == "" {
		m.errText = "Name is required"
		return m, nil
	}
	typ := strings.TrimSpace(m.addFields[addFieldType].value)
	if typ == "" {
		m.errText = "Type is required"
		return m, nil
	}

	m.commitAddTag()

	status := entityStatusOptions[m.addStatusIdx]
	scopes := normalizeScopeList(m.addScopes)
	if len(scopes) == 0 {
		scopes = []string{"private"}
	}

	input := api.CreateEntityInput{
		Scopes: scopes,
		Name:   name,
		Type:   typ,
		Status: status,
		Tags:   append([]string{}, m.addTags...),
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
	m.addFocus = 0
	m.addStatusIdx = statusIndex(entityStatusOptions, "active")
	m.addTags = nil
	m.addTagBuf = ""
	m.addScopes = nil
	m.addScopeBuf = ""
	m.addScopeIdx = 0
	m.addScopeSelecting = false
	for i := range m.addFields {
		m.addFields[i].value = ""
	}
}

// renderAddTags renders render add tags.
func (m *EntitiesModel) renderAddTags(focused bool) string {
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

// renderAddScopes renders render add scopes.
func (m *EntitiesModel) renderAddScopes(focused bool) string {
	return renderScopePills(m.addScopes, focused)
}

// commitAddTag handles commit add tag.
func (m *EntitiesModel) commitAddTag() {
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

// commitAddScope handles commit add scope.
func (m *EntitiesModel) commitAddScope() {
	raw := strings.TrimSpace(m.addScopeBuf)
	if raw == "" {
		m.addScopeBuf = ""
		return
	}
	scope := normalizeScope(raw)
	if scope == "" {
		m.addScopeBuf = ""
		return
	}
	for _, s := range m.addScopes {
		if s == scope {
			m.addScopeBuf = ""
			return
		}
	}
	m.addScopes = append(m.addScopes, scope)
	m.addScopeBuf = ""
}

// renderList renders render list.
func (m EntitiesModel) renderList() string {
	if m.loading {
		return "  " + MutedStyle.Render("Loading entities...")
	}

	if len(m.items) == 0 {
		return components.EmptyStateBox(
			"Entities",
			"No entities found.",
			[]string{"Type to live-search", "Press tab to switch Add/Library", "Press / for command palette"},
			m.width,
		)
	}

	visible := m.list.Visible()
	contentWidth := components.BoxContentWidth(m.width)
	showCheckboxes := m.bulkCount() > 0

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap
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

	typeWidth := 12
	statusWidth := 11
	atWidth := compactTimeColumnWidth
	nameWidth := availableCols - (typeWidth + statusWidth + atWidth)
	if nameWidth < 12 {
		nameWidth = 12
	}
	cols := []components.TableColumn{
		{Header: "Name", Width: nameWidth, Align: lipgloss.Left},
		{Header: "Type", Width: typeWidth, Align: lipgloss.Left},
		{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.Entity
	if idx := m.list.Selected(); idx >= 0 && idx < len(m.items) {
		previewItem = &m.items[idx]
	}
	for i := range visible {
		absIdx := m.list.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.items) {
			continue
		}

		e := m.items[absIdx]
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
			if m.isBulkSelected(absIdx) {
				checkbox = "[X]"
			}
			displayName = checkbox + " " + displayName
		}

		if m.list.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}
		tableRows = append(tableRows, []string{
			components.ClampTextWidthEllipsis(displayName, nameWidth),
			components.ClampTextWidthEllipsis(typ, typeWidth),
			components.ClampTextWidthEllipsis(status, statusWidth),
			formatLocalTimeCompact(at),
		})
	}
	if m.modeFocus {
		activeRowRel = -1
	}

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

	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderEntityPreview(*previewItem, previewBoxContentWidth(previewWidth))
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
func (m EntitiesModel) handleBulkPromptKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
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

func (m EntitiesModel) handleSearchInput(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.view = entitiesViewList
		m.searchBuf = ""
	case isEnter(msg):
		query := strings.TrimSpace(m.searchBuf)
		m.searchBuf = ""
		m.loading = true
		m.view = entitiesViewList
		return m, m.loadEntities(query)
	case isKey(msg, "backspace", "delete"):
		if len(m.searchBuf) > 0 {
			m.searchBuf = m.searchBuf[:len(m.searchBuf)-1]
		}
	default:
		if len(msg.String()) == 1 || msg.String() == " " {
			m.searchBuf += msg.String()
		}
	}
	return m, nil
}

// --- Detail ---

func (m EntitiesModel) handleDetailKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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

func (m EntitiesModel) handleContextPromptKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
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
func (m EntitiesModel) handleHistoryKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.view = entitiesViewDetail
	case isDown(msg):
		m.historyList.Down()
	case isUp(msg):
		m.historyList.Up()
	case isEnter(msg):
		if idx := m.historyList.Selected(); idx < len(m.history) {
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
	visible := m.historyList.Visible()

	previewWidth := preferredPreviewWidth(contentWidth)

	gap := 3
	tableWidth := contentWidth
	sideBySide := contentWidth >= minSideBySideContentWidth
	if sideBySide {
		tableWidth = contentWidth - previewWidth - gap
	}

	sepWidth := 1
	if b := lipgloss.RoundedBorder().Left; b != "" {
		sepWidth = lipgloss.Width(b)
	}

	// 3 columns -> 2 separators.
	availableCols := tableWidth - (2 * sepWidth)
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

	cols := []components.TableColumn{
		{Header: "At", Width: atWidth, Align: lipgloss.Left},
		{Header: "Action", Width: actionWidth, Align: lipgloss.Left},
		{Header: "Fields", Width: fieldsWidth, Align: lipgloss.Left},
	}

	tableRows := make([][]string, 0, len(visible))
	activeRowRel := -1
	var previewItem *api.AuditEntry
	if idx := m.historyList.Selected(); idx >= 0 && idx < len(m.history) {
		previewItem = &m.history[idx]
	}

	for i := range visible {
		absIdx := m.historyList.RelToAbs(i)
		if absIdx < 0 || absIdx >= len(m.history) {
			continue
		}
		entry := m.history[absIdx]

		action := strings.TrimSpace(components.SanitizeOneLine(entry.Action))
		if action == "" {
			action = "update"
		}
		fields := "-"
		if n := len(entry.ChangedFields); n > 0 {
			fields = fmt.Sprintf("%d fields", n)
		}

		if m.historyList.IsSelected(absIdx) {
			activeRowRel = len(tableRows)
		}

		tableRows = append(tableRows, []string{
			formatLocalTimeCompact(entry.ChangedAt),
			components.ClampTextWidthEllipsis(strings.ToUpper(action), actionWidth),
			components.ClampTextWidthEllipsis(fields, fieldsWidth),
		})
	}

	countLine := MutedStyle.Render(fmt.Sprintf("%d entries", len(m.history)))
	table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
	preview := ""
	if previewItem != nil {
		content := m.renderEntityHistoryPreview(*previewItem, previewBoxContentWidth(previewWidth))
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
	m.editFocus = editFieldTags
	m.editTags = append([]string{}, m.detail.Tags...)
	m.editTagBuf = ""
	m.editStatusIdx = statusIndex(entityStatusOptions, m.detail.Status)
	m.editScopes = m.scopeNamesFromIDs(m.detail.PrivacyScopeIDs)
	m.editScopeBuf = ""
	m.editScopeIdx = 0
	m.editScopeSelecting = false
	m.editScopesDirty = false
	m.editSaving = false
}

// handleEditKeys handles handle edit keys.
func (m EntitiesModel) handleEditKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
	if m.editSaving {
		return m, nil
	}
	if m.editFocus == editFieldScopes && m.editScopeSelecting {
		switch {
		case isKey(msg, "left"):
			if len(m.scopeOptions) > 0 {
				m.editScopeIdx = (m.editScopeIdx - 1 + len(m.scopeOptions)) % len(m.scopeOptions)
			}
			return m, nil
		case isKey(msg, "right"):
			if len(m.scopeOptions) > 0 {
				m.editScopeIdx = (m.editScopeIdx + 1) % len(m.scopeOptions)
			}
			return m, nil
		case isSpace(msg):
			if len(m.scopeOptions) > 0 {
				scope := m.scopeOptions[m.editScopeIdx]
				m.editScopes = toggleScope(m.editScopes, scope)
				m.editScopesDirty = true
			}
			return m, nil
		case isEnter(msg), isBack(msg):
			m.editScopeSelecting = false
			return m, nil
		}
	}
	switch {
	case isDown(msg):
		m.editScopeSelecting = false
		m.editFocus = (m.editFocus + 1) % editFieldCount
	case isUp(msg):
		m.editScopeSelecting = false
		if m.editFocus > 0 {
			m.editFocus = (m.editFocus - 1 + editFieldCount) % editFieldCount
		}
	case isKey(msg, "ctrl+s"):
		return m.saveEdit()
	case isBack(msg):
		m.editScopeSelecting = false
		m.view = entitiesViewDetail
	case isKey(msg, "backspace"):
		switch m.editFocus {
		case editFieldTags:
			if len(m.editTagBuf) > 0 {
				m.editTagBuf = m.editTagBuf[:len(m.editTagBuf)-1]
			} else if len(m.editTags) > 0 {
				m.editTags = m.editTags[:len(m.editTags)-1]
			}
		case editFieldScopes:
			if len(m.editScopes) > 0 {
				m.editScopes = m.editScopes[:len(m.editScopes)-1]
				m.editScopesDirty = true
			}
		}
	default:
		switch m.editFocus {
		case editFieldTags:
			switch {
			case isSpace(msg) || isKey(msg, ",") || isEnter(msg):
				m.commitEditTag()
			default:
				ch := msg.String()
				if len(ch) == 1 && ch != "," {
					m.editTagBuf += ch
				}
			}
		case editFieldScopes:
			if isSpace(msg) {
				m.editScopeSelecting = true
			}
		case editFieldStatus:
			switch {
			case isKey(msg, "left"):
				m.editStatusIdx = (m.editStatusIdx - 1 + len(entityStatusOptions)) % len(entityStatusOptions)
			case isKey(msg, "right"), isSpace(msg):
				m.editStatusIdx = (m.editStatusIdx + 1) % len(entityStatusOptions)
			}
		}
	}
	return m, nil
}

// renderEdit renders render edit.
func (m EntitiesModel) renderEdit() string {
	if m.detail == nil {
		return m.renderList()
	}

	var b strings.Builder
	b.WriteString(MutedStyle.Render("Entity: " + components.SanitizeOneLine(m.detail.Name)))
	b.WriteString("\n\n")

	// Tags
	if m.editFocus == editFieldTags {
		b.WriteString(SelectedStyle.Render("  Tags:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + m.renderEditTags(true)))
	} else {
		b.WriteString(MutedStyle.Render("  Tags:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + m.renderEditTags(false)))
	}

	b.WriteString("\n\n")

	// Status
	status := entityStatusOptions[m.editStatusIdx]
	if m.editFocus == editFieldStatus {
		b.WriteString(SelectedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	} else {
		b.WriteString(MutedStyle.Render("  Status:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + status))
	}

	b.WriteString("\n\n")

	// Scopes
	if m.editFocus == editFieldScopes && m.editScopeSelecting {
		b.WriteString(SelectedStyle.Render("  Scopes:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + renderScopeOptions(m.editScopes, m.scopeOptions, m.editScopeIdx)))
	} else if m.editFocus == editFieldScopes {
		b.WriteString(SelectedStyle.Render("  Scopes:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + m.renderEditScopes(true)))
	} else {
		b.WriteString(MutedStyle.Render("  Scopes:"))
		b.WriteString("\n")
		b.WriteString(NormalStyle.Render("  " + m.renderEditScopes(false)))
	}

	b.WriteString("\n\n")

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
	m.commitEditTag()

	status := entityStatusOptions[m.editStatusIdx]
	tags := append([]string{}, m.editTags...)
	input := api.UpdateEntityInput{
		Status: &status,
		Tags:   &tags,
	}

	m.editSaving = true
	return m, func() tea.Msg {
		updated, err := m.client.UpdateEntity(m.detail.ID, input)
		if err != nil {
			return errMsg{err}
		}
		if m.editScopesDirty {
			scopeInput := api.BulkUpdateEntityScopesInput{
				EntityIDs: []string{m.detail.ID},
				Scopes:    normalizeBulkScopes(m.editScopes),
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

// commitEditTag handles commit edit tag.
func (m *EntitiesModel) commitEditTag() {
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

// commitEditScope handles commit edit scope.
func (m *EntitiesModel) commitEditScope() {
	raw := strings.TrimSpace(m.editScopeBuf)
	if raw == "" {
		m.editScopeBuf = ""
		return
	}
	scope := normalizeScope(raw)
	if scope == "" {
		m.editScopeBuf = ""
		return
	}
	for _, s := range m.editScopes {
		if s == scope {
			m.editScopeBuf = ""
			return
		}
	}
	m.editScopes = append(m.editScopes, scope)
	m.editScopeBuf = ""
	m.editScopesDirty = true
}

// renderEditScopes renders render edit scopes.
func (m EntitiesModel) renderEditScopes(focused bool) string {
	return renderScopePills(m.editScopes, focused)
}

// renderEditTags renders render edit tags.
func (m EntitiesModel) renderEditTags(focused bool) string {
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

// --- Confirm ---

func (m EntitiesModel) handleConfirmKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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

func (m EntitiesModel) handleRelationshipsKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
	switch {
	case isBack(msg):
		m.view = entitiesViewDetail
	case isDown(msg):
		m.relList.Down()
	case isUp(msg):
		m.relList.Up()
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

	idx := m.relList.Selected()
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
	m.relateList.SetItems(nil)
	m.relateTarget = nil
	m.relateType = ""
	m.relateLoading = false
}

// handleRelateKeys handles handle relate keys.
func (m EntitiesModel) handleRelateKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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
			if len(msg.String()) == 1 || msg.String() == " " {
				m.relateQuery += msg.String()
			}
		}
	case entitiesViewRelateSelect:
		switch {
		case isBack(msg):
			m.view = entitiesViewRelateSearch
		case isDown(msg):
			m.relateList.Down()
		case isUp(msg):
			m.relateList.Up()
		case isEnter(msg):
			if idx := m.relateList.Selected(); idx < len(m.relateResults) {
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
			if len(msg.String()) == 1 || msg.String() == " " {
				m.relateType += msg.String()
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
		visible := m.relateList.Visible()

		previewWidth := preferredPreviewWidth(contentWidth)

		gap := 3
		tableWidth := contentWidth
		sideBySide := contentWidth >= minSideBySideContentWidth
		if sideBySide {
			tableWidth = contentWidth - previewWidth - gap
		}

		sepWidth := 1
		if br := lipgloss.RoundedBorder().Left; br != "" {
			sepWidth = lipgloss.Width(br)
		}

		// 3 columns -> 2 separators.
		availableCols := tableWidth - (2 * sepWidth)

		typeWidth := 14
		statusWidth := 11
		nameWidth := availableCols - (typeWidth + statusWidth)

		cols := []components.TableColumn{
			{Header: "Name", Width: nameWidth, Align: lipgloss.Left},
			{Header: "Type", Width: typeWidth, Align: lipgloss.Left},
			{Header: "Status", Width: statusWidth, Align: lipgloss.Left},
		}

		tableRows := make([][]string, 0, len(visible))
		activeRowRel := -1
		var previewItem *api.Entity
		if idx := m.relateList.Selected(); idx >= 0 && idx < len(m.relateResults) {
			previewItem = &m.relateResults[idx]
		}

		for i := range visible {
			absIdx := m.relateList.RelToAbs(i)
			if absIdx < 0 || absIdx >= len(m.relateResults) {
				continue
			}
			e := m.relateResults[absIdx]

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

			if m.relateList.IsSelected(absIdx) {
				activeRowRel = len(tableRows)
			}
			tableRows = append(tableRows, []string{
				components.ClampTextWidthEllipsis(name, nameWidth),
				components.ClampTextWidthEllipsis(typ, typeWidth),
				components.ClampTextWidthEllipsis(status, statusWidth),
			})
		}

		countLine := MutedStyle.Render(fmt.Sprintf("%d results", len(m.relateResults)))
		table := components.TableGridWithActiveRow(cols, tableRows, tableWidth, activeRowRel)
		preview := ""
		if previewItem != nil {
			content := m.renderRelateEntityPreview(*previewItem, previewBoxContentWidth(previewWidth))
			preview = renderPreviewBox(content, previewWidth)
		}

		body := table
		if sideBySide && preview != "" {
			body = lipgloss.JoinHorizontal(lipgloss.Top, table, strings.Repeat(" ", gap), preview)
		} else if preview != "" {
			body = table + "\n\n" + preview
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
func (m EntitiesModel) handleRelEditKeys(msg tea.KeyMsg) (EntitiesModel, tea.Cmd) {
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
			ch := msg.String()
			if len(ch) == 1 || ch == " " {
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
			m.list.Items[i] = formatEntityLine(updated)
			break
		}
	}
}

// selectedRelationship handles selected relationship.
func (m EntitiesModel) selectedRelationship() *api.Relationship {
	if len(m.rels) == 0 {
		return nil
	}
	idx := m.relList.Selected()
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

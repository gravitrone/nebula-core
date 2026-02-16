package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// --- Tab Constants ---

const (
	tabInbox     = 0
	tabEntities  = 1
	tabRelations = 2
	tabKnow      = 3
	tabJobs      = 4
	tabLogs      = 5
	tabFiles     = 6
	tabProtocols = 7
	tabHistory   = 8
	tabSearch    = 9
	tabProfile   = 10
	tabCount     = 11
)

var tabNames = []string{"Inbox", "Entities", "Relationships", "Context", "Jobs", "Logs", "Files", "Protocols", "History", "Search", "Settings"}

// --- Messages ---

type errMsg struct{ err error }
type clearToastMsg struct{}
type reloginDoneMsg struct {
	apiKey string
	err    error
}
type startupCheckedMsg struct {
	apiErr      string
	authErr     string
	taxonomyErr string
}
type onboardingLoginDoneMsg struct {
	resp *api.LoginResponse
	err  error
}
type paletteEntitiesLoadedMsg struct {
	query string
	items []api.Entity
}

type paletteAction struct {
	ID    string
	Label string
	Desc  string
}

type startupSummary struct {
	API      string
	Auth     string
	Taxonomy string
	Done     bool
}

type appToast struct {
	level string
	text  string
}

// --- App Model ---

// App is the root TUI model that routes between tabs.
type App struct {
	client            *api.Client
	config            *config.Config
	tab               int
	tabNav            bool
	width             int
	height            int
	err               string
	lastErrCode       string
	lastErrMsg        string
	helpOpen          bool
	quitConfirm       bool
	showRecoveryHints bool
	recoveryCommand   string

	onboarding     bool
	onboardingName string
	onboardingBusy bool

	quickstartOpen bool
	quickstartStep int

	startupChecking bool
	startup         startupSummary
	toast           *appToast

	paletteOpen          bool
	paletteQuery         string
	paletteIndex         int
	paletteActions       []paletteAction
	paletteFiltered      []paletteAction
	paletteEntityQuery   string
	paletteEntityLoading bool
	paletteEntities      []api.Entity

	importExportOpen bool

	inbox     InboxModel
	entities  EntitiesModel
	rels      RelationshipsModel
	know      ContextModel
	jobs      JobsModel
	logs      LogsModel
	files     FilesModel
	protocols ProtocolsModel
	history   HistoryModel
	search    SearchModel
	profile   ProfileModel
	impex     ImportExportModel
}

// NewApp creates the root application model.
func NewApp(client *api.Client, cfg *config.Config) App {
	inbox := NewInboxModel(client)
	inbox.confirmBulk = true
	if cfg != nil {
		inbox.SetPendingLimit(cfg.PendingLimit)
	}
	onboarding := cfg == nil
	quickstartPending := cfg != nil && cfg.QuickstartPending
	startupChecking := client != nil && !onboarding
	return App{
		client:          client,
		config:          cfg,
		tab:             tabInbox,
		tabNav:          true,
		recoveryCommand: "nebula login",
		onboarding:      onboarding,
		quickstartOpen:  quickstartPending,
		startupChecking: startupChecking,
		startup: startupSummary{
			API:      "checking",
			Auth:     "checking",
			Taxonomy: "checking",
		},
		paletteActions: defaultPaletteActions(),
		inbox:          inbox,
		entities:       NewEntitiesModel(client),
		rels:           NewRelationshipsModel(client),
		know:           NewContextModel(client),
		jobs:           NewJobsModel(client),
		logs:           NewLogsModel(client),
		files:          NewFilesModel(client),
		protocols:      NewProtocolsModel(client),
		history:        NewHistoryModel(client),
		search:         NewSearchModel(client),
		profile:        NewProfileModel(client, cfg),
		impex:          NewImportExportModel(client),
	}
}

func (a App) Init() tea.Cmd {
	if a.onboarding {
		return nil
	}
	cmds := []tea.Cmd{a.inbox.Init()}
	if a.startupChecking {
		cmds = append(cmds, a.runStartupCheckCmd())
	}
	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.inbox.width = msg.Width
		a.inbox.height = msg.Height
		a.entities.width = msg.Width
		a.entities.height = msg.Height
		a.rels.width = msg.Width
		a.rels.height = msg.Height
		a.know.width = msg.Width
		a.know.height = msg.Height
		a.jobs.width = msg.Width
		a.jobs.height = msg.Height
		a.logs.width = msg.Width
		a.logs.height = msg.Height
		a.files.width = msg.Width
		a.files.height = msg.Height
		a.protocols.width = msg.Width
		a.protocols.height = msg.Height
		a.history.width = msg.Width
		a.history.height = msg.Height
		a.search.width = msg.Width
		a.search.height = msg.Height
		a.profile.width = msg.Width
		a.profile.height = msg.Height
		a.impex.width = msg.Width
		a.impex.height = msg.Height
		return a, nil

	case errMsg:
		a.err = msg.err.Error()
		a.lastErrCode, a.lastErrMsg = parseErrorCodeAndMessage(a.err)
		a.showRecoveryHints = shouldShowRecoveryHints(a.lastErrCode, a.lastErrMsg)
		return a, nil
	case clearToastMsg:
		a.toast = nil
		return a, nil
	case reloginDoneMsg:
		if msg.err != nil {
			a.err = fmt.Sprintf("re-login failed: %v", msg.err)
			a.lastErrCode, a.lastErrMsg = parseErrorCodeAndMessage(a.err)
			a.showRecoveryHints = shouldShowRecoveryHints(a.lastErrCode, a.lastErrMsg)
			return a, nil
		}
		if a.config != nil {
			a.config.APIKey = msg.apiKey
			if err := a.config.Save(); err != nil {
				a.err = fmt.Sprintf("save config: %v", err)
				a.lastErrCode, a.lastErrMsg = parseErrorCodeAndMessage(a.err)
				a.showRecoveryHints = shouldShowRecoveryHints(a.lastErrCode, a.lastErrMsg)
				return a, nil
			}
		}
		if a.client != nil {
			a.client.SetAPIKey(msg.apiKey)
		}
		a.err = ""
		a.lastErrCode = ""
		a.lastErrMsg = ""
		a.showRecoveryHints = false
		return a, a.setToast("success", "Re-login complete. API key refreshed.")
	case onboardingLoginDoneMsg:
		a.onboardingBusy = false
		if msg.err != nil {
			a.err = fmt.Sprintf("login failed: %v", msg.err)
			a.lastErrCode, a.lastErrMsg = parseErrorCodeAndMessage(a.err)
			a.showRecoveryHints = shouldShowRecoveryHints(a.lastErrCode, a.lastErrMsg)
			return a, nil
		}
		if msg.resp == nil {
			a.err = "login failed: empty response"
			return a, nil
		}
		cfg := &config.Config{
			APIKey:            msg.resp.APIKey,
			UserEntityID:      msg.resp.EntityID,
			Username:          msg.resp.Username,
			Theme:             "dark",
			VimKeys:           true,
			QuickstartPending: true,
			PendingLimit:      500,
		}
		if err := cfg.Save(); err != nil {
			a.err = fmt.Sprintf("save config: %v", err)
			return a, nil
		}
		a.config = cfg
		if a.client == nil {
			a.client = api.NewDefaultClient(cfg.APIKey)
		} else {
			a.client.SetAPIKey(cfg.APIKey)
		}
		a.profile.client = a.client
		a.profile.config = cfg
		a.inbox.SetPendingLimit(cfg.PendingLimit)
		a.onboarding = false
		a.onboardingName = ""
		a.quickstartOpen = cfg.QuickstartPending
		a.err = ""
		a.lastErrCode = ""
		a.lastErrMsg = ""
		a.showRecoveryHints = false
		a.startupChecking = true
		a.startup = startupSummary{
			API:      "checking",
			Auth:     "checking",
			Taxonomy: "checking",
		}
		return a, tea.Batch(a.inbox.Init(), a.runStartupCheckCmd(), a.setToast("success", "Logged in. Welcome to Nebula."))
	case pendingLimitSavedMsg:
		a.inbox.SetPendingLimit(msg.limit)
		return a, nil
	case startupCheckedMsg:
		a.startupChecking = false
		a.startup.Done = true
		a.startup.API = classifyStartupAPI(msg.apiErr)
		if a.startup.API == "ok" {
			a.startup.Auth = classifyStartupAuth(msg.authErr, a.config)
			a.startup.Taxonomy = classifyStartupTaxonomy(msg.taxonomyErr)
		} else {
			a.startup.Auth = "missing"
			a.startup.Taxonomy = "failed"
		}
		level, text := startupToastCopy(a.startup)
		return a, a.setToast(level, text)
	case importExportDoneMsg:
		if a.importExportOpen {
			var cmd tea.Cmd
			a.impex, cmd = a.impex.Update(msg)
			if a.impex.closed {
				a.importExportOpen = false
			}
			return a, cmd
		}
	case importExportErrorMsg:
		if a.importExportOpen {
			var cmd tea.Cmd
			a.impex, cmd = a.impex.Update(msg)
			if a.impex.closed {
				a.importExportOpen = false
			}
			return a, cmd
		}
	case paletteEntitiesLoadedMsg:
		if msg.query != a.paletteEntityQuery {
			return a, nil
		}
		a.paletteEntityLoading = false
		a.paletteEntities = msg.items
		a.paletteFiltered = buildEntityPaletteActions(msg.items, a.paletteEntityQuery)
		a.paletteIndex = 0
		return a, nil
	case searchSelectionMsg:
		return a.applySearchSelection(msg)

	case tea.KeyMsg:
		if a.onboarding {
			return a.handleOnboardingKeys(msg)
		}
		if a.importExportOpen {
			var cmd tea.Cmd
			a.impex, cmd = a.impex.Update(msg)
			if a.impex.closed {
				a.importExportOpen = false
			}
			return a, cmd
		}
		if a.quitConfirm {
			switch {
			case isKey(msg, "y"), isEnter(msg):
				return a, tea.Quit
			case isKey(msg, "n"), isBack(msg):
				a.quitConfirm = false
			}
			return a, nil
		}
		if a.helpOpen {
			if isBack(msg) || isKey(msg, "?") {
				a.helpOpen = false
			}
			return a, nil
		}
		if a.paletteOpen {
			return a.handlePaletteKeys(msg)
		}
		if a.quickstartOpen {
			return a.handleQuickstartKeys(msg)
		}
		if a.showRecoveryHints {
			switch {
			case isKey(msg, "r"):
				return a, a.reloginCmd()
			case isKey(msg, "s"):
				a.profile.section = 0
				return a.switchTab(tabProfile)
			case isKey(msg, "c"):
				return a, a.setToast("info", a.recoveryCommand)
			}
		}
		if a.err != "" {
			a.err = ""
			a.lastErrCode = ""
			a.lastErrMsg = ""
			a.showRecoveryHints = false
		}

		// Global keys
		if isKey(msg, "?") {
			a.helpOpen = true
			return a, nil
		}
		if isQuit(msg) {
			if a.hasUnsaved() {
				a.quitConfirm = true
				return a, nil
			}
			return a, tea.Quit
		}

		// Command palette
		if isKey(msg, "/") {
			a.openPalette()
			return a, nil
		}

		if idx, ok := tabIndexForKey(msg.String()); ok {
			app, cmd := a.switchTab(idx)
			return app, cmd
		}

		// Arrow tab navigation until user enters content with Down
		if a.tabNav {
			if isKey(msg, "left") {
				newTab := (a.tab - 1 + tabCount) % tabCount
				app, cmd := a.switchTab(newTab)
				return app, cmd
			}
			if isKey(msg, "right") {
				newTab := (a.tab + 1) % tabCount
				app, cmd := a.switchTab(newTab)
				return app, cmd
			}
			if isDown(msg) {
				a.tabNav = false
				return a, nil
			}

			// Any other key exits tab nav so the active tab can handle it.
			a.tabNav = false
		} else {
			if isUp(msg) && a.canExitToTabNav() {
				a.tabNav = true
				return a, nil
			}
		}
	}

	// Delegate to active tab
	var cmd tea.Cmd
	switch a.tab {
	case tabInbox:
		a.inbox, cmd = a.inbox.Update(msg)
	case tabEntities:
		a.entities, cmd = a.entities.Update(msg)
	case tabRelations:
		a.rels, cmd = a.rels.Update(msg)
	case tabKnow:
		a.know, cmd = a.know.Update(msg)
	case tabJobs:
		a.jobs, cmd = a.jobs.Update(msg)
	case tabLogs:
		a.logs, cmd = a.logs.Update(msg)
	case tabFiles:
		a.files, cmd = a.files.Update(msg)
	case tabProtocols:
		a.protocols, cmd = a.protocols.Update(msg)
	case tabHistory:
		a.history, cmd = a.history.Update(msg)
	case tabSearch:
		a.search, cmd = a.search.Update(msg)
	case tabProfile:
		a.profile, cmd = a.profile.Update(msg)
	}
	toastCmd := a.toastCmdForMsg(msg)
	if toastCmd != nil && cmd != nil {
		return a, tea.Batch(cmd, toastCmd)
	}
	if toastCmd != nil {
		return a, toastCmd
	}
	return a, cmd
}

func (a App) View() string {
	banner := centerBlockUniform(RenderBanner(), a.width)
	tabs := centerBlockUniform(a.renderTabs(), a.width)
	startupPanel := ""
	if a.startupChecking {
		startupPanel = "\n\n" + centerBlockUniform(a.renderStartupPanel(), a.width)
	}

	var content string
	switch a.tab {
	case tabInbox:
		content = a.inbox.View()
	case tabEntities:
		content = a.entities.View()
	case tabRelations:
		content = a.rels.View()
	case tabKnow:
		content = a.know.View()
	case tabJobs:
		content = a.jobs.View()
	case tabLogs:
		content = a.logs.View()
	case tabFiles:
		content = a.files.View()
	case tabProtocols:
		content = a.protocols.View()
	case tabHistory:
		content = a.history.View()
	case tabSearch:
		content = a.search.View()
	case tabProfile:
		content = a.profile.View()
	}
	content = centerBlockUniform(content, a.width)

	if a.quitConfirm {
		content = a.renderQuitConfirm()
		content = centerBlockUniform(content, a.width)
	} else if a.helpOpen {
		content = a.renderHelp()
		content = centerBlockUniform(content, a.width)
	} else if a.paletteOpen {
		content = a.renderPalette()
		content = centerBlockUniform(content, a.width)
	} else if a.importExportOpen {
		content = a.impex.View()
		content = centerBlockUniform(content, a.width)
	} else if a.onboarding {
		content = a.renderOnboarding()
		content = centerBlockUniform(content, a.width)
	} else if a.quickstartOpen {
		content = a.renderQuickstart()
		content = centerBlockUniform(content, a.width)
	}

	hints := components.StatusBar(a.statusHints(), a.width)

	feedback := ""
	if a.err != "" {
		message := a.err
		if a.showRecoveryHints {
			message += "\n\nRecovery: [r] re-login  [s] settings  [c] show command"
		}
		feedback = centerBlockUniform(components.ErrorBox("Error", message, a.width), a.width)
	} else if a.toast != nil {
		feedback = centerBlockUniform(a.renderToast(), a.width)
	}
	if feedback != "" {
		content = content + "\n\n" + feedback
	}

	return fmt.Sprintf("%s\n%s%s\n\n%s\n\n%s", banner, tabs, startupPanel, content, hints)
}

func (a *App) switchTab(newTab int) (App, tea.Cmd) {
	oldTab := a.tab
	a.tab = newTab
	if oldTab != newTab {
		return *a, a.initTab(newTab)
	}
	return *a, nil
}

// tabWantsArrows returns true when the active tab needs left/right arrow keys.
func (a App) tabWantsArrows() bool {
	switch a.tab {
	case tabKnow:
		return true // type selector uses left/right
	case tabInbox:
		return a.inbox.detail != nil || a.inbox.rejecting
	case tabEntities:
		return a.entities.view != entitiesViewList
	case tabRelations:
		return a.rels.view != relsViewList
	case tabJobs:
		return a.jobs.detail != nil || a.jobs.changingSt
	case tabLogs:
		return a.logs.view != logsViewList
	case tabFiles:
		return a.files.view != filesViewList
	case tabSearch:
		return false
	case tabProfile:
		return a.profile.creating ||
			a.profile.createdKey != "" ||
			a.profile.taxPromptMode != taxPromptNone
	}
	return false
}

func (a App) renderTabs() string {
	segments := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		label := name
		if i == a.tab {
			if a.tabNav {
				segments = append(segments, TabActiveStyle.Render(label))
			} else {
				segments = append(segments, TabSelectedStyle.Render(label))
			}
		} else {
			segments = append(segments, TabInactiveStyle.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, segments...)
}

func (a App) initTab(tab int) tea.Cmd {
	switch tab {
	case tabInbox:
		return a.inbox.Init()
	case tabEntities:
		return a.entities.Init()
	case tabRelations:
		return a.rels.Init()
	case tabKnow:
		return a.know.Init()
	case tabJobs:
		return a.jobs.Init()
	case tabLogs:
		return a.logs.Init()
	case tabFiles:
		return a.files.Init()
	case tabProtocols:
		return a.protocols.Init()
	case tabHistory:
		return a.history.Init()
	case tabSearch:
		return a.search.Init()
	case tabProfile:
		return a.profile.Init()
	}
	return nil
}

func (a App) statusHints() []string {
	if a.quitConfirm {
		return []string{
			components.Hint("y", "Confirm"),
			components.Hint("n", "Cancel"),
		}
	}
	if a.helpOpen {
		return []string{
			components.Hint("esc", "Back"),
		}
	}
	if a.onboarding {
		if a.onboardingBusy {
			return []string{
				components.Hint("enter", "Logging in"),
				components.Hint("q", "Quit"),
			}
		}
		return []string{
			components.Hint("type", "Username"),
			components.Hint("enter", "Login"),
			components.Hint("q", "Quit"),
		}
	}
	if a.quickstartOpen {
		return []string{
			components.Hint("←/→", "Step"),
			components.Hint("enter", "Go"),
			components.Hint("esc", "Skip"),
		}
	}
	hints := a.statusHintsForTab()
	if a.showRecoveryHints {
		hints = append(hints,
			components.Hint("r", "Re-login"),
			components.Hint("s", "Settings"),
			components.Hint("c", "Command"),
		)
	}
	return hints
}

func (a App) statusHintsForTab() []string {
	base := []string{
		components.Hint("1-9/0/-", "Tabs"),
		components.Hint("/", "Command"),
		components.Hint("?", "Help"),
		components.Hint("q", "Quit"),
	}

	switch a.tab {
	case tabInbox:
		if a.inbox.confirming || a.inbox.rejectPreview {
			return append(base,
				components.Hint("y", "Confirm"),
				components.Hint("n", "Cancel"),
			)
		}
		if a.inbox.filtering {
			return append(base,
				components.Hint("enter", "Apply"),
				components.Hint("esc", "Clear"),
			)
		}
		if a.inbox.rejecting {
			return append(base,
				components.Hint("enter", "Submit"),
				components.Hint("esc", "Cancel"),
			)
		}
		if a.inbox.detail != nil {
			return append(base,
				components.Hint("a", "Approve"),
				components.Hint("r", "Reject"),
				components.Hint("esc", "Back"),
			)
		}
		return append(base,
			components.Hint("↑/↓", "Scroll"),
			components.Hint("space", "Select"),
			components.Hint("b", "Select All"),
			components.Hint("A", "Approve All"),
			components.Hint("a", "Approve"),
			components.Hint("r", "Reject"),
			components.Hint("enter", "Details"),
			components.Hint("f", "Filter"),
		)
	case tabEntities:
		if a.entities.bulkPrompt != "" {
			return append(base,
				components.Hint("enter", "Apply"),
				components.Hint("esc", "Cancel"),
			)
		}
		switch a.entities.view {
		case entitiesViewDetail:
			return append(base,
				components.Hint("e", "Edit"),
				components.Hint("h", "History"),
				components.Hint("r", "Relationships"),
				components.Hint("d", "Archive"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewEdit:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Cancel"),
			)
		case entitiesViewRelationships:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("n", "New"),
				components.Hint("e", "Edit"),
				components.Hint("d", "Archive"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewRelateSearch:
			return append(base,
				components.Hint("enter", "Search"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewRelateSelect:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Select"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewRelateType:
			return append(base,
				components.Hint("enter", "Create"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewRelEdit:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Cancel"),
			)
		case entitiesViewAdd:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewHistory:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Revert"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewSearch:
			return append(base,
				components.Hint("enter", "Search"),
				components.Hint("esc", "Back"),
			)
		case entitiesViewConfirm:
			return append(base,
				components.Hint("y", "Confirm"),
				components.Hint("n", "Cancel"),
			)
		default:
			hints := append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("tab", "Complete"),
				components.Hint("enter", "Details"),
			)
			if strings.TrimSpace(a.entities.searchBuf) == "" {
				hints = append(hints, components.Hint("space", "Select"))
			}
			if a.entities.bulkCount() > 0 {
				hints = append(hints,
					components.Hint("t", "Tags"),
					components.Hint("p", "Scopes"),
					components.Hint("c", "Clear"),
				)
			}
			return hints
		}
	case tabRelations:
		switch a.rels.view {
		case relsViewDetail:
			return append(base,
				components.Hint("e", "Edit"),
				components.Hint("d", "Archive"),
				components.Hint("esc", "Back"),
			)
		case relsViewEdit:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Cancel"),
			)
		case relsViewConfirm:
			return append(base,
				components.Hint("y", "Confirm"),
				components.Hint("n", "Cancel"),
			)
		case relsViewCreateSourceSearch, relsViewCreateTargetSearch, relsViewCreateSourceSelect, relsViewCreateTargetSelect:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Select"),
				components.Hint("esc", "Back"),
			)
		case relsViewCreateType:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Create"),
				components.Hint("esc", "Back"),
			)
		default:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Details"),
				components.Hint("n", "New"),
			)
		}
	case tabKnow:
		if a.know.linkSearching {
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Select"),
				components.Hint("esc", "Cancel"),
			)
		}
		switch a.know.view {
		case contextViewList:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Details"),
				components.Hint("esc", "Back"),
			)
		case contextViewDetail:
			return append(base,
				components.Hint("m", "Metadata"),
				components.Hint("c", "Content"),
				components.Hint("v", "Source"),
				components.Hint("esc", "Back"),
			)
		default:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Cancel"),
			)
		}
	case tabJobs:
		if a.jobs.view == jobsViewAdd || a.jobs.view == jobsViewEdit {
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Cancel"),
			)
		}
		if a.jobs.detail != nil {
			return append(base,
				components.Hint("s", "Status"),
				components.Hint("n", "Subtask"),
				components.Hint("esc", "Back"),
			)
		}
		return append(base,
			components.Hint("↑/↓", "Scroll"),
			components.Hint("tab", "Complete"),
			components.Hint("enter", "Details"),
			components.Hint("s", "Status"),
		)
	case tabLogs:
		switch a.logs.view {
		case logsViewDetail:
			return append(base,
				components.Hint("e", "Edit"),
				components.Hint("v", "Value"),
				components.Hint("m", "Metadata"),
				components.Hint("esc", "Back"),
			)
		case logsViewAdd, logsViewEdit:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Back"),
			)
		default:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("tab", "Complete"),
				components.Hint("enter", "Details"),
			)
		}
	case tabFiles:
		switch a.files.view {
		case filesViewDetail:
			return append(base,
				components.Hint("e", "Edit"),
				components.Hint("m", "Metadata"),
				components.Hint("esc", "Back"),
			)
		case filesViewAdd, filesViewEdit:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("space", "Select"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Back"),
			)
		default:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("tab", "Complete"),
				components.Hint("enter", "Details"),
			)
		}
	case tabProtocols:
		switch a.protocols.view {
		case protocolsViewDetail:
			return append(base,
				components.Hint("e", "Edit"),
				components.Hint("esc", "Back"),
			)
		case protocolsViewEdit, protocolsViewAdd:
			return append(base,
				components.Hint("↑/↓", "Fields"),
				components.Hint("←/→", "Cycle"),
				components.Hint("ctrl+s", "Save"),
				components.Hint("esc", "Cancel"),
			)
		default:
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("n", "New"),
				components.Hint("enter", "Details"),
			)
		}
	case tabHistory:
		if a.history.filtering {
			return append(base,
				components.Hint("enter", "Apply"),
				components.Hint("esc", "Clear"),
			)
		}
		if a.history.view == historyViewScopes || a.history.view == historyViewActors {
			return append(base,
				components.Hint("↑/↓", "Scroll"),
				components.Hint("enter", "Select"),
				components.Hint("esc", "Back"),
			)
		}
		if a.history.view == historyViewDetail {
			return append(base,
				components.Hint("esc", "Back"),
			)
		}
		return append(base,
			components.Hint("↑/↓", "Scroll"),
			components.Hint("enter", "Details"),
			components.Hint("f", "Filter"),
			components.Hint("s", "Scopes"),
			components.Hint("a", "Actors"),
		)
	case tabSearch:
		return append(base,
			components.Hint("↑/↓", "Scroll"),
			components.Hint("enter", "Open"),
			components.Hint("tab", "Mode"),
			components.Hint("esc", "Clear"),
		)
	case tabProfile:
		if a.profile.agentDetail != nil {
			return append(base,
				components.Hint("esc", "Back"),
			)
		}
		hints := []string{
			components.Hint("↑/↓", "Scroll"),
			components.Hint("←/→", "Section"),
			components.Hint("k", "API Key"),
			components.Hint("p", "Queue Limit"),
		}
		if a.profile.section == 0 {
			hints = append(hints,
				components.Hint("n", "New Key"),
				components.Hint("r", "Revoke"),
			)
		} else if a.profile.section == 1 {
			hints = append(hints,
				components.Hint("enter", "Details"),
				components.Hint("t", "Toggle Trust"),
			)
		} else {
			if a.profile.taxPromptMode != taxPromptNone {
				hints = append(hints,
					components.Hint("enter", "Apply"),
					components.Hint("esc", "Cancel"),
				)
			} else {
				hints = append(hints,
					components.Hint("[/]", "Kind"),
					components.Hint("n", "New"),
					components.Hint("e", "Edit"),
					components.Hint("d", "Archive"),
					components.Hint("a", "Activate"),
					components.Hint("f", "Filter"),
					components.Hint("i", "Inactive"),
				)
			}
		}
		return append(base, hints...)
	}
	return base
}

func (a App) renderTips() string {
	return ""
}

func (a App) renderHelp() string {
	hints := a.statusHintsForTab()
	lines := make([]string, 0, len(hints)+2)
	lines = append(lines, MutedStyle.Render("esc to close"))
	lines = append(lines, "")
	for _, hint := range hints {
		lines = append(lines, "  "+hint)
	}
	body := strings.Join(lines, "\n")
	return components.Indent(components.TitledBox("Help", body, a.width), 1)
}

func (a App) renderQuitConfirm() string {
	body := "You have unsaved changes. Quit anyway?"
	return components.Indent(components.ConfirmDialog("Quit", body), 1)
}

func (a App) runStartupCheckCmd() tea.Cmd {
	return func() tea.Msg {
		var checkClient *api.Client
		if a.client != nil {
			checkClient = a.client.WithTimeout(700 * time.Millisecond)
		} else {
			apiKey := ""
			if a.config != nil {
				apiKey = a.config.APIKey
			}
			checkClient = api.NewDefaultClient(apiKey, 700*time.Millisecond)
		}

		msg := startupCheckedMsg{}
		if _, err := checkClient.Health(); err != nil {
			msg.apiErr = err.Error()
			return msg
		}
		if _, err := checkClient.ListKeys(); err != nil {
			msg.authErr = err.Error()
		}
		if _, err := checkClient.ListTaxonomy("scopes", false, "", 1, 0); err != nil {
			msg.taxonomyErr = err.Error()
		}
		return msg
	}
}

func (a *App) reloginCmd() tea.Cmd {
	if a.client == nil || a.config == nil {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("re-login unavailable; run nebula login")}
		}
	}
	username := strings.TrimSpace(a.config.Username)
	if username == "" {
		return func() tea.Msg {
			return errMsg{err: fmt.Errorf("username missing; run nebula login")}
		}
	}
	return func() tea.Msg {
		resp, err := a.client.Login(username)
		if err != nil {
			return reloginDoneMsg{err: err}
		}
		return reloginDoneMsg{apiKey: resp.APIKey}
	}
}

func (a *App) setToast(level, text string) tea.Cmd {
	a.toast = &appToast{
		level: level,
		text:  components.SanitizeOneLine(text),
	}
	return tea.Tick(2500*time.Millisecond, func(time.Time) tea.Msg {
		return clearToastMsg{}
	})
}

func (a App) renderToast() string {
	if a.toast == nil {
		return ""
	}
	switch a.toast.level {
	case "success":
		header := lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
		return components.TitledBoxWithHeaderStyle("Success", a.toast.text, a.width, header)
	case "warning":
		header := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
		return components.TitledBoxWithHeaderStyle("Warning", a.toast.text, a.width, header)
	case "error":
		return components.ErrorBox("Error", a.toast.text, a.width)
	default:
		return components.TitledBox("Info", a.toast.text, a.width)
	}
}

func (a App) renderStartupPanel() string {
	rows := []components.TableRow{
		{Label: "API", Value: a.startup.API, ValueColor: startupStatusColor(a.startup.API)},
		{Label: "Auth", Value: a.startup.Auth, ValueColor: startupStatusColor(a.startup.Auth)},
		{Label: "Taxonomy", Value: a.startup.Taxonomy, ValueColor: startupStatusColor(a.startup.Taxonomy)},
	}
	return components.Table("Startup Checks", rows, a.width)
}

func (a *App) toastCmdForMsg(msg tea.Msg) tea.Cmd {
	var level, text string
	switch msg.(type) {
	case approvalDoneMsg:
		level, text = "success", "Approval action completed."
	case entityCreatedMsg:
		level, text = "success", "Entity created."
	case entityUpdatedMsg:
		level, text = "success", "Entity updated."
	case entityRevertedMsg:
		level, text = "success", "Entity reverted."
	case relationshipCreatedMsg:
		level, text = "success", "Relationship created."
	case relationshipUpdatedMsg:
		level, text = "success", "Relationship updated."
	case contextSavedMsg, contextUpdatedMsg:
		level, text = "success", "Context saved."
	case jobCreatedMsg:
		level, text = "success", "Job created."
	case jobStatusUpdatedMsg:
		level, text = "success", "Job status updated."
	case subtaskCreatedMsg:
		level, text = "success", "Subtask created."
	case logCreatedMsg, logUpdatedMsg:
		level, text = "success", "Log saved."
	case fileCreatedMsg, fileUpdatedMsg:
		level, text = "success", "File saved."
	case protocolCreatedMsg, protocolUpdatedMsg:
		level, text = "success", "Protocol saved."
	}
	if text == "" {
		return nil
	}
	return a.setToast(level, text)
}

func (a *App) handleQuickstartKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isBack(msg):
		return a.finishQuickstart(true)
	case isKey(msg, "left"):
		if a.quickstartStep > 0 {
			a.quickstartStep--
		}
	case isKey(msg, "right"), isKey(msg, "tab"):
		if a.quickstartStep < 2 {
			a.quickstartStep++
		}
	case isEnter(msg):
		switch a.quickstartStep {
		case 0:
			a.tab = tabEntities
			a.tabNav = false
			a.entities.view = entitiesViewAdd
			a.quickstartStep = 1
			return *a, nil
		case 1:
			a.tab = tabKnow
			a.tabNav = false
			a.know.view = contextViewAdd
			a.quickstartStep = 2
			return *a, nil
		default:
			a.tab = tabRelations
			a.tabNav = false
			a.rels.view = relsViewCreateSourceSearch
			return a.finishQuickstart(false)
		}
	}
	return *a, nil
}

func (a *App) handleOnboardingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if isQuit(msg) {
		return *a, tea.Quit
	}
	if a.onboardingBusy {
		return *a, nil
	}
	if isEnter(msg) {
		username := strings.TrimSpace(a.onboardingName)
		if username == "" {
			a.err = "username is required"
			return *a, nil
		}
		a.err = ""
		a.onboardingBusy = true
		return *a, a.onboardingLoginCmd(username)
	}
	if isKey(msg, "backspace", "ctrl+h", "delete", "backspace2") {
		runes := []rune(a.onboardingName)
		if len(runes) > 0 {
			a.onboardingName = string(runes[:len(runes)-1])
		}
		return *a, nil
	}
	if msg.Type == tea.KeyRunes {
		a.onboardingName += msg.String()
	}
	return *a, nil
}

func (a *App) onboardingLoginCmd(username string) tea.Cmd {
	name := strings.TrimSpace(username)
	return func() tea.Msg {
		client := a.client
		if client == nil {
			client = api.NewDefaultClient("")
		}
		resp, err := client.Login(name)
		return onboardingLoginDoneMsg{resp: resp, err: err}
	}
}

func (a App) renderOnboarding() string {
	prompt := components.SanitizeOneLine(strings.TrimSpace(a.onboardingName))
	if prompt == "" {
		prompt = ""
	}
	line := "> " + prompt
	if !a.onboardingBusy {
		line += "█"
	}
	status := MutedStyle.Render("Enter your Nebula username to create or resume your local session.")
	hint := MutedStyle.Render("Press Enter to login.")
	if a.onboardingBusy {
		hint = MutedStyle.Render("Logging in...")
	}
	body := strings.Join([]string{
		status,
		"",
		MetaKeyStyle.Render("Username"),
		SelectedStyle.Render(line),
		"",
		hint,
	}, "\n")
	return components.TitledBox("Onboarding", body, a.width)
}

func (a *App) finishQuickstart(skipped bool) (tea.Model, tea.Cmd) {
	a.quickstartOpen = false
	a.quickstartStep = 0
	if a.config != nil {
		a.config.QuickstartPending = false
		if err := a.config.Save(); err != nil {
			a.err = fmt.Sprintf("save config: %v", err)
			a.lastErrCode, a.lastErrMsg = parseErrorCodeAndMessage(a.err)
			a.showRecoveryHints = shouldShowRecoveryHints(a.lastErrCode, a.lastErrMsg)
			return *a, nil
		}
	}
	if skipped {
		return *a, a.setToast("info", "Quickstart skipped.")
	}
	return *a, a.setToast("success", "Quickstart complete.")
}

func (a App) renderQuickstart() string {
	type quickstartStep struct {
		title  string
		desc   string
		target string
	}
	steps := []quickstartStep{
		{title: "Step 1/3", desc: "Create your first entity.", target: "Entities -> Add"},
		{title: "Step 2/3", desc: "Add context to your context graph.", target: "Context -> Add"},
		{title: "Step 3/3", desc: "Link context with a relationship.", target: "Relationships -> New"},
	}

	if a.quickstartStep < 0 {
		a.quickstartStep = 0
	}
	if a.quickstartStep >= len(steps) {
		a.quickstartStep = len(steps) - 1
	}
	step := steps[a.quickstartStep]

	labelStyle := MetaKeyStyle.Copy().Bold(true)
	valueStyle := NormalStyle

	rows := []string{
		labelStyle.Render("Step") + "   " + valueStyle.Render(step.title),
		labelStyle.Render("Action") + " " + valueStyle.Render(step.desc),
		labelStyle.Render("Route") + "  " + valueStyle.Render(step.target),
	}
	body := strings.Join(rows, "\n\n") + "\n\n" + MutedStyle.Render("Use <-/-> to change step, Enter to continue, Esc to skip.")
	return components.Indent(components.TitledBox("Getting Started", body, a.width), 1)
}

func parseErrorCodeAndMessage(errText string) (string, string) {
	text := strings.TrimSpace(errText)
	if text == "" {
		return "", ""
	}
	parts := strings.SplitN(text, ":", 2)
	if len(parts) != 2 {
		return "", text
	}
	code := strings.TrimSpace(parts[0])
	if code == "" || strings.HasPrefix(strings.ToUpper(code), "HTTP ") {
		return "", text
	}
	for _, r := range code {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return "", text
		}
	}
	return code, strings.TrimSpace(parts[1])
}

func shouldShowRecoveryHints(code, msg string) bool {
	if !strings.EqualFold(strings.TrimSpace(code), "FORBIDDEN") {
		return false
	}
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "scope") || strings.Contains(lower, "admin")
}

func classifyStartupAPI(errText string) string {
	if strings.TrimSpace(errText) == "" {
		return "ok"
	}
	lower := strings.ToLower(errText)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded") {
		return "timeout"
	}
	return "down"
}

func classifyStartupAuth(errText string, cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.APIKey) == "" {
		return "missing"
	}
	if strings.TrimSpace(errText) == "" {
		return "ok"
	}
	return "invalid"
}

func classifyStartupTaxonomy(errText string) string {
	if strings.TrimSpace(errText) == "" {
		return "ok"
	}
	lower := strings.ToLower(errText)
	switch {
	case strings.Contains(lower, "forbidden"), strings.Contains(lower, "scope"):
		return "forbidden"
	case strings.Contains(lower, "column "), strings.Contains(lower, "relation "), strings.Contains(lower, "schema"):
		return "schema_error"
	default:
		return "failed"
	}
}

func startupToastCopy(summary startupSummary) (string, string) {
	if summary.API == "ok" && summary.Auth == "ok" && summary.Taxonomy == "ok" {
		return "success", "Startup checks passed: API, auth, and taxonomy are healthy."
	}
	if summary.API != "ok" {
		return "error", fmt.Sprintf("Startup checks failed: API is %s.", summary.API)
	}
	return "warning", fmt.Sprintf("Startup checks: auth=%s, taxonomy=%s.", summary.Auth, summary.Taxonomy)
}

func startupStatusColor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok":
		return string(ColorSuccess)
	case "checking":
		return string(ColorMuted)
	case "missing", "forbidden", "timeout":
		return string(ColorWarning)
	case "invalid", "down", "failed", "schema_error":
		return string(ColorError)
	default:
		return string(ColorMuted)
	}
}

func (a *App) openPalette() {
	a.paletteOpen = true
	a.paletteQuery = ""
	a.paletteIndex = 0
	a.paletteEntityQuery = ""
	a.paletteEntityLoading = false
	a.paletteEntities = nil
	a.paletteFiltered = filterPalette(a.paletteActions, "")
}

func (a App) renderPalette() string {
	title := "Command Palette"
	query := components.SanitizeOneLine(a.paletteQuery)
	if query == "" {
		query = ""
	}

	var b strings.Builder
	queryWidth := components.BoxContentWidth(a.width) - 10
	if queryWidth < 10 {
		queryWidth = 10
	}
	query = components.ClampTextWidthEllipsis(query, queryWidth)
	b.WriteString(MetaKeyStyle.Render("Command") + MetaPunctStyle.Render(": ") + SelectedStyle.Render(query))
	b.WriteString(AccentStyle.Render("█"))
	b.WriteString("\n\n")

	items := a.paletteFiltered
	if strings.HasPrefix(a.paletteQuery, ":") && a.paletteEntityLoading {
		b.WriteString(MutedStyle.Render("Searching entities..."))
	} else if len(items) == 0 {
		b.WriteString(MutedStyle.Render("No matches."))
	} else {
		contentWidth := components.BoxContentWidth(a.width)
		sepWidth := 1
		if br := lipgloss.RoundedBorder().Left; br != "" {
			sepWidth = lipgloss.Width(br)
		}

		// 2 columns -> 1 separator.
		availableCols := contentWidth - sepWidth
		if availableCols < 20 {
			availableCols = 20
		}

		descWidth := 34
		actionWidth := availableCols - descWidth
		if actionWidth < 14 {
			actionWidth = 14
			descWidth = availableCols - actionWidth
			if descWidth < 12 {
				descWidth = 12
			}
		}

		cols := []components.TableColumn{
			{Header: "Action", Width: actionWidth, Align: lipgloss.Left},
			{Header: "Description", Width: descWidth, Align: lipgloss.Left},
		}

		rows := make([][]string, 0, len(items))
		for _, item := range items {
			label := strings.TrimSpace(components.SanitizeOneLine(item.Label))
			if label == "" {
				label = "-"
			}
			desc := strings.TrimSpace(components.SanitizeOneLine(item.Desc))
			if desc == "" {
				desc = "-"
			}
			rows = append(rows, []string{
				components.ClampTextWidthEllipsis(label, actionWidth),
				components.ClampTextWidthEllipsis(desc, descWidth),
			})
		}

		b.WriteString(components.TableGridWithActiveRow(cols, rows, contentWidth, a.paletteIndex))
	}

	return components.TitledBox(title, b.String(), a.width)
}

func (a *App) refreshPaletteFiltered() tea.Cmd {
	if strings.HasPrefix(a.paletteQuery, ":") {
		query := strings.TrimSpace(strings.TrimPrefix(a.paletteQuery, ":"))
		if query == "" {
			a.paletteEntityQuery = ""
			a.paletteEntityLoading = false
			a.paletteEntities = nil
			a.paletteFiltered = nil
			a.paletteIndex = 0
			return nil
		}
		if query != a.paletteEntityQuery {
			a.paletteEntityQuery = query
			a.paletteEntityLoading = true
			a.paletteEntities = nil
			a.paletteFiltered = nil
			a.paletteIndex = 0
			return a.loadPaletteEntities(query)
		}
		a.paletteFiltered = buildEntityPaletteActions(a.paletteEntities, a.paletteEntityQuery)
		if a.paletteIndex >= len(a.paletteFiltered) {
			a.paletteIndex = 0
		}
		return nil
	}

	a.paletteEntityQuery = ""
	a.paletteEntityLoading = false
	a.paletteEntities = nil
	a.paletteFiltered = filterPalette(a.paletteActions, a.paletteQuery)
	if a.paletteIndex >= len(a.paletteFiltered) {
		a.paletteIndex = 0
	}
	return nil
}

func (a App) loadPaletteEntities(query string) tea.Cmd {
	return func() tea.Msg {
		items, err := a.client.QueryEntities(api.QueryParams{
			"search_text": query,
			"limit":       "15",
		})
		if err != nil {
			return errMsg{err}
		}
		return paletteEntitiesLoadedMsg{query: query, items: items}
	}
}

func buildEntityPaletteActions(items []api.Entity, query string) []paletteAction {
	query = strings.TrimSpace(strings.ToLower(query))
	actions := make([]paletteAction, 0, len(items))
	for _, e := range items {
		if query != "" {
			name := strings.ToLower(e.Name)
			id := strings.ToLower(e.ID)
			if !strings.Contains(name, query) && !strings.Contains(id, query) {
				continue
			}
		}
		kind := e.Type
		if kind == "" {
			kind = "entity"
		}
		desc := fmt.Sprintf("%s · %s", kind, shortID(e.ID))
		actions = append(actions, paletteAction{
			ID:    "entity:" + e.ID,
			Label: e.Name,
			Desc:  desc,
		})
	}
	return actions
}

func (a App) handlePaletteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case isBack(msg):
		a.paletteOpen = false
		return a, nil
	case isEnter(msg):
		if len(a.paletteFiltered) == 0 {
			return a, nil
		}
		action := a.paletteFiltered[a.paletteIndex]
		a.paletteOpen = false
		a.paletteQuery = ""
		return a.runPaletteAction(action)
	case isUp(msg):
		if a.paletteIndex > 0 {
			a.paletteIndex--
		}
	case isDown(msg):
		if a.paletteIndex < len(a.paletteFiltered)-1 {
			a.paletteIndex++
		}
	case isKey(msg, "backspace"):
		if len(a.paletteQuery) > 0 {
			a.paletteQuery = a.paletteQuery[:len(a.paletteQuery)-1]
			return a, a.refreshPaletteFiltered()
		}
	default:
		ch := msg.String()
		if len(ch) == 1 || ch == " " {
			a.paletteQuery += ch
			return a, a.refreshPaletteFiltered()
		}
	}
	return a, nil
}

func (a *App) runPaletteAction(action paletteAction) (tea.Model, tea.Cmd) {
	a.tabNav = true
	switch action.ID {
	default:
		if strings.HasPrefix(action.ID, "entity:") {
			id := strings.TrimPrefix(action.ID, "entity:")
			for _, e := range a.paletteEntities {
				if e.ID == id {
					entity := e
					a.tab = tabEntities
					a.tabNav = false
					a.entities.detail = &entity
					a.entities.view = entitiesViewDetail
					return *a, nil
				}
			}
			return *a, nil
		}
	case "tab:inbox":
		return a.switchTab(tabInbox)
	case "tab:entities":
		return a.switchTab(tabEntities)
	case "tab:relationships":
		return a.switchTab(tabRelations)
	case "tab:context":
		return a.switchTab(tabKnow)
	case "tab:jobs":
		return a.switchTab(tabJobs)
	case "tab:history":
		return a.switchTab(tabHistory)
	case "tab:search":
		return a.switchTab(tabSearch)
	case "search:semantic":
		a.tab = tabSearch
		a.search.mode = searchModeSemantic
		return *a, nil
	case "tab:settings", "tab:profile":
		return a.switchTab(tabProfile)
	case "entities:search":
		a.tab = tabEntities
		a.tabNav = false
		a.entities.view = entitiesViewSearch
		a.entities.searchBuf = ""
		return *a, nil
	case "profile:keys":
		a.tab = tabProfile
		a.profile.section = 0
		return *a, nil
	case "profile:agents":
		a.tab = tabProfile
		a.profile.section = 1
		return *a, nil
	case "profile:taxonomy":
		a.tab = tabProfile
		a.profile.section = 2
		return *a, nil
	case "ops:import":
		a.tabNav = false
		a.importExportOpen = true
		a.impex.Start(importMode)
		return *a, nil
	case "ops:export":
		a.tabNav = false
		a.importExportOpen = true
		a.impex.Start(exportMode)
		return *a, nil
	case "quit":
		if a.hasUnsaved() {
			a.quitConfirm = true
			return *a, nil
		}
		return *a, tea.Quit
	}
	return *a, nil
}

func (a *App) applySearchSelection(msg searchSelectionMsg) (tea.Model, tea.Cmd) {
	a.tabNav = false
	switch msg.kind {
	case "entity":
		if msg.entity != nil {
			entity := *msg.entity
			a.tab = tabEntities
			a.entities.detail = &entity
			a.entities.view = entitiesViewDetail
		}
	case "context":
		if msg.context != nil {
			context := *msg.context
			a.tab = tabKnow
			a.know.detail = &context
			a.know.view = contextViewDetail
		}
	case "job":
		if msg.job != nil {
			job := *msg.job
			a.tab = tabJobs
			a.jobs.detail = &job
		}
	}
	return *a, nil
}

func (a App) hasUnsaved() bool {
	if a.inbox.rejecting {
		return true
	}
	switch a.entities.view {
	case entitiesViewEdit, entitiesViewRelEdit, entitiesViewRelateSearch, entitiesViewRelateSelect, entitiesViewRelateType:
		return true
	}
	switch a.rels.view {
	case relsViewEdit, relsViewCreateSourceSearch, relsViewCreateSourceSelect, relsViewCreateTargetSearch, relsViewCreateTargetSelect, relsViewCreateType:
		return true
	}
	if a.know.view == contextViewAdd && !a.know.saved && !a.know.saving {
		if contextHasInput(a.know) {
			return true
		}
	}
	if a.jobs.changingSt || a.jobs.creatingSubtask {
		return true
	}
	if a.profile.creating {
		return true
	}
	if a.profile.taxPromptMode != taxPromptNone {
		return true
	}
	return false
}

func contextHasInput(m ContextModel) bool {
	for _, f := range m.fields {
		if strings.TrimSpace(f.value) != "" {
			return true
		}
	}
	if len(m.tags) > 0 || strings.TrimSpace(m.tagBuf) != "" {
		return true
	}
	if len(m.linkEntities) > 0 || strings.TrimSpace(m.linkQuery) != "" {
		return true
	}
	return false
}

func defaultPaletteActions() []paletteAction {
	return []paletteAction{
		{ID: "tab:inbox", Label: "Inbox", Desc: "Go to inbox"},
		{ID: "tab:entities", Label: "Entities", Desc: "Browse entities"},
		{ID: "tab:relationships", Label: "Relationships", Desc: "Browse relationships"},
		{ID: "tab:context", Label: "Context", Desc: "Add context"},
		{ID: "tab:jobs", Label: "Jobs", Desc: "View jobs"},
		{ID: "tab:history", Label: "History", Desc: "Audit log"},
		{ID: "tab:search", Label: "Search", Desc: "Global search"},
		{ID: "search:semantic", Label: "Semantic search", Desc: "Global semantic search"},
		{ID: "tab:settings", Label: "Settings", Desc: "Config, keys, and agents"},
		{ID: "ops:import", Label: "Import", Desc: "Bulk import from file"},
		{ID: "ops:export", Label: "Export", Desc: "Export data to file"},
		{ID: "entities:search", Label: "Search entities", Desc: "Open entity search"},
		{ID: "profile:keys", Label: "Settings: API keys", Desc: "Manage keys"},
		{ID: "profile:agents", Label: "Settings: agents", Desc: "Manage agents"},
		{ID: "profile:taxonomy", Label: "Settings: taxonomy", Desc: "Manage scopes and types"},
		{ID: "quit", Label: "Quit", Desc: "Exit CLI"},
	}
}

func filterPalette(items []paletteAction, query string) []paletteAction {
	if query == "" {
		return items
	}
	q := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]paletteAction, 0, len(items))
	for _, item := range items {
		label := strings.ToLower(item.Label)
		desc := strings.ToLower(item.Desc)
		if strings.Contains(label, q) || strings.Contains(desc, q) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func centerBlock(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth >= width {
			continue
		}
		pad := (width - lineWidth) / 2
		lines[i] = strings.Repeat(" ", pad) + line
	}
	return strings.Join(lines, "\n")
}

func centerBlockUniform(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	maxWidth := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth <= 0 || maxWidth >= width {
		return s
	}
	pad := (width - maxWidth) / 2
	if pad <= 0 {
		return s
	}
	prefix := strings.Repeat(" ", pad)
	for i := range lines {
		if lines[i] != "" {
			lines[i] = prefix + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func (a App) canExitToTabNav() bool {
	switch a.tab {
	case tabInbox:
		if a.inbox.detail != nil || a.inbox.rejecting || a.inbox.confirming || a.inbox.rejectPreview {
			return false
		}
		return a.inbox.list == nil || a.inbox.list.Selected() == 0
	case tabEntities:
		if a.entities.view != entitiesViewList {
			return false
		}
		return a.entities.list == nil || a.entities.list.Selected() == 0
	case tabRelations:
		if a.rels.view != relsViewList {
			return false
		}
		return a.rels.list == nil || a.rels.list.Selected() == 0
	case tabKnow:
		if a.know.view == contextViewList {
			return a.know.list == nil || a.know.list.Selected() == 0
		}
		if a.know.view != contextViewAdd {
			return false
		}
		return !a.know.modeFocus && a.know.focus == fieldTitle
	case tabJobs:
		if a.jobs.detail != nil || a.jobs.changingSt {
			return false
		}
		return a.jobs.list == nil || a.jobs.list.Selected() == 0
	case tabHistory:
		if a.history.filtering || a.history.view != historyViewList {
			return false
		}
		return a.history.list == nil || a.history.list.Selected() == 0
	case tabSearch:
		if strings.TrimSpace(a.search.query) == "" {
			return true
		}
		return a.search.list == nil || a.search.list.Selected() == 0
	case tabProfile:
		if a.profile.creating || a.profile.createdKey != "" || a.profile.agentDetail != nil {
			return false
		}
		if a.profile.section == 0 {
			return a.profile.keyList == nil || a.profile.keyList.Selected() == 0
		}
		return a.profile.agentList == nil || a.profile.agentList.Selected() == 0
	}
	return false
}

func tabIndexForKey(key string) (int, bool) {
	switch key {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(key[0] - '1')
		if idx >= 0 && idx < tabCount {
			return idx, true
		}
	case "0":
		if tabCount > 9 {
			return 9, true
		}
	case "-":
		if tabCount > 10 {
			return 10, true
		}
	}
	return 0, false
}

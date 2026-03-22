package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"

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
	tabProfile   = 9
	tabCount     = 10
)

var tabNames = []string{"Inbox", "Entities", "Relationships", "Context", "Jobs", "Logs", "Files", "Protocols", "History", "Settings"}

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
type paletteSearchLoadedMsg struct {
	query    string
	entities []api.Entity
	context  []api.Context
	jobs     []api.Job
	rels     []api.Relationship
	logs     []api.Log
	files    []api.File
	protos   []api.Protocol
}

type paletteAction struct {
	ID    string
	Label string
	Desc  string
}

type paletteSelection struct {
	entity  *api.Entity
	context *api.Context
	job     *api.Job
	rel     *api.Relationship
	log     *api.Log
	file    *api.File
	proto   *api.Protocol
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

// animTickMsg is sent each frame while animations are active.
type animTickMsg struct{}

// --- Animation Constants ---

const (
	// animFPS is the target frame rate for spring animations.
	animFPS = 60
	// animToastOffset is the initial vertical offset for toast slide-in (in lines).
	animToastOffset = 3.0
	// animSettledVel is the velocity threshold below which a spring is considered settled.
	animSettledVel = 0.01
	// animSettledPos is the position threshold below which a spring is considered settled.
	animSettledPos = 0.5
)

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
	helpModel         help.Model
	keys              KeyMap
	quitConfirm       bool
	showRecoveryHints bool
	recoveryCommand   string

	onboarding      bool
	onboardingInput textinput.Model
	onboardingBusy bool

	quickstartOpen bool
	quickstartStep int

	startupChecking bool
	startup         startupSummary
	toast           *appToast

	paletteOpen          bool
	paletteInput         textinput.Model
	paletteIndex         int
	paletteActions       []paletteAction
	paletteFiltered      []paletteAction
	paletteSearchQuery   string
	paletteSearchLoading bool
	paletteSelections    map[string]paletteSelection

	importExportOpen bool
	bodyScroll       int
	bodyViewKey      string

	// --- Animation State ---

	// toastSpring drives the toast slide-in vertical offset.
	toastSpring    harmonica.Spring
	toastOffset    float64
	toastOffsetVel float64
	toastTarget    float64
	// scrollSpring drives smooth body scroll; scrollPos is the animated position.
	scrollSpring harmonica.Spring
	scrollTarget float64
	scrollPos    float64
	scrollVel    float64
	// tabSpring drives the animated tab indicator position.
	tabSpring    harmonica.Spring
	tabIndicator float64
	tabIndVel    float64
	tabIndTarget float64

	inbox     InboxModel
	entities  EntitiesModel
	rels      RelationshipsModel
	know      ContextModel
	jobs      JobsModel
	logs      LogsModel
	files     FilesModel
	protocols ProtocolsModel
	history   HistoryModel
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
	app := App{
		client:          client,
		config:          cfg,
		tab:             tabInbox,
		tabNav:          true,
		recoveryCommand: "nebula login",
		onboarding:      onboarding,
		onboardingInput: components.NewNebulaTextInput("Enter username..."),
		paletteInput:    components.NewNebulaTextInput(""),
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
		profile:        NewProfileModel(client, cfg),
		impex:          NewImportExportModel(client),
	}
	app.keys = DefaultKeyMap()
	app.helpModel = newHelpModel()
	app.bodyViewKey = app.viewStateKey()
	// Initialize springs: critically damped for smooth, no-bounce motion.
	app.toastSpring = harmonica.NewSpring(harmonica.FPS(animFPS), 6.0, 1.0)
	app.scrollSpring = harmonica.NewSpring(harmonica.FPS(animFPS), 8.0, 1.0)
	app.tabSpring = harmonica.NewSpring(harmonica.FPS(animFPS), 10.0, 1.0)
	// Toast starts off-screen (offset at max = not visible).
	app.toastOffset = animToastOffset
	app.toastTarget = animToastOffset
	// Tab indicator starts at the initial tab.
	app.tabIndicator = float64(tabInbox)
	app.tabIndTarget = float64(tabInbox)
	return app
}

// Init handles init.
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

// Update updates update.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prevViewKey := a.viewStateKey()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.helpModel.SetWidth(msg.Width)
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
		// Clear toast immediately and reset animation state.
		a.toast = nil
		a.toastOffset = animToastOffset
		a.toastOffsetVel = 0
		a.toastTarget = animToastOffset
		return a, nil

	case animTickMsg:
		return a, a.advanceAnimations()
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
		a.onboardingInput.Reset()
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
		switch a.startup.Auth {
		case "invalid":
			a.err = "INVALID_API_KEY: Invalid API key"
			a.lastErrCode = "INVALID_API_KEY"
			a.lastErrMsg = "Invalid API key"
			a.showRecoveryHints = true
		case "multi_api_conflict":
			a.err = "MULTIPLE_API_INSTANCES_DETECTED: multiple api instances detected"
			a.lastErrCode = "MULTIPLE_API_INSTANCES_DETECTED"
			a.lastErrMsg = "multiple api instances detected"
			a.showRecoveryHints = false
		default:
			a.err = ""
			a.lastErrCode = ""
			a.lastErrMsg = ""
			a.showRecoveryHints = false
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
	case paletteSearchLoadedMsg:
		if msg.query != a.paletteSearchQuery {
			return a, nil
		}
		a.paletteSearchLoading = false
		a.paletteFiltered, a.paletteSelections = buildSearchPaletteActions(
			msg.query,
			msg.entities,
			msg.context,
			msg.jobs,
			msg.rels,
			msg.logs,
			msg.files,
			msg.protos,
		)
		a.paletteIndex = 0
		return a, nil
	case searchSelectionMsg:
		return a.applySearchSelection(msg)

	case tea.KeyPressMsg:
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
			a.bodyScroll = 0
			a.openPaletteCommand()
			return a, a.refreshPaletteFiltered()
		}

		// Global body scrolling for long detail panes.
		if isKey(msg, "pgdown", "ctrl+d") {
			a.bodyScroll += 8
			a.scrollTarget = float64(a.bodyScroll)
			return a, a.animTickCmd()
		}
		if isKey(msg, "pgup", "ctrl+u") {
			a.bodyScroll -= 8
			if a.bodyScroll < 0 {
				a.bodyScroll = 0
			}
			a.scrollTarget = float64(a.bodyScroll)
			return a, a.animTickCmd()
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
				if a.focusModeLineForActiveTab() {
					a.resetBodyScrollOnViewChange(prevViewKey)
					return a, nil
				}
			}

			// Any other key exits tab nav so the active tab can handle it.
			a.tabNav = false
		} else {
			if isUp(msg) && a.canExitToTabNav() {
				// Let settings consume Up first so it can move focus from list to
				// section tabs (API Keys/Agents/Taxonomy) before exiting to top nav.
				if a.tab == tabProfile && !a.profile.sectionFocus {
					// fall through to active tab update below
				} else {
					a.tabNav = true
					a.resetBodyScrollOnViewChange(prevViewKey)
					return a, nil
				}
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
	case tabProfile:
		a.profile, cmd = a.profile.Update(msg)
	}
	toastCmd := a.toastCmdForMsg(msg)
	a.resetBodyScrollOnViewChange(prevViewKey)
	if toastCmd != nil && cmd != nil {
		return a, tea.Batch(cmd, toastCmd)
	}
	if toastCmd != nil {
		return a, toastCmd
	}
	return a, cmd
}

// View handles view.
func (a App) View() tea.View {
	components.SetTableGridActiveRowsEnabled(a.rowHighlightEnabled())
	defer components.SetTableGridActiveRowsEnabled(true)

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

	hints := a.renderHelpBar()

	feedback := ""
	if a.err != "" {
		message := a.err
		if a.showRecoveryHints {
			message += "\n\nRecovery: [r] re-login  [s] settings  [c] show command"
		}
		if shouldShowMultiAPIRecoveryHint(a.lastErrCode, a.lastErrMsg, a.err) {
			message += "\n\nRecovery: stop duplicate API processes and restart with `nebula start`."
		}
		feedback = centerBlockUniform(components.ErrorBox("Error", message, a.width), a.width)
	} else if a.toast != nil {
		// Render toast with animated vertical offset for a slide-in effect.
		// Clamp to [0, animToastOffset-1] so toast always shows when set.
		toastPad := int(math.Round(a.toastOffset))
		if toastPad < 0 {
			toastPad = 0
		}
		maxPad := int(animToastOffset) - 1
		if toastPad > maxPad {
			toastPad = maxPad
		}
		toastRendered := centerBlockUniform(a.renderToast(), a.width)
		if toastPad > 0 {
			feedback = strings.Repeat("\n", toastPad) + toastRendered
		} else {
			feedback = toastRendered
		}
	}
	top := fmt.Sprintf("%s\n%s%s", banner, tabs, startupPanel)
	body := content
	if a.height > 0 && !a.helpOpen && !a.quitConfirm && !a.paletteOpen && !a.importExportOpen {
		reservedFeedbackLines := 0
		if feedback != "" {
			// Keep feedback boxes intact by reserving viewport budget up front.
			reservedFeedbackLines = countViewLines(feedback) + 2
		}
		// Use animated scroll position for smooth scrolling; fall back to bodyScroll.
		animScroll := int(math.Round(a.scrollPos))
		if animScroll < 0 {
			animScroll = 0
		}
		body, _ = clampBodyForViewport(
			body,
			a.height,
			countViewLines(top),
			countViewLines(hints)+reservedFeedbackLines,
			animScroll,
		)
	}
	if feedback != "" {
		body = body + "\n\n" + feedback
	}

	v := tea.NewView(fmt.Sprintf("%s\n\n%s\n\n%s", top, body, hints))
	v.AltScreen = true
	return v
}

// rowHighlightEnabled handles row highlight enabled.
func (a App) rowHighlightEnabled() bool {
	if a.tabNav {
		return false
	}
	switch a.tab {
	case tabInbox:
		return !a.inbox.filtering && !a.inbox.rejecting && !a.inbox.confirming && !a.inbox.rejectPreview && a.inbox.detail == nil
	case tabEntities:
		return !a.entities.modeFocus && !a.entities.filtering &&
			(a.entities.view == entitiesViewList || a.entities.view == entitiesViewHistory || a.entities.view == entitiesViewRelationships)
	case tabRelations:
		if a.rels.editMeta.Active && !a.rels.modeFocus {
			return true
		}
		return !a.rels.modeFocus && !a.rels.filtering && a.rels.view == relsViewList
	case tabKnow:
		return !a.know.modeFocus && !a.know.filtering && a.know.view == contextViewList
	case tabJobs:
		return !a.jobs.modeFocus && !a.jobs.filtering && a.jobs.view == jobsViewList && a.jobs.detail == nil && !a.jobs.changingSt
	case tabLogs:
		if a.logs.addMeta.Active || a.logs.editMeta.Active || a.logs.addValue.Active || a.logs.editValue.Active {
			return true
		}
		return !a.logs.modeFocus && !a.logs.filtering && a.logs.view == logsViewList
	case tabFiles:
		if a.files.addMeta.Active || a.files.editMeta.Active {
			return true
		}
		return !a.files.modeFocus && !a.files.filtering && a.files.view == filesViewList
	case tabProtocols:
		if a.protocols.addMeta.Active || a.protocols.editMeta.Active {
			return true
		}
		return !a.protocols.modeFocus && !a.protocols.filtering && a.protocols.view == protocolsViewList
	case tabHistory:
		return !a.history.filtering && a.history.view == historyViewList
	case tabProfile:
		if a.profile.sectionFocus || a.profile.creating || a.profile.editAPIKey || a.profile.editPendingLimit || a.profile.createdKey != "" || a.profile.agentDetail != nil {
			return false
		}
		return true
	}
	return false
}

// switchTab handles switch tab.
func (a *App) switchTab(newTab int) (App, tea.Cmd) {
	oldTab := a.tab
	a.tab = newTab
	a.bodyScroll = 0
	a.scrollTarget = 0
	a.scrollPos = 0
	a.scrollVel = 0
	a.bodyViewKey = a.viewStateKey()
	// Animate the tab indicator toward the new tab position.
	a.tabIndTarget = float64(newTab)
	if oldTab != newTab {
		a.clearContentFocus()
		// Enter new tabs at top-nav focus so row highlights do not leak across tabs.
		a.tabNav = true
		return *a, tea.Batch(a.initTab(newTab), a.animTickCmd())
	}
	return *a, nil
}

// clearContentFocus handles clear content focus.
func (a *App) clearContentFocus() {
	a.entities.modeFocus = false
	a.rels.modeFocus = false
	a.know.modeFocus = false
	a.jobs.modeFocus = false
	a.logs.modeFocus = false
	a.files.modeFocus = false
	a.protocols.modeFocus = false
	a.profile.sectionFocus = false
}

// resetBodyScrollOnViewChange handles reset body scroll on view change.
func (a *App) resetBodyScrollOnViewChange(prevViewKey string) {
	nextViewKey := a.viewStateKey()
	if prevViewKey != "" && prevViewKey != nextViewKey {
		a.bodyScroll = 0
		a.scrollTarget = 0
		a.scrollPos = 0
		a.scrollVel = 0
	}
	a.bodyViewKey = nextViewKey
}

// viewStateKey handles view state key.
func (a App) viewStateKey() string {
	if a.helpOpen {
		return "help"
	}
	if a.quitConfirm {
		return "quit-confirm"
	}
	if a.paletteOpen {
		return "palette"
	}
	if a.importExportOpen {
		return "import-export"
	}
	if a.onboarding {
		return "onboarding"
	}
	if a.quickstartOpen {
		return "quickstart"
	}

	base := fmt.Sprintf("tab:%d", a.tab)
	switch a.tab {
	case tabInbox:
		switch {
		case a.inbox.rejectPreview:
			return base + ":inbox:reject-preview"
		case a.inbox.rejecting:
			return base + ":inbox:reject"
		case a.inbox.confirming:
			return base + ":inbox:confirm"
		case a.inbox.detail != nil:
			return base + ":inbox:detail"
		default:
			return base + ":inbox:list"
		}
	case tabEntities:
		return fmt.Sprintf("%s:entities:%d:mode=%t:filter=%t", base, a.entities.view, a.entities.modeFocus, a.entities.filtering)
	case tabRelations:
		return fmt.Sprintf("%s:rels:%d:mode=%t:filter=%t", base, a.rels.view, a.rels.modeFocus, a.rels.filtering)
	case tabKnow:
		return fmt.Sprintf("%s:context:%d:mode=%t:filter=%t", base, a.know.view, a.know.modeFocus, a.know.filtering)
	case tabJobs:
		if a.jobs.changingSt {
			return base + ":jobs:status"
		}
		if a.jobs.creatingSubtask {
			return base + ":jobs:subtask"
		}
		return fmt.Sprintf("%s:jobs:%d:mode=%t:filter=%t", base, a.jobs.view, a.jobs.modeFocus, a.jobs.filtering)
	case tabLogs:
		return fmt.Sprintf("%s:logs:%d:mode=%t:filter=%t", base, a.logs.view, a.logs.modeFocus, a.logs.filtering)
	case tabFiles:
		return fmt.Sprintf("%s:files:%d:mode=%t:filter=%t", base, a.files.view, a.files.modeFocus, a.files.filtering)
	case tabProtocols:
		return fmt.Sprintf("%s:protocols:%d:mode=%t:filter=%t", base, a.protocols.view, a.protocols.modeFocus, a.protocols.filtering)
	case tabHistory:
		return fmt.Sprintf("%s:history:%d", base, a.history.view)
	case tabProfile:
		if a.profile.sectionFocus {
			return fmt.Sprintf("%s:settings:%d:sections", base, a.profile.section)
		}
		return fmt.Sprintf("%s:settings:%d", base, a.profile.section)
	default:
		return base
	}
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
	case tabProfile:
		return a.profile.creating ||
			a.profile.createdKey != "" ||
			a.profile.taxPromptMode != taxPromptNone
	}
	return false
}

// renderTabs renders render tabs.
func (a App) renderTabs() string {
	// Nearest animated indicator position for a sliding transition effect.
	animTab := int(math.Round(a.tabIndicator))
	if animTab < 0 {
		animTab = 0
	}
	if animTab >= tabCount {
		animTab = tabCount - 1
	}
	segments := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		label := name
		isActive := i == a.tab
		// Show a "focus" trail on the animated indicator position during transitions.
		isAnimating := i == animTab && animTab != a.tab
		if isActive {
			if a.tabNav {
				segments = append(segments, TabFocusStyle.Render(label))
			} else {
				segments = append(segments, TabActiveStyle.Render(label))
			}
		} else if isAnimating {
			segments = append(segments, TabFocusStyle.Render(label))
		} else {
			segments = append(segments, TabInactiveStyle.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, segments...)
}

// initTab handles init tab.
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
	case tabProfile:
		return a.profile.Init()
	}
	return nil
}

// renderHelpBar renders the bubbles help bar for the status line.
func (a App) renderHelpBar() string {
	h := a.helpModel
	if a.width > 0 {
		h.SetWidth(a.width)
	}
	return StatusBarStyle.Width(a.width).Align(lipgloss.Center).Render(h.View(a.keys))
}

// renderHelp renders the help overlay using the bubbles help component.
func (a App) renderHelp() string {
	h := a.helpModel
	h.ShowAll = true
	if a.width > 4 {
		h.SetWidth(a.width - 4)
	}
	body := MutedStyle.Render("esc to close") + "\n\n" + h.View(a.keys)
	return components.Indent(components.TitledBox("Help", body, a.width), 1)
}

// renderQuitConfirm renders render quit confirm.
func (a App) renderQuitConfirm() string {
	body := "You have unsaved changes. Quit anyway?"
	return components.Indent(components.ConfirmDialog("Quit", body), 1)
}

// runStartupCheckCmd runs run startup check cmd.
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

// reloginCmd handles relogin cmd.
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

// setToast sets set toast and returns the clear-timer command.
// The animation tick is started separately by callers that go through toastCmdForMsg.
func (a *App) setToast(level, text string) tea.Cmd {
	a.toast = &appToast{
		level: level,
		text:  components.SanitizeOneLine(text),
	}
	// Begin slide-in: spring from animToastOffset (below) to 0.
	a.toastOffset = animToastOffset
	a.toastOffsetVel = 0
	a.toastTarget = 0
	return tea.Tick(2500*time.Millisecond, func(time.Time) tea.Msg {
		return clearToastMsg{}
	})
}

// --- Animation Helpers ---

// animTickCmd schedules the next animation frame at animFPS.
func (a App) animTickCmd() tea.Cmd {
	return tea.Tick(time.Second/animFPS, func(time.Time) tea.Msg {
		return animTickMsg{}
	})
}

// animSettled returns true when a spring has settled close enough to its target.
func animSettled(pos, vel, target float64) bool {
	return math.Abs(vel) < animSettledVel && math.Abs(pos-target) < animSettledPos
}

// advanceAnimations steps all active spring models one frame forward and returns
// another tick command if any animation is still running.
func (a *App) advanceAnimations() tea.Cmd {
	active := false

	// Advance toast slide-in spring (active only when a toast is visible).
	if a.toast != nil && !animSettled(a.toastOffset, a.toastOffsetVel, a.toastTarget) {
		a.toastOffset, a.toastOffsetVel = a.toastSpring.Update(a.toastOffset, a.toastOffsetVel, a.toastTarget)
		active = true
	} else if a.toast != nil {
		a.toastOffset = a.toastTarget
		a.toastOffsetVel = 0
	}

	// Advance smooth scroll spring. scrollPos drives the view; bodyScroll is the logical target.
	if !animSettled(a.scrollPos, a.scrollVel, a.scrollTarget) {
		a.scrollPos, a.scrollVel = a.scrollSpring.Update(a.scrollPos, a.scrollVel, a.scrollTarget)
		active = true
	} else {
		a.scrollPos = a.scrollTarget
		a.scrollVel = 0
	}

	// Advance tab indicator spring.
	if !animSettled(a.tabIndicator, a.tabIndVel, a.tabIndTarget) {
		a.tabIndicator, a.tabIndVel = a.tabSpring.Update(a.tabIndicator, a.tabIndVel, a.tabIndTarget)
		active = true
	} else {
		a.tabIndicator = a.tabIndTarget
		a.tabIndVel = 0
	}

	if active {
		return a.animTickCmd()
	}
	return nil
}

// countViewLines handles count view lines.
func countViewLines(block string) int {
	if strings.TrimSpace(block) == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

// clampBodyForViewport handles clamp body for viewport.
func clampBodyForViewport(body string, totalHeight, topLines, hintLines, scroll int) (string, bool) {
	lines := strings.Split(body, "\n")

	// Layout is rendered as: top + blank + body + blank + hints.
	const spacerLines = 2
	budget := totalHeight - topLines - hintLines - spacerLines
	if budget < 6 {
		budget = 6
	}
	if len(lines) <= budget {
		return body, false
	}

	maxScroll := len(lines) - budget
	if scroll < 0 {
		scroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	start := scroll
	end := start + budget

	trimmed := append([]string{}, lines[start:end]...)
	if start > 0 {
		trimmed[0] = MutedStyle.Render("... ↑ more")
	}
	if end < len(lines) {
		trimmed[len(trimmed)-1] = MutedStyle.Render("... ↓ more")
	}
	return strings.Join(trimmed, "\n"), true
}

// renderToast renders render toast.
func (a App) renderToast() string {
	if a.toast == nil {
		return ""
	}
	header := lipgloss.NewStyle().Bold(true)
	switch a.toast.level {
	case "success":
		header = header.Foreground(ColorSuccess)
		return components.TitledBoxWithHeaderStyle("Success", NormalStyle.Render(a.toast.text), a.width, header)
	case "warning":
		header = header.Foreground(ColorWarning)
		return components.TitledBoxWithHeaderStyle("Warning", NormalStyle.Render(a.toast.text), a.width, header)
	case "error":
		return components.ErrorBox("Error", a.toast.text, a.width)
	default:
		header = header.Foreground(ColorMuted)
		return components.TitledBoxWithHeaderStyle("Info", NormalStyle.Render(a.toast.text), a.width, header)
	}
}

// renderStartupPanel renders render startup panel.
func (a App) renderStartupPanel() string {
	rows := []components.TableRow{
		{Label: "API", Value: a.startup.API, ValueColor: startupStatusColor(a.startup.API)},
		{Label: "Auth", Value: a.startup.Auth, ValueColor: startupStatusColor(a.startup.Auth)},
		{Label: "Taxonomy", Value: a.startup.Taxonomy, ValueColor: startupStatusColor(a.startup.Taxonomy)},
	}
	return components.Table("Startup Checks", rows, a.width)
}

// toastCmdForMsg handles toast cmd for msg.
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
	clearCmd := a.setToast(level, text)
	// Start the animation tick alongside the clear timer.
	return tea.Batch(clearCmd, a.animTickCmd())
}

// handleQuickstartKeys handles handle quickstart keys.
func (a *App) handleQuickstartKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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

// handleOnboardingKeys handles handle onboarding keys.
func (a *App) handleOnboardingKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if isQuit(msg) {
		return *a, tea.Quit
	}
	if a.onboardingBusy {
		return *a, nil
	}
	if isEnter(msg) {
		username := strings.TrimSpace(a.onboardingInput.Value())
		if username == "" {
			a.err = "username is required"
			return *a, nil
		}
		a.err = ""
		a.onboardingBusy = true
		return *a, a.onboardingLoginCmd(username)
	}
	var cmd tea.Cmd
	a.onboardingInput, cmd = a.onboardingInput.Update(msg)
	return *a, cmd
}

// onboardingLoginCmd handles onboarding login cmd.
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

// renderOnboarding renders render onboarding.
func (a App) renderOnboarding() string {
	prompt := components.SanitizeOneLine(strings.TrimSpace(a.onboardingInput.Value()))
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

// finishQuickstart handles finish quickstart.
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

// renderQuickstart renders render quickstart.
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

	labelStyle := MetaKeyStyle.Bold(true)
	valueStyle := NormalStyle

	rows := []string{
		labelStyle.Render("Step") + "   " + valueStyle.Render(step.title),
		labelStyle.Render("Action") + " " + valueStyle.Render(step.desc),
		labelStyle.Render("Route") + "  " + valueStyle.Render(step.target),
	}
	body := strings.Join(rows, "\n\n") + "\n\n" + MutedStyle.Render("Use <-/-> to change step, Enter to continue, Esc to skip.")
	return components.Indent(components.TitledBox("Getting Started", body, a.width), 1)
}

// parseErrorCodeAndMessage parses parse error code and message.
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

// shouldShowRecoveryHints handles should show recovery hints.
func shouldShowRecoveryHints(code, msg string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	switch normalized {
	case "FORBIDDEN":
		lower := strings.ToLower(msg)
		return strings.Contains(lower, "scope") || strings.Contains(lower, "admin")
	case "INVALID_API_KEY", "AUTH_REQUIRED", "UNAUTHORIZED":
		return true
	}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "invalid api key") ||
		strings.Contains(lower, "invalid_api_key") ||
		strings.Contains(lower, "auth_required") ||
		strings.Contains(lower, "not logged in") {
		return true
	}
	if strings.Contains(lower, "missing or invalid authorization") || strings.Contains(lower, "http 401") {
		return true
	}
	return false
}

// shouldShowMultiAPIRecoveryHint handles should show multi api recovery hint.
func shouldShowMultiAPIRecoveryHint(code, msg, errText string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "MULTIPLE_API_INSTANCES_DETECTED" {
		return true
	}
	combined := strings.ToLower(strings.TrimSpace(msg + " " + errText))
	return strings.Contains(combined, "multiple api instances detected") ||
		strings.Contains(combined, "multiple_api_instances_detected") ||
		strings.Contains(combined, "address already in use") ||
		strings.Contains(combined, "eaddrinuse") ||
		strings.Contains(combined, "errno 98") ||
		strings.Contains(combined, "errno 48")
}

// classifyStartupAPI handles classify startup api.
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

// classifyStartupAuth handles classify startup auth.
func classifyStartupAuth(errText string, cfg *config.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.APIKey) == "" {
		return "missing"
	}
	if strings.TrimSpace(errText) == "" {
		return "ok"
	}
	lower := strings.ToLower(errText)
	switch {
	case strings.Contains(lower, "multiple api instances detected"),
		strings.Contains(lower, "multiple_api_instances_detected"),
		strings.Contains(lower, "address already in use"),
		strings.Contains(lower, "eaddrinuse"),
		strings.Contains(lower, "errno 98"),
		strings.Contains(lower, "errno 48"):
		return "multi_api_conflict"
	case strings.Contains(lower, "http 500"), strings.Contains(lower, "internal server error"):
		return "multi_api_conflict"
	case strings.Contains(lower, "invalid api key"),
		strings.Contains(lower, "invalid_api_key"),
		strings.Contains(lower, "missing or invalid authorization"),
		strings.Contains(lower, "auth_required"),
		strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "http 401"),
		strings.Contains(lower, "http 403"),
		strings.Contains(lower, "not logged in"):
		return "invalid"
	default:
		return "failed"
	}
}

// classifyStartupTaxonomy handles classify startup taxonomy.
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

// startupToastCopy handles startup toast copy.
func startupToastCopy(summary startupSummary) (string, string) {
	if summary.API == "ok" && summary.Auth == "ok" && summary.Taxonomy == "ok" {
		return "success", "Startup checks passed: API, auth, and taxonomy are healthy."
	}
	if summary.Auth == "multi_api_conflict" {
		return "error", "multiple api instances detected. stop duplicate API processes and restart with `nebula start`."
	}
	if summary.API != "ok" {
		return "error", fmt.Sprintf("Startup checks failed: API is %s.", summary.API)
	}
	return "warning", fmt.Sprintf("Startup checks: auth=%s, taxonomy=%s.", summary.Auth, summary.Taxonomy)
}

// startupStatusColor handles startup status color.
func startupStatusColor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok":
		return "#3f866b"
	case "checking":
		return "#9ba0bf"
	case "missing", "forbidden", "timeout":
		return "#c78854"
	case "invalid", "down", "failed", "schema_error", "multi_api_conflict":
		return "#6d424b"
	default:
		return "#9ba0bf"
	}
}

// openPaletteCommand handles open palette command.
func (a *App) openPaletteCommand() {
	a.paletteOpen = true
	// Open in explicit command mode. Users can backspace this to switch to search mode.
	a.paletteInput.SetValue("/")
	a.paletteInput.Focus()
	a.paletteIndex = 0
	a.paletteSearchQuery = ""
	a.paletteSearchLoading = false
	a.paletteSelections = nil
	a.paletteFiltered = filterPalette(a.paletteActions, "")
}

// paletteCommandMode handles palette command mode.
func (a App) paletteCommandMode() bool {
	query := strings.TrimSpace(a.paletteInput.Value())
	return strings.HasPrefix(query, "/")
}

// renderPalette renders render palette.
func (a App) renderPalette() string {
	commandMode := a.paletteCommandMode()
	title := "Search"
	prompt := "Search"
	if commandMode {
		title = "Command Palette"
		prompt = "Command"
	}

	query := components.SanitizeOneLine(a.paletteInput.Value())
	if commandMode {
		query = strings.TrimLeft(query, "/")
	}

	var b strings.Builder
	queryWidth := components.BoxContentWidth(a.width) - 10
	if queryWidth < 10 {
		queryWidth = 10
	}
	query = components.ClampTextWidthEllipsis(query, queryWidth)
	b.WriteString(MetaKeyStyle.Render(prompt) + MetaPunctStyle.Render(": ") + SelectedStyle.Render(query))
	b.WriteString(AccentStyle.Render("█"))
	b.WriteString("\n\n")

	items := a.paletteFiltered
	if a.paletteSearchLoading {
		b.WriteString(MutedStyle.Render("Searching..."))
	} else if len(items) == 0 {
		if commandMode {
			b.WriteString(MutedStyle.Render("No matching actions."))
		} else if strings.TrimSpace(a.paletteInput.Value()) == "" {
			b.WriteString(MutedStyle.Render("Type to search, or prefix with / for commands."))
		} else {
			b.WriteString(MutedStyle.Render("No search results."))
		}
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

// refreshPaletteFiltered handles refresh palette filtered.
func (a *App) refreshPaletteFiltered() tea.Cmd {
	a.paletteInput.SetValue(components.SanitizeOneLine(a.paletteInput.Value()))

	if a.paletteCommandMode() {
		query := strings.TrimSpace(strings.TrimLeft(a.paletteInput.Value(), "/"))
		a.paletteSearchQuery = ""
		a.paletteSearchLoading = false
		a.paletteSelections = nil
		a.paletteFiltered = filterPalette(a.paletteActions, query)
		if a.paletteIndex >= len(a.paletteFiltered) {
			a.paletteIndex = 0
		}
		return nil
	}

	query := strings.TrimSpace(a.paletteInput.Value())
	if query == "" {
		a.paletteSearchQuery = ""
		a.paletteSearchLoading = false
		a.paletteSelections = nil
		a.paletteFiltered = nil
		a.paletteIndex = 0
		return nil
	}

	if query != a.paletteSearchQuery {
		a.paletteSearchQuery = query
		a.paletteSearchLoading = true
		a.paletteSelections = nil
		a.paletteFiltered = nil
		a.paletteIndex = 0
		return a.loadPaletteSearch(query)
	}

	if a.paletteIndex >= len(a.paletteFiltered) {
		a.paletteIndex = 0
	}
	return nil
}

// loadPaletteSearch loads load palette search.
func (a App) loadPaletteSearch(query string) tea.Cmd {
	if a.client == nil {
		return nil
	}
	return func() tea.Msg {
		entities, err := a.client.QueryEntities(api.QueryParams{
			"search_text": query,
			"limit":       "8",
		})
		if err != nil {
			return errMsg{err}
		}

		contextItems, err := a.client.QueryContext(api.QueryParams{
			"search_text": query,
			"limit":       "8",
		})
		if err != nil {
			return errMsg{err}
		}

		jobs, err := a.client.QueryJobs(api.QueryParams{
			"search_text": query,
			"limit":       "8",
		})
		if err != nil {
			return errMsg{err}
		}
		rels, err := a.client.QueryRelationships(api.QueryParams{
			"limit": "100",
		})
		if err != nil {
			return errMsg{err}
		}
		logs, err := a.client.QueryLogs(api.QueryParams{
			"limit": "100",
		})
		if err != nil {
			return errMsg{err}
		}
		files, err := a.client.QueryFiles(api.QueryParams{
			"limit": "100",
		})
		if err != nil {
			return errMsg{err}
		}
		protocols, err := a.client.QueryProtocols(api.QueryParams{
			"limit": "100",
		})
		if err != nil {
			return errMsg{err}
		}

		return paletteSearchLoadedMsg{
			query:    query,
			entities: entities,
			context:  contextItems,
			jobs:     jobs,
			rels:     rels,
			logs:     logs,
			files:    files,
			protos:   protocols,
		}
	}
}

// buildSearchPaletteActions builds build search palette actions.
func buildSearchPaletteActions(
	query string,
	entities []api.Entity,
	context []api.Context,
	jobs []api.Job,
	rels []api.Relationship,
	logs []api.Log,
	files []api.File,
	protos []api.Protocol,
) ([]paletteAction, map[string]paletteSelection) {
	entries := buildPaletteSearchEntries(query, entities, context, jobs, rels, logs, files, protos)
	actions := make([]paletteAction, 0, len(entries))
	selections := make(map[string]paletteSelection, len(entries))
	for _, entry := range entries {
		id := fmt.Sprintf("%s:%s", entry.kind, entry.id)
		label := strings.TrimSpace(components.SanitizeOneLine(entry.label))
		if label == "" {
			if sid := strings.TrimSpace(shortID(entry.id)); sid != "" {
				label = sid
			} else {
				label = strings.TrimSpace(entry.kind)
			}
		}
		desc := strings.TrimSpace(components.SanitizeOneLine(entry.desc))
		desc = strings.TrimSpace(strings.Trim(desc, " ·"))
		if desc == "" {
			desc = strings.TrimSpace(entry.kind)
		}
		actions = append(actions, paletteAction{
			ID:    id,
			Label: label,
			Desc:  desc,
		})
		selections[id] = paletteSelection{
			entity:  entry.entity,
			context: entry.context,
			job:     entry.job,
			rel:     entry.rel,
			log:     entry.log,
			file:    entry.file,
			proto:   entry.proto,
		}
	}
	return actions, selections
}

// handlePaletteKeys handles handle palette keys.
func (a App) handlePaletteKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		a.paletteInput.Reset()
		return a.runPaletteAction(action)
	case isUp(msg):
		if a.paletteIndex > 0 {
			a.paletteIndex--
		}
	case isDown(msg):
		if a.paletteIndex < len(a.paletteFiltered)-1 {
			a.paletteIndex++
		}
	default:
		prev := a.paletteInput.Value()
		a.paletteInput, _ = a.paletteInput.Update(msg)
		if a.paletteInput.Value() != prev {
			return a, a.refreshPaletteFiltered()
		}
	}
	return a, nil
}

// runPaletteAction runs run palette action.
func (a *App) runPaletteAction(action paletteAction) (tea.Model, tea.Cmd) {
	a.tabNav = false

	if selection, ok := a.paletteSelections[action.ID]; ok {
		switch {
		case selection.entity != nil:
			entity := *selection.entity
			a.tab = tabEntities
			a.entities.detail = &entity
			a.entities.view = entitiesViewDetail
		case selection.context != nil:
			context := *selection.context
			a.tab = tabKnow
			a.know.detail = &context
			a.know.view = contextViewDetail
		case selection.job != nil:
			job := *selection.job
			a.tab = tabJobs
			a.jobs.detail = &job
		case selection.rel != nil:
			rel := *selection.rel
			a.tab = tabRelations
			a.rels.view = relsViewDetail
			a.rels.detail = &rel
		case selection.log != nil:
			log := *selection.log
			a.tab = tabLogs
			a.logs.view = logsViewDetail
			a.logs.detail = &log
		case selection.file != nil:
			file := *selection.file
			a.tab = tabFiles
			a.files.view = filesViewDetail
			a.files.detail = &file
		case selection.proto != nil:
			protocol := *selection.proto
			a.tab = tabProtocols
			a.protocols.view = protocolsViewDetail
			a.protocols.detail = &protocol
		}
		return *a, nil
	}

	switch action.ID {
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
	case "tab:settings", "tab:profile":
		return a.switchTab(tabProfile)
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

// applySearchSelection handles apply search selection.
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
	case "relationship":
		if msg.rel != nil {
			rel := *msg.rel
			a.tab = tabRelations
			a.rels.view = relsViewDetail
			a.rels.detail = &rel
		}
	case "log":
		if msg.log != nil {
			log := *msg.log
			a.tab = tabLogs
			a.logs.view = logsViewDetail
			a.logs.detail = &log
		}
	case "file":
		if msg.file != nil {
			file := *msg.file
			a.tab = tabFiles
			a.files.view = filesViewDetail
			a.files.detail = &file
		}
	case "protocol":
		if msg.proto != nil {
			protocol := *msg.proto
			a.tab = tabProtocols
			a.protocols.view = protocolsViewDetail
			a.protocols.detail = &protocol
		}
	}
	return *a, nil
}

// hasUnsaved handles has unsaved.
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

// contextHasInput handles context has input.
func contextHasInput(m ContextModel) bool {
	for _, f := range m.fields {
		if strings.TrimSpace(f.value) != "" {
			return true
		}
	}
	if len(m.tags) > 0 || strings.TrimSpace(m.tagInput.Value()) != "" {
		return true
	}
	if len(m.linkEntities) > 0 || strings.TrimSpace(m.linkQueryInput.Value()) != "" {
		return true
	}
	return false
}

// defaultPaletteActions handles default palette actions.
func defaultPaletteActions() []paletteAction {
	return []paletteAction{
		{ID: "tab:inbox", Label: "Inbox", Desc: "Go to inbox"},
		{ID: "tab:entities", Label: "Entities", Desc: "Browse entities"},
		{ID: "tab:relationships", Label: "Relationships", Desc: "Browse relationships"},
		{ID: "tab:context", Label: "Context", Desc: "Add context"},
		{ID: "tab:jobs", Label: "Jobs", Desc: "View jobs"},
		{ID: "tab:history", Label: "History", Desc: "Audit log"},
		{ID: "tab:settings", Label: "Settings", Desc: "Config, keys, and agents"},
		{ID: "ops:import", Label: "Import", Desc: "Bulk import from file"},
		{ID: "ops:export", Label: "Export", Desc: "Export data to file"},
		{ID: "profile:keys", Label: "Settings: API keys", Desc: "Manage keys"},
		{ID: "profile:agents", Label: "Settings: agents", Desc: "Manage agents"},
		{ID: "profile:taxonomy", Label: "Settings: taxonomy", Desc: "Manage scopes and types"},
		{ID: "quit", Label: "Quit", Desc: "Exit CLI"},
	}
}

// filterPalette handles filter palette.
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

// centerBlock handles center block.
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

// centerBlockUniform handles center block uniform.
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

// canExitToTabNav handles can exit to tab nav.
func (a App) canExitToTabNav() bool {
	switch a.tab {
	case tabInbox:
		if a.inbox.detail != nil || a.inbox.rejecting || a.inbox.confirming || a.inbox.rejectPreview {
			return false
		}
		return a.inbox.list == nil || a.inbox.list.Selected() == 0
	case tabEntities:
		if a.entities.filtering || a.entities.view != entitiesViewList {
			return false
		}
		return a.entities.list == nil || a.entities.list.Selected() == 0
	case tabRelations:
		if a.rels.filtering || a.rels.view != relsViewList {
			return false
		}
		return a.rels.list == nil || a.rels.list.Selected() == 0
	case tabKnow:
		if a.know.view == contextViewList {
			if a.know.filtering {
				return false
			}
			return a.know.list == nil || a.know.list.Selected() == 0
		}
		if a.know.view != contextViewAdd {
			return false
		}
		return !a.know.modeFocus && a.know.focus == fieldTitle
	case tabJobs:
		if a.jobs.filtering || a.jobs.detail != nil || a.jobs.changingSt {
			return false
		}
		return a.jobs.list == nil || a.jobs.list.Selected() == 0
	case tabLogs:
		if a.logs.filtering || a.logs.view != logsViewList {
			return false
		}
		return a.logs.list == nil || a.logs.list.Selected() == 0
	case tabFiles:
		if a.files.filtering || a.files.view != filesViewList {
			return false
		}
		return a.files.list == nil || a.files.list.Selected() == 0
	case tabProtocols:
		if a.protocols.filtering || a.protocols.view != protocolsViewList {
			return false
		}
		return a.protocols.list == nil || a.protocols.list.Selected() == 0
	case tabHistory:
		if a.history.filtering || a.history.view != historyViewList {
			return false
		}
		return a.history.list == nil || a.history.list.Selected() == 0
	case tabProfile:
		if a.profile.creating || a.profile.createdKey != "" || a.profile.agentDetail != nil {
			return false
		}
		if a.profile.sectionFocus {
			return true
		}
		if a.profile.section == 0 {
			return a.profile.keyList == nil || a.profile.keyList.Selected() == 0
		}
		if a.profile.section == 1 {
			return a.profile.agentList == nil || a.profile.agentList.Selected() == 0
		}
		return a.profile.taxList == nil || a.profile.taxList.Selected() == 0
	}
	return false
}

// focusModeLineForActiveTab handles focus mode line for active tab.
func (a *App) focusModeLineForActiveTab() bool {
	switch a.tab {
	case tabEntities:
		if a.entities.view == entitiesViewList || a.entities.view == entitiesViewAdd {
			a.entities.modeFocus = true
			return true
		}
	case tabRelations:
		if a.rels.view == relsViewList || a.rels.isAddView() {
			a.rels.modeFocus = true
			return true
		}
	case tabKnow:
		if a.know.view == contextViewList || a.know.view == contextViewAdd {
			a.know.modeFocus = true
			return true
		}
	case tabJobs:
		if a.jobs.view == jobsViewList || a.jobs.view == jobsViewAdd {
			a.jobs.modeFocus = true
			return true
		}
	case tabLogs:
		if a.logs.view == logsViewList || a.logs.view == logsViewAdd {
			a.logs.modeFocus = true
			return true
		}
	case tabFiles:
		if a.files.view == filesViewList || a.files.view == filesViewAdd {
			a.files.modeFocus = true
			return true
		}
	case tabProtocols:
		if a.protocols.view == protocolsViewList || a.protocols.view == protocolsViewAdd || a.protocols.view == protocolsViewEdit {
			a.protocols.modeFocus = true
			return true
		}
	case tabProfile:
		if a.profile.taxPromptMode == taxPromptNone &&
			!a.profile.creating &&
			!a.profile.editAPIKey &&
			!a.profile.editPendingLimit &&
			a.profile.createdKey == "" &&
			a.profile.agentDetail == nil {
			a.profile.sectionFocus = true
			return true
		}
	}
	return false
}

// tabIndexForKey handles tab index for key.
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
	}
	return 0, false
}

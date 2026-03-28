package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/gravitrone/nebula-core/cli/internal/api"
)

// --- Golden Snapshot Tests ---
//
// Each test creates a deterministic App state, renders View(), and compares
// against a stored .golden file under testdata/.
//
// Generate golden files:  go test ./internal/ui/ -run TestGolden -update -count=1
// Verify golden files:    go test ./internal/ui/ -run TestGolden -count=1

// TestGolden_AppStartupBanner captures the initial render with banner and tabs.
func TestGolden_AppStartupBanner(t *testing.T) {
	app := goldenAppNoData(t)
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_InboxEmpty captures inbox tab with no pending approvals.
func TestGolden_InboxEmpty(t *testing.T) {
	app := goldenAppNoData(t)
	// Feed an empty approvals loaded message to stop loading state.
	app = feedMsg(app, approvalsLoadedMsg{items: nil})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_InboxWithApprovals captures inbox with 3 pending approval items.
func TestGolden_InboxWithApprovals(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = feedMsg(app, approvalsLoadedMsg{items: []api.Approval{
		{
			ID:              "apr-001",
			RequestType:     "create_entity",
			RequestedBy:     "ent-agent-01",
			RequestedByName: "ResearchBot",
			AgentName:       "ResearchBot",
			Status:          "pending",
			CreatedAt:       goldenTime1,
		},
		{
			ID:              "apr-002",
			RequestType:     "update_context",
			RequestedBy:     "ent-agent-02",
			RequestedByName: "IndexBot",
			AgentName:       "IndexBot",
			Status:          "pending",
			CreatedAt:       goldenTime2,
		},
		{
			ID:              "apr-003",
			RequestType:     "create_relationship",
			RequestedBy:     "ent-agent-03",
			RequestedByName: "LinkBot",
			AgentName:       "LinkBot",
			Status:          "pending",
			CreatedAt:       goldenTime3,
		},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_EntitiesListEmpty captures the entities tab with no data loaded.
func TestGolden_EntitiesListEmpty(t *testing.T) {
	app := goldenAppNoData(t)
	app = driveToTab(app, 2) // entities tab
	app.entities.loading = false
	app.entities.items = nil
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_EntitiesListPopulated captures entities tab with 5 items.
func TestGolden_EntitiesListPopulated(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 2)
	app = feedMsg(app, entitiesLoadedMsg{items: []api.Entity{
		{ID: "ent-001", Name: "AlphaAgent", Type: "agent", Status: "active", Tags: []string{"ai", "research"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1},
		{ID: "ent-002", Name: "BetaModel", Type: "model", Status: "active", Tags: []string{"ml", "production"}, CreatedAt: goldenTime2, UpdatedAt: goldenTime2},
		{ID: "ent-003", Name: "GammaDataset", Type: "dataset", Status: "active", Tags: []string{"training"}, CreatedAt: goldenTime3, UpdatedAt: goldenTime3},
		{ID: "ent-004", Name: "DeltaPipeline", Type: "pipeline", Status: "paused", Tags: []string{"etl"}, CreatedAt: goldenTime4, UpdatedAt: goldenTime4},
		{ID: "ent-005", Name: "EpsilonService", Type: "service", Status: "active", Tags: []string{"api", "backend"}, CreatedAt: goldenTime5, UpdatedAt: goldenTime5},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_EntitiesDetailView captures entity detail after selecting an item.
func TestGolden_EntitiesDetailView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 2)
	app = feedMsg(app, entitiesLoadedMsg{items: []api.Entity{
		{ID: "ent-001", Name: "AlphaAgent", Type: "agent", Status: "active", Tags: []string{"ai", "research"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1},
	}})
	// Set detail directly to avoid async loading.
	entity := api.Entity{ID: "ent-001", Name: "AlphaAgent", Type: "agent", Status: "active", Tags: []string{"ai", "research"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1}
	app.entities.detail = &entity
	app.entities.view = entitiesViewDetail
	app.entities.loading = false
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_EntitiesAddForm captures the entities add form view.
func TestGolden_EntitiesAddForm(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 2)
	app = feedMsg(app, entitiesLoadedMsg{items: nil})
	app.entities.view = entitiesViewAdd
	app.entities.loading = false
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_RelationshipsListView captures the relationships tab with data.
func TestGolden_RelationshipsListView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 3) // relationships
	app = feedMsg(app, relationshipsLoadedMsg{items: []api.Relationship{
		{ID: "rel-001", SourceID: "ent-001", SourceName: "AlphaAgent", TargetID: "ent-002", TargetName: "BetaModel", Type: "uses", Status: "active", CreatedAt: goldenTime1},
		{ID: "rel-002", SourceID: "ent-002", SourceName: "BetaModel", TargetID: "ent-003", TargetName: "GammaDataset", Type: "trained-on", Status: "active", CreatedAt: goldenTime2},
		{ID: "rel-003", SourceID: "ent-004", SourceName: "DeltaPipeline", TargetID: "ent-003", TargetName: "GammaDataset", Type: "produces", Status: "active", CreatedAt: goldenTime3},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_ContextListView captures the context tab with items loaded.
func TestGolden_ContextListView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 4) // context
	app.know.view = contextViewList
	app.know.loadingList = false
	app.know.items = []api.Context{
		{ID: "ctx-001", Title: "Architecture Overview", Name: "Architecture Overview", SourceType: "note", Status: "active", Tags: []string{"docs", "architecture"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1},
		{ID: "ctx-002", Title: "Training Pipeline Docs", Name: "Training Pipeline Docs", SourceType: "article", Status: "active", Tags: []string{"ml", "pipeline"}, CreatedAt: goldenTime2, UpdatedAt: goldenTime2},
		{ID: "ctx-003", Title: "Safety Guidelines", Name: "Safety Guidelines", SourceType: "paper", Status: "active", Tags: []string{"safety"}, CreatedAt: goldenTime3, UpdatedAt: goldenTime3},
	}
	app.know.allItems = app.know.items
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_ContextAddView captures the context add form.
func TestGolden_ContextAddView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 4)
	app.know.view = contextViewAdd
	app.know.loadingList = false
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_JobsListView captures jobs tab with items.
func TestGolden_JobsListView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 5)
	desc1 := "Fine-tune BetaModel on new dataset"
	desc2 := "Run safety evaluation suite"
	desc3 := "Deploy to staging environment"
	app = feedMsg(app, jobsLoadedMsg{items: []api.Job{
		{ID: "2026Q1-0001", Title: "Fine-tune BetaModel", Description: &desc1, Status: "in_progress", CreatedAt: goldenTime1, UpdatedAt: goldenTime2},
		{ID: "2026Q1-0002", Title: "Safety Evaluation", Description: &desc2, Status: "pending", CreatedAt: goldenTime2, UpdatedAt: goldenTime2},
		{ID: "2026Q1-0003", Title: "Staging Deploy", Description: &desc3, Status: "done", CreatedAt: goldenTime3, UpdatedAt: goldenTime4},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_JobsDetailView captures a job detail view.
func TestGolden_JobsDetailView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 5)
	desc := "Fine-tune BetaModel on new dataset"
	prio := "high"
	job := api.Job{ID: "2026Q1-0001", Title: "Fine-tune BetaModel", Description: &desc, Status: "in_progress", Priority: &prio, CreatedAt: goldenTime1, UpdatedAt: goldenTime2}
	app = feedMsg(app, jobsLoadedMsg{items: []api.Job{job}})
	app.jobs.detail = &job
	app.jobs.loading = false
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_LogsListView captures the logs tab with data.
func TestGolden_LogsListView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 6)
	app = feedMsg(app, logsLoadedMsg{items: []api.Log{
		{ID: "log-001", LogType: "training", Timestamp: goldenTime1, Content: "epoch: 10, loss: 0.032", Status: "active", Tags: []string{"ml"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1},
		{ID: "log-002", LogType: "deployment", Timestamp: goldenTime2, Content: "version: 1.2.0, env: staging", Status: "active", Tags: []string{"ops"}, CreatedAt: goldenTime2, UpdatedAt: goldenTime2},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_FilesListView captures the files tab with data.
func TestGolden_FilesListView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 7)
	mime1 := "application/octet-stream"
	mime2 := "text/csv"
	size1 := int64(1048576)
	size2 := int64(524288)
	app = feedMsg(app, filesLoadedMsg{items: []api.File{
		{ID: "file-001", Filename: "model-weights-v3.bin", URI: "s3://nebula/models/weights-v3.bin", FilePath: "/data/models/weights-v3.bin", MimeType: &mime1, SizeBytes: &size1, Status: "active", Tags: []string{"model", "weights"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1},
		{ID: "file-002", Filename: "training-data.csv", URI: "s3://nebula/data/training.csv", FilePath: "/data/training/data.csv", MimeType: &mime2, SizeBytes: &size2, Status: "active", Tags: []string{"data", "training"}, CreatedAt: goldenTime2, UpdatedAt: goldenTime2},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_ProtocolsListView captures the protocols tab with data.
func TestGolden_ProtocolsListView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 8)
	ver1 := "1.0"
	ver2 := "2.1"
	ptype1 := "review"
	ptype2 := "checklist"
	app = feedMsg(app, protocolsLoadedMsg{items: []api.Protocol{
		{ID: "proto-001", Name: "safety-review", Title: "Safety Review Protocol", Version: &ver1, ProtocolType: &ptype1, Status: "active", Tags: []string{"safety"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1},
		{ID: "proto-002", Name: "deployment-checklist", Title: "Deployment Checklist", Version: &ver2, ProtocolType: &ptype2, Status: "active", Tags: []string{"ops", "deploy"}, CreatedAt: goldenTime2, UpdatedAt: goldenTime2},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_HistoryListView captures the history/audit tab with entries.
func TestGolden_HistoryListView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 9) // history
	actorType := "entity"
	actorID := "ent-001"
	actorName := "AlphaAgent"
	app = feedMsg(app, historyLoadedMsg{items: []api.AuditEntry{
		{ID: "audit-001", TableName: "entities", RecordID: "ent-002", Action: "UPDATE", ChangedByType: &actorType, ChangedByID: &actorID, ActorName: &actorName, OldValues: `{"status":"draft"}`, NewValues: `{"status":"active"}`, ChangedFields: []string{"status"}, ChangedAt: goldenTime1},
		{ID: "audit-002", TableName: "context_items", RecordID: "ctx-001", Action: "INSERT", ChangedByType: &actorType, ChangedByID: &actorID, ActorName: &actorName, NewValues: `{"title":"Architecture Overview"}`, ChangedFields: []string{"title", "source_type"}, ChangedAt: goldenTime2},
	}})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_ProfileView captures the settings/profile tab.
func TestGolden_ProfileView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 0) // profile is tab 0 in display (mapped to "0" key)
	// Profile is tab index 9 internally. The "0" key maps to tab 10 which wraps.
	// Use direct assignment instead.
	app.tab = tabProfile
	app.profile.loading = false
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_HelpOverlay captures the help overlay.
func TestGolden_HelpOverlay(t *testing.T) {
	app := goldenAppNoData(t)
	app.helpOpen = true
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_CommandPalette captures the command palette open state.
func TestGolden_CommandPalette(t *testing.T) {
	app := goldenAppNoData(t)
	app.paletteOpen = true
	app.paletteFiltered = app.paletteActions
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_ImportExportView captures the import/export wizard at step 1.
func TestGolden_ImportExportView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app.importExportOpen = true
	app.impex.Start(exportMode)
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_QuitConfirm captures the quit confirmation dialog.
func TestGolden_QuitConfirm(t *testing.T) {
	app := goldenAppNoData(t)
	app.quitConfirm = true
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_ErrorState captures the app with an error message visible.
func TestGolden_ErrorState(t *testing.T) {
	app := goldenAppNoData(t)
	app.err = "connection refused: server not reachable"
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_OnboardingView captures the onboarding screen for new users.
func TestGolden_OnboardingView(t *testing.T) {
	app := NewApp(nil, nil)
	app.onboarding = true
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app = model.(App)
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_EntitiesListEmpty_WithStartup captures entities empty with startup done.
func TestGolden_EntitiesListEmpty_WithStartup(t *testing.T) {
	app := goldenAppNoData(t)
	app = driveToTab(app, 2)
	app.entities.loading = false
	app.entities.items = []api.Entity{}
	app.entities.allItems = []api.Entity{}
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_RelationshipsEmpty captures relationships tab with no data.
func TestGolden_RelationshipsEmpty(t *testing.T) {
	app := goldenAppNoData(t)
	app = driveToTab(app, 3)
	app = feedMsg(app, relationshipsLoadedMsg{items: nil})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_HistoryEmpty captures history tab with no audit entries.
func TestGolden_HistoryEmpty(t *testing.T) {
	app := goldenAppNoData(t)
	app = driveToTab(app, 9)
	app = feedMsg(app, historyLoadedMsg{items: nil})
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_RecoveryHints captures recovery hint display after auth error.
func TestGolden_RecoveryHints(t *testing.T) {
	app := goldenAppNoData(t)
	app.err = "authentication failed: invalid API key"
	app.showRecoveryHints = true
	app.recoveryCommand = "nebula login"
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_StartupChecking captures the startup panel with checking spinners.
func TestGolden_StartupChecking(t *testing.T) {
	app := goldenAppNoData(t)
	app.startupChecking = true
	app.startup = startupSummary{
		API:      "ok",
		Auth:     "checking",
		Taxonomy: "checking",
	}
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_InboxDetail captures inbox with a detail view open.
func TestGolden_InboxDetail(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	approval := api.Approval{
		ID:              "apr-001",
		RequestType:     "create_entity",
		RequestedBy:     "ent-agent-01",
		RequestedByName: "ResearchBot",
		AgentName:       "ResearchBot",
		ChangeDetails:   `{"name":"NewEntity","type":"dataset"}`,
		Status:          "pending",
		CreatedAt:       goldenTime1,
	}
	app = feedMsg(app, approvalsLoadedMsg{items: []api.Approval{approval}})
	app.inbox.detail = &approval
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_ContextDetailView captures context detail after selecting an item.
func TestGolden_ContextDetailView(t *testing.T) {
	app := goldenApp(t, goldenDataHandler)
	app = driveToTab(app, 4)
	ctx := api.Context{ID: "ctx-001", Title: "Architecture Overview", Name: "Architecture Overview", SourceType: "note", Status: "active", Tags: []string{"docs", "architecture"}, CreatedAt: goldenTime1, UpdatedAt: goldenTime1}
	app.know.view = contextViewDetail
	app.know.detail = &ctx
	app.know.loadingList = false
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_QuickstartWizard captures the quickstart wizard overlay.
func TestGolden_QuickstartWizard(t *testing.T) {
	app := goldenAppNoData(t)
	app.quickstartOpen = true
	app.quickstartStep = 0
	golden.RequireEqual(t, []byte(viewContent(app)))
}

// TestGolden_StartupAllOK captures the startup panel when all checks pass.
func TestGolden_StartupAllOK(t *testing.T) {
	app := goldenAppNoData(t)
	app = feedMsg(app, startupCheckedMsg{})
	_ = time.Now() // ensure no timestamp drift in test
	golden.RequireEqual(t, []byte(viewContent(app)))
}

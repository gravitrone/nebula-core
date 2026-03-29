package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
)

// --- Fixed Timestamps ---

var (
	goldenTime1 = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	goldenTime2 = time.Date(2026, 1, 16, 14, 0, 0, 0, time.UTC)
	goldenTime3 = time.Date(2026, 1, 17, 9, 15, 0, 0, time.UTC)
	goldenTime4 = time.Date(2026, 1, 18, 11, 45, 0, 0, time.UTC)
	goldenTime5 = time.Date(2026, 1, 19, 16, 20, 0, 0, time.UTC)
)

// --- Helper Functions ---

// goldenApp creates an App with deterministic settings for golden file tests.
// It disables startup checking and sets a fixed terminal size.
func goldenApp(t *testing.T, handler http.HandlerFunc) App {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := api.NewClient(srv.URL, "test-key")
	cfg := &config.Config{}
	app := NewApp(client, cfg)
	// Disable startup checking so the banner panel doesn't show spinners.
	app.startupChecking = false
	app.startup = startupSummary{Done: true}
	// Set deterministic terminal size.
	model, _ := app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return model.(App)
}

// goldenAppNoData creates an App backed by an empty data handler.
func goldenAppNoData(t *testing.T) App {
	t.Helper()
	return goldenApp(t, emptyDataHandler)
}

// driveToTab sends a numeric key to switch to the given tab index (1-based).
func driveToTab(app App, tabIdx int) App {
	ch := rune('0' + rune(tabIdx)) //nolint:gosec // tabIdx is always 0-9
	model, _ := app.Update(tea.KeyPressMsg{Code: ch, Text: string(ch)})
	return model.(App)
}

// feedMsg sends a message through Update and returns the updated App.
func feedMsg(app App, msg tea.Msg) App {
	m, _ := app.Update(msg)
	return m.(App)
}

// viewContent extracts the plain-text content from app.View().
func viewContent(app App) string {
	return app.View().Content
}

// --- Empty Data Handler ---

// emptyDataHandler returns empty arrays for all API endpoints.
func emptyDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
}

// --- Rich Data Handler ---

// goldenDataHandler returns realistic deterministic data for all major endpoints.
func goldenDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path

	switch {
	case path == "/api/approvals" || path == "/api/approvals/pending":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenApprovals()})
	case strings.HasPrefix(path, "/api/approvals/") && strings.HasSuffix(path, "/diff"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
			"approval_id":  "apr-001",
			"request_type": "create_entity",
			"changes":      map[string]any{"name": "NewAgent"},
		}})
	case path == "/api/entities" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenEntities()})
	case strings.HasPrefix(path, "/api/entities/") && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(goldenEntities()[0])
	case path == "/api/relationships" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenRelationships()})
	case strings.HasPrefix(path, "/api/relationships/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case path == "/api/context" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenContextItems()})
	case strings.HasPrefix(path, "/api/context/by-owner/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case path == "/api/jobs" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenJobs()})
	case strings.HasPrefix(path, "/api/jobs/"):
		_ = json.NewEncoder(w).Encode(goldenJobs()[0])
	case path == "/api/logs" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenLogs()})
	case path == "/api/files" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenFiles()})
	case path == "/api/protocols" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenProtocols()})
	case path == "/api/audit" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": goldenHistory()})
	case path == "/api/audit/scopes":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case path == "/api/audit/actors":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case path == "/api/keys" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case path == "/api/agents" && r.Method == "GET":
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	case strings.HasPrefix(path, "/api/taxonomy/"):
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}
}

// --- Mock Data Factories ---

func goldenApprovals() []map[string]any {
	return []map[string]any{
		{
			"id":                "apr-001",
			"job_id":            "2026Q1-0001",
			"request_type":      "create_entity",
			"requested_by":      "ent-agent-01",
			"requested_by_name": "ResearchBot",
			"agent_name":        "ResearchBot",
			"change_details":    map[string]any{"name": "NewEntity", "type": "dataset"},
			"review_details":    map[string]any{},
			"status":            "pending",
			"created_at":        goldenTime1.Format(time.RFC3339),
		},
		{
			"id":                "apr-002",
			"request_type":      "update_context",
			"requested_by":      "ent-agent-02",
			"requested_by_name": "IndexBot",
			"agent_name":        "IndexBot",
			"change_details":    map[string]any{"title": "Updated Notes"},
			"review_details":    map[string]any{},
			"status":            "pending",
			"created_at":        goldenTime2.Format(time.RFC3339),
		},
		{
			"id":                "apr-003",
			"request_type":      "create_relationship",
			"requested_by":      "ent-agent-03",
			"requested_by_name": "LinkBot",
			"agent_name":        "LinkBot",
			"change_details":    map[string]any{"source_id": "ent-001", "target_id": "ent-002"},
			"review_details":    map[string]any{},
			"status":            "pending",
			"created_at":        goldenTime3.Format(time.RFC3339),
		},
	}
}

func goldenEntities() []map[string]any {
	return []map[string]any{
		{
			"id":         "ent-001",
			"name":       "AlphaAgent",
			"type":       "agent",
			"status":     "active",
			"tags":       []string{"ai", "research"},
			"created_at": goldenTime1.Format(time.RFC3339),
			"updated_at": goldenTime1.Format(time.RFC3339),
		},
		{
			"id":         "ent-002",
			"name":       "BetaModel",
			"type":       "model",
			"status":     "active",
			"tags":       []string{"ml", "production"},
			"created_at": goldenTime2.Format(time.RFC3339),
			"updated_at": goldenTime2.Format(time.RFC3339),
		},
		{
			"id":         "ent-003",
			"name":       "GammaDataset",
			"type":       "dataset",
			"status":     "active",
			"tags":       []string{"training"},
			"created_at": goldenTime3.Format(time.RFC3339),
			"updated_at": goldenTime3.Format(time.RFC3339),
		},
		{
			"id":         "ent-004",
			"name":       "DeltaPipeline",
			"type":       "pipeline",
			"status":     "paused",
			"tags":       []string{"etl"},
			"created_at": goldenTime4.Format(time.RFC3339),
			"updated_at": goldenTime4.Format(time.RFC3339),
		},
		{
			"id":         "ent-005",
			"name":       "EpsilonService",
			"type":       "service",
			"status":     "active",
			"tags":       []string{"api", "backend"},
			"created_at": goldenTime5.Format(time.RFC3339),
			"updated_at": goldenTime5.Format(time.RFC3339),
		},
	}
}

func goldenRelationships() []map[string]any {
	return []map[string]any{
		{
			"id":                "rel-001",
			"source_type":       "entity",
			"source_id":         "ent-001",
			"source_name":       "AlphaAgent",
			"target_type":       "entity",
			"target_id":         "ent-002",
			"target_name":       "BetaModel",
			"relationship_type": "uses",
			"status":            "active",
			"notes":             "",
			"created_at":        goldenTime1.Format(time.RFC3339),
		},
		{
			"id":                "rel-002",
			"source_type":       "entity",
			"source_id":         "ent-002",
			"source_name":       "BetaModel",
			"target_type":       "entity",
			"target_id":         "ent-003",
			"target_name":       "GammaDataset",
			"relationship_type": "trained-on",
			"status":            "active",
			"notes":             "",
			"created_at":        goldenTime2.Format(time.RFC3339),
		},
		{
			"id":                "rel-003",
			"source_type":       "entity",
			"source_id":         "ent-004",
			"source_name":       "DeltaPipeline",
			"target_type":       "entity",
			"target_id":         "ent-003",
			"target_name":       "GammaDataset",
			"relationship_type": "produces",
			"status":            "active",
			"notes":             "",
			"created_at":        goldenTime3.Format(time.RFC3339),
		},
	}
}

func goldenContextItems() []map[string]any {
	return []map[string]any{
		{
			"id":          "ctx-001",
			"title":       "Architecture Overview",
			"name":        "Architecture Overview",
			"source_type": "note",
			"status":      "active",
			"tags":        []string{"docs", "architecture"},
			"created_at":  goldenTime1.Format(time.RFC3339),
			"updated_at":  goldenTime1.Format(time.RFC3339),
		},
		{
			"id":          "ctx-002",
			"title":       "Training Pipeline Docs",
			"name":        "Training Pipeline Docs",
			"source_type": "article",
			"status":      "active",
			"tags":        []string{"ml", "pipeline"},
			"created_at":  goldenTime2.Format(time.RFC3339),
			"updated_at":  goldenTime2.Format(time.RFC3339),
		},
		{
			"id":          "ctx-003",
			"title":       "Safety Guidelines",
			"name":        "Safety Guidelines",
			"source_type": "paper",
			"status":      "active",
			"tags":        []string{"safety"},
			"created_at":  goldenTime3.Format(time.RFC3339),
			"updated_at":  goldenTime3.Format(time.RFC3339),
		},
	}
}

func goldenJobs() []map[string]any {
	desc1 := "Fine-tune BetaModel on new dataset"
	desc2 := "Run safety evaluation suite"
	desc3 := "Deploy to staging environment"
	return []map[string]any{
		{
			"id":          "2026Q1-0001",
			"title":       "Fine-tune BetaModel",
			"description": desc1,
			"status":      "in_progress",
			"priority":    "high",
			"created_at":  goldenTime1.Format(time.RFC3339),
			"updated_at":  goldenTime2.Format(time.RFC3339),
		},
		{
			"id":          "2026Q1-0002",
			"title":       "Safety Evaluation",
			"description": desc2,
			"status":      "pending",
			"priority":    "critical",
			"created_at":  goldenTime2.Format(time.RFC3339),
			"updated_at":  goldenTime2.Format(time.RFC3339),
		},
		{
			"id":          "2026Q1-0003",
			"title":       "Staging Deploy",
			"description": desc3,
			"status":      "done",
			"priority":    "medium",
			"created_at":  goldenTime3.Format(time.RFC3339),
			"updated_at":  goldenTime4.Format(time.RFC3339),
		},
	}
}

func goldenLogs() []map[string]any {
	return []map[string]any{
		{
			"id":         "log-001",
			"log_type":   "training",
			"timestamp":  goldenTime1.Format(time.RFC3339),
			"value":      map[string]any{"epoch": 10, "loss": 0.032},
			"status":     "active",
			"tags":       []string{"ml"},
			"metadata":   map[string]any{},
			"created_at": goldenTime1.Format(time.RFC3339),
			"updated_at": goldenTime1.Format(time.RFC3339),
		},
		{
			"id":         "log-002",
			"log_type":   "deployment",
			"timestamp":  goldenTime2.Format(time.RFC3339),
			"value":      map[string]any{"version": "1.2.0", "env": "staging"},
			"status":     "active",
			"tags":       []string{"ops"},
			"metadata":   map[string]any{},
			"created_at": goldenTime2.Format(time.RFC3339),
			"updated_at": goldenTime2.Format(time.RFC3339),
		},
	}
}

func goldenFiles() []map[string]any {
	size1 := int64(1048576)
	size2 := int64(524288)
	return []map[string]any{
		{
			"id":         "file-001",
			"filename":   "model-weights-v3.bin",
			"uri":        "s3://nebula/models/weights-v3.bin",
			"file_path":  "/data/models/weights-v3.bin",
			"mime_type":  "application/octet-stream",
			"size_bytes": size1,
			"status":     "active",
			"tags":       []string{"model", "weights"},
			"metadata":   map[string]any{},
			"created_at": goldenTime1.Format(time.RFC3339),
			"updated_at": goldenTime1.Format(time.RFC3339),
		},
		{
			"id":         "file-002",
			"filename":   "training-data.csv",
			"uri":        "s3://nebula/data/training.csv",
			"file_path":  "/data/training/data.csv",
			"mime_type":  "text/csv",
			"size_bytes": size2,
			"status":     "active",
			"tags":       []string{"data", "training"},
			"metadata":   map[string]any{},
			"created_at": goldenTime2.Format(time.RFC3339),
			"updated_at": goldenTime2.Format(time.RFC3339),
		},
	}
}

func goldenProtocols() []map[string]any {
	return []map[string]any{
		{
			"id":            "proto-001",
			"name":          "safety-review",
			"title":         "Safety Review Protocol",
			"version":       "1.0",
			"protocol_type": "review",
			"status":        "active",
			"tags":          []string{"safety"},
			"metadata":      map[string]any{},
			"created_at":    goldenTime1.Format(time.RFC3339),
			"updated_at":    goldenTime1.Format(time.RFC3339),
		},
		{
			"id":            "proto-002",
			"name":          "deployment-checklist",
			"title":         "Deployment Checklist",
			"version":       "2.1",
			"protocol_type": "checklist",
			"status":        "active",
			"tags":          []string{"ops", "deploy"},
			"metadata":      map[string]any{},
			"created_at":    goldenTime2.Format(time.RFC3339),
			"updated_at":    goldenTime2.Format(time.RFC3339),
		},
	}
}

func goldenHistory() []map[string]any {
	actorType := "entity"
	actorID := "ent-001"
	actorName := "AlphaAgent"
	return []map[string]any{
		{
			"id":              "audit-001",
			"table_name":      "entities",
			"record_id":       "ent-002",
			"action":          "UPDATE",
			"changed_by_type": actorType,
			"changed_by_id":   actorID,
			"actor_name":      actorName,
			"old_data":        map[string]any{"status": "draft"},
			"new_data":        map[string]any{"status": "active"},
			"changed_fields":  []string{"status"},
			"metadata":        map[string]any{},
			"changed_at":      goldenTime1.Format(time.RFC3339),
		},
		{
			"id":              "audit-002",
			"table_name":      "context_items",
			"record_id":       "ctx-001",
			"action":          "INSERT",
			"changed_by_type": actorType,
			"changed_by_id":   actorID,
			"actor_name":      actorName,
			"old_data":        map[string]any{},
			"new_data":        map[string]any{"title": "Architecture Overview"},
			"changed_fields":  []string{"title", "source_type"},
			"metadata":        map[string]any{},
			"changed_at":      goldenTime2.Format(time.RFC3339),
		},
	}
}

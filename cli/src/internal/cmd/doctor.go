package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// DoctorCmd runs local shell-first diagnostics for non-interactive workflows.
func DoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run local diagnostics and show exact fix commands",
		RunE: func(command *cobra.Command, _ []string) error {
			checks := runDoctorChecks()
			mode := resolveOutputMode(OutputModeTable)
			switch mode {
			case OutputModeJSON, OutputModePlain:
				return writeCleanJSON(command.OutOrStdout(), map[string]any{
					"ok":     doctorChecksHealthy(checks),
					"checks": checks,
				})
			default:
				rows := make([]components.TableRow, 0, len(checks)*2+1)
				summary := "ok"
				if !doctorChecksHealthy(checks) {
					summary = "action needed"
				}
				rows = append(rows, components.TableRow{Label: "summary", Value: summary})
				for _, check := range checks {
					value := fmt.Sprintf("%s - %s", check.Status, check.Detail)
					rows = append(rows, components.TableRow{Label: check.Name, Value: value})
					if strings.TrimSpace(check.Fix) != "" {
						rows = append(rows, components.TableRow{Label: "  fix", Value: check.Fix})
					}
				}
				renderCommandPanel(command.OutOrStdout(), "Nebula Doctor", rows)
				return nil
			}
		},
	}
}

// runDoctorChecks runs deterministic local checks without side effects.
func runDoctorChecks() []doctorCheck {
	checks := make([]doctorCheck, 0, 5)

	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		checks = append(checks, doctorCheck{
			Name:   "config",
			Status: "warn",
			Detail: "not logged in or config unreadable",
			Fix:    "nebula login",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:   "config",
			Status: "ok",
			Detail: fmt.Sprintf("user %s, file %s", safeDoctorValue(cfg.Username, "configured"), config.Path()),
		})
	}

	apiClient := newDefaultClient("")
	if cfg != nil {
		apiClient = newDefaultClient(cfg.APIKey, 1200*time.Millisecond)
	}
	health, err := apiClient.Health()
	if err != nil {
		checks = append(checks, doctorCheck{
			Name:   "api",
			Status: "warn",
			Detail: "unreachable or unhealthy",
			Fix:    "nebula start",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:   "api",
			Status: "ok",
			Detail: "health " + health,
		})
	}

	serverDir, err := resolveServerDir()
	if err != nil {
		checks = append(checks, doctorCheck{
			Name:   "server_dir",
			Status: "warn",
			Detail: "server path not resolved",
			Fix:    "export NEBULA_SERVER_DIR=\"<path-to-nebula-core>/server\"",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name:   "server_dir",
			Status: "ok",
			Detail: serverDir,
		})
		uvicornPath := filepath.Join(serverDir, ".venv", "bin", "uvicorn")
		if _, statErr := os.Stat(uvicornPath); statErr != nil {
			checks = append(checks, doctorCheck{
				Name:   "uvicorn",
				Status: "warn",
				Detail: ".venv uvicorn missing",
				Fix:    fmt.Sprintf("cd %s && uv sync", serverDir),
			})
		} else {
			checks = append(checks, doctorCheck{
				Name:   "uvicorn",
				Status: "ok",
				Detail: uvicornPath,
			})
		}
	}

	return checks
}

// doctorChecksHealthy returns true when all checks passed.
func doctorChecksHealthy(checks []doctorCheck) bool {
	for _, check := range checks {
		if strings.ToLower(strings.TrimSpace(check.Status)) != "ok" {
			return false
		}
	}
	return true
}

// safeDoctorValue sanitizes optionally-empty values in check summaries.
func safeDoctorValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

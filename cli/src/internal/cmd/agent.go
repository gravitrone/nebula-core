package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// AgentCmd returns the `nebula agent` command group.
func AgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage agents",
	}
	cmd.AddCommand(agentRegisterCmd())
	cmd.AddCommand(agentListCmd())
	return cmd
}

// agentRegisterCmd handles agent register cmd.
func agentRegisterCmd() *cobra.Command {
	var desc string
	cmd := &cobra.Command{
		Use:   "register <name>",
		Short: "Register a new agent (creates approval request)",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}
			client := api.NewDefaultClient(cfg.APIKey)

			input := api.RegisterAgentInput{
				Name:            args[0],
				Description:     desc,
				RequestedScopes: []string{"public"},
			}

			resp, err := client.RegisterAgent(input)
			if err != nil {
				return fmt.Errorf("register agent: %w", err)
			}

			renderCommandPanel(command.OutOrStdout(), "Agent Registered", []components.TableRow{
				{Label: "agent_id", Value: resp.AgentID},
				{Label: "status", Value: resp.Status},
				{Label: "approval_request", Value: resp.ApprovalRequestID},
				{Label: "next", Value: "approve in nebula inbox or via api"},
			})
			return nil
		},
	}
	cmd.Flags().StringVarP(&desc, "description", "d", "", "agent description")
	return cmd
}

// agentListCmd handles agent list cmd.
func agentListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all agents",
		RunE: func(command *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}
			client := api.NewDefaultClient(cfg.APIKey)

			agents, err := client.ListAgents("active")
			if err != nil {
				return fmt.Errorf("list agents: %w", err)
			}

			if len(agents) == 0 {
				renderCommandMessage(command.OutOrStdout(), "Agents", "No agents found.")
				return nil
			}

			rows := make([]components.TableRow, 0, len(agents))
			for _, a := range agents {
				trust := "trusted"
				if a.RequiresApproval {
					trust = "untrusted"
				}
				desc := ""
				if a.Description != nil {
					desc = " - " + *a.Description
				}
				rows = append(rows, components.TableRow{
					Label: a.Name,
					Value: fmt.Sprintf("%s%s", trust, desc),
				})
			}
			renderCommandPanel(command.OutOrStdout(), "Agents", rows)
			return nil
		},
	}
}

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// KeysCmd returns the `nebula keys` command group.
func KeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "Manage API keys",
	}
	cmd.AddCommand(keysListCmd())
	cmd.AddCommand(keysCreateCmd())
	cmd.AddCommand(keysRevokeCmd())
	return cmd
}

// keysListCmd handles keys list cmd.
func keysListCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List API keys",
		RunE: func(command *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}
			client := api.NewDefaultClient(cfg.APIKey)

			var keys []api.APIKey
			if all {
				keys, err = client.ListAllKeys()
			} else {
				keys, err = client.ListKeys()
			}
			if err != nil {
				return fmt.Errorf("list keys: %w", err)
			}

			if len(keys) == 0 {
				renderCommandMessage(command.OutOrStdout(), "API Keys", "No keys found.")
				return nil
			}

			rows := make([]components.TableRow, 0, len(keys))
			for _, k := range keys {
				owner := k.Name
				if k.OwnerType == "agent" && k.AgentName != nil {
					owner = fmt.Sprintf("agent:%s", *k.AgentName)
				} else if k.EntityName != nil {
					owner = fmt.Sprintf("user:%s", *k.EntityName)
				}
				lastUsed := "never"
				if k.LastUsedAt != nil {
					lastUsed = k.LastUsedAt.Format("2006-01-02 15:04")
				}
				rows = append(rows, components.TableRow{
					Label: k.KeyPrefix + "...",
					Value: fmt.Sprintf("%s (%s) · last used %s", k.Name, owner, lastUsed),
				})
			}
			renderCommandPanel(command.OutOrStdout(), "API Keys", rows)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "show all keys (user + agent)")
	return cmd
}

// keysCreateCmd handles keys create cmd.
func keysCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}
			client := api.NewDefaultClient(cfg.APIKey)

			resp, err := client.CreateKey(args[0])
			if err != nil {
				return fmt.Errorf("create key: %w", err)
			}

			renderCommandPanel(command.OutOrStdout(), "API Key Created", []components.TableRow{
				{Label: "name", Value: resp.Name},
				{Label: "api_key", Value: resp.APIKey},
				{Label: "note", Value: "save this key now, it is not shown again"},
			})
			return nil
		},
	}
}

// keysRevokeCmd handles keys revoke cmd.
func keysRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <key-id>",
		Short: "Revoke an API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("not logged in: %w", err)
			}
			client := api.NewDefaultClient(cfg.APIKey)

			if err := client.RevokeKey(args[0]); err != nil {
				return fmt.Errorf("revoke key: %w", err)
			}

			renderCommandPanel(command.OutOrStdout(), "API Keys", []components.TableRow{
				{Label: "status", Value: "revoked"},
				{Label: "key_id", Value: args[0]},
			})
			return nil
		},
	}
}

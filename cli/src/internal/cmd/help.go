package cmd

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// ApplyNebulaHelp replaces default cobra help/usage output with Nebula boxed rendering.
func ApplyNebulaHelp(root *cobra.Command) {
	if root == nil {
		return
	}

	applyHelpRecursively(root)

	root.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "Show help for command",
		Hidden: true,
		RunE: func(command *cobra.Command, args []string) error {
			target, _, err := command.Root().Find(args)
			if err != nil || target == nil {
				target = command.Root()
			}
			renderNebulaHelp(command.OutOrStdout(), target)
			return nil
		},
	})
}

// applyHelpRecursively handles apply help recursively.
func applyHelpRecursively(command *cobra.Command) {
	command.SetHelpFunc(func(c *cobra.Command, _ []string) {
		renderNebulaHelp(c.OutOrStdout(), c)
	})
	command.SetUsageFunc(func(c *cobra.Command) error {
		renderNebulaHelp(c.OutOrStdout(), c)
		return nil
	})
	for _, child := range command.Commands() {
		applyHelpRecursively(child)
	}
}

// renderNebulaHelp renders render nebula help.
func renderNebulaHelp(out io.Writer, command *cobra.Command) {
	rows := []components.TableRow{
		{Label: "command", Value: command.CommandPath()},
		{Label: "usage", Value: command.UseLine()},
	}

	desc := strings.TrimSpace(command.Long)
	if desc == "" {
		desc = strings.TrimSpace(command.Short)
	}
	if desc != "" {
		rows = append(rows, components.TableRow{Label: "about", Value: desc})
	}

	aliases := strings.Join(command.Aliases, ", ")
	if aliases != "" {
		rows = append(rows, components.TableRow{Label: "aliases", Value: aliases})
	}

	subcommands := visibleSubcommands(command)
	if len(subcommands) > 0 {
		rows = append(rows, components.TableRow{Label: "subcommands", Value: fmt.Sprintf("%d available", len(subcommands))})
		for _, sub := range subcommands {
			summary := strings.TrimSpace(sub.Short)
			if summary == "" {
				summary = "-"
			}
			subPath := sub.CommandPath()
			if subPath == "" {
				subPath = sub.Name()
			}
			rows = append(rows, components.TableRow{
				Label: "  " + subPath,
				Value: summary,
			})
		}
	}

	flags := visibleFlags(command)
	if len(flags) > 0 {
		rows = append(rows, components.TableRow{Label: "flags", Value: fmt.Sprintf("%d available", len(flags))})
		rows = append(rows, flags...)
	}

	rows = append(rows, components.TableRow{
		Label: "tip",
		Value: "use `nebula <command> --help` for command details",
	})
	rows = append(rows, helpRecipeRows(command)...)
	renderCommandPanel(out, "Help", rows)
}

// helpRecipeRows adds explicit shell examples so non-interactive usage is obvious.
func helpRecipeRows(command *cobra.Command) []components.TableRow {
	if command == nil {
		return nil
	}
	path := command.CommandPath()
	recipes, ok := helpRecipeCatalog()[path]
	if !ok && command.Parent() != nil {
		parent := command.Parent().CommandPath()
		if parentRecipes, hasParent := helpRecipeCatalog()[parent]; hasParent {
			recipes = []string{
				command.CommandPath() + " --help",
				parentRecipes[0],
			}
			ok = true
		}
	}
	if !ok {
		recipes = []string{command.CommandPath() + " --help"}
	}
	rows := make([]components.TableRow, 0, len(recipes)+1)
	rows = append(rows, components.TableRow{Label: "recipes", Value: "quick examples"})
	for idx, recipe := range recipes {
		rows = append(rows, components.TableRow{
			Label: fmt.Sprintf("  %d", idx+1),
			Value: recipe,
		})
	}
	return rows
}

// helpRecipeCatalog keeps command help concrete for terminal-first use cases.
func helpRecipeCatalog() map[string][]string {
	return map[string][]string{
		"nebula": {
			"nebula doctor",
			"nebula api health --output plain",
			"nebula start | nebula logs --api | nebula stop",
		},
		"nebula doctor": {
			"nebula doctor --output table",
			"nebula doctor --output json",
			"nebula doctor --plain",
		},
		"nebula api": {
			"nebula api entities query --param limit=5 --output json",
			"nebula api approvals diff <approval-id> --only changed --output table",
			"nebula api approvals diff <approval-id> --only section=metadata --max-lines 4 --plain",
		},
		"nebula api entities": {
			"nebula api entities query --param limit=10 --output table",
			"nebula api entities create --input-file ./entity.json --output json",
			"nebula api entities update <entity-id> --input '{\"status\":\"inactive\"}' --plain",
		},
		"nebula api context": {
			"nebula api context query --param limit=10 --output table",
			"nebula api context create --input-file ./context.json --output json",
			"nebula api context link <context-id> --owner-type entity --owner-id <entity-id>",
		},
		"nebula api relationships": {
			"nebula api relationships query --param limit=10 --output table",
			"nebula api relationships create --input-file ./relationship.json --output json",
			"nebula api relationships for-source entity <entity-id>",
		},
		"nebula api jobs": {
			"nebula api jobs query --param limit=10 --output table",
			"nebula api jobs create --input-file ./job.json",
			"nebula api jobs set-status <job-id> --status completed",
		},
		"nebula api logs": {
			"nebula api logs query --param limit=10 --output table",
			"nebula api logs create --input-file ./log.json",
			"nebula logs --api --tail 100",
		},
		"nebula api files": {
			"nebula api files query --param limit=10 --output table",
			"nebula api files create --input-file ./file.json",
			"nebula api files update <file-id> --input '{\"metadata\":{\"status\":\"indexed\"}}'",
		},
		"nebula api protocols": {
			"nebula api protocols query --param limit=10 --output table",
			"nebula api protocols create --input-file ./protocol.json",
			"nebula api protocols get <protocol-name> --plain",
		},
		"nebula api approvals": {
			"nebula api approvals pending --limit 20 --output table",
			"nebula api approvals diff <approval-id> --only changed --only section=core --output table",
			"nebula api approvals approve <approval-id> --input '{\"review_notes\":\"ok\"}'",
		},
		"nebula api agents": {
			"nebula api agents list --status-category active --output table",
			"nebula api agents register --input-file ./agent-register.json",
			"nebula api agents update <agent-id> --input-file ./agent-update.json",
		},
		"nebula api keys": {
			"nebula api keys list --output table",
			"nebula api keys create ci-key --output json",
			"nebula api keys revoke <key-id>",
		},
		"nebula api audit": {
			"nebula api audit query --param limit=50 --output table",
			"nebula api audit scopes",
			"nebula api audit actors --output json",
		},
		"nebula api taxonomy": {
			"nebula api taxonomy list scopes --limit 50 --output table",
			"nebula api taxonomy create scopes --input-file ./scope.json",
			"nebula api taxonomy update scopes <id> --input '{\"description\":\"updated\"}'",
		},
		"nebula api search": {
			"nebula api search semantic --query \"approval diff\" --limit 10",
		},
		"nebula api import": {
			"nebula api import entities --input-file ./entities.json",
			"nebula api import jobs --input-file ./jobs.json",
		},
		"nebula api export": {
			"nebula api export entities --param limit=100 --output json",
			"nebula api export snapshot --param format=json --plain",
		},
		"nebula start": {
			"nebula doctor",
			"nebula start",
			"nebula logs --api --tail 100",
		},
		"nebula stop": {
			"nebula stop",
			"nebula start",
		},
		"nebula logs": {
			"nebula logs --api --tail 100",
			"nebula logs --server --tail 200 --plain",
		},
		"nebula login": {
			"nebula login",
			"nebula doctor",
		},
		"nebula keys": {
			"nebula keys list",
			"nebula keys create ci-key",
			"nebula keys revoke <key-id>",
		},
		"nebula agent": {
			"nebula agent register --name codex-agent",
			"nebula agent list",
			"nebula agent update <agent-id>",
		},
	}
}

// visibleSubcommands handles visible subcommands.
func visibleSubcommands(command *cobra.Command) []*cobra.Command {
	list := make([]*cobra.Command, 0)
	for _, sub := range command.Commands() {
		if !sub.IsAvailableCommand() || sub.Hidden {
			continue
		}
		list = append(list, sub)
	}
	sort.SliceStable(list, func(i, j int) bool {
		return strings.ToLower(list[i].Name()) < strings.ToLower(list[j].Name())
	})
	return list
}

// visibleFlags handles visible flags.
func visibleFlags(command *cobra.Command) []components.TableRow {
	flagRows := make([]components.TableRow, 0, 8)
	seen := make(map[string]struct{})
	collect := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			key := flag.Name
			if _, ok := seen[key]; ok {
				return
			}
			seen[key] = struct{}{}
			name := "--" + flag.Name
			if flag.Shorthand != "" {
				name = "-" + flag.Shorthand + ", " + name
			}
			flagRows = append(flagRows, components.TableRow{
				Label: "  " + name,
				Value: strings.TrimSpace(flag.Usage),
			})
		})
	}
	collect(command.NonInheritedFlags())
	collect(command.InheritedFlags())
	sort.SliceStable(flagRows, func(i, j int) bool {
		return strings.ToLower(flagRows[i].Label) < strings.ToLower(flagRows[j].Label)
	})
	return flagRows
}

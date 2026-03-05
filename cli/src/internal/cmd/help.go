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
	if command.Parent() != nil {
		return []components.TableRow{
			{Label: "example", Value: command.CommandPath() + " --help"},
		}
	}
	return []components.TableRow{
		{Label: "recipes", Value: "quick examples"},
		{Label: "  1", Value: "nebula api entities query --param limit=5"},
		{Label: "  2", Value: "nebula api approvals pending"},
		{Label: "  3", Value: "nebula api approvals diff <approval-id>"},
		{Label: "  4", Value: "nebula start | nebula logs --api | nebula stop"},
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

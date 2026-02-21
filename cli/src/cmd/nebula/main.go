package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/cmd"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui"
)

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "nebula",
		Short: "Nebula - agent context layer",
		Long:  "Nebula CLI: manage entities, approve agent actions, add context, and monitor jobs.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runTUI()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(cmd.LoginCmd())
	root.AddCommand(cmd.AgentCmd())
	root.AddCommand(cmd.KeysCmd())
	root.AddCommand(cmd.StartCmd())
	root.AddCommand(cmd.StopCmd())
	root.AddCommand(cmd.LogsCmd())
	cmd.ApplyNebulaHelp(root)

	return root
}

func init() {
	// Force truecolor so hex colors render correctly
	// Must be set before any lipgloss style initialization
	os.Setenv("COLORTERM", "truecolor")
}

func runTUI() error {
	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if !isInteractiveTerminal(os.Stdin) || !isInteractiveTerminal(os.Stdout) {
				fmt.Println("not logged in. run 'nebula login' first.")
				return err
			}
			cfg = nil
		} else {
			return err
		}
	}

	apiKey := ""
	if cfg != nil {
		apiKey = cfg.APIKey
	}
	client := api.NewDefaultClient(apiKey)
	app := ui.NewApp(client, cfg)

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui error: %w", err)
	}
	return nil
}

func isInteractiveTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

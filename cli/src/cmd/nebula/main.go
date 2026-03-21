package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/cmd"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui"
)

var runBubbleTUI = func(app tea.Model) error {
	p := tea.NewProgram(app)
	_, err := p.Run()
	return err
}

// main runs the CLI entrypoint.
func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// newRootCommand handles new root command.
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
	root.AddCommand(cmd.DoctorCmd())
	root.AddCommand(cmd.APICmd())
	cmd.AttachOutputFlags(root, cmd.OutputModeAuto)
	cmd.ApplyNebulaHelp(root)

	return root
}

// init initializes package defaults.
func init() {
	// Force truecolor so hex colors render correctly
	// Must be set before any lipgloss style initialization
	_ = os.Setenv("COLORTERM", "truecolor")
}

// runTUI runs run tui.
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

	if err := runBubbleTUI(app); err != nil {
		return fmt.Errorf("tui error: %w", err)
	}
	return nil
}

// isInteractiveTerminal handles is interactive terminal.
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

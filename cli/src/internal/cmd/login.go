package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// RunInteractiveLogin prompts for username, calls login API, and persists config.
func RunInteractiveLogin(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)

	renderCommandMessage(out, "Nebula Login", "Enter username to create or resume your local session.")
	if _, err := fmt.Fprint(out, "\nusername: "); err != nil {
		return fmt.Errorf("write prompt: %w", err)
	}
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	if username == "" {
		return fmt.Errorf("username is required")
	}

	client := newDefaultClient("")
	resp, err := client.Login(username)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	cfg := &config.Config{
		APIKey:            resp.APIKey,
		UserEntityID:      resp.EntityID,
		Username:          resp.Username,
		Theme:             "dark",
		VimKeys:           true,
		QuickstartPending: true,
		PendingLimit:      500,
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	renderCommandPanel(out, "Login Success", []components.TableRow{
		{Label: "username", Value: resp.Username},
		{Label: "entity_id", Value: resp.EntityID},
		{Label: "api_url", Value: api.DefaultBaseURL},
		{Label: "config", Value: config.Path()},
	})
	return nil
}

// LoginCmd returns the `nebula login` command.
func LoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with a Nebula server",
		RunE: func(_ *cobra.Command, _ []string) error {
			return RunInteractiveLogin(os.Stdin, os.Stdout)
		},
	}
}

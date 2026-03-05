package cmd

import (
	"bytes"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderNebulaHelpShowsCommandSpecificRecipes(t *testing.T) {
	root := &cobra.Command{Use: "nebula", Short: "root"}
	apiCmd := &cobra.Command{Use: "api", Short: "api"}
	approvals := &cobra.Command{Use: "approvals", Short: "approvals"}
	apiCmd.AddCommand(approvals)
	root.AddCommand(apiCmd)

	var out bytes.Buffer
	renderNebulaHelp(&out, approvals)
	text := components.SanitizeText(out.String())

	assert.Contains(t, text, "recipes")
	assert.Contains(t, text, "nebula api approvals pending --limit 20 --output table")
	assert.Contains(t, text, "nebula api approvals diff <approval-id> --only changed --only section=core --output table")
}

func TestHelpRecipeRowsFallsBackToParentCatalog(t *testing.T) {
	root := &cobra.Command{Use: "nebula", Short: "root"}
	apiCmd := &cobra.Command{Use: "api", Short: "api"}
	custom := &cobra.Command{Use: "custom", Short: "custom"}
	apiCmd.AddCommand(custom)
	root.AddCommand(apiCmd)

	rows := helpRecipeRows(custom)
	require.NotEmpty(t, rows)

	joined := ""
	for _, row := range rows {
		joined += row.Value + "\n"
	}
	assert.Contains(t, joined, "nebula api custom --help")
	assert.Contains(t, joined, "nebula api entities query --param limit=5 --output json")
}

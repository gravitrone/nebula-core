package cmd

import (
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVisibleFlagsSkipsDuplicateAndHiddenEntries(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("dup", "", "persistent duplicate")
	root.Flags().String("dup", "", "local duplicate")
	root.Flags().String("hidden-flag", "", "should be hidden")
	require.NoError(t, root.Flags().MarkHidden("hidden-flag"))

	rows := visibleFlags(root)
	require.NotEmpty(t, rows)

	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.Label)
	}
	joined := strings.ToLower(components.SanitizeText(strings.Join(labels, " ")))
	assert.Equal(t, 1, strings.Count(joined, "--dup"))
	assert.NotContains(t, joined, "--hidden-flag")
}

func TestVisibleSubcommandsSkipsUnavailableCommand(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(&cobra.Command{Use: "available", RunE: func(*cobra.Command, []string) error { return nil }})
	root.AddCommand(&cobra.Command{Use: "unavailable"})

	subs := visibleSubcommands(root)
	require.Len(t, subs, 1)
	assert.Equal(t, "available", subs[0].Name())
}

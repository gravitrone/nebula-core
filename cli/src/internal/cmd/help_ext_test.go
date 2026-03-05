package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderNebulaHelpIncludesLongAliasesAndEmptySummaryFallback(t *testing.T) {
	root := &cobra.Command{
		Use:     "nebula",
		Short:   "short desc",
		Long:    "long form description",
		Aliases: []string{"nb", "n"},
	}
	root.Flags().String("no-sh", "", "  long flag usage  ")
	require.NoError(t, root.Flags().MarkHidden("no-sh"))

	child := &cobra.Command{
		Use: "child",
		Run: func(*cobra.Command, []string) {},
	}
	root.AddCommand(child)

	var out bytes.Buffer
	renderNebulaHelp(&out, root)
	text := out.String()

	assert.Contains(t, text, "long form description")
	assert.Contains(t, text, "nb, n")
	assert.Contains(t, text, "subcommands")
	assert.Contains(t, text, "nebula child")
	assert.Contains(t, text, "  -")
}

func TestVisibleFlagsLongOnlyAndTrimmedUsage(t *testing.T) {
	cmd := &cobra.Command{Use: "cmd"}
	cmd.Flags().String("output", "", "  output path  ")
	cmd.Flags().Bool("debug", false, "debug mode")
	require.NoError(t, cmd.Flags().MarkHidden("debug"))

	rows := visibleFlags(cmd)
	require.Len(t, rows, 1)
	assert.Equal(t, "  --output", rows[0].Label)
	assert.Equal(t, "output path", rows[0].Value)
}

func TestVisibleFlagsDedupesInheritedAndLocalByName(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("config", "", "root config")

	child := &cobra.Command{Use: "child", Run: func(*cobra.Command, []string) {}}
	child.Flags().String("config", "", "child config")
	root.AddCommand(child)

	rows := visibleFlags(child)
	configCount := 0
	for _, row := range rows {
		if row.Label == "  --config" {
			configCount++
		}
	}
	assert.Equal(t, 1, configCount)
}

func TestVisibleFlagsSkipsDuplicateWhenPresentInBothFlagSets(t *testing.T) {
	cmd := &cobra.Command{Use: "cmd"}
	cmd.Flags().String("dup", "", "duplicate guard")

	dup := cmd.Flags().Lookup("dup")
	require.NotNil(t, dup)
	cmd.InheritedFlags().AddFlag(dup)

	rows := visibleFlags(cmd)
	dupCount := 0
	for _, row := range rows {
		if row.Label == "  --dup" {
			dupCount++
		}
	}
	assert.Equal(t, 1, dupCount)
}

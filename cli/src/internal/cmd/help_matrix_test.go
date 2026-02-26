package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyNebulaHelpHandlesNilCommand(t *testing.T) {
	assert.NotPanics(t, func() {
		ApplyNebulaHelp(nil)
	})
}

func TestApplyNebulaHelpInstallsHiddenHelpCommand(t *testing.T) {
	root := &cobra.Command{Use: "root", RunE: func(*cobra.Command, []string) error { return nil }}
	child := &cobra.Command{Use: "child", Short: "Child", RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(child)
	ApplyNebulaHelp(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"help"})
	require.NoError(t, root.Execute())
	assert.Contains(t, components.SanitizeText(out.String()), "[ Help ]")
}

func TestApplyNebulaHelpUnknownTargetFallsBackToRoot(t *testing.T) {
	root := &cobra.Command{Use: "root", Short: "Root command"}
	root.RunE = func(*cobra.Command, []string) error { return nil }
	root.AddCommand(&cobra.Command{
		Use:   "child",
		Short: "Child command",
		RunE:  func(*cobra.Command, []string) error { return nil },
	})
	ApplyNebulaHelp(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"help", "does-not-exist"})
	require.NoError(t, root.Execute())

	text := components.SanitizeText(out.String())
	assert.Contains(t, text, "[ Help ]")
	assert.Contains(t, text, "root")
}

func TestApplyHelpRecursivelyOverridesChildUsage(t *testing.T) {
	root := &cobra.Command{Use: "root", Short: "Root", RunE: func(*cobra.Command, []string) error { return nil }}
	child := &cobra.Command{Use: "child", Short: "Child", RunE: func(*cobra.Command, []string) error { return nil }}
	leaf := &cobra.Command{Use: "leaf", Short: "Leaf", RunE: func(*cobra.Command, []string) error { return nil }}
	child.AddCommand(leaf)
	root.AddCommand(child)

	applyHelpRecursively(root)

	var out bytes.Buffer
	child.SetOut(&out)
	require.NoError(t, child.Usage())

	text := components.SanitizeText(out.String())
	assert.Contains(t, text, "[ Help ]")
	assert.Contains(t, text, "root child")
}

func TestVisibleSubcommandsFiltersHiddenAndSorts(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.AddCommand(&cobra.Command{Use: "zeta", Short: "z", RunE: func(*cobra.Command, []string) error { return nil }})
	root.AddCommand(&cobra.Command{Use: "alpha", Short: "a", RunE: func(*cobra.Command, []string) error { return nil }})
	root.AddCommand(&cobra.Command{Use: "hidden", Hidden: true, Short: "h", RunE: func(*cobra.Command, []string) error { return nil }})

	subs := visibleSubcommands(root)
	require.Len(t, subs, 2)
	assert.Equal(t, "alpha", subs[0].Name())
	assert.Equal(t, "zeta", subs[1].Name())
}

func TestVisibleFlagsCollectsAndSortsRows(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().StringP("verbose", "v", "", "Verbose mode")
	root.Flags().StringP("config", "c", "", "Config path")
	root.Flags().String("alpha", "", "Alpha flag")

	rows := visibleFlags(root)
	require.GreaterOrEqual(t, len(rows), 3)

	joined := strings.ToLower(components.SanitizeText(rows[0].Label + rows[1].Label + rows[2].Label))
	assert.Contains(t, joined, "--alpha")
	assert.Contains(t, strings.ToLower(components.SanitizeText(strings.Join([]string{
		rows[0].Label, rows[1].Label, rows[2].Label,
	}, " "))), "--config")
	assert.Contains(t, strings.ToLower(components.SanitizeText(strings.Join([]string{
		rows[0].Label, rows[1].Label, rows[2].Label,
	}, " "))), "--verbose")
}

func TestRenderNebulaHelpUsesShortWhenLongMissing(t *testing.T) {
	command := &cobra.Command{
		Use:   "child",
		Short: "Short only description",
		RunE:  func(*cobra.Command, []string) error { return nil },
	}

	var out bytes.Buffer
	renderNebulaHelp(&out, command)
	text := components.SanitizeText(out.String())

	assert.Contains(t, strings.ToLower(text), "short only description")
	assert.Contains(t, text, "tip")
}

func TestRenderNebulaHelpShowsAliasesAndSubcommands(t *testing.T) {
	root := &cobra.Command{
		Use:     "root",
		Long:    "Root command",
		Aliases: []string{"r", "rt"},
		RunE:    func(*cobra.Command, []string) error { return nil },
	}
	root.AddCommand(&cobra.Command{
		Use:   "child",
		Short: "Child command",
		RunE:  func(*cobra.Command, []string) error { return nil },
	})

	var out bytes.Buffer
	renderNebulaHelp(&out, root)
	text := strings.ToLower(components.SanitizeText(out.String()))

	assert.Contains(t, text, "aliases")
	assert.Contains(t, text, "r, rt")
	assert.Contains(t, text, "subcommands")
	assert.Contains(t, text, "/child")
}

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// OutputMode controls non-interactive command rendering.
type OutputMode string

const (
	OutputModeAuto  OutputMode = "auto"
	OutputModeJSON  OutputMode = "json"
	OutputModeTable OutputMode = "table"
	OutputModePlain OutputMode = "plain"
)

const outputModeEnv = "NEBULA_OUTPUT_MODE"

// AttachOutputFlags wires --output/--plain and configures mode before command execution.
func AttachOutputFlags(command *cobra.Command, defaultMode OutputMode) {
	if command == nil {
		return
	}

	var output string
	var plain bool
	command.PersistentFlags().StringVar(
		&output,
		"output",
		string(OutputModeAuto),
		"output mode: auto|json|table|plain",
	)
	command.PersistentFlags().BoolVar(&plain, "plain", false, "alias for --output plain")

	prev := command.PersistentPreRunE
	command.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if prev != nil {
			if err := prev(cmd, args); err != nil {
				return err
			}
		}
		return configureOutputMode(output, plain, defaultMode)
	}
}

// resolveOutputMode returns the active mode, defaulting when no explicit mode was set.
func resolveOutputMode(defaultMode OutputMode) OutputMode {
	if parsed, ok := parseOutputMode(os.Getenv(outputModeEnv)); ok && parsed != OutputModeAuto {
		return parsed
	}
	if defaultMode == "" || defaultMode == OutputModeAuto {
		return OutputModeTable
	}
	return defaultMode
}

// configureOutputMode validates and stores output mode in process env for command execution.
func configureOutputMode(raw string, plain bool, defaultMode OutputMode) error {
	if plain {
		raw = string(OutputModePlain)
	}

	mode, ok := parseOutputMode(raw)
	if !ok {
		return fmt.Errorf("invalid --output value %q (expected auto|json|table|plain)", raw)
	}
	return os.Setenv(outputModeEnv, string(mode))
}

// parseOutputMode parses raw mode values with safe fallback behavior.
func parseOutputMode(raw string) (OutputMode, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(OutputModeAuto):
		return OutputModeAuto, true
	case string(OutputModeJSON):
		return OutputModeJSON, true
	case string(OutputModeTable):
		return OutputModeTable, true
	case string(OutputModePlain):
		return OutputModePlain, true
	default:
		return OutputModeAuto, false
	}
}

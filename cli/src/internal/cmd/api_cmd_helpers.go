package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/gravitrone/nebula-core/cli/internal/config"
	"github.com/gravitrone/nebula-core/cli/internal/ui/components"
)

// loadCommandClient builds an API client for non-interactive command flows.
func loadCommandClient(requireAuth bool) (*api.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		if requireAuth {
			return nil, fmt.Errorf("not logged in: %w", err)
		}
		return newDefaultClient(""), nil
	}
	return newDefaultClient(cfg.APIKey), nil
}

// writeCleanJSON renders predictable command output without banners.
func writeCleanJSON(out io.Writer, value any) error {
	if value == nil {
		value = map[string]any{"ok": true}
	}
	switch resolveOutputMode(OutputModeJSON) {
	case OutputModePlain:
		return encodeJSON(out, value, false)
	case OutputModeTable:
		return writeTableOutput(out, value)
	default:
		return encodeJSON(out, value, true)
	}
}

// encodeJSON handles JSON encoding with optional indentation.
func encodeJSON(out io.Writer, value any, pretty bool) error {
	enc := json.NewEncoder(out)
	if pretty {
		enc.SetIndent("", "  ")
	}
	enc.SetEscapeHTML(false)
	return enc.Encode(value)
}

// writeTableOutput renders API responses as concise table-style command panels.
func writeTableOutput(out io.Writer, value any) error {
	normalized, err := normalizeOutputValue(value)
	if err != nil {
		return err
	}

	switch typed := normalized.(type) {
	case map[string]any:
		rows := mapRows(typed)
		if len(rows) == 0 {
			renderCommandMessage(out, "Result", "ok")
			return nil
		}
		renderCommandPanel(out, "Result", rows)
		return nil
	case []any:
		rows := make([]components.TableRow, 0, len(typed)+1)
		rows = append(rows, components.TableRow{
			Label: "count",
			Value: strconv.Itoa(len(typed)),
		})
		for idx, item := range typed {
			rows = append(rows, components.TableRow{
				Label: fmt.Sprintf("[%d]", idx),
				Value: formatOutputValue(item),
			})
		}
		renderCommandPanel(out, "Result", rows)
		return nil
	default:
		renderCommandMessage(out, "Result", formatOutputValue(typed))
		return nil
	}
}

// normalizeOutputValue normalizes typed responses into map or slice structures.
func normalizeOutputValue(value any) (any, error) {
	if value == nil {
		return map[string]any{"ok": true}, nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode output: %w", err)
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, fmt.Errorf("decode output: %w", err)
	}
	return normalized, nil
}

// mapRows renders a deterministic key/value view for map outputs.
func mapRows(data map[string]any) []components.TableRow {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rows := make([]components.TableRow, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, components.TableRow{
			Label: key,
			Value: formatOutputValue(data[key]),
		})
	}
	return rows
}

// formatOutputValue compacts structured values while keeping scalar values readable.
func formatOutputValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		v := strings.TrimSpace(typed)
		if v == "" {
			return "-"
		}
		return components.SanitizeText(v)
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case json.Number:
		return typed.String()
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return components.SanitizeText(fmt.Sprintf("%v", typed))
		}
		return components.SanitizeText(string(raw))
	}
}

// parseQueryParams converts repeated --param key=value flags into query params.
func parseQueryParams(raw []string) (api.QueryParams, error) {
	params := api.QueryParams{}
	for _, item := range raw {
		key, value, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, fmt.Errorf("invalid --param value %q (expected key=value)", item)
		}
		params[key] = strings.TrimSpace(value)
	}
	return params, nil
}

// readInputJSON reads JSON payload from --input or --input-file.
func readInputJSON(input string, inputFile string, required bool) (json.RawMessage, error) {
	if strings.TrimSpace(input) != "" && strings.TrimSpace(inputFile) != "" {
		return nil, fmt.Errorf("use either --input or --input-file, not both")
	}

	switch {
	case strings.TrimSpace(input) != "":
		raw := []byte(strings.TrimSpace(input))
		if !json.Valid(raw) {
			return nil, fmt.Errorf("invalid JSON passed to --input")
		}
		return raw, nil
	case strings.TrimSpace(inputFile) != "":
		raw, err := os.ReadFile(strings.TrimSpace(inputFile))
		if err != nil {
			return nil, fmt.Errorf("read input file: %w", err)
		}
		raw = []byte(strings.TrimSpace(string(raw)))
		if !json.Valid(raw) {
			return nil, fmt.Errorf("invalid JSON in --input-file")
		}
		return raw, nil
	default:
		if required {
			return nil, fmt.Errorf("missing input: pass --input '<json>' or --input-file <path>")
		}
		return nil, nil
	}
}

package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gravitrone/nebula-core/cli/internal/api"
)

// APICmd exposes a non-interactive command suite for full API/MCP-style operations.
func APICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api",
		Short: "Non-interactive API command suite (json|table|plain output)",
		Long: strings.TrimSpace(
			`Run Nebula operations without opening the TUI.
Use --output json|table|plain (or --plain) for predictable output formatting.
Use --input or --input-file for create/update actions, and --param key=value for query filters.`,
		),
	}

	cmd.AddCommand(apiHealthCmd())
	cmd.AddCommand(apiEntitiesCmd())
	cmd.AddCommand(apiContextCmd())
	cmd.AddCommand(apiRelationshipsCmd())
	cmd.AddCommand(apiJobsCmd())
	cmd.AddCommand(apiLogsResourceCmd())
	cmd.AddCommand(apiFilesCmd())
	cmd.AddCommand(apiProtocolsCmd())
	cmd.AddCommand(apiApprovalsCmd())
	cmd.AddCommand(apiAgentsCmd())
	cmd.AddCommand(apiKeysCmd())
	cmd.AddCommand(apiAuditCmd())
	cmd.AddCommand(apiTaxonomyCmd())
	cmd.AddCommand(apiSearchCmd())
	cmd.AddCommand(apiImportsCmd())
	cmd.AddCommand(apiExportsCmd())

	return cmd
}

func decodeJSONInput(raw json.RawMessage, dst any) error {
	if len(raw) == 0 {
		return fmt.Errorf("missing JSON input")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("parse JSON input: %w", err)
	}
	return nil
}

func bindInputFlags(cmd *cobra.Command, input *string, inputFile *string) {
	cmd.Flags().StringVar(input, "input", "", "inline JSON payload")
	cmd.Flags().StringVar(inputFile, "input-file", "", "path to JSON payload file")
}

func bindParamFlags(cmd *cobra.Command, rawParams *[]string) {
	cmd.Flags().StringArrayVar(rawParams, "param", nil, "query parameter key=value (repeat)")
}

func apiHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check API health",
		RunE: func(command *cobra.Command, _ []string) error {
			client, _ := loadCommandClient(false)
			status, err := client.Health()
			if err != nil {
				return fmt.Errorf("health check: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), map[string]any{"status": status})
		},
	}
}

func apiEntitiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entities",
		Short: "Entity operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query entities",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryEntities(params)
			if err != nil {
				return fmt.Errorf("query entities: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Get entity by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetEntity(args[0])
			if err != nil {
				return fmt.Errorf("get entity: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create entity from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateEntityInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateEntity(payload)
			if err != nil {
				return fmt.Errorf("create entity: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <id>",
		Short: "Update entity from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateEntityInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateEntity(args[0], payload)
			if err != nil {
				return fmt.Errorf("update entity: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	var historyLimit int
	var historyOffset int
	history := &cobra.Command{
		Use:   "history <id>",
		Short: "Get entity audit history",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.GetEntityHistory(args[0], historyLimit, historyOffset)
			if err != nil {
				return fmt.Errorf("entity history: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	history.Flags().IntVar(&historyLimit, "limit", 50, "max history rows")
	history.Flags().IntVar(&historyOffset, "offset", 0, "history offset")

	var revertAuditID string
	revert := &cobra.Command{
		Use:   "revert <id>",
		Short: "Revert entity to a specific audit entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			revertAuditID = strings.TrimSpace(revertAuditID)
			if revertAuditID == "" {
				return fmt.Errorf("missing --audit-id")
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.RevertEntity(args[0], revertAuditID)
			if err != nil {
				return fmt.Errorf("revert entity: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	revert.Flags().StringVar(&revertAuditID, "audit-id", "", "audit entry id to restore")

	var bulkTagsInput string
	var bulkTagsInputFile string
	bulkTags := &cobra.Command{
		Use:   "bulk-tags",
		Short: "Bulk update entity tags from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(bulkTagsInput, bulkTagsInputFile, true)
			if err != nil {
				return err
			}
			var payload api.BulkUpdateEntityTagsInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			result, err := client.BulkUpdateEntityTags(payload)
			if err != nil {
				return fmt.Errorf("bulk update tags: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), result)
		},
	}
	bindInputFlags(bulkTags, &bulkTagsInput, &bulkTagsInputFile)

	var bulkScopesInput string
	var bulkScopesInputFile string
	bulkScopes := &cobra.Command{
		Use:   "bulk-scopes",
		Short: "Bulk update entity scopes from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(bulkScopesInput, bulkScopesInputFile, true)
			if err != nil {
				return err
			}
			var payload api.BulkUpdateEntityScopesInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			result, err := client.BulkUpdateEntityScopes(payload)
			if err != nil {
				return fmt.Errorf("bulk update scopes: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), result)
		},
	}
	bindInputFlags(bulkScopes, &bulkScopesInput, &bulkScopesInputFile)

	cmd.AddCommand(query, get, create, update, history, revert, bulkTags, bulkScopes)
	return cmd
}

func apiContextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context",
		Short: "Context item operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query context items",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryContext(params)
			if err != nil {
				return fmt.Errorf("query context: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Get context item by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetContext(args[0])
			if err != nil {
				return fmt.Errorf("get context: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create context item from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateContextInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateContext(payload)
			if err != nil {
				return fmt.Errorf("create context: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <id>",
		Short: "Update context item from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateContextInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateContext(args[0], payload)
			if err != nil {
				return fmt.Errorf("update context: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	var entityID string
	var ownerType string
	var ownerID string
	link := &cobra.Command{
		Use:   "link <context-id>",
		Short: "Link context item to an owner",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			entityID = strings.TrimSpace(entityID)
			ownerType = strings.TrimSpace(ownerType)
			ownerID = strings.TrimSpace(ownerID)
			if ownerID == "" && entityID != "" {
				ownerType = "entity"
				ownerID = entityID
			}
			if ownerType == "" {
				ownerType = "entity"
			}
			if ownerID == "" {
				return fmt.Errorf("missing --owner-id")
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			if err := client.LinkContext(args[0], api.LinkContextInput{
				OwnerType: ownerType,
				OwnerID:   ownerID,
			}); err != nil {
				return fmt.Errorf("link context: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), map[string]any{
				"context_id": args[0],
				"owner_type": ownerType,
				"owner_id":   ownerID,
				"linked":     true,
			})
		},
	}
	link.Flags().StringVar(&ownerType, "owner-type", "entity", "owner type (entity or job)")
	link.Flags().StringVar(&ownerID, "owner-id", "", "owner id")
	link.Flags().StringVar(&entityID, "entity-id", "", "deprecated alias for --owner-id (entity)")

	cmd.AddCommand(query, get, create, update, link)
	return cmd
}

func apiRelationshipsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relationships",
		Short: "Relationship operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query relationships",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryRelationships(params)
			if err != nil {
				return fmt.Errorf("query relationships: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	forSource := &cobra.Command{
		Use:   "for-source <source-type> <source-id>",
		Short: "Get relationships connected to one source node",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.GetRelationships(args[0], args[1])
			if err != nil {
				return fmt.Errorf("get relationships: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create relationship from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateRelationshipInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateRelationship(payload)
			if err != nil {
				return fmt.Errorf("create relationship: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <relationship-id>",
		Short: "Update relationship from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateRelationshipInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateRelationship(args[0], payload)
			if err != nil {
				return fmt.Errorf("update relationship: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	cmd.AddCommand(query, forSource, create, update)
	return cmd
}

func apiJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Job operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query jobs",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryJobs(params)
			if err != nil {
				return fmt.Errorf("query jobs: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Get job by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetJob(args[0])
			if err != nil {
				return fmt.Errorf("get job: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create job from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateJobInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateJob(payload)
			if err != nil {
				return fmt.Errorf("create job: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <id>",
		Short: "Update job from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateJobInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateJob(args[0], payload)
			if err != nil {
				return fmt.Errorf("update job: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	var status string
	setStatus := &cobra.Command{
		Use:   "set-status <id>",
		Short: "Set job status",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			status = strings.TrimSpace(status)
			if status == "" {
				return fmt.Errorf("missing --status")
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateJobStatus(args[0], status)
			if err != nil {
				return fmt.Errorf("set job status: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	setStatus.Flags().StringVar(&status, "status", "", "new job status")

	var subtaskInput string
	var subtaskInputFile string
	subtask := &cobra.Command{
		Use:   "subtask <job-id>",
		Short: "Create subtask from JSON payload",
		Long:  "Input supports map payload used by API route, for example: {\"title\":\"subtask\"}.",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(subtaskInput, subtaskInputFile, true)
			if err != nil {
				return err
			}
			var payload map[string]string
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateSubtask(args[0], payload)
			if err != nil {
				return fmt.Errorf("create subtask: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(subtask, &subtaskInput, &subtaskInputFile)

	cmd.AddCommand(query, get, create, update, setStatus, subtask)
	return cmd
}

func apiLogsResourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Structured log entry operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query log entries",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryLogs(params)
			if err != nil {
				return fmt.Errorf("query logs: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Get log entry by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetLog(args[0])
			if err != nil {
				return fmt.Errorf("get log: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create log entry from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateLogInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateLog(payload)
			if err != nil {
				return fmt.Errorf("create log: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <id>",
		Short: "Update log entry from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateLogInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateLog(args[0], payload)
			if err != nil {
				return fmt.Errorf("update log: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	cmd.AddCommand(query, get, create, update)
	return cmd
}

func apiFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "File metadata operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query file metadata entries",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryFiles(params)
			if err != nil {
				return fmt.Errorf("query files: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Get file metadata entry by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetFile(args[0])
			if err != nil {
				return fmt.Errorf("get file: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create file metadata entry from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateFileInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateFile(payload)
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <id>",
		Short: "Update file metadata entry from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateFileInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateFile(args[0], payload)
			if err != nil {
				return fmt.Errorf("update file: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	cmd.AddCommand(query, get, create, update)
	return cmd
}

func apiProtocolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protocols",
		Short: "Protocol operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query protocols",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryProtocols(params)
			if err != nil {
				return fmt.Errorf("query protocols: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	get := &cobra.Command{
		Use:   "get <name>",
		Short: "Get protocol by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetProtocol(args[0])
			if err != nil {
				return fmt.Errorf("get protocol: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create protocol from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateProtocolInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateProtocol(payload)
			if err != nil {
				return fmt.Errorf("create protocol: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <name>",
		Short: "Update protocol from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateProtocolInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateProtocol(args[0], payload)
			if err != nil {
				return fmt.Errorf("update protocol: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	cmd.AddCommand(query, get, create, update)
	return cmd
}

func apiApprovalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approvals",
		Short: "Approval queue operations",
	}

	var limit int
	var offset int
	pending := &cobra.Command{
		Use:   "pending",
		Short: "List pending approvals",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.GetPendingApprovalsWithParams(limit, offset)
			if err != nil {
				return fmt.Errorf("list approvals: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	pending.Flags().IntVar(&limit, "limit", 200, "max rows")
	pending.Flags().IntVar(&offset, "offset", 0, "row offset")

	get := &cobra.Command{
		Use:   "get <id>",
		Short: "Get approval by id",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetApproval(args[0])
			if err != nil {
				return fmt.Errorf("get approval: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var diffOnly []string
	var diffMaxLines int
	diff := &cobra.Command{
		Use:   "diff <id>",
		Short: "Get computed approval diff",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			opts, err := parseApprovalDiffViewOptions(diffOnly, diffMaxLines)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetApprovalDiff(args[0])
			if err != nil {
				return fmt.Errorf("approval diff: %w", err)
			}
			rows := approvalDiffRows(item.Changes, opts.MaxLines)
			rows = applyApprovalDiffFilters(rows, opts)
			if resolveOutputMode(OutputModeJSON) == OutputModeTable {
				renderApprovalDiffTable(command.OutOrStdout(), item, rows, opts)
				return nil
			}
			return writeCleanJSON(command.OutOrStdout(), buildApprovalDiffResponse(item, rows, opts))
		},
	}
	diff.Flags().StringArrayVar(
		&diffOnly,
		"only",
		nil,
		"focus output: changed or section=<core|metadata|tags|scopes|content|source|other> (repeatable)",
	)
	diff.Flags().IntVar(&diffMaxLines, "max-lines", 6, "max rendered lines per diff value")

	var approveInput string
	var approveInputFile string
	approve := &cobra.Command{
		Use:   "approve <id>",
		Short: "Approve request (optional JSON grants payload)",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(approveInput, approveInputFile, false)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			if len(raw) == 0 {
				item, err := client.ApproveRequest(args[0])
				if err != nil {
					return fmt.Errorf("approve request: %w", err)
				}
				return writeCleanJSON(command.OutOrStdout(), item)
			}
			var payload api.ApproveRequestInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			item, err := client.ApproveRequestWithInput(args[0], &payload)
			if err != nil {
				return fmt.Errorf("approve request: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(approve, &approveInput, &approveInputFile)

	var rejectNotes string
	reject := &cobra.Command{
		Use:   "reject <id>",
		Short: "Reject request with review notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			rejectNotes = strings.TrimSpace(rejectNotes)
			if rejectNotes == "" {
				return fmt.Errorf("missing --notes")
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.RejectRequest(args[0], rejectNotes)
			if err != nil {
				return fmt.Errorf("reject request: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	reject.Flags().StringVar(&rejectNotes, "notes", "", "review notes for rejection")

	cmd.AddCommand(pending, get, diff, approve, reject)
	return cmd
}

func apiAgentsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agents",
		Short: "Agent operations",
	}

	var statusCategory string
	list := &cobra.Command{
		Use:   "list",
		Short: "List agents",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.ListAgents(statusCategory)
			if err != nil {
				return fmt.Errorf("list agents: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	list.Flags().StringVar(&statusCategory, "status-category", "active", "status filter category")

	get := &cobra.Command{
		Use:   "get <name>",
		Short: "Get agent by name",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.GetAgent(args[0])
			if err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	var registerInput string
	var registerInputFile string
	register := &cobra.Command{
		Use:   "register",
		Short: "Register agent from JSON payload",
		RunE: func(command *cobra.Command, _ []string) error {
			raw, err := readInputJSON(registerInput, registerInputFile, true)
			if err != nil {
				return err
			}
			var payload api.RegisterAgentInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.RegisterAgent(payload)
			if err != nil {
				return fmt.Errorf("register agent: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(register, &registerInput, &registerInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <id>",
		Short: "Update agent from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateAgentInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateAgent(args[0], payload)
			if err != nil {
				return fmt.Errorf("update agent: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	cmd.AddCommand(list, get, register, update)
	return cmd
}

func apiKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keys",
		Short: "API key operations",
	}

	login := &cobra.Command{
		Use:   "login <username>",
		Short: "Login with username and print API key payload as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _ := loadCommandClient(false)
			resp, err := client.Login(args[0])
			if err != nil {
				return fmt.Errorf("login: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), resp)
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List caller API keys",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.ListKeys()
			if err != nil {
				return fmt.Errorf("list keys: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}

	listAll := &cobra.Command{
		Use:   "list-all",
		Short: "List all keys (admin route)",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.ListAllKeys()
			if err != nil {
				return fmt.Errorf("list all keys: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}

	create := &cobra.Command{
		Use:   "create <name>",
		Short: "Create API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateKey(args[0])
			if err != nil {
				return fmt.Errorf("create key: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	revoke := &cobra.Command{
		Use:   "revoke <key-id>",
		Short: "Revoke API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			if err := client.RevokeKey(args[0]); err != nil {
				return fmt.Errorf("revoke key: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), map[string]any{
				"key_id":  args[0],
				"revoked": true,
			})
		},
	}

	cmd.AddCommand(login, list, listAll, create, revoke)
	return cmd
}

func apiAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit log operations",
	}

	var rawParams []string
	query := &cobra.Command{
		Use:   "query",
		Short: "Query audit log",
		RunE: func(command *cobra.Command, _ []string) error {
			params, err := parseQueryParams(rawParams)
			if err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.QueryAuditLog(params)
			if err != nil {
				return fmt.Errorf("query audit log: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	bindParamFlags(query, &rawParams)

	scopes := &cobra.Command{
		Use:   "scopes",
		Short: "List audit scope summaries",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.ListAuditScopes()
			if err != nil {
				return fmt.Errorf("list audit scopes: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}

	var actorType string
	actors := &cobra.Command{
		Use:   "actors",
		Short: "List audit actor summaries",
		RunE: func(command *cobra.Command, _ []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.ListAuditActors(actorType)
			if err != nil {
				return fmt.Errorf("list audit actors: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	actors.Flags().StringVar(&actorType, "actor-type", "", "filter by actor type")

	cmd.AddCommand(query, scopes, actors)
	return cmd
}

func apiTaxonomyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "taxonomy",
		Short: "Taxonomy operations",
	}

	var includeInactive bool
	var search string
	var limit int
	var offset int
	list := &cobra.Command{
		Use:   "list <kind>",
		Short: "List taxonomy entries by kind",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.ListTaxonomy(args[0], includeInactive, search, limit, offset)
			if err != nil {
				return fmt.Errorf("list taxonomy: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	list.Flags().BoolVar(&includeInactive, "include-inactive", false, "include archived taxonomy entries")
	list.Flags().StringVar(&search, "search", "", "name search text")
	list.Flags().IntVar(&limit, "limit", 200, "max rows")
	list.Flags().IntVar(&offset, "offset", 0, "row offset")

	var createInput string
	var createInputFile string
	create := &cobra.Command{
		Use:   "create <kind>",
		Short: "Create taxonomy entry from JSON payload",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(createInput, createInputFile, true)
			if err != nil {
				return err
			}
			var payload api.CreateTaxonomyInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.CreateTaxonomy(args[0], payload)
			if err != nil {
				return fmt.Errorf("create taxonomy: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(create, &createInput, &createInputFile)

	var updateInput string
	var updateInputFile string
	update := &cobra.Command{
		Use:   "update <kind> <id>",
		Short: "Update taxonomy entry from JSON payload",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			raw, err := readInputJSON(updateInput, updateInputFile, true)
			if err != nil {
				return err
			}
			var payload api.UpdateTaxonomyInput
			if err := decodeJSONInput(raw, &payload); err != nil {
				return err
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.UpdateTaxonomy(args[0], args[1], payload)
			if err != nil {
				return fmt.Errorf("update taxonomy: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}
	bindInputFlags(update, &updateInput, &updateInputFile)

	archive := &cobra.Command{
		Use:   "archive <kind> <id>",
		Short: "Archive taxonomy entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.ArchiveTaxonomy(args[0], args[1])
			if err != nil {
				return fmt.Errorf("archive taxonomy: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	activate := &cobra.Command{
		Use:   "activate <kind> <id>",
		Short: "Activate taxonomy entry",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			item, err := client.ActivateTaxonomy(args[0], args[1])
			if err != nil {
				return fmt.Errorf("activate taxonomy: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), item)
		},
	}

	cmd.AddCommand(list, create, update, archive, activate)
	return cmd
}

func apiSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search operations",
	}

	var query string
	var kinds []string
	var limit int
	semantic := &cobra.Command{
		Use:   "semantic",
		Short: "Run semantic search",
		RunE: func(command *cobra.Command, _ []string) error {
			query = strings.TrimSpace(query)
			if query == "" {
				return fmt.Errorf("missing --query")
			}
			client, err := loadCommandClient(true)
			if err != nil {
				return err
			}
			items, err := client.SemanticSearch(query, kinds, limit)
			if err != nil {
				return fmt.Errorf("semantic search: %w", err)
			}
			return writeCleanJSON(command.OutOrStdout(), items)
		},
	}
	semantic.Flags().StringVar(&query, "query", "", "search query")
	semantic.Flags().StringArrayVar(&kinds, "kind", nil, "search kind filter (repeat)")
	semantic.Flags().IntVar(&limit, "limit", 20, "max results")

	cmd.AddCommand(semantic)
	return cmd
}

func apiImportsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Bulk import operations",
	}

	makeImportCmd := func(use string, short string, fn func(*api.Client, api.BulkImportRequest) (*api.BulkImportResult, error)) *cobra.Command {
		var input string
		var inputFile string
		sub := &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(command *cobra.Command, _ []string) error {
				raw, err := readInputJSON(input, inputFile, true)
				if err != nil {
					return err
				}
				var payload api.BulkImportRequest
				if err := decodeJSONInput(raw, &payload); err != nil {
					return err
				}
				client, err := loadCommandClient(true)
				if err != nil {
					return err
				}
				result, err := fn(client, payload)
				if err != nil {
					return err
				}
				return writeCleanJSON(command.OutOrStdout(), result)
			},
		}
		bindInputFlags(sub, &input, &inputFile)
		return sub
	}

	cmd.AddCommand(makeImportCmd("entities", "Import entities from JSON payload", func(client *api.Client, payload api.BulkImportRequest) (*api.BulkImportResult, error) {
		result, err := client.ImportEntities(payload)
		if err != nil {
			return nil, fmt.Errorf("import entities: %w", err)
		}
		return result, nil
	}))
	cmd.AddCommand(makeImportCmd("context", "Import context items from JSON payload", func(client *api.Client, payload api.BulkImportRequest) (*api.BulkImportResult, error) {
		result, err := client.ImportContext(payload)
		if err != nil {
			return nil, fmt.Errorf("import context: %w", err)
		}
		return result, nil
	}))
	cmd.AddCommand(makeImportCmd("relationships", "Import relationships from JSON payload", func(client *api.Client, payload api.BulkImportRequest) (*api.BulkImportResult, error) {
		result, err := client.ImportRelationships(payload)
		if err != nil {
			return nil, fmt.Errorf("import relationships: %w", err)
		}
		return result, nil
	}))
	cmd.AddCommand(makeImportCmd("jobs", "Import jobs from JSON payload", func(client *api.Client, payload api.BulkImportRequest) (*api.BulkImportResult, error) {
		result, err := client.ImportJobs(payload)
		if err != nil {
			return nil, fmt.Errorf("import jobs: %w", err)
		}
		return result, nil
	}))

	return cmd
}

func apiExportsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export operations",
	}

	makeExportCmd := func(use string, short string, fn func(*api.Client, api.QueryParams) (*api.ExportResult, error)) *cobra.Command {
		var rawParams []string
		sub := &cobra.Command{
			Use:   use,
			Short: short,
			RunE: func(command *cobra.Command, _ []string) error {
				params, err := parseQueryParams(rawParams)
				if err != nil {
					return err
				}
				client, err := loadCommandClient(true)
				if err != nil {
					return err
				}
				result, err := fn(client, params)
				if err != nil {
					return err
				}
				return writeCleanJSON(command.OutOrStdout(), result)
			},
		}
		bindParamFlags(sub, &rawParams)
		return sub
	}

	cmd.AddCommand(makeExportCmd("entities", "Export entities", func(client *api.Client, params api.QueryParams) (*api.ExportResult, error) {
		result, err := client.ExportEntities(params)
		if err != nil {
			return nil, fmt.Errorf("export entities: %w", err)
		}
		return result, nil
	}))
	cmd.AddCommand(makeExportCmd("context", "Export context items", func(client *api.Client, params api.QueryParams) (*api.ExportResult, error) {
		result, err := client.ExportContextItems(params)
		if err != nil {
			return nil, fmt.Errorf("export context: %w", err)
		}
		return result, nil
	}))
	cmd.AddCommand(makeExportCmd("relationships", "Export relationships", func(client *api.Client, params api.QueryParams) (*api.ExportResult, error) {
		result, err := client.ExportRelationships(params)
		if err != nil {
			return nil, fmt.Errorf("export relationships: %w", err)
		}
		return result, nil
	}))
	cmd.AddCommand(makeExportCmd("jobs", "Export jobs", func(client *api.Client, params api.QueryParams) (*api.ExportResult, error) {
		result, err := client.ExportJobs(params)
		if err != nil {
			return nil, fmt.Errorf("export jobs: %w", err)
		}
		return result, nil
	}))
	cmd.AddCommand(makeExportCmd("snapshot", "Export full context snapshot", func(client *api.Client, params api.QueryParams) (*api.ExportResult, error) {
		result, err := client.ExportContext(params)
		if err != nil {
			return nil, fmt.Errorf("export snapshot: %w", err)
		}
		return result, nil
	}))

	return cmd
}

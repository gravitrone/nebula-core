package api

import (
	"encoding/json"
	"time"
)

// --- API Response Envelope ---

type apiResponse[T any] struct {
	Data  T       `json:"data"`
	Error *apiErr `json:"error,omitempty"`
}

type apiErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSONMap handles JSONB fields that asyncpg may return as strings.
type JSONMap map[string]any

// UnmarshalJSON handles unmarshal json.
func (j *JSONMap) UnmarshalJSON(data []byte) error {
	// Try as object first
	var m map[string]any
	if err := json.Unmarshal(data, &m); err == nil {
		*j = m
		return nil
	}
	// Try as string containing JSON
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" || s == "null" {
			*j = make(map[string]any)
			return nil
		}
		return json.Unmarshal([]byte(s), (*map[string]any)(j))
	}
	*j = make(map[string]any)
	return nil
}

// --- Entity ---

// Entity represents a core data object in the system.
type Entity struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	TypeID          string    `json:"type_id,omitempty"`
	Type            string    `json:"type,omitempty"`
	StatusID        string    `json:"status_id,omitempty"`
	Status          string    `json:"status,omitempty"`
	PrivacyScopeIDs []string  `json:"privacy_scope_ids,omitempty"`
	Tags            []string  `json:"tags"`
	SourcePath      *string   `json:"source_path,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateEntityInput defines the fields required to create a new entity.
type CreateEntityInput struct {
	Scopes []string `json:"scopes"`
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Status string   `json:"status"`
	Tags   []string `json:"tags"`
}

// UpdateEntityInput defines the fields for updating an existing entity.
type UpdateEntityInput struct {
	Name         *string   `json:"name,omitempty"`
	Status       *string   `json:"status,omitempty"`
	Tags         *[]string `json:"tags,omitempty"`
	StatusReason *string   `json:"status_reason,omitempty"`
}

// BulkUpdateEntityTagsInput defines the fields for bulk tag updates.
type BulkUpdateEntityTagsInput struct {
	EntityIDs []string `json:"entity_ids"`
	Tags      []string `json:"tags"`
	Op        string   `json:"op"`
}

// BulkUpdateEntityScopesInput defines the fields for bulk scope updates.
type BulkUpdateEntityScopesInput struct {
	EntityIDs []string `json:"entity_ids"`
	Scopes    []string `json:"scopes"`
	Op        string   `json:"op"`
}

// BulkUpdateResult returns ids and count.
type BulkUpdateResult struct {
	Updated   int      `json:"updated"`
	EntityIDs []string `json:"entity_ids"`
}

// --- Context ---

// Context represents a piece of information or documentation.
type Context struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Name            string    `json:"name,omitempty"`
	URL             *string   `json:"url,omitempty"`
	SourceType      string    `json:"source_type,omitempty"`
	Content         *string   `json:"content,omitempty"`
	PrivacyScopeIDs []string  `json:"privacy_scope_ids,omitempty"`
	Status          string    `json:"status,omitempty"`
	Tags            []string  `json:"tags"`
	SourcePath      *string   `json:"source_path,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UnmarshalJSON keeps compatibility with legacy payloads that still return name.
func (c *Context) UnmarshalJSON(data []byte) error {
	type rawContext Context
	var raw rawContext
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = Context(raw)
	if c.Title == "" && c.Name != "" {
		c.Title = c.Name
	}
	if c.Name == "" && c.Title != "" {
		c.Name = c.Title
	}
	return nil
}

// CreateContextInput defines the fields required to create new context.
type CreateContextInput struct {
	Title      string   `json:"title"`
	URL        string   `json:"url,omitempty"`
	SourceType string   `json:"source_type"`
	Content    string   `json:"content,omitempty"`
	Scopes     []string `json:"scopes"`
	Tags       []string `json:"tags"`
}

// UpdateContextInput defines the fields for updating context.
type UpdateContextInput struct {
	Title      *string   `json:"title,omitempty"`
	URL        *string   `json:"url,omitempty"`
	SourceType *string   `json:"source_type,omitempty"`
	Content    *string   `json:"content,omitempty"`
	Status     *string   `json:"status,omitempty"`
	Scopes     *[]string `json:"scopes,omitempty"`
	Tags       *[]string `json:"tags,omitempty"`
}

// LinkContextInput defines the fields for linking context to an owner.
type LinkContextInput struct {
	OwnerType string `json:"owner_type"`
	OwnerID   string `json:"owner_id"`
}

// --- Protocol ---

// Protocol represents a protocol entry.
type Protocol struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Title        string    `json:"title"`
	Version      *string   `json:"version,omitempty"`
	Content      *string   `json:"content,omitempty"`
	ProtocolType *string   `json:"protocol_type,omitempty"`
	AppliesTo    []string  `json:"applies_to,omitempty"`
	Status       string    `json:"status,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	Trusted      *bool     `json:"trusted,omitempty"`
	Notes        string   `json:"notes"`
	SourcePath   *string   `json:"source_path,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateProtocolInput defines the fields required to create a protocol.
type CreateProtocolInput struct {
	Name         string         `json:"name"`
	Title        string         `json:"title"`
	Version      string         `json:"version,omitempty"`
	Content      string         `json:"content"`
	ProtocolType string         `json:"protocol_type,omitempty"`
	AppliesTo    []string       `json:"applies_to,omitempty"`
	Status       string         `json:"status,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Trusted      bool           `json:"trusted,omitempty"`
	Notes        string         `json:"notes,omitempty"`
	SourcePath   *string        `json:"source_path,omitempty"`
}

// UpdateProtocolInput defines the fields for updating a protocol.
type UpdateProtocolInput struct {
	Title        *string        `json:"title,omitempty"`
	Version      *string        `json:"version,omitempty"`
	Content      *string        `json:"content,omitempty"`
	ProtocolType *string        `json:"protocol_type,omitempty"`
	AppliesTo    *[]string      `json:"applies_to,omitempty"`
	Status       *string        `json:"status,omitempty"`
	Tags         *[]string      `json:"tags,omitempty"`
	Trusted      *bool          `json:"trusted,omitempty"`
	Notes        string         `json:"notes,omitempty"`
	SourcePath   *string        `json:"source_path,omitempty"`
}

// --- Logs ---

// Log represents a log entry.
type Log struct {
	ID        string    `json:"id"`
	LogType   string    `json:"log_type"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	Status    string    `json:"status,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateLogInput defines fields for creating a log entry.
type CreateLogInput struct {
	LogType   string         `json:"log_type"`
	Timestamp *time.Time     `json:"timestamp,omitempty"`
	Content   string         `json:"content,omitempty"`
	Status    string         `json:"status,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	Notes     string         `json:"notes,omitempty"`
}

// UpdateLogInput defines fields for updating a log entry.
type UpdateLogInput struct {
	LogType   *string        `json:"log_type,omitempty"`
	Timestamp *time.Time     `json:"timestamp,omitempty"`
	Content   string         `json:"content,omitempty"`
	Status    *string        `json:"status,omitempty"`
	Tags      *[]string      `json:"tags,omitempty"`
	Notes     string         `json:"notes,omitempty"`
}

// --- Files ---

// File represents a file metadata record.
type File struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	URI       string    `json:"uri"`
	FilePath  string    `json:"file_path"`
	MimeType  *string   `json:"mime_type,omitempty"`
	SizeBytes *int64    `json:"size_bytes,omitempty"`
	Checksum  *string   `json:"checksum,omitempty"`
	Status    string    `json:"status,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateFileInput defines fields for creating a file record.
type CreateFileInput struct {
	Filename  string         `json:"filename"`
	URI       string         `json:"uri,omitempty"`
	FilePath  string         `json:"file_path,omitempty"`
	MimeType  string         `json:"mime_type,omitempty"`
	SizeBytes *int64         `json:"size_bytes,omitempty"`
	Checksum  string         `json:"checksum,omitempty"`
	Status    string         `json:"status,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	Notes     string         `json:"notes,omitempty"`
}

// UpdateFileInput defines fields for updating a file record.
type UpdateFileInput struct {
	Filename  *string        `json:"filename,omitempty"`
	URI       *string        `json:"uri,omitempty"`
	FilePath  *string        `json:"file_path,omitempty"`
	MimeType  *string        `json:"mime_type,omitempty"`
	SizeBytes *int64         `json:"size_bytes,omitempty"`
	Checksum  *string        `json:"checksum,omitempty"`
	Status    *string        `json:"status,omitempty"`
	Tags      *[]string      `json:"tags,omitempty"`
	Notes     string         `json:"notes,omitempty"`
}

// --- Relationship ---

// Relationship represents a directed connection between two entities.
type Relationship struct {
	ID         string    `json:"id"`
	SourceType string    `json:"source_type,omitempty"`
	SourceID   string    `json:"source_id"`
	SourceName string    `json:"source_name,omitempty"`
	TargetType string    `json:"target_type,omitempty"`
	TargetID   string    `json:"target_id"`
	TargetName string    `json:"target_name,omitempty"`
	TypeID     string    `json:"type_id,omitempty"`
	Type       string    `json:"relationship_type,omitempty"`
	Status     string    `json:"status,omitempty"`
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
}

// CreateRelationshipInput defines the fields required to create a new relationship.
type CreateRelationshipInput struct {
	SourceType string         `json:"source_type"`
	SourceID   string         `json:"source_id"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	Type       string         `json:"relationship_type"`
	Notes      string         `json:"notes,omitempty"`
}

// UpdateRelationshipInput defines the fields for updating a relationship.
type UpdateRelationshipInput struct {
	Type       *string        `json:"type,omitempty"`
	Notes      string         `json:"notes,omitempty"`
	Status     *string        `json:"status,omitempty"`
}

// --- Job ---

// Job represents an asynchronous task or workflow.
type Job struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Status      string    `json:"status"`
	Priority    *string   `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateJobInput defines the fields required to create a new job.
type CreateJobInput struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	Priority    string `json:"priority,omitempty"`
}

// UpdateJobInput defines the fields for updating an existing job.
type UpdateJobInput struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
	Priority    *string `json:"priority,omitempty"`
}

// --- Approval ---

// Approval represents a request requiring human intervention.
type Approval struct {
	ID              string    `json:"id"`
	JobID           *string   `json:"job_id"`
	RequestType     string    `json:"request_type"`
	RequestedBy     string    `json:"requested_by"`
	RequestedByName string    `json:"requested_by_name"`
	AgentName       string    `json:"agent_name"`
	ChangeDetails   string    `json:"change_details"`
	Status          string    `json:"status"`
	Notes           *string   `json:"review_notes"`
	CreatedAt       time.Time `json:"created_at"`
}

// ApproveRequestInput defines optional reviewer grants for approval execution.
type ApproveRequestInput struct {
	ReviewNotes           *string  `json:"review_notes,omitempty"`
	GrantScopes           []string `json:"grant_scopes,omitempty"`
	GrantRequiresApproval *bool    `json:"grant_requires_approval,omitempty"`
}

// ApprovalDiff represents server computed diff for approval requests.
type ApprovalDiff struct {
	ApprovalID  string         `json:"approval_id"`
	RequestType string         `json:"request_type"`
	Changes     map[string]any `json:"changes"`
}

// AuditEntry represents a history entry from audit_log.
type AuditEntry struct {
	ID            string    `json:"id"`
	TableName     string    `json:"table_name"`
	RecordID      string    `json:"record_id"`
	Action        string    `json:"action"`
	ChangedByType *string   `json:"changed_by_type"`
	ChangedByID   *string   `json:"changed_by_id"`
	ActorName     *string   `json:"actor_name"`
	OldValues     string    `json:"old_values"`
	NewValues     string    `json:"new_values"`
	ChangedFields []string  `json:"changed_fields"`
	ChangeReason  *string   `json:"change_reason"`
	Notes         string    `json:"notes"`
	ChangedAt     time.Time `json:"changed_at"`
}

// AuditScope represents privacy scope usage stats.
type AuditScope struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	AgentCount   int     `json:"agent_count"`
	EntityCount  int     `json:"entity_count"`
	ContextCount int     `json:"context_count"`
}

// AuditActor represents an audit actor and activity summary.
type AuditActor struct {
	ActorType   string    `json:"changed_by_type"`
	ActorID     string    `json:"changed_by_id"`
	ActorName   *string   `json:"actor_name"`
	ActionCount int       `json:"action_count"`
	LastSeen    time.Time `json:"last_seen"`
}

// BulkImportError represents a per-row import error.
type BulkImportError struct {
	Row   int    `json:"row"`
	Error string `json:"error"`
}

// BulkImportResult represents the result of a bulk import.
type BulkImportResult struct {
	Created int               `json:"created"`
	Failed  int               `json:"failed"`
	Errors  []BulkImportError `json:"errors"`
	Items   []map[string]any  `json:"items"`
}

// ExportResult represents export payload data.
type ExportResult struct {
	Format  string           `json:"format"`
	Content string           `json:"content,omitempty"`
	Items   []map[string]any `json:"items,omitempty"`
	Count   int              `json:"count"`
}

// --- Taxonomy ---

// TaxonomyEntry represents a taxonomy row for scopes/types.
type TaxonomyEntry struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsBuiltin   bool      `json:"is_builtin"`
	IsActive    bool      `json:"is_active"`
	Notes       string    `json:"notes,omitempty"`
	IsSymmetric *bool     `json:"is_symmetric,omitempty"`
	ValueSchema JSONMap   `json:"value_schema,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateTaxonomyInput defines fields for creating taxonomy entries.
type CreateTaxonomyInput struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Notes       string         `json:"notes,omitempty"`
	IsSymmetric *bool          `json:"is_symmetric,omitempty"`
	ValueSchema map[string]any `json:"value_schema,omitempty"`
}

// UpdateTaxonomyInput defines fields for updating taxonomy entries.
type UpdateTaxonomyInput struct {
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Notes       string         `json:"notes,omitempty"`
	IsSymmetric *bool          `json:"is_symmetric,omitempty"`
	ValueSchema map[string]any `json:"value_schema,omitempty"`
}

// --- Agent ---

// Agent represents an AI agent or automated system.
type Agent struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      *string   `json:"description"`
	Scopes           []string  `json:"scopes"`
	Capabilities     []string  `json:"capabilities"`
	Status           string    `json:"status"`
	RequiresApproval bool      `json:"requires_approval"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// RegisterAgentInput defines the fields required to register a new agent.
type RegisterAgentInput struct {
	Name                      string   `json:"name"`
	Description               string   `json:"description,omitempty"`
	RequestedScopes           []string `json:"requested_scopes"`
	RequestedRequiresApproval bool     `json:"requested_requires_approval,omitempty"`
	Capabilities              []string `json:"capabilities,omitempty"`
}

// AgentRegistration response containing the ID and approval status.
type AgentRegistration struct {
	AgentID           string `json:"agent_id"`
	ApprovalRequestID string `json:"approval_request_id"`
	RegistrationID    string `json:"registration_id,omitempty"`
	EnrollmentToken   string `json:"enrollment_token,omitempty"`
	Status            string `json:"status"`
}

// UpdateAgentInput defines the fields for updating an agent's settings.
type UpdateAgentInput struct {
	Description      *string  `json:"description,omitempty"`
	RequiresApproval *bool    `json:"requires_approval,omitempty"`
	Scopes           []string `json:"scopes,omitempty"`
}

// --- API Key ---

// APIKey represents an authentication token.
type APIKey struct {
	ID         string     `json:"id"`
	KeyPrefix  string     `json:"key_prefix"`
	Name       string     `json:"name"`
	OwnerType  string     `json:"owner_type,omitempty"`
	EntityName *string    `json:"entity_name,omitempty"`
	AgentName  *string    `json:"agent_name,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// CreateKeyResponse contains the generated API key and its metadata.
type CreateKeyResponse struct {
	APIKey string `json:"api_key"`
	KeyID  string `json:"key_id"`
	Prefix string `json:"prefix"`
	Name   string `json:"name"`
}

// LoginInput defines the credentials for logging in.
type LoginInput struct {
	Username string `json:"username"`
}

// LoginResponse contains the session information after successful login.
type LoginResponse struct {
	APIKey   string `json:"api_key"`
	EntityID string `json:"entity_id"`
	Username string `json:"username"`
}

// --- Query ---

// QueryParams is a map of URL query parameters.
type QueryParams map[string]string

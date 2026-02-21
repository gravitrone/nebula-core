"""Pydantic models for Nebula MCP."""

# Standard Library
import unicodedata
from datetime import date, datetime, time, timezone
from typing import Self

# Third-Party
from pydantic import (
    BaseModel,
    ConfigDict,
    Field,
    StrictInt,
    field_validator,
    model_validator,
)

MAX_TAGS = 50
MAX_TAG_LENGTH = 64
MAX_PAGE_LIMIT = 1000
MAX_GRAPH_HOPS = 6

BASIC_NODE_TYPES = {
    "entity",
    "context",
    "log",
    "job",
    "agent",
    "file",
    "protocol",
}

TAXONOMY_KINDS = {
    "scopes",
    "entity-types",
    "relationship-types",
    "log-types",
}

BIDI_CONTROLS = {
    "\u202a",
    "\u202b",
    "\u202c",
    "\u202d",
    "\u202e",
    "\u2066",
    "\u2067",
    "\u2068",
    "\u2069",
    "\u200e",
    "\u200f",
}

BANNED_METADATA_KEYS = {
    "__proto__",
    "prototype",
    "constructor",
    "visibility",
}


def _strip_control(text: str) -> str:
    """Handle strip control.

    Args:
        text: Input parameter for _strip_control.

    Returns:
        Result value from the operation.
    """

    cleaned: list[str] = []
    for ch in text:
        if ch in BIDI_CONTROLS:
            continue
        if unicodedata.category(ch).startswith("C"):
            continue
        cleaned.append(ch)
    return "".join(cleaned).strip()


def _sanitize_text(value: str | None) -> str | None:
    """Handle sanitize text.

    Args:
        value: Input parameter for _sanitize_text.

    Returns:
        Result value from the operation.
    """

    if value is None:
        return None
    return _strip_control(value)


def _sanitize_tags(tags: list[str] | None) -> list[str] | None:
    """Handle sanitize tags.

    Args:
        tags: Input parameter for _sanitize_tags.

    Returns:
        Result value from the operation.
    """

    if tags is None:
        return None
    cleaned = [_strip_control(t) for t in tags]
    cleaned = [t for t in cleaned if t]
    if len(cleaned) > MAX_TAGS:
        raise ValueError("Too many tags")
    for tag in cleaned:
        if len(tag) > MAX_TAG_LENGTH:
            raise ValueError("Tag too long")
    return cleaned


def _validate_node_type(value: str | None) -> str | None:
    """Handle validate node type.

    Args:
        value: Input parameter for _validate_node_type.

    Returns:
        Result value from the operation.
    """

    if value is None:
        return None
    cleaned = _strip_control(value)
    if cleaned not in BASIC_NODE_TYPES:
        raise ValueError("Invalid node type")
    return cleaned


def _reject_metadata_keys(value: object) -> None:
    """Handle reject metadata keys.

    Args:
        value: Input parameter for _reject_metadata_keys.
    """

    if isinstance(value, dict):
        for key, item in value.items():
            if isinstance(key, str) and key == "visibility":
                raise ValueError(
                    "Metadata key 'visibility' is not supported. "
                    "Use context_segments with explicit scopes."
                )
            if isinstance(key, str) and key in BANNED_METADATA_KEYS:
                raise ValueError(f"Metadata key '{key}' is not allowed")
            _reject_metadata_keys(item)
        return
    if isinstance(value, list):
        for item in value:
            _reject_metadata_keys(item)


def _sanitize_metadata(value: dict | None) -> dict | None:
    """Handle sanitize metadata.

    Args:
        value: Input parameter for _sanitize_metadata.

    Returns:
        Result value from the operation.
    """

    if value is None:
        return None
    if not isinstance(value, dict):
        raise ValueError("Metadata must be an object")
    if "visibility" in value:
        raise ValueError(
            "Metadata key 'visibility' is not supported. "
            "Use context_segments with explicit scopes."
        )
    _reject_metadata_keys(value)
    return value


def validate_metadata_payload(value: dict | None) -> dict | None:
    """Validate and sanitize arbitrary metadata payloads.

    Args:
        value: Metadata object from REST or MCP payloads.

    Returns:
        Sanitized metadata payload or None.

    Raises:
        ValueError: If payload shape or keys are invalid.
    """

    return _sanitize_metadata(value)


def _validate_taxonomy_kind(value: str | None) -> str:
    """Handle validate taxonomy kind.

    Args:
        value: Input parameter for _validate_taxonomy_kind.

    Returns:
        Result value from the operation.
    """

    if value is None:
        raise ValueError("Taxonomy kind is required")
    cleaned = _strip_control(value)
    if cleaned not in TAXONOMY_KINDS:
        raise ValueError("Invalid taxonomy kind")
    return cleaned


def _sanitize_source_path(value: str | None) -> str | None:
    """Handle sanitize source path.

    Args:
        value: Input parameter for _sanitize_source_path.

    Returns:
        Result value from the operation.
    """

    if value is None:
        return None
    cleaned = _strip_control(value)
    if not cleaned:
        return None
    return cleaned


def parse_optional_datetime(
    value: str | datetime | date | None, field_name: str
) -> datetime | None:
    """Parse optional ISO datetime/date strings into datetime instances.

    Args:
        value: Raw input value.
        field_name: Field label used in validation errors.

    Returns:
        Parsed datetime or None.

    Raises:
        ValueError: If the input is not a valid ISO date/datetime.
    """

    if value is None:
        return None
    if isinstance(value, datetime):
        return value
    if isinstance(value, date):
        return datetime.combine(value, time.min, tzinfo=timezone.utc)

    text = _strip_control(str(value))
    if text == "":
        return None

    normalized = text
    if normalized.endswith("Z"):
        normalized = normalized[:-1] + "+00:00"

    try:
        return datetime.fromisoformat(normalized)
    except ValueError as exc:
        raise ValueError(f"Invalid {field_name}: expected ISO8601 datetime") from exc


# --- Core Input Models ---
class CreateEntityInput(BaseModel):
    """Input payload for creating an entity."""

    name: str = Field(..., description="Display name for the entity")
    type: str = Field(..., description="Entity type name")
    status: str = Field(..., description="Status name")
    scopes: list[str] = Field(..., description="Privacy scope names")
    tags: list[str] = Field(default_factory=list, description="Kebab-case tags")
    metadata: dict = Field(
        default_factory=dict, description="Flexible metadata payload"
    )
    source_path: str | None = Field(default=None, description="Source path")

    @field_validator("name", "type", mode="before")
    @classmethod
    def _clean_text_fields(cls, v: str | None) -> str | None:
        """Handle clean text fields.

        Args:
            v: Input parameter for _clean_text_fields.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean tags.

        Args:
            v: Input parameter for _clean_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean metadata.

        Args:
            v: Input parameter for _clean_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)

    @field_validator("source_path", mode="before")
    @classmethod
    def _clean_source_path(cls, v: str | None) -> str | None:
        """Handle clean source path.

        Args:
            v: Input parameter for _clean_source_path.

        Returns:
            Result value from the operation.
        """

        return _sanitize_source_path(v)


# --- Shared Metadata Models ---


class ContextSegment(BaseModel):
    """A single scoped context segment."""

    text: str
    scopes: list[str]


class BaseMetadata(BaseModel):
    """Base metadata shared across entity types."""

    # Allow extra keys to support schema evolution without blocking writes
    model_config = ConfigDict(extra="allow")

    description: str | None = None
    urls: list[str] | None = None
    aliases: list[str] | None = None
    context_segments: list[ContextSegment] | None = None


# --- Type-Specific Metadata Models ---


class PersonMetadata(BaseMetadata):
    """Person metadata with optional structured name and birth fields."""

    first_name: str | None = None
    second_name: str | None = None
    last_name: str | None = None

    birth_year: StrictInt | None = None
    birth_month: StrictInt | None = None
    birth_day: StrictInt | None = None

    location: str | None = None
    uni: str | None = None
    relation: str | None = None
    contact: dict | None = None

    @field_validator("birth_month")
    @classmethod
    def _month_range(cls, v: StrictInt | None) -> StrictInt | None:
        """Validate that birth month is within 1-12 when provided."""

        if v is None:
            return v
        if v < 1 or v > 12:
            raise ValueError("Birth month out of range")
        return v

    @field_validator("birth_day")
    @classmethod
    def _day_range(cls, v: StrictInt | None) -> StrictInt | None:
        """Validate that birth day is within 1-31 when provided."""

        if v is None:
            return v
        if v < 1 or v > 31:
            raise ValueError("Birth day out of range")
        return v

    @model_validator(mode="after")
    def _validate_date_combo(self) -> Self:
        """Validate day ranges for the given month, including leap years."""

        if self.birth_month is not None and self.birth_day is not None:
            max_day = 31
            if self.birth_month in {4, 6, 9, 11}:
                max_day = 30
            elif self.birth_month == 2:
                if self.birth_year and (
                    self.birth_year % 400 == 0
                    or (self.birth_year % 4 == 0 and self.birth_year % 100 != 0)
                ):
                    max_day = 29
                else:
                    max_day = 28
            if self.birth_day > max_day:
                raise ValueError("Birth day invalid for birth month")
        return self


class ProjectMetadata(BaseMetadata):
    """Lightweight project metadata."""

    repository: str | None = None
    tech_stack: list[str] | None = None
    status_note: str | None = None
    start_date: str | None = None


class ToolMetadata(BaseMetadata):
    """Lightweight tool metadata."""

    vendor: str | None = None
    license: str | None = None
    category: str | None = None


class OrganizationMetadata(BaseMetadata):
    """Lightweight organization metadata."""

    industry: str | None = None
    location: str | None = None
    website: str | None = None


class CourseMetadata(BaseMetadata):
    """Lightweight course metadata."""

    institution: str | None = None
    term: str | None = None
    instructor: str | None = None


class IdeaMetadata(BaseMetadata):
    """Lightweight idea metadata."""

    stage: str | None = None
    priority: str | None = None


class FrameworkMetadata(BaseMetadata):
    """Lightweight framework metadata."""

    language: str | None = None
    version: str | None = None


class PaperMetadata(BaseMetadata):
    """Lightweight paper metadata."""

    authors: list[str] | None = None
    year: StrictInt | None = None
    venue: str | None = None


class UniversityMetadata(BaseMetadata):
    """Lightweight university metadata."""

    country: str | None = None
    city: str | None = None


# --- Metadata Validation Helpers ---


def validate_entity_metadata(entity_type: str, metadata: dict) -> dict:
    """Validate metadata by entity type and return a normalized dict.

    Args:
        entity_type: Type name to select validation model.
        metadata: Raw metadata dict to validate.

    Returns:
        Normalized metadata dict with None values excluded.
    """

    type_map: dict[str, type[BaseMetadata]] = {
        "person": PersonMetadata,
        "project": ProjectMetadata,
        "tool": ToolMetadata,
        "organization": OrganizationMetadata,
        "course": CourseMetadata,
        "idea": IdeaMetadata,
        "framework": FrameworkMetadata,
        "paper": PaperMetadata,
        "university": UniversityMetadata,
    }

    model_cls = type_map.get(entity_type, BaseMetadata)
    model = model_cls.model_validate(metadata or {})
    return model.model_dump(exclude_none=True)


# --- Approval Workflow Models ---


class CreateApprovalRequestInput(BaseModel):
    """Input payload for creating an approval request."""

    request_type: str = Field(..., description="Type of action (e.g., create_entity)")
    change_details: dict = Field(..., description="Full payload of requested change")
    job_id: str | None = Field(default=None, description="Optional related job ID")


class ApproveRequestInput(BaseModel):
    """Input payload for approving a request."""

    approval_id: str = Field(..., description="UUID of approval request")
    reviewed_by: str | None = Field(
        default=None, description="Optional reviewer entity UUID"
    )
    review_notes: str | None = Field(
        default=None, description="Optional reviewer notes"
    )
    grant_scopes: list[str] | None = Field(
        default=None, description="Optional scope grants for register_agent"
    )
    grant_requires_approval: bool | None = Field(
        default=None, description="Optional trust grant for register_agent"
    )


class RejectRequestInput(BaseModel):
    """Input payload for rejecting a request."""

    approval_id: str = Field(..., description="UUID of approval request")
    reviewed_by: str | None = Field(
        default=None, description="Optional reviewer entity UUID"
    )
    review_notes: str = Field(..., min_length=1, description="Reason for rejection")


class GetApprovalInput(BaseModel):
    """Input payload for fetching an approval request."""

    approval_id: str = Field(..., description="UUID of approval request")


class PendingApprovalsInput(BaseModel):
    """Input payload for listing pending approvals."""

    limit: int = Field(default=200, ge=1, le=5000, description="Max rows")
    offset: int = Field(default=0, ge=0, description="Pagination offset")


class GetApprovalDiffInput(BaseModel):
    """Input payload for approval diff."""

    approval_id: str = Field(..., description="UUID of approval request")


class BulkImportInput(BaseModel):
    """Input payload for bulk imports."""

    format: str = Field(default="json", description="json or csv")
    data: str | None = Field(default=None, description="CSV content when format=csv")
    items: list[dict] | None = Field(default=None, description="JSON item list")
    defaults: dict | None = Field(default=None, description="Defaults for items")


# --- Entity Input Models ---


class GetEntityInput(BaseModel):
    """Input payload for retrieving an entity."""

    entity_id: str = Field(..., description="Entity UUID to retrieve")


class QueryEntitiesInput(BaseModel):
    """Input payload for searching entities."""

    type: str | None = Field(default=None, description="Entity type filter")
    tags: list[str] = Field(default_factory=list, description="Tag filters")
    search_text: str | None = Field(default=None, description="Full-text search query")
    status_category: str = Field(default="active", description="active or archived")
    scopes: list[str] = Field(default_factory=list, description="Privacy scope filters")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    offset: int = Field(default=0, description="Pagination offset")

    @field_validator("type", "search_text", mode="before")
    @classmethod
    def _clean_query_text(cls, v: str | None) -> str | None:
        """Handle clean query text.

        Args:
            v: Input parameter for _clean_query_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_query_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean query tags.

        Args:
            v: Input parameter for _clean_query_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)


class SemanticSearchInput(BaseModel):
    """Input payload for semantic search."""

    query: str = Field(..., min_length=2, max_length=512, description="Search query")
    kinds: list[str] = Field(
        default_factory=lambda: ["entity", "context"],
        description="Search kinds: entity, context",
    )
    limit: int = Field(
        default=20, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    candidate_limit: int = Field(
        default=250, ge=50, le=2000, description="Candidate pool size per kind"
    )

    @field_validator("query", mode="before")
    @classmethod
    def _clean_semantic_query(cls, v: str | None) -> str | None:
        """Handle clean semantic query.

        Args:
            v: Input parameter for _clean_semantic_query.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("kinds", mode="before")
    @classmethod
    def _clean_semantic_kinds(cls, v: list[str] | None) -> list[str]:
        """Handle clean semantic kinds.

        Args:
            v: Input parameter for _clean_semantic_kinds.

        Returns:
            Result value from the operation.
        """

        if not v:
            return ["entity", "context"]
        out: list[str] = []
        for item in v:
            name = str(item or "").strip().lower()
            if name and name in {"entity", "context"} and name not in out:
                out.append(name)
        return out or ["entity", "context"]


class UpdateEntityInput(BaseModel):
    """Input payload for updating an entity."""

    entity_id: str = Field(..., description="Entity UUID to update")
    metadata: dict | None = Field(default=None, description="Updated metadata")
    tags: list[str] | None = Field(default=None, description="Updated tags")
    status: str | None = Field(default=None, description="New status name")
    status_reason: str | None = Field(
        default=None, description="Reason for status change"
    )

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_update_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean update tags.

        Args:
            v: Input parameter for _clean_update_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_update_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean update metadata.

        Args:
            v: Input parameter for _clean_update_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


class BulkUpdateEntityTagsInput(BaseModel):
    """Input payload for bulk updating entity tags."""

    entity_ids: list[str] = Field(..., description="Entity UUIDs to update")
    tags: list[str] = Field(default_factory=list, description="Tag values")
    op: str = Field(default="add", description="add, remove, or set")

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_bulk_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean bulk tags.

        Args:
            v: Input parameter for _clean_bulk_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)


class BulkUpdateEntityScopesInput(BaseModel):
    """Input payload for bulk updating entity scopes."""

    entity_ids: list[str] = Field(..., description="Entity UUIDs to update")
    scopes: list[str] = Field(default_factory=list, description="Scope names")
    op: str = Field(default="add", description="add, remove, or set")


class GetEntityHistoryInput(BaseModel):
    """Input payload for listing entity audit history."""

    entity_id: str = Field(..., description="Entity UUID")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    offset: int = Field(default=0, description="Pagination offset")


class RevertEntityInput(BaseModel):
    """Input payload for reverting entity to a history entry."""

    entity_id: str = Field(..., description="Entity UUID to revert")
    audit_id: str = Field(..., description="Audit log entry UUID")


class QueryAuditLogInput(BaseModel):
    """Input payload for querying audit log entries."""

    table_name: str | None = Field(default=None, description="Table name filter")
    action: str | None = Field(default=None, description="insert, update, delete")
    actor_type: str | None = Field(default=None, description="agent, entity, system")
    actor_id: str | None = Field(default=None, description="Actor UUID")
    record_id: str | None = Field(default=None, description="Record id filter")
    scope_id: str | None = Field(default=None, description="Scope UUID filter")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    offset: int = Field(default=0, description="Pagination offset")


class ListAuditActorsInput(BaseModel):
    """Input payload for listing audit actors."""

    actor_type: str | None = Field(default=None, description="agent, entity, system")


class SearchEntitiesByMetadataInput(BaseModel):
    """Input payload for searching entities by metadata fields."""

    metadata_query: dict = Field(..., description="JSONB query object")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )


# --- Context Input Models ---


class CreateContextInput(BaseModel):
    """Input payload for creating a context item."""

    title: str = Field(..., description="Context item title")
    url: str | None = Field(default=None, description="Source URL")
    source_type: str = Field(..., description="article, video, paper, tweet, note")
    content: str | None = Field(default=None, description="Full text content")
    scopes: list[str] = Field(..., description="Privacy scope names")
    tags: list[str] = Field(default_factory=list, description="Kebab-case tags")
    metadata: dict = Field(default_factory=dict, description="Additional metadata")

    @field_validator("title", "source_type", mode="before")
    @classmethod
    def _clean_context_text(cls, v: str | None) -> str | None:
        """Handle clean context text.

        Args:
            v: Input parameter for _clean_context_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("url", mode="before")
    @classmethod
    def _validate_context_url(cls, v: str | None) -> str | None:
        """Handle validate context url.

        Args:
            v: Input parameter for _validate_context_url.

        Returns:
            Result value from the operation.
        """

        if not v:
            return v
        v = v.strip()
        if not (v.startswith("http://") or v.startswith("https://")):
            raise ValueError("URL must start with http:// or https://")
        return v

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_context_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean context tags.

        Args:
            v: Input parameter for _clean_context_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_context_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean context metadata.

        Args:
            v: Input parameter for _clean_context_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


class UpdateContextInput(BaseModel):
    """Input payload for updating a context item."""

    context_id: str = Field(..., description="Context item UUID")
    title: str | None = Field(default=None, description="Updated title")
    url: str | None = Field(default=None, description="Updated URL")
    source_type: str | None = Field(default=None, description="Updated source type")
    content: str | None = Field(default=None, description="Updated content")
    status: str | None = Field(default=None, description="Updated status name")
    tags: list[str] | None = Field(default=None, description="Updated tags")
    scopes: list[str] | None = Field(default=None, description="Updated scopes")
    metadata: dict | None = Field(default=None, description="Updated metadata")

    @field_validator("title", "source_type", mode="before")
    @classmethod
    def _clean_update_context_text(cls, v: str | None) -> str | None:
        """Handle clean update context text.

        Args:
            v: Input parameter for _clean_update_context_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("url", mode="before")
    @classmethod
    def _validate_update_context_url(cls, v: str | None) -> str | None:
        """Handle validate update context url.

        Args:
            v: Input parameter for _validate_update_context_url.

        Returns:
            Result value from the operation.
        """

        if not v:
            return v
        v = v.strip()
        if not (v.startswith("http://") or v.startswith("https://")):
            raise ValueError("URL must start with http:// or https://")
        return v

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_update_context_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean update context tags.

        Args:
            v: Input parameter for _clean_update_context_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_update_context_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean update context metadata.

        Args:
            v: Input parameter for _clean_update_context_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


class QueryContextInput(BaseModel):
    """Input payload for searching context items."""

    source_type: str | None = Field(default=None, description="Filter by source type")
    tags: list[str] = Field(default_factory=list, description="Tag filters")
    search_text: str | None = Field(default=None, description="Full-text search query")
    scopes: list[str] = Field(default_factory=list, description="Privacy scope filters")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    offset: int = Field(default=0, description="Pagination offset")

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_context_query_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean context query tags.

        Args:
            v: Input parameter for _clean_context_query_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)


class LinkContextInput(BaseModel):
    """Input payload for linking context to entity."""

    context_id: str = Field(..., description="Context item UUID")
    entity_id: str = Field(..., description="Entity UUID")
    relationship_type: str = Field(..., description="about, mentions, created-by")


class GetContextInput(BaseModel):
    """Input payload for fetching a context item."""

    context_id: str = Field(..., description="Context item UUID")


# --- Log Input Models ---


class CreateLogInput(BaseModel):
    """Input payload for creating a log entry."""

    log_type: str = Field(..., description="Log type name")
    timestamp: datetime | None = Field(default=None, description="Timestamp for log")
    value: dict = Field(default_factory=dict, description="Log value payload")
    status: str = Field(default="active", description="Status name")
    tags: list[str] = Field(default_factory=list, description="Kebab-case tags")
    metadata: dict = Field(default_factory=dict, description="Additional metadata")

    @field_validator("log_type", mode="before")
    @classmethod
    def _clean_log_type(cls, v: str | None) -> str | None:
        """Handle clean log type.

        Args:
            v: Input parameter for _clean_log_type.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_log_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean log tags.

        Args:
            v: Input parameter for _clean_log_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_log_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean log metadata.

        Args:
            v: Input parameter for _clean_log_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


class GetLogInput(BaseModel):
    """Input payload for retrieving a log entry."""

    log_id: str = Field(..., description="Log UUID")


class QueryLogsInput(BaseModel):
    """Input payload for querying log entries."""

    log_type: str | None = Field(default=None, description="Filter by log type")
    tags: list[str] = Field(default_factory=list, description="Tag filters")
    status_category: str = Field(default="active", description="active or archived")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    offset: int = Field(default=0, description="Pagination offset")

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_log_query_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean log query tags.

        Args:
            v: Input parameter for _clean_log_query_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)


class UpdateLogInput(BaseModel):
    """Input payload for updating a log entry."""

    id: str = Field(..., description="Log UUID")
    log_type: str | None = Field(default=None, description="Log type name")
    timestamp: datetime | None = Field(default=None, description="Timestamp")
    value: dict | None = Field(default=None, description="Updated value payload")
    status: str | None = Field(default=None, description="Status name")
    tags: list[str] | None = Field(default=None, description="Tags")
    metadata: dict | None = Field(default=None, description="Metadata")

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_update_log_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean update log tags.

        Args:
            v: Input parameter for _clean_update_log_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_update_log_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean update log metadata.

        Args:
            v: Input parameter for _clean_update_log_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


# --- Relationship Input Models ---


class CreateRelationshipInput(BaseModel):
    """Input payload for creating a relationship."""

    source_type: str = Field(
        ..., description="entity, context, log, job, agent, file, protocol"
    )
    source_id: str = Field(..., description="Source item UUID")
    target_type: str = Field(
        ..., description="entity, context, log, job, agent, file, protocol"
    )
    target_id: str = Field(..., description="Target item UUID")
    relationship_type: str = Field(..., description="Relationship type name")
    properties: dict = Field(default_factory=dict, description="Additional properties")

    @field_validator("source_type", "target_type", mode="before")
    @classmethod
    def _clean_rel_types(cls, v: str | None) -> str | None:
        """Handle clean rel types.

        Args:
            v: Input parameter for _clean_rel_types.

        Returns:
            Result value from the operation.
        """

        return _validate_node_type(v)

    @field_validator("relationship_type", mode="before")
    @classmethod
    def _clean_rel_name(cls, v: str | None) -> str | None:
        """Handle clean rel name.

        Args:
            v: Input parameter for _clean_rel_name.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("properties", mode="before")
    @classmethod
    def _clean_rel_properties(cls, v: dict | None) -> dict | None:
        """Handle clean rel properties.

        Args:
            v: Input parameter for _clean_rel_properties.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


class GetRelationshipsInput(BaseModel):
    """Input payload for retrieving relationships."""

    source_type: str = Field(
        ..., description="entity, context, log, job, agent, file, protocol"
    )
    source_id: str = Field(..., description="Source item UUID")
    relationship_type: str | None = Field(
        default=None, description="Filter by relationship type"
    )
    direction: str = Field(default="both", description="outgoing, incoming, or both")

    @field_validator("source_type", mode="before")
    @classmethod
    def _clean_get_rel_type(cls, v: str | None) -> str | None:
        """Handle clean get rel type.

        Args:
            v: Input parameter for _clean_get_rel_type.

        Returns:
            Result value from the operation.
        """

        return _validate_node_type(v)


class QueryRelationshipsInput(BaseModel):
    """Input payload for searching relationships."""

    source_type: str | None = Field(default=None, description="Filter by source type")
    target_type: str | None = Field(default=None, description="Filter by target type")
    relationship_types: list[str] = Field(
        default_factory=list, description="Filter by relationship types"
    )
    status_category: str = Field(default="active", description="active or archived")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )

    @field_validator("source_type", "target_type", mode="before")
    @classmethod
    def _clean_query_rel_types(cls, v: str | None) -> str | None:
        """Handle clean query rel types.

        Args:
            v: Input parameter for _clean_query_rel_types.

        Returns:
            Result value from the operation.
        """

        return _validate_node_type(v)


class UpdateRelationshipInput(BaseModel):
    """Input payload for updating a relationship."""

    relationship_id: str = Field(..., description="Relationship UUID")
    properties: dict | None = Field(default=None, description="Updated properties")
    status: str | None = Field(default=None, description="New status name")


class GraphNeighborsInput(BaseModel):
    """Input payload for graph neighbors."""

    source_type: str = Field(
        ..., description="entity, context, log, job, agent, file, protocol"
    )
    source_id: str = Field(..., description="Source item UUID")
    max_hops: int = Field(
        default=2, ge=1, le=MAX_GRAPH_HOPS, description="Max hop depth"
    )

    @field_validator("source_type", mode="before")
    @classmethod
    def _clean_graph_source_type(cls, v: str | None) -> str | None:
        """Handle clean graph source type.

        Args:
            v: Input parameter for _clean_graph_source_type.

        Returns:
            Result value from the operation.
        """

        return _validate_node_type(v)

    limit: int = Field(
        default=100, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )


class GraphShortestPathInput(BaseModel):
    """Input payload for shortest path search."""

    source_type: str = Field(
        ..., description="entity, context, log, job, agent, file, protocol"
    )
    source_id: str = Field(..., description="Source item UUID")
    target_type: str = Field(
        ..., description="entity, context, log, job, agent, file, protocol"
    )
    target_id: str = Field(..., description="Target item UUID")
    max_hops: int = Field(
        default=6, ge=1, le=MAX_GRAPH_HOPS, description="Max hop depth"
    )

    @field_validator("source_type", "target_type", mode="before")
    @classmethod
    def _clean_graph_types(cls, v: str | None) -> str | None:
        """Handle clean graph types.

        Args:
            v: Input parameter for _clean_graph_types.

        Returns:
            Result value from the operation.
        """

        return _validate_node_type(v)


# --- Job Input Models ---


class CreateJobInput(BaseModel):
    """Input payload for creating a job."""

    model_config = ConfigDict(extra="forbid")

    title: str = Field(..., description="Job title")
    description: str | None = Field(default=None, description="Job description")
    job_type: str | None = Field(default=None, description="Job type classification")
    assigned_to: str | None = Field(default=None, description="Assignee entity UUID")
    agent_id: str | None = Field(default=None, description="Agent UUID")
    priority: str = Field(default="medium", description="low, medium, high, critical")
    scopes: list[str] = Field(
        default_factory=lambda: ["public"],
        description="Privacy scopes (defaults to public)",
    )
    parent_job_id: str | None = Field(
        default=None, description="Parent job ID for subtasks"
    )
    due_at: str | None = Field(default=None, description="ISO8601 due date")
    metadata: dict = Field(default_factory=dict, description="Additional metadata")

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_job_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean job metadata.

        Args:
            v: Input parameter for _clean_job_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


class GetJobInput(BaseModel):
    """Input payload for retrieving a job."""

    job_id: str = Field(..., description="Job ID (YYYYQ#-NNNN format)")


class QueryJobsInput(BaseModel):
    """Input payload for searching jobs."""

    status_names: list[str] = Field(
        default_factory=list, description="Filter by status names"
    )
    assigned_to: str | None = Field(default=None, description="Filter by assignee UUID")
    agent_id: str | None = Field(default=None, description="Filter by agent UUID")
    priority: str | None = Field(default=None, description="Filter by priority")
    due_before: str | None = Field(
        default=None, description="ISO8601 date for due_at filter"
    )
    due_after: str | None = Field(
        default=None, description="ISO8601 date for due_at filter"
    )
    overdue_only: bool = Field(
        default=False, description="Only overdue incomplete jobs"
    )
    parent_job_id: str | None = Field(default=None, description="Filter by parent job")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )


class UpdateJobStatusInput(BaseModel):
    """Input payload for updating job status."""

    job_id: str = Field(..., description="Job ID")
    status: str = Field(..., description="New status name")
    status_reason: str | None = Field(
        default=None, description="Reason for status change"
    )
    completed_at: str | None = Field(
        default=None, description="ISO8601 completion timestamp"
    )


class UpdateJobInput(BaseModel):
    """Input payload for updating a job."""

    job_id: str = Field(..., description="Job ID")
    title: str | None = Field(default=None, description="Updated title")
    description: str | None = Field(default=None, description="Updated description")
    status: str | None = Field(default=None, description="Updated status name")
    priority: str | None = Field(default=None, description="Updated priority")
    assigned_to: str | None = Field(default=None, description="Updated assignee UUID")
    due_at: str | None = Field(default=None, description="ISO8601 due date")
    metadata: dict | None = Field(default=None, description="Updated metadata")

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_update_job_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean update job metadata.

        Args:
            v: Input parameter for _clean_update_job_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)


class CreateSubtaskInput(BaseModel):
    """Input payload for creating a subtask."""

    parent_job_id: str = Field(..., description="Parent job ID")
    title: str = Field(..., description="Subtask title")
    description: str | None = Field(default=None, description="Subtask description")
    priority: str = Field(default="medium", description="low, medium, high, critical")
    due_at: str | None = Field(default=None, description="ISO8601 due date")


# --- File Input Models ---


class CreateFileInput(BaseModel):
    """Input payload for creating a file record."""

    filename: str = Field(..., description="File name")
    uri: str | None = Field(default=None, description="Canonical file URI")
    file_path: str | None = Field(default=None, description="Legacy file path fallback")
    mime_type: str | None = Field(default=None, description="MIME type")
    size_bytes: int | None = Field(default=None, description="File size in bytes")
    checksum: str | None = Field(default=None, description="Checksum hash")
    status: str = Field(default="active", description="Status name")
    tags: list[str] = Field(default_factory=list, description="File tags")
    metadata: dict = Field(default_factory=dict, description="Additional metadata")

    @field_validator("filename", mode="before")
    @classmethod
    def _clean_filename(cls, v: str | None) -> str | None:
        """Handle clean filename.

        Args:
            v: Input parameter for _clean_filename.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("uri", "file_path", mode="before")
    @classmethod
    def _clean_file_location(cls, v: str | None) -> str | None:
        """Handle clean file location.

        Args:
            v: Input parameter for _clean_file_location.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_file_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean file tags.

        Args:
            v: Input parameter for _clean_file_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_file_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean file metadata.

        Args:
            v: Input parameter for _clean_file_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)

    @model_validator(mode="after")
    def _ensure_file_location(self) -> Self:
        """Handle ensure file location.

        Returns:
            Result value from the operation.
        """

        if not self.uri and not self.file_path:
            raise ValueError("uri or file_path is required")
        if not self.uri and self.file_path:
            self.uri = self.file_path
        if not self.file_path and self.uri:
            self.file_path = self.uri
        return self


class GetFileInput(BaseModel):
    """Input payload for retrieving a file record."""

    file_id: str = Field(..., description="File UUID")


class QueryFilesInput(BaseModel):
    """Input payload for listing files."""

    tags: list[str] = Field(default_factory=list, description="Tag filters")
    mime_type: str | None = Field(default=None, description="Filter by MIME type")
    status_category: str = Field(default="active", description="active or archived")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    offset: int = Field(default=0, description="Pagination offset")

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_file_query_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean file query tags.

        Args:
            v: Input parameter for _clean_file_query_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)


class UpdateFileInput(BaseModel):
    """Input payload for updating a file record."""

    file_id: str = Field(..., description="File UUID")
    filename: str | None = Field(default=None, description="File name")
    uri: str | None = Field(default=None, description="Canonical file URI")
    file_path: str | None = Field(
        default=None,
        description="Legacy file path fallback",
    )
    mime_type: str | None = Field(default=None, description="MIME type")
    size_bytes: int | None = Field(default=None, description="File size in bytes")
    checksum: str | None = Field(default=None, description="Checksum hash")
    status: str | None = Field(default=None, description="Status name")
    tags: list[str] | None = Field(default=None, description="File tags")
    metadata: dict | None = Field(default=None, description="Additional metadata")

    @field_validator("filename", mode="before")
    @classmethod
    def _clean_update_filename(cls, v: str | None) -> str | None:
        """Handle clean update filename.

        Args:
            v: Input parameter for _clean_update_filename.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("uri", "file_path", mode="before")
    @classmethod
    def _clean_update_file_location(cls, v: str | None) -> str | None:
        """Handle clean update file location.

        Args:
            v: Input parameter for _clean_update_file_location.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_update_file_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean update file tags.

        Args:
            v: Input parameter for _clean_update_file_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_update_file_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean update file metadata.

        Args:
            v: Input parameter for _clean_update_file_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)

    @model_validator(mode="after")
    def _sync_update_file_location(self) -> Self:
        """Handle sync update file location.

        Returns:
            Result value from the operation.
        """

        if self.uri is None and self.file_path is not None:
            self.uri = self.file_path
        if self.file_path is None and self.uri is not None:
            self.file_path = self.uri
        return self


class AttachFileInput(BaseModel):
    """Input payload for attaching a file to another record."""

    file_id: str = Field(..., description="File UUID")
    target_id: str = Field(..., description="Target record id")
    relationship_type: str = Field(default="has-file", description="Relationship type")


# --- Protocol Input Models ---


class GetProtocolInput(BaseModel):
    """Input payload for retrieving a protocol."""

    protocol_name: str = Field(..., description="Protocol name (unique identifier)")


class QueryProtocolsInput(BaseModel):
    """Input payload for querying protocols."""

    status_category: str | None = Field(default=None, description="active or archived")
    protocol_type: str | None = Field(default=None, description="Protocol type filter")
    search: str | None = Field(default=None, description="Name/title search")
    limit: int = Field(
        default=50, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )


class CreateProtocolInput(BaseModel):
    """Input payload for creating a protocol."""

    name: str = Field(..., description="Protocol name (unique identifier)")
    title: str = Field(..., description="Protocol title")
    version: str | None = Field(default=None, description="Protocol version")
    content: str = Field(..., description="Protocol content")
    protocol_type: str | None = Field(default=None, description="Protocol type")
    applies_to: list[str] = Field(
        default_factory=list, description="Applies-to categories"
    )
    status: str = Field(default="active", description="Status name")
    tags: list[str] = Field(default_factory=list, description="Tags")
    metadata: dict = Field(default_factory=dict, description="Metadata")
    source_path: str | None = Field(default=None, description="Source path")
    trusted: bool = Field(default=False, description="Trusted for prompt use")

    @field_validator("name", "title", "protocol_type", mode="before")
    @classmethod
    def _clean_protocol_text(cls, v: str | None) -> str | None:
        """Handle clean protocol text.

        Args:
            v: Input parameter for _clean_protocol_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_protocol_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean protocol tags.

        Args:
            v: Input parameter for _clean_protocol_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_protocol_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean protocol metadata.

        Args:
            v: Input parameter for _clean_protocol_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)

    @field_validator("source_path", mode="before")
    @classmethod
    def _clean_protocol_source_path(cls, v: str | None) -> str | None:
        """Handle clean protocol source path.

        Args:
            v: Input parameter for _clean_protocol_source_path.

        Returns:
            Result value from the operation.
        """

        return _sanitize_source_path(v)


class UpdateProtocolInput(BaseModel):
    """Input payload for updating a protocol."""

    name: str = Field(..., description="Protocol name (unique identifier)")
    title: str | None = Field(default=None, description="Protocol title")
    version: str | None = Field(default=None, description="Protocol version")
    content: str | None = Field(default=None, description="Protocol content")
    protocol_type: str | None = Field(default=None, description="Protocol type")
    applies_to: list[str] | None = Field(default=None, description="Applies-to list")
    status: str | None = Field(default=None, description="Status name")
    tags: list[str] | None = Field(default=None, description="Tags")
    metadata: dict | None = Field(default=None, description="Metadata")
    source_path: str | None = Field(default=None, description="Source path")
    trusted: bool | None = Field(default=None, description="Trusted for prompt use")

    @field_validator("name", "title", "protocol_type", mode="before")
    @classmethod
    def _clean_update_protocol_text(cls, v: str | None) -> str | None:
        """Handle clean update protocol text.

        Args:
            v: Input parameter for _clean_update_protocol_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("tags", mode="before")
    @classmethod
    def _clean_update_protocol_tags(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean update protocol tags.

        Args:
            v: Input parameter for _clean_update_protocol_tags.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("metadata", mode="before")
    @classmethod
    def _clean_update_protocol_metadata(cls, v: dict | None) -> dict | None:
        """Handle clean update protocol metadata.

        Args:
            v: Input parameter for _clean_update_protocol_metadata.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)

    @field_validator("source_path", mode="before")
    @classmethod
    def _clean_update_protocol_source_path(cls, v: str | None) -> str | None:
        """Handle clean update protocol source path.

        Args:
            v: Input parameter for _clean_update_protocol_source_path.

        Returns:
            Result value from the operation.
        """

        return _sanitize_source_path(v)


# --- Agent Input Models ---


class GetAgentInfoInput(BaseModel):
    """Input payload for retrieving agent configuration."""

    name: str = Field(..., description="Agent name to retrieve")


class GetAgentInput(BaseModel):
    """Input payload for retrieving a single agent."""

    name: str = Field(..., description="Agent name to retrieve")


class ListAgentsInput(BaseModel):
    """Input payload for listing agents."""

    status_category: str = Field(default="active", description="active or archived")


class UpdateAgentInput(BaseModel):
    """Input payload for updating an agent."""

    agent_id: str = Field(..., description="Agent UUID")
    description: str | None = Field(default=None, description="Updated description")
    requires_approval: bool | None = Field(
        default=None, description="Updated trust mode"
    )
    scopes: list[str] | None = Field(default=None, description="Updated scope names")

    @field_validator("description", mode="before")
    @classmethod
    def _clean_update_agent_description(cls, v: str | None) -> str | None:
        """Handle clean update agent description.

        Args:
            v: Input parameter for _clean_update_agent_description.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)


class AgentEnrollStartInput(BaseModel):
    """Input payload for MCP bootstrap enrollment start."""

    name: str = Field(..., description="Unique agent name")
    description: str | None = Field(default=None, description="Optional description")
    requested_scopes: list[str] = Field(
        default_factory=lambda: ["public"], description="Requested scope names"
    )
    requested_requires_approval: bool = Field(
        default=True, description="Requested trust mode"
    )
    capabilities: list[str] = Field(
        default_factory=list, description="Optional capability tags"
    )

    @field_validator("name", "description", mode="before")
    @classmethod
    def _clean_enroll_text(cls, v: str | None) -> str | None:
        """Handle clean enroll text.

        Args:
            v: Input parameter for _clean_enroll_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("requested_scopes", mode="before")
    @classmethod
    def _clean_enroll_scopes(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean enroll scopes.

        Args:
            v: Input parameter for _clean_enroll_scopes.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)

    @field_validator("capabilities", mode="before")
    @classmethod
    def _clean_enroll_capabilities(cls, v: list[str] | None) -> list[str] | None:
        """Handle clean enroll capabilities.

        Args:
            v: Input parameter for _clean_enroll_capabilities.

        Returns:
            Result value from the operation.
        """

        return _sanitize_tags(v)


class AgentEnrollWaitInput(BaseModel):
    """Input payload for MCP enrollment long-poll wait."""

    registration_id: str = Field(..., description="Enrollment registration UUID")
    enrollment_token: str = Field(..., description="Enrollment token")
    timeout_seconds: int = Field(default=20, ge=1, le=60, description="Wait timeout")

    @field_validator("registration_id", "enrollment_token", mode="before")
    @classmethod
    def _clean_wait_text(cls, v: str | None) -> str | None:
        """Handle clean wait text.

        Args:
            v: Input parameter for _clean_wait_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)


class AgentEnrollRedeemInput(BaseModel):
    """Input payload for one-time enrollment redemption."""

    registration_id: str = Field(..., description="Enrollment registration UUID")
    enrollment_token: str = Field(..., description="Enrollment token")

    @field_validator("registration_id", "enrollment_token", mode="before")
    @classmethod
    def _clean_redeem_text(cls, v: str | None) -> str | None:
        """Handle clean redeem text.

        Args:
            v: Input parameter for _clean_redeem_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)


class AgentAuthAttachInput(BaseModel):
    """Input payload for attaching an API key to current MCP session."""

    api_key: str = Field(..., description="Agent API key to attach for this session")

    @field_validator("api_key", mode="before")
    @classmethod
    def _clean_api_key(cls, v: str | None) -> str | None:
        """Handle clean api key.

        Args:
            v: Input parameter for _clean_api_key.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)


class LoginInput(BaseModel):
    """Input payload for user login key bootstrap."""

    username: str = Field(..., description="Username for entity login")

    @field_validator("username", mode="before")
    @classmethod
    def _clean_login_username(cls, v: str | None) -> str | None:
        """Handle clean login username.

        Args:
            v: Input parameter for _clean_login_username.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)


class CreateAPIKeyInput(BaseModel):
    """Input payload for creating an API key."""

    entity_id: str | None = Field(
        default=None, description="Entity UUID owner (admin path)"
    )
    name: str = Field(default="default", description="Friendly key name")

    @field_validator("name", mode="before")
    @classmethod
    def _clean_key_name(cls, v: str | None) -> str | None:
        """Handle clean key name.

        Args:
            v: Input parameter for _clean_key_name.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)


class ListAPIKeysInput(BaseModel):
    """Input payload for listing keys for one entity."""

    entity_id: str | None = Field(
        default=None, description="Entity UUID owner (admin path)"
    )


class ListAllKeysInput(BaseModel):
    """Input payload for listing all active API keys."""

    limit: int = Field(default=500, ge=1, le=5000, description="Max rows")
    offset: int = Field(default=0, ge=0, description="Pagination offset")


class RevokeKeyInput(BaseModel):
    """Input payload for revoking an API key."""

    key_id: str = Field(..., description="API key UUID")
    entity_id: str | None = Field(
        default=None, description="Entity UUID owner for scoped revoke"
    )


class ExportDataInput(BaseModel):
    """Input payload for exporting workspace data."""

    resource: str = Field(
        ...,
        description="entities, context, relationships, jobs, or snapshot",
    )
    format: str = Field(default="json", description="json or csv")
    params: dict = Field(default_factory=dict, description="Resource filter params")

    @field_validator("resource", mode="before")
    @classmethod
    def _clean_export_resource(cls, v: str | None) -> str:
        """Handle clean export resource.

        Args:
            v: Input parameter for _clean_export_resource.

        Returns:
            Result value from the operation.
        """

        value = _sanitize_text(v)
        if value not in {"entities", "context", "relationships", "jobs", "snapshot"}:
            raise ValueError("Invalid export resource")
        return value

    @field_validator("format", mode="before")
    @classmethod
    def _clean_export_format(cls, v: str | None) -> str:
        """Handle clean export format.

        Args:
            v: Input parameter for _clean_export_format.

        Returns:
            Result value from the operation.
        """

        value = _sanitize_text(v) or "json"
        lowered = value.lower()
        if lowered not in {"json", "csv"}:
            raise ValueError("Format must be json or csv")
        return lowered

    @field_validator("params", mode="before")
    @classmethod
    def _clean_export_params(cls, v: dict | None) -> dict:
        """Handle clean export params.

        Args:
            v: Input parameter for _clean_export_params.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v) or {}


# --- Taxonomy Input Models ---


class ListTaxonomyInput(BaseModel):
    """Input payload for listing taxonomy rows."""

    kind: str = Field(default="scopes", description="Taxonomy kind")
    include_inactive: bool = Field(default=False, description="Include inactive rows")
    search: str | None = Field(default=None, description="Name search filter")
    limit: int = Field(
        default=200, ge=1, le=MAX_PAGE_LIMIT, description="Max results to return"
    )
    offset: int = Field(default=0, description="Pagination offset")

    @field_validator("kind", mode="before")
    @classmethod
    def _clean_kind(cls, v: str | None) -> str:
        """Handle clean kind.

        Args:
            v: Input parameter for _clean_kind.

        Returns:
            Result value from the operation.
        """

        return _validate_taxonomy_kind(v)

    @field_validator("search", mode="before")
    @classmethod
    def _clean_search(cls, v: str | None) -> str | None:
        """Handle clean search.

        Args:
            v: Input parameter for _clean_search.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)


class CreateTaxonomyInput(BaseModel):
    """Input payload for creating taxonomy rows."""

    kind: str = Field(..., description="Taxonomy kind")
    name: str = Field(..., description="Taxonomy row name")
    description: str | None = Field(default=None, description="Optional description")
    metadata: dict | None = Field(default=None, description="Optional metadata object")
    is_symmetric: bool | None = Field(
        default=None, description="Relationship symmetry flag"
    )
    value_schema: dict | None = Field(default=None, description="Log type value schema")

    @field_validator("kind", mode="before")
    @classmethod
    def _clean_kind(cls, v: str | None) -> str:
        """Handle clean kind.

        Args:
            v: Input parameter for _clean_kind.

        Returns:
            Result value from the operation.
        """

        return _validate_taxonomy_kind(v)

    @field_validator("name", "description", mode="before")
    @classmethod
    def _clean_text(cls, v: str | None) -> str | None:
        """Handle clean text.

        Args:
            v: Input parameter for _clean_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("metadata", "value_schema", mode="before")
    @classmethod
    def _clean_json_objects(cls, v: dict | None) -> dict | None:
        """Handle clean json objects.

        Args:
            v: Input parameter for _clean_json_objects.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)

    @model_validator(mode="after")
    def _validate_kind_fields(self) -> Self:
        """Handle validate kind fields.

        Returns:
            Result value from the operation.
        """

        if self.kind != "relationship-types" and self.is_symmetric is not None:
            raise ValueError("is_symmetric is only valid for relationship-types")
        if self.kind != "log-types" and self.value_schema is not None:
            raise ValueError("value_schema is only valid for log-types")
        return self


class UpdateTaxonomyInput(BaseModel):
    """Input payload for updating taxonomy rows."""

    kind: str = Field(..., description="Taxonomy kind")
    item_id: str = Field(..., description="Taxonomy row UUID")
    name: str | None = Field(default=None, description="Optional name")
    description: str | None = Field(default=None, description="Optional description")
    metadata: dict | None = Field(default=None, description="Optional metadata object")
    is_symmetric: bool | None = Field(
        default=None, description="Relationship symmetry flag"
    )
    value_schema: dict | None = Field(default=None, description="Log type value schema")

    @field_validator("kind", mode="before")
    @classmethod
    def _clean_kind(cls, v: str | None) -> str:
        """Handle clean kind.

        Args:
            v: Input parameter for _clean_kind.

        Returns:
            Result value from the operation.
        """

        return _validate_taxonomy_kind(v)

    @field_validator("name", "description", "item_id", mode="before")
    @classmethod
    def _clean_text(cls, v: str | None) -> str | None:
        """Handle clean text.

        Args:
            v: Input parameter for _clean_text.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

    @field_validator("metadata", "value_schema", mode="before")
    @classmethod
    def _clean_json_objects(cls, v: dict | None) -> dict | None:
        """Handle clean json objects.

        Args:
            v: Input parameter for _clean_json_objects.

        Returns:
            Result value from the operation.
        """

        return _sanitize_metadata(v)

    @model_validator(mode="after")
    def _validate_kind_fields(self) -> Self:
        """Handle validate kind fields.

        Returns:
            Result value from the operation.
        """

        if self.kind != "relationship-types" and self.is_symmetric is not None:
            raise ValueError("is_symmetric is only valid for relationship-types")
        if self.kind != "log-types" and self.value_schema is not None:
            raise ValueError("value_schema is only valid for log-types")
        return self


class ToggleTaxonomyInput(BaseModel):
    """Input payload for archive/activate taxonomy operations."""

    kind: str = Field(..., description="Taxonomy kind")
    item_id: str = Field(..., description="Taxonomy row UUID")

    @field_validator("kind", mode="before")
    @classmethod
    def _clean_kind(cls, v: str | None) -> str:
        """Handle clean kind.

        Args:
            v: Input parameter for _clean_kind.

        Returns:
            Result value from the operation.
        """

        return _validate_taxonomy_kind(v)

    @field_validator("item_id", mode="before")
    @classmethod
    def _clean_item_id(cls, v: str | None) -> str | None:
        """Handle clean item id.

        Args:
            v: Input parameter for _clean_item_id.

        Returns:
            Result value from the operation.
        """

        return _sanitize_text(v)

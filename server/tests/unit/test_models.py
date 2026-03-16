"""Unit tests for pydantic models (no DB needed)."""

# Standard Library
from datetime import date, datetime, timezone

# Third-Party
import pytest
from pydantic import ValidationError

from nebula_mcp.models import (
    AgentAuthAttachInput,
    AgentEnrollStartInput,
    AgentEnrollRedeemInput,
    AgentEnrollWaitInput,
    BulkUpdateEntityTagsInput,
    CreateContextInput,
    CreateLogInput,
    CreateJobInput,
    CreateFileInput,
    CreateAPIKeyInput,
    CreateProtocolInput,
    CreateEntityInput,
    CreateRelationshipInput,
    CreateTaxonomyInput,
    ExportDataInput,
    GetAgentInput,
    GetRelationshipsInput,
    GraphNeighborsInput,
    GraphShortestPathInput,
    ListAgentsInput,
    ListTaxonomyInput,
    LoginInput,
    QueryContextInput,
    QueryEntitiesInput,
    QueryFilesInput,
    QueryJobsInput,
    QueryLogsInput,
    QueryRelationshipsInput,
    RejectRequestInput,
    SemanticSearchInput,
    UpdateContextInput,
    UpdateAgentInput,
    UpdateEntityInput,
    UpdateFileInput,
    UpdateJobInput,
    UpdateLogInput,
    UpdateProtocolInput,
    UpdateTaxonomyInput,
    ToggleTaxonomyInput,
    _sanitize_metadata,
    _sanitize_source_path,
    _sanitize_tags,
    _sanitize_text,
    _validate_node_type,
    _strip_control,
    _validate_taxonomy_kind,
    parse_optional_datetime,
    validate_metadata_payload,
)

pytestmark = pytest.mark.unit


# --- CreateEntityInput ---


class TestCreateEntityInput:
    """Tests for the CreateEntityInput model."""

    def test_valid_minimal(self):
        """Accept a valid payload with all required fields."""

        inp = CreateEntityInput(
            name="Alice",
            type="person",
            status="active",
            scopes=["public"],
        )
        assert inp.name == "Alice"

    def test_missing_name_raises(self):
        """Raise ValidationError when name is omitted."""

        with pytest.raises(ValidationError):
            CreateEntityInput(
                type="person",
                status="active",
                scopes=["public"],
            )

    def test_defaults_tags_empty_list(self):
        """Default tags to an empty list."""

        inp = CreateEntityInput(
            name="Alice",
            type="person",
            status="active",
            scopes=["public"],
        )
        assert inp.tags == []


# --- Query/Filter Models ---


class TestQueryFilterModels:
    """Tests for query and filter input models."""

    def test_query_entities_defaults(self):
        """QueryEntitiesInput has correct defaults."""

        q = QueryEntitiesInput()
        assert q.status_category == "active"
        assert q.limit == 50
        assert q.offset == 0
        assert q.tags == []
        assert q.scopes == []

    def test_query_jobs_defaults(self):
        """QueryJobsInput has correct defaults."""

        q = QueryJobsInput()
        assert q.limit == 50
        assert q.overdue_only is False
        assert q.status_names == []

    def test_create_job_rejects_unknown_status_field(self):
        """CreateJobInput should reject unsupported status field payloads."""

        with pytest.raises(ValidationError):
            CreateJobInput(
                title="Queue Probe",
                priority="medium",
                status="todo",
            )

    def test_update_entity_only_required(self):
        """UpdateEntityInput only requires entity_id."""

        u = UpdateEntityInput(entity_id="abc-123")
        assert u.tags is None
        assert u.status is None
        assert u.status_reason is None

    def test_list_agents_default(self):
        """ListAgentsInput defaults status_category to active."""

        la = ListAgentsInput()
        assert la.status_category == "active"


# --- Approval Models ---


class TestApprovalModels:
    """Tests for approval workflow input models."""

    def test_reject_requires_review_notes(self):
        """RejectRequestInput requires review_notes field."""

        with pytest.raises(ValidationError):
            RejectRequestInput(
                approval_id="some-uuid",
                reviewed_by="reviewer-uuid",
            )


# --- Helper + Edge Branch Models ---


class TestModelSanitizerHelpers:
    """Direct helper branch coverage for model sanitizers."""

    def test_strip_control_removes_bidi_and_control_chars(self):
        """Bidi and generic control chars should be dropped from text."""

        cleaned = _strip_control("ab\u202ecd\x00ef")
        assert cleaned == "abcdef"

    def test_strip_control_rejects_non_string_values(self):
        """Strip helper should fail fast on non-string payloads."""

        with pytest.raises(ValueError, match="Expected string"):
            _strip_control(123)  # type: ignore[arg-type]

    def test_validate_metadata_payload_rejects_non_object(self):
        """Metadata payload must be a dict object."""

        with pytest.raises(ValueError, match="Metadata must be an object"):
            validate_metadata_payload(["bad"])  # type: ignore[arg-type]

    def test_validate_metadata_payload_rejects_nested_visibility_key(self):
        """Nested visibility keys should be rejected as unsupported."""

        with pytest.raises(ValueError, match="Metadata key 'visibility' is not supported"):
            validate_metadata_payload({"nested": {"visibility": "private"}})

    def test_validate_metadata_payload_rejects_banned_key_inside_list_items(self):
        """Banned metadata keys should be rejected even when nested in list values."""

        with pytest.raises(ValueError, match="Metadata key 'constructor' is not allowed"):
            validate_metadata_payload({"nested": [{"constructor": "bad"}]})

    def test_validate_taxonomy_kind_none_raises_required_error(self):
        """Taxonomy validator should reject missing kind values."""

        with pytest.raises(ValueError, match="Taxonomy kind is required"):
            _validate_taxonomy_kind(None)

    def test_validate_taxonomy_kind_rejects_unknown_value(self):
        """Taxonomy validator should reject unsupported kind names."""

        with pytest.raises(ValueError, match="Invalid taxonomy kind"):
            _validate_taxonomy_kind("bad-kind")

    def test_parse_optional_datetime_handles_datetime_date_and_empty_text(self):
        """Datetime parser should accept datetime/date and blank text."""

        now = datetime(2026, 1, 1, 12, 0, tzinfo=timezone.utc)
        assert parse_optional_datetime(now, "ts") == now
        assert parse_optional_datetime(date(2026, 1, 1), "ts") == datetime(
            2026, 1, 1, 0, 0, tzinfo=timezone.utc
        )
        assert parse_optional_datetime("   ", "ts") is None

    def test_parse_optional_datetime_handles_z_suffix_and_invalid_values(self):
        """Datetime parser should normalize trailing Z and reject bad timestamps."""

        parsed = parse_optional_datetime("2026-01-01T12:34:56Z", "ts")
        assert parsed == datetime(2026, 1, 1, 12, 34, 56, tzinfo=timezone.utc)
        with pytest.raises(ValueError, match="Invalid ts: expected ISO8601 datetime"):
            parse_optional_datetime("not-a-timestamp", "ts")

    def test_parse_optional_datetime_rejects_invalid_offset_values(self):
        """Datetime parser should reject malformed timezone offsets."""

        with pytest.raises(ValueError, match="Invalid ts: expected ISO8601 datetime"):
            parse_optional_datetime("2026-01-01T12:00:00+25:00", "ts")

    def test_parse_optional_datetime_handles_iso_date_and_offset_datetime(self):
        """Datetime parser should accept date-only strings and explicit offsets."""

        date_only = parse_optional_datetime("2026-01-02", "ts")
        assert date_only == datetime(2026, 1, 2, 0, 0)

        offset = parse_optional_datetime("2026-01-02T09:30:00+02:30", "ts")
        assert offset == datetime.fromisoformat("2026-01-02T09:30:00+02:30")

    def test_private_sanitizers_normalize_control_only_source_and_node_type(self):
        """Source-path and node-type sanitizers should strip control chars safely."""

        assert _sanitize_source_path(" \u202e\x00 ") is None
        assert _validate_node_type(" entity\u202e ") == "entity"

    def test_private_sanitizers_keep_clean_source_path_and_strip_control_tags(self):
        """Source-path and tag sanitizers should keep clean values and drop empty tags."""

        assert _sanitize_source_path(" /tmp/nebula.log ") == "/tmp/nebula.log"
        assert _sanitize_tags(["\u202e", "alpha", ""]) == ["alpha"]

    def test_validate_metadata_payload_rejects_other_banned_keys(self):
        """Metadata payload should reject __proto__ and prototype keys recursively."""

        with pytest.raises(ValueError, match="Metadata key '__proto__' is not allowed"):
            validate_metadata_payload({"nested": {"__proto__": {"x": 1}}})
        with pytest.raises(ValueError, match="Metadata key 'prototype' is not allowed"):
            validate_metadata_payload({"items": [{"prototype": "bad"}]})

    def test_validate_metadata_payload_rejects_top_level_prototype_key(self):
        """Top-level banned keys should be rejected with a clear error."""

        with pytest.raises(ValueError, match="Metadata key 'prototype' is not allowed"):
            validate_metadata_payload({"prototype": "bad"})

    def test_private_sanitizers_return_none_for_none_inputs(self):
        """Private sanitizer helpers should preserve None values."""

        assert _sanitize_text(None) is None
        assert _sanitize_tags(None) is None
        assert _validate_node_type(None) is None
        assert _sanitize_metadata(None) is None
        assert _sanitize_source_path(None) is None
        assert parse_optional_datetime(None, "ts") is None


class TestModelEdgeInputs:
    """Coverage tests for low-hit input model branches."""

    def test_query_entities_cleans_type_text_and_tags(self):
        """Entity query should sanitize text fields and tag list inputs."""

        query = QueryEntitiesInput(
            type=" person\u202e ",
            search_text="\x00notes",
            tags=["a", "", "b"],
        )
        assert query.type == "person"
        assert query.search_text == "notes"
        assert query.tags == ["a", "b"]

    def test_query_entities_rejects_too_many_tags(self):
        """Entity query should reject payloads that exceed tag limit."""

        with pytest.raises(ValidationError, match="Too many tags"):
            QueryEntitiesInput(tags=[f"t{i}" for i in range(51)])

    def test_query_entities_rejects_non_list_tags_payload(self):
        """Entity query should reject non-list tag payloads."""

        with pytest.raises(ValidationError, match="Tags must be a list"):
            QueryEntitiesInput(tags="alpha")  # type: ignore[arg-type]

    def test_query_entities_rejects_oversized_tag(self):
        """Entity query should reject overly long tag strings."""

        with pytest.raises(ValidationError, match="Tag too long"):
            QueryEntitiesInput(tags=["x" * 65])

    def test_create_relationship_rejects_invalid_node_type(self):
        """Relationship input should reject unsupported node types."""

        with pytest.raises(ValidationError, match="Invalid node type"):
            CreateRelationshipInput(
                source_type="bad-node",
                source_id="s1",
                target_type="entity",
                target_id="t1",
                relationship_type="related-to",
            )

    def test_create_relationship_rejects_non_string_node_type(self):
        """Relationship input should reject non-string node type payloads."""

        with pytest.raises(ValidationError, match="Expected string"):
            CreateRelationshipInput(
                source_type=1,  # type: ignore[arg-type]
                source_id="s1",
                target_type="entity",
                target_id="t1",
                relationship_type="related-to",
            )

    def test_create_entity_blank_source_path_normalizes_to_none(self):
        """Blank source_path should normalize to None."""

        model = CreateEntityInput(
            name="Alice",
            type="person",
            status="active",
            scopes=["public"],
            source_path=" \u202e ",
        )
        assert model.source_path is None

    def test_create_entity_sanitizes_tags_when_provided(self):
        """Create entity should sanitize provided tag lists."""

        model = CreateEntityInput(
            name="Alice",
            type="person",
            status="active",
            scopes=["public"],
            tags=["alpha", "", "beta"],
        )
        assert model.tags == ["alpha", "beta"]

    def test_create_entity_rejects_non_list_tags_payload(self):
        """Create entity should reject non-list tag payloads."""

        with pytest.raises(ValidationError, match="Tags must be a list"):
            CreateEntityInput(
                name="Alice",
                type="person",
                status="active",
                scopes=["public"],
                tags="alpha",  # type: ignore[arg-type]
            )

    def test_create_entity_rejects_non_string_tag_items(self):
        """Create entity should reject tag lists with non-string entries."""

        with pytest.raises(ValidationError, match="Expected string"):
            CreateEntityInput(
                name="Alice",
                type="person",
                status="active",
                scopes=["public"],
                tags=["alpha", None],  # type: ignore[list-item]
            )

    def test_create_entity_rejects_non_string_name(self):
        """Create entity should reject non-string name payloads."""

        with pytest.raises(ValidationError, match="Expected string"):
            CreateEntityInput(
                name=123,  # type: ignore[arg-type]
                type="person",
                status="active",
                scopes=["public"],
            )

    def test_create_context_url_validator_handles_empty_and_invalid_values(self):
        """Create context URL validator should allow empty values and reject non-http."""

        model = CreateContextInput(
            title="Notes",
            source_type="article",
            content="body",
            scopes=["public"],
            url="",
        )
        assert model.url == ""
        with pytest.raises(ValidationError, match="URL must start with http:// or https://"):
            CreateContextInput(
                title="Notes",
                source_type="article",
                content="body",
                scopes=["public"],
                url="ftp://example.com",
            )

    def test_update_context_rejects_non_http_url(self):
        """Update context should enforce http/https URL prefix."""

        with pytest.raises(ValidationError, match="URL must start with http:// or https://"):
            UpdateContextInput(context_id="ctx-1", url="ftp://example.com")

    def test_context_models_reject_whitespace_only_url(self):
        """Context create/update models should reject whitespace-only URL values."""

        with pytest.raises(ValidationError, match="URL must start with http:// or https://"):
            CreateContextInput(
                title="Notes",
                source_type="article",
                content="body",
                scopes=["public"],
                url="   ",
            )

        with pytest.raises(ValidationError, match="URL must start with http:// or https://"):
            UpdateContextInput(context_id="ctx-1", url="   ")

    def test_update_context_sanitizes_title_tags_and_accepts_empty_url(self):
        """Update context should sanitize optional fields and preserve empty URL values."""

        model = UpdateContextInput(
            context_id="ctx-1",
            title="  Notes\u202e ",
            source_type=" article ",
            url="",
            tags=["alpha", "", "beta"],
        )
        assert model.title == "Notes"
        assert model.source_type == "article"
        assert model.url == ""
        assert model.tags == ["alpha", "beta"]

    def test_update_context_accepts_valid_http_url(self):
        """Update context should keep valid http/https URLs."""

        model = UpdateContextInput(context_id="ctx-1", url=" https://example.com/path ")
        assert model.url == "https://example.com/path"

    def test_create_context_rejects_non_string_url_payload(self):
        """Create context should reject non-string URL payloads without crashing."""

        with pytest.raises(ValidationError, match="URL must be a string"):
            CreateContextInput(
                title="x",
                source_type="article",
                scopes=["public"],
                url=123,  # type: ignore[arg-type]
            )

    def test_update_context_rejects_non_string_url_payload(self):
        """Update context should reject non-string URL payloads without crashing."""

        with pytest.raises(ValidationError, match="URL must be a string"):
            UpdateContextInput(context_id="ctx-1", url=123)  # type: ignore[arg-type]

    def test_query_context_sanitizes_tags(self):
        """Context query should sanitize and keep non-empty tags."""

        query = QueryContextInput(tags=["alpha", "", "beta"])
        assert query.tags == ["alpha", "beta"]

    def test_query_logs_sanitizes_tags(self):
        """Log query should sanitize and keep non-empty tags."""

        query = QueryLogsInput(tags=["ops", "", "infra"])
        assert query.tags == ["ops", "infra"]

    def test_create_file_requires_uri_or_file_path(self):
        """Create file should fail when both location fields are missing."""

        with pytest.raises(ValidationError, match="uri or file_path is required"):
            CreateFileInput(filename="x.txt")

    def test_create_file_syncs_uri_from_file_path(self):
        """Create file should mirror file_path into uri when uri is missing."""

        model = CreateFileInput(filename="x.txt", file_path="/tmp/x.txt")
        assert model.uri == "/tmp/x.txt"
        assert model.file_path == "/tmp/x.txt"

    def test_update_file_syncs_uri_from_file_path(self):
        """Update file should mirror file_path into uri when uri is missing."""

        model = UpdateFileInput(file_id="f1", file_path="/tmp/a.txt")
        assert model.uri == "/tmp/a.txt"
        assert model.file_path == "/tmp/a.txt"

    def test_update_file_syncs_file_path_from_uri(self):
        """Update file should mirror uri into file_path when file_path is missing."""

        model = UpdateFileInput(file_id="f1", uri="file:///tmp/a.txt")
        assert model.uri == "file:///tmp/a.txt"
        assert model.file_path == "file:///tmp/a.txt"

    def test_agent_enroll_start_sanitizes_capabilities(self):
        """Enroll input should sanitize capabilities through tag sanitizer."""

        model = AgentEnrollStartInput(name="agent-x", capabilities=["a", "", "b"])
        assert model.capabilities == ["a", "b"]

    def test_semantic_search_defaults_kinds_when_none(self):
        """Semantic search should fall back to default kinds for null input."""

        model = SemanticSearchInput(query="agent memory", kinds=None)
        assert model.kinds == ["entity", "context"]

    def test_semantic_search_rejects_non_list_kinds_payload(self):
        """Semantic search should reject non-list kinds payloads."""

        with pytest.raises(ValidationError, match="Kinds must be a list"):
            SemanticSearchInput(query="agent memory", kinds="entity")  # type: ignore[arg-type]

    def test_semantic_search_rejects_empty_string_kinds_payload(self):
        """Semantic search should reject falsy non-list kinds payloads."""

        with pytest.raises(ValidationError, match="Kinds must be a list"):
            SemanticSearchInput(query="agent memory", kinds="")  # type: ignore[arg-type]

    def test_semantic_search_rejects_boolean_kinds_payload(self):
        """Semantic search should reject boolean kinds payloads."""

        with pytest.raises(ValidationError, match="Kinds must be a list"):
            SemanticSearchInput(query="agent memory", kinds=False)  # type: ignore[arg-type]

    def test_semantic_search_rejects_object_kinds_payload(self):
        """Semantic search should reject object kinds payloads."""

        with pytest.raises(ValidationError, match="Kinds must be a list"):
            SemanticSearchInput(query="agent memory", kinds={"kind": "entity"})  # type: ignore[arg-type]

    def test_semantic_search_defaults_kinds_when_empty_list(self):
        """Semantic search should restore default kinds for empty-list input."""

        model = SemanticSearchInput(query="agent memory", kinds=[])
        assert model.kinds == ["entity", "context"]

    def test_semantic_search_defaults_when_kinds_are_all_invalid(self):
        """Semantic search should restore defaults when all kinds are unsupported."""

        model = SemanticSearchInput(query="agent memory", kinds=["bad", "", "  "])
        assert model.kinds == ["entity", "context"]

    def test_semantic_search_normalizes_kind_case_and_whitespace(self):
        """Semantic search should normalize kind casing and trim whitespace."""

        model = SemanticSearchInput(
            query="agent memory",
            kinds=[" Entity ", "CONTEXT", "entity"],
        )
        assert model.kinds == ["entity", "context"]

    def test_export_data_rejects_invalid_resource(self):
        """Export model should reject unknown resources."""

        with pytest.raises(ValidationError, match="Invalid export resource"):
            ExportDataInput(resource="files")

    def test_export_data_rejects_invalid_format(self):
        """Export model should reject non json/csv format values."""

        with pytest.raises(ValidationError, match="Format must be json or csv"):
            ExportDataInput(resource="entities", format="yaml")

    def test_export_data_normalizes_format_and_empty_params(self):
        """Export model should normalize format and fallback params to an empty dict."""

        model = ExportDataInput(resource="entities", format="CSV", params=None)
        assert model.format == "csv"
        assert model.params == {}

    def test_create_taxonomy_rejects_is_symmetric_for_non_relationship_kind(self):
        """Create taxonomy should gate is_symmetric to relationship-types."""

        with pytest.raises(
            ValidationError, match="is_symmetric is only valid for relationship-types"
        ):
            CreateTaxonomyInput(kind="scopes", name="public", is_symmetric=True)

    def test_create_taxonomy_rejects_value_schema_for_non_log_type(self):
        """Create taxonomy should gate value_schema to log-types."""

        with pytest.raises(ValidationError, match="value_schema is only valid for log-types"):
            CreateTaxonomyInput(kind="scopes", name="public", value_schema={"a": "b"})

    def test_update_taxonomy_rejects_is_symmetric_for_non_relationship_kind(self):
        """Update taxonomy should gate is_symmetric to relationship-types."""

        with pytest.raises(
            ValidationError, match="is_symmetric is only valid for relationship-types"
        ):
            UpdateTaxonomyInput(
                kind="scopes",
                item_id="scope-1",
                is_symmetric=True,
            )

    def test_update_taxonomy_rejects_value_schema_for_non_log_type(self):
        """Update taxonomy should gate value_schema to log-types."""

        with pytest.raises(ValidationError, match="value_schema is only valid for log-types"):
            UpdateTaxonomyInput(
                kind="scopes",
                item_id="scope-1",
                value_schema={"type": "object"},
            )

    def test_semantic_search_kinds_deduplicates_and_filters_invalid_values(self):
        """Semantic search kinds should dedupe and drop unsupported entries."""

        model = SemanticSearchInput(
            query="memory search",
            kinds=["entity", "entity", "context", "bad", ""],
        )
        assert model.kinds == ["entity", "context"]

    def test_update_entity_sanitizes_tags(self):
        """Update entity should sanitize incoming tags."""

        model = UpdateEntityInput(
            entity_id="ent-1",
            tags=["alpha", "", "beta"],
        )
        assert model.tags == ["alpha", "beta"]

    def test_context_log_and_relationship_models_sanitize_inputs(self):
        """Context, log, and relationship models should sanitize text/list/json fields."""

        context_model = CreateContextInput(
            title="  Notes\u202e  ",
            source_type=" article ",
            url=" https://example.com ",
            content="x",
            scopes=["public"],
            tags=["a", "", "b"],
        )
        assert context_model.title == "Notes"
        assert context_model.source_type == "article"
        assert context_model.url == "https://example.com"
        assert context_model.tags == ["a", "b"]

        create_log = CreateLogInput(
            log_type=" note ",
            tags=["ops", "", "infra"],
            metadata={"x": 1},
        )
        assert create_log.log_type == "note"
        assert create_log.tags == ["ops", "infra"]
        assert create_log.metadata == {"x": 1}

        query_logs = QueryLogsInput(tags=["ops", "", "infra"])
        assert query_logs.tags == ["ops", "infra"]

        update_log = UpdateLogInput(id="log-1", tags=["a", "", "b"], metadata={"ok": True})
        assert update_log.tags == ["a", "b"]
        assert update_log.metadata == {"ok": True}

        relationship = CreateRelationshipInput(
            source_type="entity",
            source_id="s1",
            target_type="job",
            target_id="t1",
            relationship_type=" related-to ",
            properties={"weight": 1},
        )
        assert relationship.relationship_type == "related-to"
        assert relationship.properties == {"weight": 1}

        get_rels = GetRelationshipsInput(source_type="entity", source_id="s1")
        assert get_rels.source_type == "entity"

        query_rels = QueryRelationshipsInput(source_type="entity", target_type="job")
        assert query_rels.source_type == "entity"
        assert query_rels.target_type == "job"

    def test_graph_job_file_and_protocol_models_sanitize_inputs(self):
        """Graph, job, file, and protocol models should sanitize typed fields."""

        neighbors = GraphNeighborsInput(source_type="entity", source_id="s1")
        assert neighbors.source_type == "entity"

        shortest = GraphShortestPathInput(
            source_type="entity",
            source_id="s1",
            target_type="job",
            target_id="t1",
        )
        assert shortest.source_type == "entity"
        assert shortest.target_type == "job"

        create_job = CreateJobInput(title="Queue task")
        assert create_job.title == "Queue task"

        update_job = UpdateJobInput(job_id="2026Q1-0001")
        assert update_job.job_id == "2026Q1-0001"

        create_file = CreateFileInput(
            filename="  report.txt  ",
            uri=" file:///tmp/r.txt ",
            tags=["ops", "", "infra"],
            metadata={"safe": True},
        )
        assert create_file.filename == "report.txt"
        assert create_file.uri == "file:///tmp/r.txt"
        assert create_file.file_path == "file:///tmp/r.txt"
        assert create_file.tags == ["ops", "infra"]
        assert create_file.metadata == {"safe": True}

        query_files = QueryFilesInput(tags=["ops", "", "infra"])
        assert query_files.tags == ["ops", "infra"]

        update_file = UpdateFileInput(
            file_id="f-1",
            filename="  report-v2.txt  ",
            uri=" file:///tmp/r2.txt ",
            tags=["a", "", "b"],
            metadata={"safe": True},
        )
        assert update_file.filename == "report-v2.txt"
        assert update_file.uri == "file:///tmp/r2.txt"
        assert update_file.file_path == "file:///tmp/r2.txt"
        assert update_file.tags == ["a", "b"]
        assert update_file.metadata == {"safe": True}

        create_protocol = CreateProtocolInput(
            name=" proto-a ",
            title=" Protocol A ",
            content="body",
            protocol_type=" guide ",
            tags=["a", "", "b"],
            metadata={"safe": True},
            source_path=" /tmp/proto-a.md ",
        )
        assert create_protocol.name == "proto-a"
        assert create_protocol.title == "Protocol A"
        assert create_protocol.protocol_type == "guide"
        assert create_protocol.tags == ["a", "b"]
        assert create_protocol.metadata == {"safe": True}
        assert create_protocol.source_path == "/tmp/proto-a.md"

        update_protocol = UpdateProtocolInput(
            name=" proto-a ",
            title=" Protocol A2 ",
            protocol_type=" guide ",
            tags=["x", "", "y"],
            metadata={"safe": True},
            source_path=" /tmp/proto-a2.md ",
        )
        assert update_protocol.name == "proto-a"
        assert update_protocol.title == "Protocol A2"
        assert update_protocol.protocol_type == "guide"
        assert update_protocol.tags == ["x", "y"]
        assert update_protocol.metadata == {"safe": True}
        assert update_protocol.source_path == "/tmp/proto-a2.md"

    def test_agent_and_auth_models_sanitize_text_fields(self):
        """Agent and auth-related models should sanitize text fields consistently."""

        get_agent = GetAgentInput(agent_id="  agent-1  ")
        assert get_agent.agent_id == "agent-1"

        update_agent = UpdateAgentInput(agent_id="a-1", description="  helper\u202e ")
        assert update_agent.description == "helper"

        enroll_start = AgentEnrollStartInput(
            name=" bot-a ",
            description=" helper ",
            requested_scopes=["public", "", "admin"],
            capabilities=["read", "", "write"],
        )
        assert enroll_start.name == "bot-a"
        assert enroll_start.description == "helper"
        assert enroll_start.requested_scopes == ["public", "admin"]
        assert enroll_start.capabilities == ["read", "write"]

        wait_model = AgentEnrollWaitInput(
            registration_id=" reg-1 ",
            enrollment_token=" tok-1 ",
        )
        assert wait_model.registration_id == "reg-1"
        assert wait_model.enrollment_token == "tok-1"

        redeem_model = AgentEnrollRedeemInput(
            registration_id=" reg-2 ",
            enrollment_token=" tok-2 ",
        )
        assert redeem_model.registration_id == "reg-2"
        assert redeem_model.enrollment_token == "tok-2"

        auth_attach = AgentAuthAttachInput(api_key=" nbl_test ")
        assert auth_attach.api_key == "nbl_test"

        login = LoginInput(username="  alxx ")
        assert login.username == "alxx"

        create_key = CreateAPIKeyInput(name="  default key ")
        assert create_key.name == "default key"

    def test_taxonomy_and_bulk_models_sanitize_and_validate_positive_paths(self):
        """Taxonomy and bulk update models should handle happy-path sanitization."""

        bulk_tags = BulkUpdateEntityTagsInput(entity_ids=["e1"], tags=["a", "", "b"], op="add")
        assert bulk_tags.tags == ["a", "b"]

        list_taxonomy = ListTaxonomyInput(kind="scopes", search="  visibility ")
        assert list_taxonomy.kind == "scopes"
        assert list_taxonomy.search == "visibility"

        create_taxonomy = CreateTaxonomyInput(
            kind="relationship-types",
            name=" related-to ",
            description=" desc ",
            metadata={"x": 1},
            is_symmetric=True,
        )
        assert create_taxonomy.name == "related-to"
        assert create_taxonomy.description == "desc"
        assert create_taxonomy.metadata == {"x": 1}
        assert create_taxonomy.is_symmetric is True

        create_log_taxonomy = CreateTaxonomyInput(
            kind="log-types",
            name=" custom-log ",
            value_schema={"type": "object"},
        )
        assert create_log_taxonomy.value_schema == {"type": "object"}

        update_taxonomy = UpdateTaxonomyInput(
            kind="relationship-types",
            item_id=" rel-1 ",
            name=" related-to ",
            description=" desc ",
            metadata={"x": 1},
            is_symmetric=False,
        )
        assert update_taxonomy.item_id == "rel-1"
        assert update_taxonomy.name == "related-to"
        assert update_taxonomy.description == "desc"
        assert update_taxonomy.metadata == {"x": 1}
        assert update_taxonomy.is_symmetric is False

        update_log_taxonomy = UpdateTaxonomyInput(
            kind="log-types",
            item_id=" log-1 ",
            value_schema={"type": "object"},
        )
        assert update_log_taxonomy.item_id == "log-1"
        assert update_log_taxonomy.value_schema == {"type": "object"}

        toggle_taxonomy = ToggleTaxonomyInput(kind="scopes", item_id=" scope-1 ")
        assert toggle_taxonomy.kind == "scopes"
        assert toggle_taxonomy.item_id == "scope-1"

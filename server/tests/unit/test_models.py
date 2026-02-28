"""Unit tests for pydantic models (no DB needed)."""

# Standard Library
from datetime import date, datetime, timezone

# Third-Party
import pytest
from pydantic import ValidationError

from nebula_mcp.models import (
    AgentEnrollStartInput,
    BaseMetadata,
    ContextSegment,
    CreateJobInput,
    CreateFileInput,
    CourseMetadata,
    CreateEntityInput,
    CreateRelationshipInput,
    CreateTaxonomyInput,
    ExportDataInput,
    FrameworkMetadata,
    IdeaMetadata,
    ListAgentsInput,
    OrganizationMetadata,
    PaperMetadata,
    PersonMetadata,
    ProjectMetadata,
    QueryContextInput,
    QueryEntitiesInput,
    QueryJobsInput,
    QueryLogsInput,
    RejectRequestInput,
    SemanticSearchInput,
    ToolMetadata,
    UniversityMetadata,
    UpdateContextInput,
    UpdateEntityInput,
    UpdateFileInput,
    UpdateJobInput,
    UpdateTaxonomyInput,
    _strip_control,
    _validate_taxonomy_kind,
    parse_optional_datetime,
    validate_entity_metadata,
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

    def test_defaults_metadata_empty_dict(self):
        """Default metadata to an empty dict."""

        inp = CreateEntityInput(
            name="Alice",
            type="person",
            status="active",
            scopes=["public"],
        )
        assert inp.metadata == {}

    def test_rejects_visibility_metadata_key(self):
        """Reject ad-hoc visibility metadata in favor of scoped segments."""

        with pytest.raises(
            ValidationError,
            match="Metadata key 'visibility' is not supported",
        ):
            CreateEntityInput(
                name="Alice",
                type="person",
                status="active",
                scopes=["public"],
                metadata={"visibility": "private"},
            )


# --- PersonMetadata Birth Date Validation ---


class TestPersonMetadataBirthDate:
    """Tests for PersonMetadata date validation logic."""

    def test_valid_full_date(self):
        """Accept a valid full birth date."""

        p = PersonMetadata(birth_year=1990, birth_month=6, birth_day=15)
        assert p.birth_day == 15

    def test_month_zero_raises(self):
        """Reject month value of 0."""

        with pytest.raises(ValidationError, match="Birth month out of range"):
            PersonMetadata(birth_month=0)

    def test_month_thirteen_raises(self):
        """Reject month value of 13."""

        with pytest.raises(ValidationError, match="Birth month out of range"):
            PersonMetadata(birth_month=13)

    def test_day_zero_raises(self):
        """Reject day value of 0."""

        with pytest.raises(ValidationError, match="Birth day out of range"):
            PersonMetadata(birth_day=0)

    def test_day_thirty_two_raises(self):
        """Reject day value of 32."""

        with pytest.raises(ValidationError, match="Birth day out of range"):
            PersonMetadata(birth_day=32)

    def test_day_31_in_30_day_month_raises(self):
        """Reject day 31 for a 30-day month like April."""

        with pytest.raises(ValidationError, match="Birth day invalid for birth month"):
            PersonMetadata(birth_month=4, birth_day=31)

    def test_feb_29_leap_year_valid(self):
        """Accept Feb 29 for leap year 2000 (divisible by 400)."""

        p = PersonMetadata(birth_year=2000, birth_month=2, birth_day=29)
        assert p.birth_day == 29

    def test_feb_29_non_leap_raises(self):
        """Reject Feb 29 for non-leap year 2023."""

        with pytest.raises(ValidationError, match="Birth day invalid for birth month"):
            PersonMetadata(birth_year=2023, birth_month=2, birth_day=29)

    def test_feb_29_century_non_leap_raises(self):
        """Reject Feb 29 for century year 1900 (not divisible by 400)."""

        with pytest.raises(ValidationError, match="Birth day invalid for birth month"):
            PersonMetadata(birth_year=1900, birth_month=2, birth_day=29)

    def test_feb_28_no_year_valid(self):
        """Accept Feb 28 when no year is provided."""

        p = PersonMetadata(birth_month=2, birth_day=28)
        assert p.birth_day == 28

    def test_feb_29_no_year_raises(self):
        """Reject Feb 29 when no year is provided (defaults to non-leap)."""

        with pytest.raises(ValidationError, match="Birth day invalid for birth month"):
            PersonMetadata(birth_month=2, birth_day=29)

    def test_partial_fields(self):
        """Accept partial birth fields (month only)."""

        p = PersonMetadata(birth_month=3)
        assert p.birth_month == 3
        assert p.birth_day is None

    def test_extra_fields_allowed(self):
        """Extra fields are allowed via BaseMetadata config."""

        p = PersonMetadata(nickname="Al")
        assert p.model_extra["nickname"] == "Al"

    def test_strict_int_rejects_float(self):
        """StrictInt fields reject float values."""

        with pytest.raises(ValidationError):
            PersonMetadata(birth_year=1990.5)


# --- Parametrized Metadata Models ---


@pytest.mark.parametrize(
    "model_cls, data",
    [
        (
            ProjectMetadata,
            {"repository": "https://github.com/x/y", "tech_stack": ["python"]},
        ),
        (ToolMetadata, {"vendor": "JetBrains", "license": "MIT"}),
        (OrganizationMetadata, {"industry": "tech", "location": "NYC"}),
        (CourseMetadata, {"institution": "MIT", "term": "Fall 2025"}),
        (IdeaMetadata, {"stage": "draft", "priority": "high"}),
        (FrameworkMetadata, {"language": "rust", "version": "1.0"}),
        (PaperMetadata, {"authors": ["Smith"], "year": 2024, "venue": "NeurIPS"}),
        (UniversityMetadata, {"country": "US", "city": "Boston"}),
    ],
    ids=[
        "project",
        "tool",
        "organization",
        "course",
        "idea",
        "framework",
        "paper",
        "university",
    ],
)
def test_metadata_model_valid(model_cls, data):
    """Accept valid data for each metadata model subclass."""

    m = model_cls(**data)
    dumped = m.model_dump(exclude_none=True)
    for key, value in data.items():
        assert dumped[key] == value


# --- BaseMetadata ---


class TestBaseMetadata:
    """Tests for the BaseMetadata model."""

    def test_context_segments(self):
        """Accept context_segments as a list of ContextSegment dicts."""

        m = BaseMetadata(
            context_segments=[
                ContextSegment(text="hello", scopes=["public"]),
            ]
        )
        assert len(m.context_segments) == 1
        assert m.context_segments[0].text == "hello"

    def test_extra_fields_allowed(self):
        """Extra fields are preserved via model_config extra=allow."""

        m = BaseMetadata(custom_key="custom_value")
        assert m.model_extra["custom_key"] == "custom_value"


# --- validate_entity_metadata ---


class TestValidateEntityMetadata:
    """Tests for the validate_entity_metadata dispatch function."""

    def test_dispatches_to_person(self):
        """Route 'person' type to PersonMetadata validation."""

        result = validate_entity_metadata("person", {"first_name": "Alice"})
        assert result["first_name"] == "Alice"

    def test_dispatches_to_project(self):
        """Route 'project' type to ProjectMetadata validation."""

        result = validate_entity_metadata("project", {"repository": "gh/x"})
        assert result["repository"] == "gh/x"

    def test_unknown_type_uses_base(self):
        """Fall back to BaseMetadata for unknown entity types."""

        result = validate_entity_metadata("spaceship", {"description": "fast"})
        assert result["description"] == "fast"

    def test_empty_metadata_returns_empty(self):
        """Return empty dict for empty metadata input."""

        result = validate_entity_metadata("person", {})
        assert result == {}

    def test_none_metadata_returns_empty(self):
        """Return empty dict for None metadata input."""

        result = validate_entity_metadata("person", None)
        assert result == {}

    def test_excludes_none_fields(self):
        """Exclude None-valued fields from the output dict."""

        result = validate_entity_metadata("person", {"first_name": "Bob"})
        assert "last_name" not in result


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
        assert u.metadata is None
        assert u.tags is None
        assert u.status is None

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

    def test_validate_metadata_payload_rejects_non_object(self):
        """Metadata payload must be a dict object."""

        with pytest.raises(ValueError, match="Metadata must be an object"):
            validate_metadata_payload(["bad"])  # type: ignore[arg-type]

    def test_validate_metadata_payload_rejects_nested_visibility_key(self):
        """Nested visibility keys should be rejected as unsupported."""

        with pytest.raises(ValueError, match="Metadata key 'visibility' is not supported"):
            validate_metadata_payload({"nested": {"visibility": "private"}})

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

    def test_person_metadata_month_and_day_none_branches(self):
        """Month/day validators should return None when value is None."""

        assert PersonMetadata._month_range(None) is None
        assert PersonMetadata._day_range(None) is None


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

    def test_update_context_rejects_non_http_url(self):
        """Update context should enforce http/https URL prefix."""

        with pytest.raises(ValidationError, match="URL must start with http:// or https://"):
            UpdateContextInput(context_id="ctx-1", url="ftp://example.com")

    def test_query_context_sanitizes_tags(self):
        """Context query should sanitize and keep non-empty tags."""

        query = QueryContextInput(tags=["alpha", "", "beta"])
        assert query.tags == ["alpha", "beta"]

    def test_query_logs_sanitizes_tags(self):
        """Log query should sanitize and keep non-empty tags."""

        query = QueryLogsInput(tags=["ops", "", "infra"])
        assert query.tags == ["ops", "infra"]

    def test_update_job_accepts_sanitized_metadata(self):
        """Update job should pass metadata through sanitizer."""

        model = UpdateJobInput(job_id="job-1", metadata={"ok": True})
        assert model.metadata == {"ok": True}

    def test_create_file_requires_uri_or_file_path(self):
        """Create file should fail when both location fields are missing."""

        with pytest.raises(ValidationError, match="uri or file_path is required"):
            CreateFileInput(filename="x.txt")

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

    def test_export_data_rejects_invalid_resource(self):
        """Export model should reject unknown resources."""

        with pytest.raises(ValidationError, match="Invalid export resource"):
            ExportDataInput(resource="files")

    def test_export_data_rejects_invalid_format(self):
        """Export model should reject non json/csv format values."""

        with pytest.raises(ValidationError, match="Format must be json or csv"):
            ExportDataInput(resource="entities", format="yaml")

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

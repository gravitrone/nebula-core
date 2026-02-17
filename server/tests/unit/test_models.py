"""Unit tests for pydantic models (no DB needed)."""

# Third-Party
import pytest
from pydantic import ValidationError

from nebula_mcp.models import (
    BaseMetadata,
    ContextSegment,
    CreateJobInput,
    CourseMetadata,
    CreateEntityInput,
    FrameworkMetadata,
    IdeaMetadata,
    ListAgentsInput,
    OrganizationMetadata,
    PaperMetadata,
    PersonMetadata,
    ProjectMetadata,
    QueryEntitiesInput,
    QueryJobsInput,
    RejectRequestInput,
    ToolMetadata,
    UniversityMetadata,
    UpdateEntityInput,
    validate_entity_metadata,
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

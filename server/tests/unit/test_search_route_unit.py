"""Unit tests for semantic search route helper branches."""

# Third-Party
import pytest
from pydantic import ValidationError

# Local
from nebula_api.routes.search import (
    SemanticSearchBody,
    _context_candidate,
    _scope_filter_ids,
)

pytestmark = pytest.mark.unit


def test_semantic_body_empty_kinds_defaults_to_allowed():
    """Empty kinds should normalize to default kinds list."""

    payload = SemanticSearchBody(query="abc", kinds=[])
    assert payload.kinds == ["entity", "context"]


def test_semantic_body_none_kinds_defaults_to_allowed():
    """None kinds should normalize to default kinds list."""

    payload = SemanticSearchBody(query="abc", kinds=None)
    assert payload.kinds == ["entity", "context"]


def test_semantic_body_rejects_non_list_kinds_payload():
    """Kinds payload should be list-only for deterministic normalization."""

    with pytest.raises(ValidationError, match="Kinds must be a list"):
        SemanticSearchBody(query="abc", kinds="entity")  # type: ignore[arg-type]


def test_semantic_body_rejects_falsy_non_list_kinds_payload():
    """Falsy non-list kinds payloads should not silently map to defaults."""

    with pytest.raises(ValidationError, match="Kinds must be a list"):
        SemanticSearchBody(query="abc", kinds=False)  # type: ignore[arg-type]


def test_semantic_body_rejects_non_string_query_payload():
    """Query payload should fail fast on non-string input."""

    with pytest.raises(ValidationError, match="Query must be a string"):
        SemanticSearchBody(query=123)  # type: ignore[arg-type]


def test_scope_filter_ids_agent_without_scopes_returns_none(mock_enums):
    """Agent callers without scopes should return None for full access."""

    auth = {"caller_type": "agent"}
    assert _scope_filter_ids(auth, mock_enums) is None


def test_context_candidate_truncates_long_snippet():
    """Context snippet should truncate long content with ellipsis."""

    long_content = "x" * 150
    row = {
        "id": "c1",
        "title": "Long Context",
        "source_type": "note",
        "content": long_content,
        "tags": [],
    }

    candidate = _context_candidate(row)

    assert candidate["snippet"].endswith("...")
    assert len(candidate["snippet"]) < len("note · " + long_content)

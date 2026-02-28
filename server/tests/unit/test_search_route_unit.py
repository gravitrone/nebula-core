"""Unit tests for semantic search route helper branches."""

# Third-Party
import pytest

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


def test_scope_filter_ids_agent_without_scopes_returns_empty_list(mock_enums):
    """Agent callers without scopes should return empty scope filter list."""

    auth = {"caller_type": "agent"}
    assert _scope_filter_ids(auth, mock_enums) == []


def test_context_candidate_truncates_long_snippet():
    """Context snippet should truncate long content with ellipsis."""

    long_content = "x" * 150
    row = {
        "id": "c1",
        "title": "Long Context",
        "source_type": "note",
        "content": long_content,
        "tags": [],
        "metadata": {},
    }

    candidate = _context_candidate(row)

    assert candidate["snippet"].endswith("...")
    assert len(candidate["snippet"]) < len("note · " + long_content)

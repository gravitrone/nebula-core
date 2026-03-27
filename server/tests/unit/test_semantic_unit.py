"""Unit tests for deterministic semantic scoring helpers."""

# Third-Party
import pytest

# Local
from nebula_mcp.semantic import _embed_text, rank_semantic_candidates

pytestmark = pytest.mark.unit


def test_embed_text_returns_zero_vector_for_empty_input():
    """Empty input should hit zero-norm path and return all-zero vector."""

    vec = _embed_text("")
    assert len(vec) > 0
    assert all(v == 0.0 for v in vec)


def test_rank_semantic_candidates_skips_empty_text_values():
    """Candidates with blank text fields should be skipped."""

    ranked = rank_semantic_candidates(
        "agent memory",
        [
            {"id": "a", "text": ""},
            {"id": "b", "text": "   "},
            {"id": "c", "text": "agent memory notes"},
        ],
    )

    assert [row["id"] for row in ranked] == ["c"]


def test_rank_semantic_candidates_applies_min_score_filter():
    """Low-score rows should be filtered out by min_score threshold."""

    ranked = rank_semantic_candidates(
        "agent memory",
        [
            {"id": "c1", "text": "agent memory notes"},
            {"id": "c2", "text": "completely unrelated zebra banana"},
        ],
        min_score=0.2,
    )

    ids = [row["id"] for row in ranked]
    assert "c1" in ids
    assert "c2" not in ids


def test_rank_semantic_candidates_respects_limit_and_adds_scores():
    """Ranking should cap by limit and include rounded score values."""

    ranked = rank_semantic_candidates(
        "agent memory",
        [
            {"id": "a", "kind": "entity", "text": "agent memory retrieval"},
            {"id": "b", "kind": "context", "text": "agent memory notes"},
            {"id": "c", "kind": "entity", "text": "agent memory history"},
        ],
        limit=2,
        min_score=0.0,
    )

    assert len(ranked) == 2
    assert all("score" in row for row in ranked)
    assert all(isinstance(row["score"], float) for row in ranked)

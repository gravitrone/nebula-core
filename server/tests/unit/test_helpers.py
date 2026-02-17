"""Unit tests for pure helper functions (no DB needed)."""

# Third-Party
import copy

import pytest

from nebula_mcp.helpers import (
    _extract_string_list,
    _normalize_change_details,
    _pending_approval_limit,
    _safe_parse_uuid,
    _to_uuid_list,
    enforce_scope_subset,
    filter_context_segments,
    generate_enrollment_token,
    verify_enrollment_token,
)

pytestmark = pytest.mark.unit


# --- filter_context_segments ---


class TestFilterContextSegments:
    """Tests for the filter_context_segments function."""

    def test_none_metadata_returns_none(self):
        """Return None when metadata is None."""

        result = filter_context_segments(None, ["public"])
        assert result is None

    def test_no_context_segments_key_returns_as_is(self):
        """Return metadata unchanged when context_segments key is absent."""

        meta = {"description": "hello"}
        result = filter_context_segments(meta, ["public"])
        assert result == {"description": "hello"}

    def test_empty_segments_returns_empty_list(self):
        """Return empty segments list when input segments are empty."""

        meta = {"context_segments": []}
        result = filter_context_segments(meta, ["public"])
        assert result["context_segments"] == []

    def test_all_match(self):
        """Keep all segments when agent has all required scopes."""

        meta = {
            "context_segments": [
                {"text": "a", "scopes": ["public"]},
                {"text": "b", "scopes": ["private"]},
            ]
        }
        result = filter_context_segments(meta, ["public", "private"])
        assert len(result["context_segments"]) == 2

    def test_none_match(self):
        """Remove all segments when agent scopes are disjoint."""

        meta = {
            "context_segments": [
                {"text": "a", "scopes": ["admin"]},
                {"text": "b", "scopes": ["sensitive"]},
            ]
        }
        result = filter_context_segments(meta, ["public"])
        assert result["context_segments"] == []

    def test_partial_match(self):
        """Keep only segments whose scopes overlap with agent scopes."""

        meta = {
            "context_segments": [
                {"text": "public note", "scopes": ["public"]},
                {"text": "secret note", "scopes": ["admin"]},
                {"text": "private note", "scopes": ["private"]},
            ]
        }
        result = filter_context_segments(meta, ["public", "private"])
        texts = [s["text"] for s in result["context_segments"]]
        assert texts == ["public note", "private note"]

    def test_multi_scope_segment(self):
        """Keep segment if any of its scopes match agent scopes."""

        meta = {
            "context_segments": [
                {"text": "multi", "scopes": ["public", "private"]},
            ]
        }
        result = filter_context_segments(meta, ["public"])
        assert len(result["context_segments"]) == 1
        assert result["context_segments"][0]["text"] == "multi"

    def test_returns_copy_original_not_mutated(self):
        """Return a copy so the original metadata dict is not mutated."""

        meta = {
            "context_segments": [
                {"text": "a", "scopes": ["public"]},
                {"text": "b", "scopes": ["admin"]},
            ]
        }
        original = copy.deepcopy(meta)
        filter_context_segments(meta, ["public"])
        assert meta == original


class TestPendingApprovalLimit:
    """Tests for pending queue cap normalization."""

    def test_default_limit_when_env_missing(self, monkeypatch):
        """Fallback to default when env is absent."""

        monkeypatch.delenv("NEBULA_MAX_PENDING_APPROVALS", raising=False)
        assert _pending_approval_limit() == 500

    def test_invalid_env_uses_default(self, monkeypatch):
        """Invalid values should not break queue cap parsing."""

        monkeypatch.setenv("NEBULA_MAX_PENDING_APPROVALS", "not-a-number")
        assert _pending_approval_limit() == 500

    def test_limit_is_clamped_to_max(self, monkeypatch):
        """Large values should be capped to hard max."""

        monkeypatch.setenv("NEBULA_MAX_PENDING_APPROVALS", "999999")
        assert _pending_approval_limit() == 5000

    def test_limit_is_clamped_to_min(self, monkeypatch):
        """Zero or negative values should be lifted to 1."""

        monkeypatch.setenv("NEBULA_MAX_PENDING_APPROVALS", "0")
        assert _pending_approval_limit() == 1


class TestHelperUtilities:
    """Coverage for helper utility edge cases."""

    def test_enforce_scope_subset_returns_allowed_when_requested_empty(self):
        """Empty request should inherit allowed set."""

        assert enforce_scope_subset([], ["public", "private"]) == ["public", "private"]

    def test_enforce_scope_subset_raises_on_missing_scope(self):
        """Unknown requested scopes should fail with clear error."""

        with pytest.raises(ValueError):
            enforce_scope_subset(["admin"], ["public"])

    def test_enrollment_token_roundtrip(self):
        """Generated enrollment token should verify against its hash."""

        raw, hashed = generate_enrollment_token()
        assert raw.startswith("nbe_")
        assert verify_enrollment_token(hashed, raw) is True
        assert verify_enrollment_token(hashed, raw + "-bad") is False

    def test_safe_parse_uuid_handles_invalid_values(self):
        """UUID parser should normalize valid inputs and reject bad values."""

        valid = "123e4567-e89b-12d3-a456-426614174000"
        assert _safe_parse_uuid(valid) == valid
        assert _safe_parse_uuid("  " + valid + "  ") == valid
        assert _safe_parse_uuid("not-a-uuid") is None
        assert _safe_parse_uuid("") is None

    def test_to_uuid_list_filters_invalid_entries(self):
        """UUID list extractor should drop invalid ids."""

        values = {
            "123e4567-e89b-12d3-a456-426614174000",
            "bad-id",
        }
        assert set(_to_uuid_list(values)) == {"123e4567-e89b-12d3-a456-426614174000"}

    def test_normalize_change_details(self):
        """Change detail parser should accept dict/json and reject invalid payloads."""

        assert _normalize_change_details({"name": "ok"}) == {"name": "ok"}
        assert _normalize_change_details('{"name":"ok"}') == {"name": "ok"}
        assert _normalize_change_details("[1,2,3]") == {}
        assert _normalize_change_details("{bad-json") == {}
        assert _normalize_change_details(None) == {}

    def test_extract_string_list_variants(self):
        """String list extraction should normalize list and string forms."""

        assert _extract_string_list(["a", " b ", ""]) == ["a", "b"]
        assert _extract_string_list(("a", "b")) == ["a", "b"]
        assert _extract_string_list({"a", "b"}) in (["a", "b"], ["b", "a"])
        assert _extract_string_list('{"a","b"}') == ["a", "b"]
        assert _extract_string_list('["a","b"]') == ["a", "b"]
        assert _extract_string_list("a,b") == ["a", "b"]
        assert _extract_string_list("") == []

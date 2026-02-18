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
    create_approval_request,
    enforce_scope_subset,
    ensure_approval_capacity,
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

    def test_json_string_metadata_is_supported(self):
        """JSON string metadata should be parsed before filtering."""

        payload = {
            "context_segments": [
                {"text": "public note", "scopes": ["public"]},
                {"text": "private note", "scopes": ["private"]},
            ]
        }
        result = filter_context_segments(str(payload).replace("'", '"'), ["public"])
        assert [s["text"] for s in result["context_segments"]] == ["public note"]


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


class _DummyTransaction:
    """Async no-op transaction context for helper tests."""

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False


class _DummyAcquire:
    """Async wrapper returning a provided connection."""

    def __init__(self, conn):
        self._conn = conn

    async def __aenter__(self):
        return self._conn

    async def __aexit__(self, exc_type, exc, tb):
        return False


class _DummyConn:
    """Tiny asyncpg-like connection stub for approval helper tests."""

    def __init__(self, fetchval_result=0, fetchrow_result=None):
        self.fetchval_result = fetchval_result
        self.fetchrow_result = fetchrow_result
        self.executed = []
        self.fetchval_calls = []
        self.fetchrow_calls = []

    async def execute(self, query, *args):
        self.executed.append((query, args))

    async def fetchval(self, query, *args):
        self.fetchval_calls.append((query, args))
        return self.fetchval_result

    async def fetchrow(self, query, *args):
        self.fetchrow_calls.append((query, args))
        return self.fetchrow_result

    def transaction(self):
        return _DummyTransaction()


class _DummyPoolNoAcquire:
    """Pool stub without acquire; drives fetchval fallback branch."""

    def __init__(self, fetchval_result=0):
        self.fetchval_result = fetchval_result
        self.calls = []

    async def fetchval(self, query, *args):
        self.calls.append((query, args))
        return self.fetchval_result


class _DummyPoolNoFetch:
    """Pool stub missing both acquire and fetchval."""

    pass


class _DummyPoolAsyncAcquire:
    """Pool stub where acquire is async (triggers fallback path)."""

    def __init__(self, fetchval_result=0):
        self.fetchval_result = fetchval_result
        self.calls = []

    async def acquire(self):  # pragma: no cover - never awaited in helper path
        return None

    async def fetchval(self, query, *args):
        self.calls.append((query, args))
        return self.fetchval_result


class _DummyPoolWithAcquire:
    """Pool stub with acquire context manager."""

    def __init__(self, conn):
        self._conn = conn

    def acquire(self):
        return _DummyAcquire(self._conn)


class TestApprovalCapacityAndCreation:
    """Async coverage for approval queue helper branches."""

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fetchval_fallback_accepts(self, monkeypatch):
        """Fallback fetchval path should pass when queue is below cap."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolNoAcquire(fetchval_result=3)
        await ensure_approval_capacity(pool, "agent-1", requested=2)
        assert len(pool.calls) == 1

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fetchval_fallback_rejects(self, monkeypatch):
        """Fallback fetchval path should reject when queue exceeds cap."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolNoAcquire(fetchval_result=10)
        with pytest.raises(ValueError):
            await ensure_approval_capacity(pool, "agent-1", requested=1)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fallback_ignores_missing_fetchval(self):
        """Pool without fetchval should be a no-op."""

        pool = _DummyPoolNoFetch()
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=1)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fallback_handles_none_count(self, monkeypatch):
        """Fallback fetchval path should no-op on None count."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolNoAcquire(fetchval_result=None)
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=1)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fallback_handles_non_int_count(self, monkeypatch):
        """Fallback fetchval path should no-op on non-int count."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolNoAcquire(fetchval_result="not-a-number")
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=1)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fallback_requested_zero(self, monkeypatch):
        """Fallback fetchval path should no-op when requested is zero."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolNoAcquire(fetchval_result=100)
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=0)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fallback_when_acquire_is_async(self, monkeypatch):
        """Async acquire should route to fallback fetchval path."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolAsyncAcquire(fetchval_result=2)
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=1)
        assert len(pool.calls) == 1

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_requested_zero(self, monkeypatch):
        """Conn branch should no-op when requested is zero."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result=10)
        await ensure_approval_capacity(pool=None, agent_id="agent-1", requested=0, conn=conn)
        assert len(conn.executed) == 1  # advisory lock still acquired

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_rejects(self, monkeypatch):
        """Conn branch should raise when queue exceeds cap."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result=10)
        with pytest.raises(ValueError):
            await ensure_approval_capacity(pool=None, agent_id="agent-1", requested=1, conn=conn)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_handles_none_count(self, monkeypatch):
        """Conn branch should no-op on None count."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result=None)
        await ensure_approval_capacity(pool=None, agent_id="agent-1", requested=1, conn=conn)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_handles_non_int_count(self, monkeypatch):
        """Conn branch should no-op on non-int count."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result="oops")
        await ensure_approval_capacity(pool=None, agent_id="agent-1", requested=1, conn=conn)

    @pytest.mark.asyncio
    async def test_create_approval_request_with_conn(self, monkeypatch):
        """create_approval_request should use provided connection directly."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(
            fetchval_result=0,
            fetchrow_result={"id": "approval-1", "request_type": "create_entity"},
        )
        result = await create_approval_request(
            pool=None,
            agent_id="agent-1",
            request_type="create_entity",
            change_details={"name": "x"},
            conn=conn,
        )
        assert result["id"] == "approval-1"
        assert len(conn.fetchrow_calls) == 1
        _, args = conn.fetchrow_calls[0]
        assert args[2] == '{"name": "x"}'

    @pytest.mark.asyncio
    async def test_create_approval_request_with_pool_acquire(self, monkeypatch):
        """create_approval_request should use acquire+transaction when conn omitted."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(
            fetchval_result=0,
            fetchrow_result={"id": "approval-2", "request_type": "create_entity"},
        )
        pool = _DummyPoolWithAcquire(conn)
        result = await create_approval_request(
            pool=pool,
            agent_id="agent-2",
            request_type="create_entity",
            change_details={"name": "y"},
            conn=None,
        )
        assert result["id"] == "approval-2"
        assert len(conn.fetchrow_calls) == 1

    @pytest.mark.asyncio
    async def test_create_approval_request_returns_empty_on_missing_row(self, monkeypatch):
        """create_approval_request should return empty dict when row is missing."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result=0, fetchrow_result=None)
        result = await create_approval_request(
            pool=None,
            agent_id="agent-1",
            request_type="create_entity",
            change_details='{"name":"x"}',
            conn=conn,
        )
        assert result == {}
        _, args = conn.fetchrow_calls[0]
        assert args[2] == '{"name":"x"}'

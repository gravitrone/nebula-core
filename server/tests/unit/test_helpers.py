"""Unit tests for pure helper functions (no DB needed)."""

# Third-Party
import copy
from datetime import UTC, datetime, timedelta
import json

import pytest

from nebula_mcp.helpers import (
    _extract_string_list,
    _enrich_approval_rows,
    _normalize_diff_value,
    _normalize_change_details,
    _pending_approval_limit,
    _safe_parse_uuid,
    _to_uuid_list,
    bulk_update_entity_scopes,
    bulk_update_entity_tags,
    approve_request,
    create_approval_request,
    create_enrollment_session,
    enforce_scope_subset,
    ensure_approval_capacity,
    filter_context_segments,
    get_approval_diff,
    get_approval_request,
    get_entity_history,
    get_enrollment_for_wait,
    get_pending_approvals_all,
    list_audit_actors,
    list_audit_scopes,
    maybe_expire_enrollment,
    normalize_bulk_operation,
    query_audit_log,
    reject_request,
    redeem_enrollment_key,
    revert_entity,
    scope_names_from_ids,
    wait_for_enrollment_status,
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

    def test_enforce_scope_subset_accepts_valid_non_empty_subset(self):
        """Valid non-empty requests should be returned unchanged."""

        assert enforce_scope_subset(["public"], ["public", "private"]) == ["public"]

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
        assert _safe_parse_uuid("   ") is None

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

    def test_extract_string_list_edge_strings(self):
        """String-list parser should handle empty braces and malformed JSON arrays."""

        assert _extract_string_list("{}") == []
        assert _extract_string_list("[not-json") == ["[not-json"]
        assert _extract_string_list("[oops]") == ["[oops]"]

    def test_scope_names_from_ids_ignores_unknown_ids(self):
        """Scope name mapping should skip unknown scope IDs."""

        enums = type(
            "Enums",
            (),
            {"scopes": type("Scopes", (), {"id_to_name": {"id-public": "public"}})()},
        )()
        assert scope_names_from_ids(["id-public", "id-missing"], enums) == ["public"]


class _DummyTransaction:
    """Async no-op transaction context for helper tests."""

    async def __aenter__(self):
        """Handle aenter.

        Returns:
            Result value from the operation.
        """

        return self

    async def __aexit__(self, exc_type, exc, tb):
        """Handle aexit.

        Args:
            exc_type: Input parameter for __aexit__.
            exc: Input parameter for __aexit__.
            tb: Input parameter for __aexit__.

        Returns:
            Result value from the operation.
        """

        return False


class _DummyAcquire:
    """Async wrapper returning a provided connection."""

    def __init__(self, conn):
        """Handle init.

        Args:
            conn: Input parameter for __init__.
        """

        self._conn = conn

    async def __aenter__(self):
        """Handle aenter.

        Returns:
            Result value from the operation.
        """

        return self._conn

    async def __aexit__(self, exc_type, exc, tb):
        """Handle aexit.

        Args:
            exc_type: Input parameter for __aexit__.
            exc: Input parameter for __aexit__.
            tb: Input parameter for __aexit__.

        Returns:
            Result value from the operation.
        """

        return False


class _DummyConn:
    """Tiny asyncpg-like connection stub for approval helper tests."""

    def __init__(self, fetchval_result=0, fetchrow_result=None):
        """Handle init.

        Args:
            fetchval_result: Input parameter for __init__.
            fetchrow_result: Input parameter for __init__.
        """

        self.fetchval_result = fetchval_result
        self.fetchrow_result = fetchrow_result
        self.executed = []
        self.fetchval_calls = []
        self.fetchrow_calls = []

    async def execute(self, query, *args):
        """Handle execute.

        Args:
            query: Input parameter for execute.
            *args: Input parameter for execute.
        """

        self.executed.append((query, args))

    async def fetchval(self, query, *args):
        """Get fetchval.

        Args:
            query: Input parameter for fetchval.
            *args: Input parameter for fetchval.

        Returns:
            Result value from the operation.
        """

        self.fetchval_calls.append((query, args))
        return self.fetchval_result

    async def fetchrow(self, query, *args):
        """Get fetchrow.

        Args:
            query: Input parameter for fetchrow.
            *args: Input parameter for fetchrow.

        Returns:
            Result value from the operation.
        """

        self.fetchrow_calls.append((query, args))
        return self.fetchrow_result

    def transaction(self):
        """Handle transaction.

        Returns:
            Result value from the operation.
        """

        return _DummyTransaction()


class _DummyPoolNoAcquire:
    """Pool stub without acquire; drives fetchval fallback branch."""

    def __init__(self, fetchval_result=0):
        """Handle init.

        Args:
            fetchval_result: Input parameter for __init__.
        """

        self.fetchval_result = fetchval_result
        self.calls = []

    async def fetchval(self, query, *args):
        """Get fetchval.

        Args:
            query: Input parameter for fetchval.
            *args: Input parameter for fetchval.

        Returns:
            Result value from the operation.
        """

        self.calls.append((query, args))
        return self.fetchval_result


class _DummyPoolNoFetch:
    """Pool stub missing both acquire and fetchval."""

    pass


class _DummyPoolAsyncAcquire:
    """Pool stub where acquire is async (triggers fallback path)."""

    def __init__(self, fetchval_result=0):
        """Handle init.

        Args:
            fetchval_result: Input parameter for __init__.
        """

        self.fetchval_result = fetchval_result
        self.calls = []

    async def acquire(self):  # pragma: no cover - never awaited in helper path
        """Handle acquire.

        Returns:
            Result value from the operation.
        """

        return None

    async def fetchval(self, query, *args):
        """Get fetchval.

        Args:
            query: Input parameter for fetchval.
            *args: Input parameter for fetchval.

        Returns:
            Result value from the operation.
        """

        self.calls.append((query, args))
        return self.fetchval_result


class _DummyPoolWithAcquire:
    """Pool stub with acquire context manager."""

    def __init__(self, conn):
        """Handle init.

        Args:
            conn: Input parameter for __init__.
        """

        self._conn = conn

    def acquire(self):
        """Handle acquire.

        Returns:
            Result value from the operation.
        """

        return _DummyAcquire(self._conn)


class _EnrollmentConn:
    """Connection stub for enrollment helper lifecycle tests."""

    def __init__(self, fetchrow_results=None):
        """Handle init.

        Args:
            fetchrow_results: Input parameter for __init__.
        """

        self._fetchrow_results = list(fetchrow_results or [])
        self.fetchrow_calls = []
        self.execute_calls = []

    async def fetchrow(self, query, *args):
        """Get fetchrow.

        Args:
            query: Input parameter for fetchrow.
            *args: Input parameter for fetchrow.

        Returns:
            Result value from the operation.
        """

        self.fetchrow_calls.append((query, args))
        if not self._fetchrow_results:
            return None
        return self._fetchrow_results.pop(0)

    async def execute(self, query, *args):
        """Handle execute.

        Args:
            query: Input parameter for execute.
            *args: Input parameter for execute.
        """

        self.execute_calls.append((query, args))

    def transaction(self):
        """Handle transaction.

        Returns:
            Result value from the operation.
        """

        return _DummyTransaction()


class _EnrollmentPool:
    """Pool stub exposing fetchrow/fetch/acquire for enrollment helpers."""

    def __init__(
        self,
        conn=None,
        fetchrow_results=None,
        fetch_results=None,
        fetchval_results=None,
    ):
        """Handle init.

        Args:
            conn: Input parameter for __init__.
            fetchrow_results: Input parameter for __init__.
            fetch_results: Input parameter for __init__.
            fetchval_results: Input parameter for __init__.
        """

        self._conn = conn
        self._fetchrow_results = list(fetchrow_results or [])
        self._fetch_results = list(fetch_results or [])
        self._fetchval_results = list(fetchval_results or [])
        self.fetchrow_calls = []
        self.fetch_calls = []
        self.fetchval_calls = []
        self.execute_calls = []

    async def fetchrow(self, query, *args):
        """Get fetchrow.

        Args:
            query: Input parameter for fetchrow.
            *args: Input parameter for fetchrow.

        Returns:
            Result value from the operation.
        """

        self.fetchrow_calls.append((query, args))
        if not self._fetchrow_results:
            return None
        item = self._fetchrow_results.pop(0)
        return item() if callable(item) else item

    async def fetch(self, query, *args):
        """Get fetch.

        Args:
            query: Input parameter for fetch.
            *args: Input parameter for fetch.

        Returns:
            Result value from the operation.
        """

        self.fetch_calls.append((query, args))
        if not self._fetch_results:
            return []
        return self._fetch_results.pop(0)

    async def fetchval(self, query, *args):
        """Get fetchval.

        Args:
            query: Input parameter for fetchval.
            *args: Input parameter for fetchval.

        Returns:
            Result value from the operation.
        """

        self.fetchval_calls.append((query, args))
        if not self._fetchval_results:
            return None
        return self._fetchval_results.pop(0)

    async def execute(self, query, *args):
        """Handle execute.

        Args:
            query: Input parameter for execute.
            *args: Input parameter for execute.
        """

        self.execute_calls.append((query, args))
        if args and isinstance(args[0], (int, float)):
            await __import__("asyncio").sleep(args[0])

    def acquire(self):
        """Handle acquire.

        Returns:
            Result value from the operation.
        """

        return _DummyAcquire(self._conn)


class TestApprovalCapacityAndCreation:
    """Async coverage for approval queue helper branches."""

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fetchval_fallback_accepts(
        self, monkeypatch
    ):
        """Fallback fetchval path should pass when queue is below cap."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolNoAcquire(fetchval_result=3)
        await ensure_approval_capacity(pool, "agent-1", requested=2)
        assert len(pool.calls) == 1

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fetchval_fallback_rejects(
        self, monkeypatch
    ):
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
    async def test_ensure_approval_capacity_fallback_handles_none_count(
        self, monkeypatch
    ):
        """Fallback fetchval path should no-op on None count."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolNoAcquire(fetchval_result=None)
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=1)

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_fallback_handles_non_int_count(
        self, monkeypatch
    ):
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
    async def test_ensure_approval_capacity_fallback_when_acquire_is_async(
        self, monkeypatch
    ):
        """Async acquire should route to fallback fetchval path."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolAsyncAcquire(fetchval_result=2)
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=1)
        assert len(pool.calls) == 1

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_acquire_branch(self, monkeypatch):
        """Sync acquire path should recurse into connection branch."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        pool = _DummyPoolWithAcquire(_DummyConn(fetchval_result=3))
        await ensure_approval_capacity(pool=pool, agent_id="agent-1", requested=2)

        overflow = _DummyPoolWithAcquire(_DummyConn(fetchval_result=10))
        with pytest.raises(ValueError):
            await ensure_approval_capacity(
                pool=overflow, agent_id="agent-1", requested=1
            )

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_requested_zero(
        self, monkeypatch
    ):
        """Conn branch should no-op when requested is zero."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result=10)
        await ensure_approval_capacity(
            pool=None, agent_id="agent-1", requested=0, conn=conn
        )
        assert len(conn.executed) == 1  # advisory lock still acquired

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_rejects(self, monkeypatch):
        """Conn branch should raise when queue exceeds cap."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result=10)
        with pytest.raises(ValueError):
            await ensure_approval_capacity(
                pool=None, agent_id="agent-1", requested=1, conn=conn
            )

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_handles_none_count(
        self, monkeypatch
    ):
        """Conn branch should no-op on None count."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result=None)
        await ensure_approval_capacity(
            pool=None, agent_id="agent-1", requested=1, conn=conn
        )

    @pytest.mark.asyncio
    async def test_ensure_approval_capacity_conn_branch_handles_non_int_count(
        self, monkeypatch
    ):
        """Conn branch should no-op on non-int count."""

        monkeypatch.setattr("nebula_mcp.helpers.MAX_PENDING_APPROVALS", 10)
        conn = _DummyConn(fetchval_result="oops")
        await ensure_approval_capacity(
            pool=None, agent_id="agent-1", requested=1, conn=conn
        )

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
    async def test_create_approval_request_returns_empty_on_missing_row(
        self, monkeypatch
    ):
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


class TestEnrollmentHelpers:
    """Coverage for enrollment lifecycle helpers."""

    @pytest.mark.asyncio
    async def test_create_enrollment_session_success(self):
        """Session creation should include a one-time raw enrollment token."""

        pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "id": "sess-1",
                    "status": "pending_approval",
                    "agent_id": "agent-1",
                }
            ]
        )
        result = await create_enrollment_session(
            pool,
            agent_id="agent-1",
            approval_request_id="approval-1",
            requested_scope_ids=["scope-public"],
            requested_requires_approval=True,
        )
        assert result["id"] == "sess-1"
        assert result["enrollment_token"].startswith("nbe_")

    @pytest.mark.asyncio
    async def test_create_enrollment_session_raises_when_missing_row(self):
        """Session creation should fail when insert returns no row."""

        pool = _EnrollmentPool(fetchrow_results=[None])
        with pytest.raises(ValueError):
            await create_enrollment_session(
                pool,
                agent_id="agent-1",
                approval_request_id="approval-1",
                requested_scope_ids=["scope-public"],
                requested_requires_approval=True,
            )

    @pytest.mark.asyncio
    async def test_get_enrollment_for_wait_happy_path(self):
        """Wait lookup should validate token and return enrollment row."""

        raw_token, token_hash = generate_enrollment_token()
        pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "id": "sess-1",
                    "status": "pending_approval",
                    "enrollment_token_hash": token_hash,
                }
            ]
        )
        result = await get_enrollment_for_wait(
            pool, registration_id="sess-1", enrollment_token=raw_token
        )
        assert result["id"] == "sess-1"

    @pytest.mark.asyncio
    async def test_get_enrollment_for_wait_rejects_missing_and_bad_token(self):
        """Wait lookup should reject missing session and invalid tokens."""

        with pytest.raises(ValueError):
            await get_enrollment_for_wait(
                _EnrollmentPool(fetchrow_results=[None]),
                registration_id="sess-404",
                enrollment_token="bad",
            )

        raw_token, token_hash = generate_enrollment_token()
        with pytest.raises(ValueError):
            await get_enrollment_for_wait(
                _EnrollmentPool(
                    fetchrow_results=[
                        {
                            "id": "sess-1",
                            "status": "pending_approval",
                            "enrollment_token_hash": token_hash,
                        }
                    ]
                ),
                registration_id="sess-1",
                enrollment_token=raw_token + "-bad",
            )

    @pytest.mark.asyncio
    async def test_maybe_expire_enrollment_variants(self):
        """Expiration helper should handle no row, live row, and expired row."""

        assert (
            await maybe_expire_enrollment(
                _EnrollmentPool(fetchrow_results=[None]), "sess-missing"
            )
            is None
        )

        live_row = {
            "id": "sess-live",
            "status": "pending_approval",
            "expires_at": datetime.now(UTC) + timedelta(hours=1),
        }
        live = await maybe_expire_enrollment(
            _EnrollmentPool(fetchrow_results=[live_row]), "sess-live"
        )
        assert live["id"] == "sess-live"

        expired_row = {
            "id": "sess-expired",
            "status": "approved",
            "expires_at": datetime.now(UTC) - timedelta(minutes=1),
        }
        updated_row = {**expired_row, "status": "expired"}
        expired = await maybe_expire_enrollment(
            _EnrollmentPool(fetchrow_results=[expired_row, updated_row]),
            "sess-expired",
        )
        assert expired["status"] == "expired"

    @pytest.mark.asyncio
    async def test_wait_for_enrollment_status_terminal_and_timeout(self, monkeypatch):
        """Wait helper should return terminal state or timeout with retry hint."""

        raw_token, token_hash = generate_enrollment_token()
        approved_row = {
            "id": "sess-approved",
            "status": "approved",
            "enrollment_token_hash": token_hash,
            "expires_at": datetime.now(UTC) + timedelta(hours=1),
        }
        approved = await wait_for_enrollment_status(
            _EnrollmentPool(fetchrow_results=[approved_row, approved_row]),
            registration_id="sess-approved",
            enrollment_token=raw_token,
            timeout_seconds=20,
        )
        assert approved["status"] == "approved"

        monkeypatch.setattr("nebula_mcp.helpers.ENROLLMENT_WAIT_POLL_SECONDS", 0.2)
        pending_row = {
            "id": "sess-pending",
            "status": "pending_approval",
            "enrollment_token_hash": token_hash,
            "expires_at": datetime.now(UTC) + timedelta(hours=1),
        }
        timeout_pool = _EnrollmentPool(
            fetchrow_results=[
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
                lambda: dict(pending_row),
            ]
        )
        timed_out = await wait_for_enrollment_status(
            timeout_pool,
            registration_id="sess-pending",
            enrollment_token=raw_token,
            timeout_seconds=1,
        )
        assert timed_out["status"] == "pending_approval"
        assert timed_out["retry_after_ms"] == 200

    @pytest.mark.asyncio
    async def test_redeem_enrollment_key_error_paths(self):
        """Redeem helper should fail cleanly on invalid lifecycle states."""

        raw_token, token_hash = generate_enrollment_token()

        missing = _EnrollmentPool(conn=_EnrollmentConn(fetchrow_results=[None]))
        with pytest.raises(ValueError):
            await redeem_enrollment_key(
                missing,
                registration_id="sess-missing",
                enrollment_token=raw_token,
            )

        invalid_token = _EnrollmentPool(
            conn=_EnrollmentConn(
                fetchrow_results=[
                    {
                        "status": "approved",
                        "enrollment_token_hash": token_hash,
                        "expires_at": datetime.now(UTC) + timedelta(hours=1),
                    }
                ]
            )
        )
        with pytest.raises(ValueError):
            await redeem_enrollment_key(
                invalid_token,
                registration_id="sess-invalid",
                enrollment_token=raw_token + "-bad",
            )

        expired = _EnrollmentConn(
            fetchrow_results=[
                {
                    "status": "approved",
                    "enrollment_token_hash": token_hash,
                    "expires_at": datetime.now(UTC) - timedelta(minutes=1),
                }
            ]
        )
        with pytest.raises(ValueError):
            await redeem_enrollment_key(
                _EnrollmentPool(conn=expired),
                registration_id="sess-expired",
                enrollment_token=raw_token,
            )
        assert len(expired.execute_calls) == 1

        redeemed = _EnrollmentPool(
            conn=_EnrollmentConn(
                fetchrow_results=[
                    {
                        "status": "redeemed",
                        "enrollment_token_hash": token_hash,
                        "expires_at": datetime.now(UTC) + timedelta(hours=1),
                    }
                ]
            )
        )
        with pytest.raises(ValueError):
            await redeem_enrollment_key(
                redeemed,
                registration_id="sess-redeemed",
                enrollment_token=raw_token,
            )

        not_approved = _EnrollmentPool(
            conn=_EnrollmentConn(
                fetchrow_results=[
                    {
                        "status": "pending_approval",
                        "enrollment_token_hash": token_hash,
                        "expires_at": datetime.now(UTC) + timedelta(hours=1),
                    }
                ]
            )
        )
        with pytest.raises(ValueError):
            await redeem_enrollment_key(
                not_approved,
                registration_id="sess-pending",
                enrollment_token=raw_token,
            )

    @pytest.mark.asyncio
    async def test_redeem_enrollment_key_success_and_mark_redeemed_failure(
        self, monkeypatch
    ):
        """Redeem helper should mint key once and fail if mark_redeemed is missing."""

        raw_token, token_hash = generate_enrollment_token()
        monkeypatch.setattr(
            "nebula_api.auth.generate_api_key",
            lambda: ("nebula-key-1", "nbk_1", "hash_1"),
        )

        ok_conn = _EnrollmentConn(
            fetchrow_results=[
                {
                    "status": "approved",
                    "agent_id": "agent-1",
                    "agent_name": "agent-one",
                    "enrollment_token_hash": token_hash,
                    "expires_at": datetime.now(UTC) + timedelta(hours=1),
                    "requested_scope_ids": ["scope-public"],
                    "granted_scope_ids": None,
                    "requested_requires_approval": True,
                    "granted_requires_approval": None,
                },
                {"id": "sess-1", "status": "redeemed"},
            ]
        )
        ok = await redeem_enrollment_key(
            _EnrollmentPool(conn=ok_conn),
            registration_id="sess-1",
            enrollment_token=raw_token,
        )
        assert ok["api_key"] == "nebula-key-1"
        assert ok["agent_name"] == "agent-one"
        assert ok["scope_ids"] == ["scope-public"]
        assert ok["requires_approval"] is True

        fail_conn = _EnrollmentConn(
            fetchrow_results=[
                {
                    "status": "approved",
                    "agent_id": "agent-1",
                    "agent_name": "agent-one",
                    "enrollment_token_hash": token_hash,
                    "expires_at": datetime.now(UTC) + timedelta(hours=1),
                    "requested_scope_ids": ["scope-public"],
                    "granted_scope_ids": ["scope-private"],
                    "requested_requires_approval": True,
                    "granted_requires_approval": False,
                },
                None,
            ]
        )
        with pytest.raises(ValueError):
            await redeem_enrollment_key(
                _EnrollmentPool(conn=fail_conn),
                registration_id="sess-fail",
                enrollment_token=raw_token,
            )


class TestApprovalEnrichmentAndAuditHelpers:
    """Coverage for approval enrichment, bulk ops, and audit utilities."""

    @pytest.mark.asyncio
    async def test_enrich_approval_rows_handles_empty_input(self):
        """Empty row lists should pass through without DB calls."""

        pool = _EnrollmentPool(fetch_results=[])
        assert await _enrich_approval_rows(pool, []) == []
        assert pool.fetch_calls == []

    @pytest.mark.asyncio
    async def test_enrich_approval_rows_populates_labels_and_relationship_fallbacks(
        self,
    ):
        """Enrichment should attach readable labels for ids and relationship fields."""

        requested_by = "6d0cdce8-f930-4518-b261-a0bac7ac89d0"
        entity_id = "4f26cf1f-12ad-48dc-a6e0-954d7fd1fe66"
        relationship_id = "2940971e-937a-4ba0-a7b6-30f3302da49e"
        rows = [
            {
                "requested_by": requested_by,
                "change_details": {
                    "relationship_id": relationship_id,
                    "source_type": "job",
                    "source_id": "2026Q1-ABCD",
                    "target_type": "entity",
                    "target_id": entity_id,
                    "entity_ids": [entity_id, "not-a-uuid"],
                },
            }
        ]
        pool = _EnrollmentPool(
            fetch_results=[
                [{"id": entity_id, "label": "Bro"}],  # entities
                [{"id": "2026Q1-ABCD", "label": "Sprint Job"}],  # jobs
                [{"id": requested_by, "label": "agent-1"}],  # agents
                [],  # requested_by entities fallback map
                [  # relationship map
                    {
                        "id": relationship_id,
                        "relationship_type": "owns",
                        "source_type": "job",
                        "source_id": "2026Q1-ABCD",
                        "target_type": "entity",
                        "target_id": entity_id,
                    }
                ],
            ]
        )
        enriched = await _enrich_approval_rows(pool, rows)
        details = enriched[0]["change_details"]
        assert enriched[0]["requested_by_name"] == "agent-1"
        assert details["source_name"] == "Sprint Job"
        assert details["target_name"] == "Bro"
        assert details["entity_names"] == ["Bro"]
        assert details["entity_name"] == "Bro"

    @pytest.mark.asyncio
    async def test_enrich_approval_rows_covers_node_type_maps_and_fallback_edges(self):
        """Enrichment should handle all node types and relationship fallback hydration."""

        requested_by = "6d0cdce8-f930-4518-b261-a0bac7ac89d0"
        entity_id = "4f26cf1f-12ad-48dc-a6e0-954d7fd1fe66"
        context_id = "2de6c91b-770c-4ef1-bf8a-568f1f2dc6a6"
        log_id = "5b855890-490a-47f1-8dd2-c7ce93f71ebf"
        file_id = "17010f1b-d80a-4413-bc89-f338ae3d4ec0"
        protocol_id = "3395e6cd-4068-4254-9db7-721bd2d4a21a"
        agent_id = "043cd753-f356-4890-8481-f4ce7f9f4937"
        relationship_id = "2940971e-937a-4ba0-a7b6-30f3302da49e"
        rows = [
            {
                "requested_by": requested_by,
                "change_details": {
                    "relationship_id": relationship_id,
                    "source_id": "",  # trigger source_id fallback
                    "target_id": "",  # trigger target_id fallback
                    "entity_id": entity_id,
                },
            },
            {
                "requested_by": requested_by,
                "change_details": {
                    "source_type": "agent",
                    "source_id": agent_id,
                    "target_type": "file",
                    "target_id": file_id,
                },
            },
            {
                "requested_by": requested_by,
                "change_details": {
                    "source_type": "protocol",
                    "source_id": protocol_id,
                    "target_type": "log",
                    "target_id": log_id,
                },
            },
            {
                "requested_by": requested_by,
                "change_details": {
                    "source_type": "job",
                    "source_id": "",  # empty job id path
                    "target_type": "context",
                    "target_id": context_id,
                },
            },
            {
                "requested_by": requested_by,
                "change_details": {
                    "source_type": "entity",
                    "source_id": entity_id,
                    "target_type": "entity",
                    "target_id": "not-a-uuid",  # invalid uuid parse-skip branch
                },
            },
            {
                "requested_by": requested_by,
                "change_details": {
                    "source_type": "weird",
                    "source_id": entity_id,  # unknown node type return branch
                },
            },
        ]
        pool = _EnrollmentPool(
            fetch_results=[
                [{"id": entity_id, "label": "Bro"}],  # entities
                [{"id": context_id, "label": "Ctx"}],  # context
                [{"id": log_id, "label": "event"}],  # logs
                [{"id": file_id, "label": "spec.md"}],  # files
                [{"id": protocol_id, "label": "Protocol A"}],  # protocols
                [  # agents
                    {"id": requested_by, "label": "req-agent"},
                    {"id": agent_id, "label": "source-agent"},
                ],
                [],  # requested_by entities fallback map
                [  # relationship map for fallback hydration
                    {
                        "id": relationship_id,
                        "relationship_type": "references",
                        "source_type": "context",
                        "source_id": context_id,
                        "target_type": "entity",
                        "target_id": entity_id,
                    }
                ],
            ]
        )
        enriched = await _enrich_approval_rows(pool, rows)
        first = enriched[0]["change_details"]
        assert first["source_type"] == "context"
        assert first["target_type"] == "entity"
        assert first["source_name"] == "Ctx"
        assert first["target_name"] == "Bro"
        assert enriched[1]["change_details"]["source_name"] == "source-agent"
        assert enriched[1]["change_details"]["target_name"] == "spec.md"
        assert enriched[2]["change_details"]["source_name"] == "Protocol A"
        assert enriched[2]["change_details"]["target_name"] == "event"
        assert "target_name" not in enriched[4]["change_details"]
        assert "source_name" not in enriched[5]["change_details"]

    @pytest.mark.asyncio
    async def test_get_pending_and_single_approval_helpers(self, monkeypatch):
        """Approval list/get helpers should route through enrichment utility."""

        async def _fake_enrich(_pool, rows):
            """Handle fake enrich.

            Args:
                _pool: Input parameter for _fake_enrich.
                rows: Input parameter for _fake_enrich.

            Returns:
                Result value from the operation.
            """

            for row in rows:
                row["enriched"] = True
            return rows

        monkeypatch.setattr("nebula_mcp.helpers._enrich_approval_rows", _fake_enrich)
        pool = _EnrollmentPool(
            fetch_results=[[{"id": "ap-1"}]],
            fetchrow_results=[{"id": "ap-2"}],
        )
        pending = await get_pending_approvals_all(pool, limit=5, offset=2)
        single = await get_approval_request(pool, "ap-2")
        missing = await get_approval_request(pool, "ap-missing")
        assert pending[0]["enriched"] is True
        assert single["enriched"] is True
        assert missing is None

    @pytest.mark.asyncio
    async def test_reject_request_marks_register_agent_enrollment(self):
        """Register-agent rejects should mark enrollment status."""

        pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "id": "approval-1",
                    "request_type": "register_agent",
                    "status": "rejected",
                }
            ]
        )
        result = await reject_request(
            pool,
            approval_id="approval-1",
            reviewed_by="reviewer-1",
            review_notes="nope",
        )
        assert result["status"] == "rejected"
        assert len(pool.execute_calls) == 1

        with pytest.raises(ValueError):
            await reject_request(
                _EnrollmentPool(fetchrow_results=[None]),
                approval_id="approval-404",
                reviewed_by="reviewer-1",
                review_notes="nope",
            )

    @pytest.mark.asyncio
    async def test_get_entity_history_and_revert_entity_error_paths(self):
        """History and revert helpers should validate audit input states."""

        history_pool = _EnrollmentPool(
            fetch_results=[[{"id": "audit-1", "table_name": "entities"}]]
        )
        history = await get_entity_history(
            history_pool,
            entity_id="4f26cf1f-12ad-48dc-a6e0-954d7fd1fe66",
        )
        assert history[0]["id"] == "audit-1"

        with pytest.raises(ValueError):
            await revert_entity(
                _EnrollmentPool(fetchrow_results=[None]),
                "entity-1",
                "audit-1",
            )

        wrong_table = _EnrollmentPool(
            fetchrow_results=[
                {
                    "table_name": "jobs",
                    "record_id": "entity-1",
                    "new_data": {"name": "x"},
                }
            ]
        )
        with pytest.raises(ValueError):
            await revert_entity(wrong_table, "entity-1", "audit-1")

        wrong_record = _EnrollmentPool(
            fetchrow_results=[
                {
                    "table_name": "entities",
                    "record_id": "entity-2",
                    "new_data": {"name": "x"},
                }
            ]
        )
        with pytest.raises(ValueError):
            await revert_entity(wrong_record, "entity-1", "audit-1")

        empty_snapshot = _EnrollmentPool(
            fetchrow_results=[
                {"table_name": "entities", "record_id": "entity-1", "new_data": None}
            ]
        )
        with pytest.raises(ValueError):
            await revert_entity(empty_snapshot, "entity-1", "audit-1")

    @pytest.mark.asyncio
    async def test_revert_entity_uses_old_data_for_delete_actions(self):
        """Delete audit actions should use old_data snapshot on revert."""

        entity_id = "4f26cf1f-12ad-48dc-a6e0-954d7fd1fe66"
        snapshot = {
            "privacy_scope_ids": [],
            "name": "Restored Name",
            "type_id": "type-1",
            "status_id": "status-1",
            "status_changed_at": None,
            "status_reason": None,
            "tags": ["a"],
            "metadata": {"k": "v"},
            "source_path": None,
        }
        pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "table_name": "entities",
                    "record_id": entity_id,
                    "action": "delete",
                    "new_data": None,
                    "old_data": json.dumps(snapshot),
                },
                {"id": entity_id, "name": "Restored Name"},
            ]
        )
        result = await revert_entity(pool, entity_id, "audit-1")
        assert result["name"] == "Restored Name"

    def test_normalize_bulk_operation_aliases_and_invalid(self):
        """Bulk op normalizer should map aliases and reject unknown ops."""

        assert normalize_bulk_operation("add") == "add"
        assert normalize_bulk_operation("remove") == "remove"
        assert normalize_bulk_operation("-") == "remove"
        assert normalize_bulk_operation("set") == "set"
        with pytest.raises(ValueError):
            normalize_bulk_operation("merge")

    @pytest.mark.asyncio
    async def test_bulk_update_helpers_handle_rows_without_id_column(self):
        """Bulk update helpers should fallback to first row value when id key is absent."""

        pool = _EnrollmentPool(
            fetch_results=[
                [{"entity_id": "id-a"}, {"id": "id-b"}],  # tags
                [{"entity_id": "id-c"}, {"id": "id-d"}],  # scopes
            ]
        )
        tags = await bulk_update_entity_tags(pool, ["id-a"], ["x"], "add")
        enums = type(
            "Enums",
            (),
            {
                "scopes": type(
                    "Scopes", (), {"name_to_id": {"public": "scope-public"}}
                )()
            },
        )()
        scopes = await bulk_update_entity_scopes(
            pool,
            enums,
            ["id-c"],
            ["public"],
            "set",
        )
        assert tags == ["id-a", "id-b"]
        assert scopes == ["id-c", "id-d"]

    @pytest.mark.asyncio
    async def test_audit_query_and_actor_normalization(self):
        """Audit helpers should pass filters and normalize unknown actor rows."""

        pool = _EnrollmentPool(
            fetch_results=[
                [{"id": "audit-1"}],  # query_audit_log
                [{"scope_name": "public", "count": 1}],  # list_audit_scopes
                [  # list_audit_actors
                    {
                        "changed_by_type": "unknown",
                        "changed_by_id": "abc",
                        "actor_name": "unknown",
                    },
                    {
                        "changed_by_type": "agent",
                        "changed_by_id": "agent-1",
                        "actor_name": "codex",
                    },
                ],
            ]
        )
        audit_rows = await query_audit_log(pool, actor_type="agent", actor_id="")
        scope_rows = await list_audit_scopes(pool)
        actor_rows = await list_audit_actors(pool, actor_type="agent")

        assert audit_rows[0]["id"] == "audit-1"
        assert scope_rows[0]["scope_name"] == "public"
        assert actor_rows[0]["changed_by_type"] == "system"
        assert actor_rows[0]["changed_by_id"] == ""
        assert actor_rows[0]["actor_name"] is None
        assert actor_rows[1]["actor_name"] == "codex"

    @pytest.mark.asyncio
    async def test_get_approval_diff_branches_and_errors(self):
        """Approval diff helper should cover create/update and missing-resource errors."""

        with pytest.raises(ValueError):
            await get_approval_diff(
                _EnrollmentPool(fetchrow_results=[None]),
                "approval-missing",
            )

        create_pool = _EnrollmentPool(
            fetchrow_results=[
                {"request_type": "create_entity", "change_details": {"name": "N"}},
            ]
        )
        create_diff = await get_approval_diff(create_pool, "approval-create")
        assert create_diff["changes"]["name"]["to"] == "N"

        missing_entity_id_pool = _EnrollmentPool(
            fetchrow_results=[
                {"request_type": "update_entity", "change_details": {}},
            ]
        )
        with pytest.raises(ValueError):
            await get_approval_diff(
                missing_entity_id_pool, "approval-update-missing-id"
            )

        missing_entity_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "request_type": "update_entity",
                    "change_details": {"entity_id": "entity-1", "name": "new"},
                },
                None,
            ]
        )
        with pytest.raises(ValueError):
            await get_approval_diff(
                missing_entity_pool, "approval-update-missing-entity"
            )

        missing_rel_id_pool = _EnrollmentPool(
            fetchrow_results=[
                {"request_type": "update_relationship", "change_details": {}},
            ]
        )
        with pytest.raises(ValueError):
            await get_approval_diff(missing_rel_id_pool, "approval-rel-missing-id")

        missing_rel_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "request_type": "update_relationship",
                    "change_details": {"relationship_id": "rel-1", "status": "active"},
                },
                None,
            ]
        )
        with pytest.raises(ValueError):
            await get_approval_diff(missing_rel_pool, "approval-rel-missing")

        missing_job_id_pool = _EnrollmentPool(
            fetchrow_results=[
                {"request_type": "update_job_status", "change_details": {}},
            ]
        )
        with pytest.raises(ValueError):
            await get_approval_diff(missing_job_id_pool, "approval-job-missing-id")

        missing_job_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "request_type": "update_job_status",
                    "change_details": {"job_id": "job-1", "status": "done"},
                },
                None,
            ]
        )
        with pytest.raises(ValueError):
            await get_approval_diff(missing_job_pool, "approval-job-missing")

    @pytest.mark.asyncio
    async def test_get_approval_diff_update_entity_success(self):
        """update_entity diff should include changed fields only."""

        pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "request_type": "update_entity",
                    "change_details": {
                        "entity_id": "entity-1",
                        "name": "new",
                        "tags": ["x"],
                    },
                },
                {"name": "old", "tags": ["x"]},
            ]
        )
        diff = await get_approval_diff(pool, "approval-update-entity")
        assert diff["changes"]["name"]["from"] == "old"
        assert diff["changes"]["name"]["to"] == "new"
        assert "tags" not in diff["changes"]

    @pytest.mark.asyncio
    async def test_get_approval_diff_other_request_type_branches(self):
        """Diff helper should cover create-context plus relationship/job updates."""

        create_context_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "request_type": "create_context",
                    "change_details": '{"title":"Ctx","status":"active"}',
                }
            ]
        )
        create_context_diff = await get_approval_diff(
            create_context_pool, "approval-create-context"
        )
        assert create_context_diff["changes"]["title"]["to"] == "Ctx"

        rel_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "request_type": "update_relationship",
                    "change_details": {
                        "relationship_id": "rel-1",
                        "status": "archived",
                        "metadata": {"a": 2},
                    },
                },
                {"status": "active", "metadata": {"a": 1}},
            ]
        )
        rel_diff = await get_approval_diff(rel_pool, "approval-rel-update")
        assert rel_diff["changes"]["status"]["from"] == "active"
        assert rel_diff["changes"]["metadata"]["to"] == {"a": 2}

        job_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "request_type": "update_job_status",
                    "change_details": {"job_id": "2026Q1-ABCD", "status": "done"},
                },
                {"status": "in-progress"},
            ]
        )
        job_diff = await get_approval_diff(job_pool, "approval-job-update")
        assert job_diff["changes"]["status"]["from"] == "in-progress"
        assert job_diff["changes"]["status"]["to"] == "done"

    def test_normalize_diff_value_for_structured_data(self):
        """Diff value normalizer should JSON-normalize lists and dicts."""

        assert _normalize_diff_value({"b": 2, "a": 1}) == '{"a": 1, "b": 2}'
        assert _normalize_diff_value([2, 1]) == "[2, 1]"
        assert _normalize_diff_value("plain") == "plain"

    @pytest.mark.asyncio
    async def test_approve_request_register_agent_with_system_actor_variants(
        self, monkeypatch
    ):
        """Register-agent approvals should normalize malformed review_details."""

        from nebula_mcp import executors as executor_mod

        seen = []

        async def _executor(_pool, _enums, _details, review_details):
            """Handle executor.

            Args:
                _pool: Input parameter for _executor.
                _enums: Input parameter for _executor.
                _details: Input parameter for _executor.
                review_details: Input parameter for _executor.

            Returns:
                Result value from the operation.
            """

            seen.append(review_details)
            return {"id": "agent-created"}

        monkeypatch.setitem(executor_mod.EXECUTORS, "register_agent", _executor)

        approval_base = {
            "id": "approval-1",
            "request_type": "register_agent",
            "change_details": {"name": "agent-a"},
        }
        # Invalid JSON string -> {}
        pool_invalid = _EnrollmentPool(
            fetchrow_results=[{**approval_base, "review_details": "{bad-json"}]
        )
        result_invalid = await approve_request(
            pool_invalid,
            enums=None,
            approval_id="approval-1",
            reviewed_by=None,
            review_details=None,
            review_notes=None,
        )
        assert result_invalid["entity"]["id"] == "agent-created"

        # JSON list -> normalized to {}
        pool_list = _EnrollmentPool(
            fetchrow_results=[{**approval_base, "review_details": "[]"}]
        )
        await approve_request(
            pool_list,
            enums=None,
            approval_id="approval-2",
            reviewed_by=None,
            review_details=None,
            review_notes=None,
        )
        assert all("_approval_id" in item for item in seen)
        assert all("_reviewed_by" not in item for item in seen)
        # system actor path should set + reset change tracking keys.
        assert any(
            "set_config('app.changed_by_type'" in call[0]
            for call in pool_invalid.execute_calls
        )
        assert any(
            "RESET app.changed_by_id" in call[0] for call in pool_invalid.execute_calls
        )

    @pytest.mark.asyncio
    async def test_approve_request_error_and_non_register_paths(self, monkeypatch):
        """approve_request should fail cleanly for missing/executor/runtime paths."""

        from nebula_mcp import executors as executor_mod

        with pytest.raises(ValueError):
            await approve_request(
                _EnrollmentPool(fetchrow_results=[None]),
                enums=None,
                approval_id="approval-missing",
                reviewed_by=None,
            )

        no_exec_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "id": "approval-no-exec",
                    "request_type": "unknown_type",
                    "change_details": {},
                }
            ]
        )
        with pytest.raises(ValueError):
            await approve_request(
                no_exec_pool,
                enums=None,
                approval_id="approval-no-exec",
                reviewed_by=None,
            )
        assert any(
            str(call[1][0]).startswith("No executor for:")
            for call in no_exec_pool.execute_calls
            if call[1]
        )

        async def _create_entity_exec(_pool, _enums, _details):
            """Handle create entity exec.

            Args:
                _pool: Input parameter for _create_entity_exec.
                _enums: Input parameter for _create_entity_exec.
                _details: Input parameter for _create_entity_exec.

            Returns:
                Result value from the operation.
            """

            return {"id": "entity-1"}

        async def _raise_exec(_pool, _enums, _details):
            """Handle raise exec.

            Args:
                _pool: Input parameter for _raise_exec.
                _enums: Input parameter for _raise_exec.
                _details: Input parameter for _raise_exec.
            """

            raise RuntimeError("boom")

        monkeypatch.setitem(
            executor_mod.EXECUTORS, "create_entity", _create_entity_exec
        )
        good_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "id": "approval-ok",
                    "request_type": "create_entity",
                    "change_details": {"name": "N"},
                }
            ],
            fetchval_results=[None],
        )
        result = await approve_request(
            good_pool,
            enums=None,
            approval_id="approval-ok",
            reviewed_by="reviewer-1",
        )
        assert result["entity"]["id"] == "entity-1"
        assert any(
            "set_config('app.changed_by_type'" in call[0]
            for call in good_pool.execute_calls
        )
        assert (
            any("_reviewed_by" in str(call[1]) for call in good_pool.fetchval_calls)
            is False
        )

        monkeypatch.setitem(executor_mod.EXECUTORS, "create_entity", _raise_exec)
        fail_pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "id": "approval-fail",
                    "request_type": "create_entity",
                    "change_details": {"name": "N"},
                }
            ],
            fetchval_results=[None],
        )
        with pytest.raises(RuntimeError):
            await approve_request(
                fail_pool,
                enums=None,
                approval_id="approval-fail",
                reviewed_by="reviewer-1",
            )
        assert any(
            str(call[1][0]) == "boom" for call in fail_pool.execute_calls if call[1]
        )

    @pytest.mark.asyncio
    async def test_approve_request_register_agent_with_reviewed_by_sets_marker(
        self, monkeypatch
    ):
        """register_agent approvals should include reviewed-by marker when set."""

        from nebula_mcp import executors as executor_mod

        seen: list[dict] = []

        async def _executor(_pool, _enums, _details, review_details):
            """Handle executor.

            Args:
                _pool: Input parameter for _executor.
                _enums: Input parameter for _executor.
                _details: Input parameter for _executor.
                review_details: Input parameter for _executor.

            Returns:
                Result value from the operation.
            """

            seen.append(review_details)
            return {"id": "agent-created"}

        monkeypatch.setitem(executor_mod.EXECUTORS, "register_agent", _executor)
        pool = _EnrollmentPool(
            fetchrow_results=[
                {
                    "id": "approval-3",
                    "request_type": "register_agent",
                    "change_details": {"name": "agent-a"},
                    "review_details": {},
                }
            ],
            fetchval_results=[None],
        )
        await approve_request(
            pool,
            enums=None,
            approval_id="approval-3",
            reviewed_by="reviewer-1",
            review_details=None,
            review_notes=None,
        )
        assert seen[0]["_reviewed_by"] == "reviewer-1"

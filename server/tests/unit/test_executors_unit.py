"""Unit tests for nebula_mcp.executors branch-heavy paths."""

from __future__ import annotations

import json

# Standard Library
from datetime import UTC
from unittest.mock import AsyncMock
from uuid import uuid4

# Third-Party
import pytest
from pydantic import ValidationError

# Local
import nebula_mcp.executors as executors


class _AsyncContext:
    """Small async context manager used by pool transaction stubs."""

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc, tb):
        return False


class _PoolStub:
    """Minimal asyncpg-like stub for executor unit tests."""

    def __init__(
        self,
        *,
        fetchrow_rows: list[object] | None = None,
        fetch_rows: list[object] | None = None,
        fetchval_rows: list[object] | None = None,
        in_transaction: bool = True,
    ):
        self.fetchrow_rows = list(fetchrow_rows or [])
        self.fetch_rows = list(fetch_rows or [])
        self.fetchval_rows = list(fetchval_rows or [])
        self._in_transaction = in_transaction
        self.transaction_calls = 0
        self.fetchrow_calls: list[tuple] = []
        self.fetch_calls: list[tuple] = []
        self.fetchval_calls: list[tuple] = []
        self.execute = AsyncMock()

    def is_in_transaction(self) -> bool:
        return self._in_transaction

    def transaction(self):
        self.transaction_calls += 1
        return _AsyncContext()

    async def fetchrow(self, *args):
        self.fetchrow_calls.append(args)
        if not self.fetchrow_rows:
            return None
        value = self.fetchrow_rows.pop(0)
        if isinstance(value, Exception):
            raise value
        return value

    async def fetch(self, *args):
        self.fetch_calls.append(args)
        if not self.fetch_rows:
            return []
        value = self.fetch_rows.pop(0)
        if isinstance(value, Exception):
            raise value
        return value

    async def fetchval(self, *args):
        self.fetchval_calls.append(args)
        if not self.fetchval_rows:
            return None
        value = self.fetchval_rows.pop(0)
        if isinstance(value, Exception):
            raise value
        return value


def test_scope_name_from_id_resolves_known_uuid(mock_enums):
    """Known scope UUIDs should map back to names."""

    scope_name, scope_id = next(iter(mock_enums.scopes.name_to_id.items()))
    assert executors._scope_name_from_id(mock_enums, scope_id) == scope_name


def test_scope_name_from_id_returns_raw_for_non_uuid(mock_enums):
    """Non-UUID scope ids should pass through as strings."""

    assert executors._scope_name_from_id(mock_enums, "not-a-uuid") == "not-a-uuid"


def test_scope_name_from_id_returns_uuid_text_for_unknown_uuid(mock_enums):
    """Unknown UUID scope ids should fall back to UUID text."""

    unknown_scope = uuid4()
    assert executors._scope_name_from_id(mock_enums, unknown_scope) == str(unknown_scope)


@pytest.mark.asyncio
async def test_execute_create_entity_uses_explicit_transaction_when_not_in_tx(
    mock_enums,
):
    """Non-transactional connection stubs should wrap create in transaction()."""

    pool = _PoolStub(
        fetchrow_rows=[
            None,
            {"id": str(uuid4()), "name": "n"},
        ],
        in_transaction=False,
    )

    result = await executors.execute_create_entity(
        pool,
        mock_enums,
        {
            "name": "n",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    assert pool.transaction_calls == 1
    assert result["name"] == "n"


@pytest.mark.asyncio
async def test_execute_create_entity_uses_pool_acquire_branch(mock_enums, monkeypatch):
    """Pool instances should execute through acquire()+transaction path."""

    class _AcquireCtx:
        def __init__(self, conn):
            self._conn = conn

        async def __aenter__(self):
            return self._conn

        async def __aexit__(self, exc_type, exc, tb):
            return False

    class _PoolAcquireLike:
        def __init__(self, conn):
            self._conn = conn

        def acquire(self):
            return _AcquireCtx(self._conn)

    conn = _PoolStub(
        fetchrow_rows=[None, {"id": str(uuid4()), "name": "acq"}],
        in_transaction=True,
    )
    pool_like = _PoolAcquireLike(conn)
    monkeypatch.setattr(executors, "Pool", _PoolAcquireLike)

    result = await executors.execute_create_entity(
        pool_like,
        mock_enums,
        {
            "name": "acq",
            "type": "project",
            "status": "active",
            "scopes": ["public"],
        },
    )

    assert result["name"] == "acq"


@pytest.mark.asyncio
async def test_execute_update_context_raises_when_target_missing(mock_enums):
    """Updates should fail when the target context item does not exist."""

    pool = _PoolStub(fetchrow_rows=[None])

    with pytest.raises(ValueError, match="Context not found"):
        await executors.execute_update_context(
            pool,
            mock_enums,
            {"context_id": str(uuid4()), "title": "missing"},
        )


@pytest.mark.asyncio
async def test_execute_create_context_parses_json_change_details(mock_enums):
    """String payloads should decode before CreateContextInput validation."""

    context_id = str(uuid4())
    pool = _PoolStub(
        fetchrow_rows=[
            None,
            {"id": context_id},
        ]
    )

    result = await executors.execute_create_context(
        pool,
        mock_enums,
        json.dumps(
            {
                "title": "alpha",
                "url": "https://example.com",
                "source_type": "note",
                "scopes": ["public"],
            }
        ),
    )

    assert result["id"] == context_id


@pytest.mark.asyncio
async def test_execute_create_context_duplicate_url_raises(mock_enums):
    """Duplicate context URLs should raise with existing row details."""

    pool = _PoolStub(fetchrow_rows=[{"id": "ctx-1", "title": "existing"}])

    with pytest.raises(ValueError, match="already exists for URL"):
        await executors.execute_create_context(
            pool,
            mock_enums,
            {
                "title": "alpha",
                "url": "https://example.com",
                "source_type": "note",
                "scopes": ["public"],
            },
        )


@pytest.mark.asyncio
async def test_execute_create_context_returns_empty_dict_when_insert_missing(
    mock_enums,
):
    """Create context should normalize missing insert rows to empty dict."""

    pool = _PoolStub(fetchrow_rows=[None, None])
    result = await executors.execute_create_context(
        pool,
        mock_enums,
        {
            "title": "alpha",
            "url": None,
            "source_type": "note",
            "scopes": ["public"],
        },
    )

    assert result == {}


@pytest.mark.asyncio
async def test_execute_create_entity_rejects_metadata_payload(mock_enums):
    """Entity payloads should reject metadata keys."""

    pool = _PoolStub()

    with pytest.raises(ValidationError):
        await executors.execute_create_entity(
            pool,
            mock_enums,
            {
                "name": "bad-segment",
                "type": "project",
                "status": "active",
                "scopes": ["public"],
                "metadata": {"context_segments": [{"text": "x", "scopes": []}]},
            },
        )


@pytest.mark.asyncio
async def test_execute_update_context_status_and_scope_paths(mock_enums):
    """Status/scopes branches should be exercised for context updates."""

    context_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": context_id}])

    result = await executors.execute_update_context(
        pool,
        mock_enums,
        {
            "context_id": context_id,
            "status": "active",
            "scopes": ["public"],
        },
    )

    assert result["id"] == context_id
    assert len(pool.fetchrow_calls) == 1
    call = pool.fetchrow_calls[0]
    assert call[6] is not None
    assert call[8] is not None


@pytest.mark.asyncio
async def test_execute_update_context_string_payload_path(mock_enums):
    """String payloads should decode for update_context."""

    context_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": context_id}])

    result = await executors.execute_update_context(
        pool,
        mock_enums,
        json.dumps({"context_id": context_id, "title": "renamed"}),
    )

    assert result["id"] == context_id


@pytest.mark.asyncio
async def test_execute_update_context_raises_when_update_returns_missing(mock_enums):
    """Context update should raise when the update query returns no row."""

    pool = _PoolStub(fetchrow_rows=[None])

    with pytest.raises(ValueError, match="Context not found"):
        await executors.execute_update_context(
            pool,
            mock_enums,
            {"context_id": str(uuid4()), "title": "new"},
        )


@pytest.mark.asyncio
async def test_execute_create_relationship_reraises_unexpected_unique_violation(
    monkeypatch, mock_enums
):
    """Unexpected unique constraints should be re-raised unchanged."""

    class _FakeUniqueViolation(Exception):
        def __init__(self, constraint_name: str):
            super().__init__(constraint_name)
            self.constraint_name = constraint_name

    pool = _PoolStub(
        fetchrow_rows=[_FakeUniqueViolation("some_other_constraint")],
    )
    monkeypatch.setattr(executors, "UniqueViolationError", _FakeUniqueViolation)

    with pytest.raises(_FakeUniqueViolation):
        await executors.execute_create_relationship(
            pool,
            mock_enums,
            {
                "source_type": "entity",
                "source_id": str(uuid4()),
                "target_type": "entity",
                "target_id": str(uuid4()),
                "relationship_type": "related-to",
            },
        )


@pytest.mark.asyncio
async def test_execute_create_relationship_rejects_cycle(mock_enums):
    """Cycle-sensitive relationship types should reject cycle paths."""

    pool = _PoolStub(fetchval_rows=[True])

    with pytest.raises(ValueError, match="create a cycle"):
        await executors.execute_create_relationship(
            pool,
            mock_enums,
            {
                "source_type": "entity",
                "source_id": str(uuid4()),
                "target_type": "entity",
                "target_id": str(uuid4()),
                "relationship_type": "depends-on",
            },
        )


@pytest.mark.asyncio
async def test_execute_create_relationship_string_payload_success(mock_enums):
    """String payloads should decode for create_relationship success path."""

    relationship_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": relationship_id, "notes": ""}])

    result = await executors.execute_create_relationship(
        pool,
        mock_enums,
        json.dumps(
            {
                "source_type": "entity",
                "source_id": str(uuid4()),
                "target_type": "entity",
                "target_id": str(uuid4()),
                "relationship_type": "related-to",
            }
        ),
    )

    assert result["id"] == relationship_id
    assert result["notes"] == ""


@pytest.mark.asyncio
async def test_execute_create_relationship_rejects_self_reference(mock_enums):
    """Relationships must reject source==target edges for the same node."""

    node_id = str(uuid4())
    pool = _PoolStub()

    with pytest.raises(ValueError, match="Self-referential"):
        await executors.execute_create_relationship(
            pool,
            mock_enums,
            {
                "source_type": "entity",
                "source_id": node_id,
                "target_type": "entity",
                "target_id": node_id,
                "relationship_type": "related-to",
            },
        )


@pytest.mark.asyncio
async def test_execute_create_relationship_duplicate_maps_value_error(monkeypatch, mock_enums):
    """Known relationship duplicate constraint should map to ValueError."""

    class _FakeUniqueViolation(Exception):
        def __init__(self, constraint_name: str):
            super().__init__(constraint_name)
            self.constraint_name = constraint_name

    pool = _PoolStub(
        fetchrow_rows=[
            _FakeUniqueViolation("relationships_source_type_source_id_target_type_target_id_t_key")
        ],
    )
    monkeypatch.setattr(executors, "UniqueViolationError", _FakeUniqueViolation)

    with pytest.raises(ValueError, match="Relationship already exists"):
        await executors.execute_create_relationship(
            pool,
            mock_enums,
            {
                "source_type": "entity",
                "source_id": str(uuid4()),
                "target_type": "entity",
                "target_id": str(uuid4()),
                "relationship_type": "related-to",
            },
        )


@pytest.mark.asyncio
async def test_execute_update_job_raises_when_missing(mock_enums):
    """Job updates should raise not found when row update returns nothing."""

    pool = _PoolStub(fetchrow_rows=[None])

    with pytest.raises(ValueError, match="Job not found"):
        await executors.execute_update_job(
            pool,
            mock_enums,
            {"job_id": "2026Q1-ABCD"},
        )


@pytest.mark.asyncio
async def test_execute_update_job_status_field_resolves_status_id(mock_enums):
    """Job update path should resolve status names when status is supplied."""

    pool = _PoolStub(fetchrow_rows=[{"id": "2026Q1-ABCD"}])

    result = await executors.execute_update_job(
        pool,
        mock_enums,
        {"job_id": "2026Q1-ABCD", "status": "active"},
    )

    assert result == {"id": "2026Q1-ABCD"}


@pytest.mark.asyncio
async def test_execute_update_job_status_raises_when_missing(mock_enums):
    """Status updates should raise when the target job does not exist."""

    pool = _PoolStub(fetchrow_rows=[None])

    with pytest.raises(ValueError, match="Job not found"):
        await executors.execute_update_job_status(
            pool,
            mock_enums,
            {"job_id": "2026Q1-ABCD", "status": "active"},
        )


@pytest.mark.asyncio
async def test_execute_update_relationship_raises_when_missing(mock_enums):
    """Relationship updates should raise not found for missing ids."""

    pool = _PoolStub(fetchrow_rows=[None])

    with pytest.raises(ValueError, match="Relationship not found"):
        await executors.execute_update_relationship(
            pool,
            mock_enums,
            {"relationship_id": str(uuid4()), "status": "active"},
        )


@pytest.mark.asyncio
async def test_execute_update_relationship_success_returns_row(mock_enums):
    """Relationship updates should return normalized row on success."""

    relationship_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": relationship_id}])

    result = await executors.execute_update_relationship(
        pool,
        mock_enums,
        {"relationship_id": relationship_id, "status": "active"},
    )

    assert result == {"id": relationship_id}


@pytest.mark.asyncio
async def test_execute_update_file_status_branch(mock_enums):
    """File updates should resolve status names when supplied."""

    file_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": file_id}])
    result = await executors.execute_update_file(
        pool,
        mock_enums,
        {"file_id": file_id, "status": "active"},
    )
    assert result == {"id": file_id}


@pytest.mark.asyncio
async def test_execute_create_file_success(mock_enums):
    """File create executor should resolve status and return created row."""

    file_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": file_id, "filename": "alpha.txt"}])
    result = await executors.execute_create_file(
        pool,
        mock_enums,
        {
            "filename": "alpha.txt",
            "file_path": "/tmp/alpha.txt",
            "status": "active",
            "notes": "kind: text",
        },
    )
    assert result["id"] == file_id
    assert pool.fetchrow_calls[0][0] == executors.QUERIES["files/create"]


@pytest.mark.asyncio
async def test_execute_update_protocol_status_branch(mock_enums):
    """Protocol updates should resolve status names when supplied."""

    pool = _PoolStub(fetchrow_rows=[{"name": "alpha"}])
    result = await executors.execute_update_protocol(
        pool,
        mock_enums,
        {"name": "alpha", "status": "active"},
    )
    assert result == {"name": "alpha"}


@pytest.mark.asyncio
async def test_execute_create_protocol_success(mock_enums):
    """Protocol create executor should resolve status and return created row."""

    pool = _PoolStub(fetchrow_rows=[{"name": "p1", "status": "active"}])
    result = await executors.execute_create_protocol(
        pool,
        mock_enums,
        {
            "name": "p1",
            "title": "Protocol 1",
            "version": "1.0.0",
            "content": "steps",
            "protocol_type": "process",
            "applies_to": ["entity"],
            "status": "active",
            "tags": ["core"],
            "trusted": False,
            "notes": "owner: ops",
        },
    )
    assert result["name"] == "p1"
    assert pool.fetchrow_calls[0][0] == executors.QUERIES["protocols/create"]


@pytest.mark.asyncio
async def test_execute_update_log_log_type_and_status_branches(mock_enums):
    """Log updates should resolve both log type and status names."""

    log_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": log_id}])
    result = await executors.execute_update_log(
        pool,
        mock_enums,
        {"id": log_id, "log_type": "event", "status": "active"},
    )
    assert result == {"id": log_id}


@pytest.mark.asyncio
async def test_execute_update_entity_string_payload_decodes(mock_enums):
    """String payloads should decode for update_entity."""

    entity_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": entity_id}])

    result = await executors.execute_update_entity(
        pool,
        mock_enums,
        json.dumps(
            {
                "entity_id": entity_id,
                "status": "active",
            }
        ),
    )

    assert result["id"] == entity_id


@pytest.mark.asyncio
async def test_execute_update_entity_rejects_metadata_payload(mock_enums):
    """Update payloads should reject metadata keys."""

    pool = _PoolStub()
    with pytest.raises(ValidationError):
        await executors.execute_update_entity(
            pool,
            mock_enums,
            {"entity_id": str(uuid4()), "metadata": {"x": 1}},
        )


@pytest.mark.asyncio
async def test_execute_create_log_defaults_timestamp_and_maps_types(mock_enums):
    """Log create executor should resolve log/status ids and default timestamp."""

    log_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"id": log_id}])
    result = await executors.execute_create_log(
        pool,
        mock_enums,
        {
            "log_type": "event",
            "status": "active",
            "content": "k: v",
            "notes": "origin: unit",
        },
    )
    assert result == {"id": log_id}
    call = pool.fetchrow_calls[0]
    assert call[0] == executors.QUERIES["logs/create"]
    assert call[2].tzinfo == UTC


@pytest.mark.asyncio
async def test_execute_bulk_update_entity_tags_falls_back_to_first_row_value(
    mock_enums,
):
    """Bulk tag update should collect ids even when row key is not named 'id'."""

    raw_id = str(uuid4())
    pool = _PoolStub(fetch_rows=[[{"entity_id": raw_id}]])
    result = await executors.execute_bulk_update_entity_tags(
        pool,
        mock_enums,
        {"entity_ids": [str(uuid4())], "op": "add", "tags": ["alpha"]},
    )
    assert result == {"updated": 1, "entity_ids": [raw_id]}


@pytest.mark.asyncio
async def test_execute_bulk_update_entity_scopes_falls_back_to_first_row_value(
    mock_enums,
):
    """Bulk scope update should collect ids even when row key is not named 'id'."""

    raw_id = str(uuid4())
    pool = _PoolStub(fetch_rows=[[{"entity_id": raw_id}]])
    result = await executors.execute_bulk_update_entity_scopes(
        pool,
        mock_enums,
        {"entity_ids": [str(uuid4())], "op": "remove", "scopes": ["public"]},
    )
    assert result == {"updated": 1, "entity_ids": [raw_id]}


@pytest.mark.asyncio
async def test_execute_register_agent_parses_review_details_json_and_raises_missing(
    mock_enums,
):
    """JSON review payloads should parse before activation and still raise if missing."""

    agent_id = str(uuid4())
    pool = _PoolStub(fetchrow_rows=[{"requires_approval": True}, None])

    with pytest.raises(ValueError, match="not found"):
        await executors.execute_register_agent(
            pool,
            mock_enums,
            {"agent_id": agent_id, "requested_scopes": ["public"]},
            review_details='{"grant_scopes":["public"]}',
        )


@pytest.mark.asyncio
async def test_execute_register_agent_preserves_trusted_agent_on_reenroll(mock_enums):
    """Trusted agents should not be flipped back to approval mode implicitly."""

    agent_id = str(uuid4())
    pool = _PoolStub(
        fetchrow_rows=[
            {"requires_approval": False},
            {"id": uuid4(), "name": "trusted-agent"},
        ]
    )

    result = await executors.execute_register_agent(
        pool,
        mock_enums,
        {
            "agent_id": agent_id,
            "requested_scopes": ["public"],
            "requested_requires_approval": True,
        },
        review_details={},
    )

    assert result["requires_approval"] is False
    activate_call = pool.fetchrow_calls[1]
    assert activate_call[3] is False


@pytest.mark.asyncio
async def test_execute_register_agent_marks_approved_enrollment_when_metadata_present(
    mock_enums,
):
    """Enrollment should be marked approved when reviewer metadata is provided."""

    agent_id = str(uuid4())
    approval_id = str(uuid4())
    reviewed_by = str(uuid4())
    granted_scope_id = next(iter(mock_enums.scopes.name_to_id.values()))
    pool = _PoolStub(
        fetchrow_rows=[
            {"requires_approval": True},
            {"id": uuid4(), "name": "approved-agent"},
        ]
    )

    result = await executors.execute_register_agent(
        pool,
        mock_enums,
        {"agent_id": agent_id, "requested_scopes": ["public"]},
        review_details={
            "grant_scope_ids": [granted_scope_id],
            "grant_requires_approval": True,
            "_approval_id": approval_id,
            "_reviewed_by": reviewed_by,
        },
    )

    assert result["approval_id"] == approval_id
    pool.execute.assert_awaited_once()


@pytest.mark.asyncio
async def test_execute_create_entity_json_payload_duplicate_raises(mock_enums):
    """JSON payloads should decode and still hit duplicate guardrails."""

    pool = _PoolStub(fetchrow_rows=[{"id": uuid4()}])
    with pytest.raises(ValueError, match="already exists"):
        await executors.execute_create_entity(
            pool,
            mock_enums,
            json.dumps(
                {
                    "name": "dup",
                    "type": "project",
                    "status": "active",
                    "scopes": ["public"],
                    "source_path": "notes/dup.md",
                }
            ),
        )


@pytest.mark.asyncio
async def test_execute_string_payload_paths_for_core_mutations(mock_enums):
    """String change_details should decode across create/update executors."""

    job_id = "2026Q1-ABCD"
    file_id = str(uuid4())
    relationship_id = str(uuid4())
    log_id = str(uuid4())
    pool = _PoolStub(
        fetchrow_rows=[
            {"id": job_id},  # create_job
            {"id": job_id},  # update_job
            {"id": relationship_id},  # update_relationship
            {"id": job_id},  # update_job_status
            {"id": file_id},  # create_file
            {"id": file_id},  # update_file
            {"name": "proto-1"},  # create_protocol
            {"name": "proto-1"},  # update_protocol
            {"id": log_id},  # create_log
            {"id": log_id},  # update_log
        ],
        fetchval_rows=[False],  # relationship cycle check
    )

    created_job = await executors.execute_create_job(
        pool,
        mock_enums,
        json.dumps({"title": "job-1", "scopes": ["public"]}),
    )
    assert created_job["id"] == job_id

    updated_job = await executors.execute_update_job(
        pool,
        mock_enums,
        json.dumps({"job_id": job_id, "title": "job-1b"}),
    )
    assert updated_job["id"] == job_id

    updated_rel = await executors.execute_update_relationship(
        pool,
        mock_enums,
        json.dumps({"relationship_id": relationship_id}),
    )
    assert updated_rel["id"] == relationship_id

    updated_status = await executors.execute_update_job_status(
        pool,
        mock_enums,
        json.dumps({"job_id": job_id, "status": "active"}),
    )
    assert updated_status["id"] == job_id

    created_file = await executors.execute_create_file(
        pool,
        mock_enums,
        json.dumps({"filename": "file.txt", "file_path": "/tmp/file.txt"}),
    )
    assert created_file["id"] == file_id

    updated_file = await executors.execute_update_file(
        pool,
        mock_enums,
        json.dumps({"file_id": file_id, "filename": "file-renamed.txt"}),
    )
    assert updated_file["id"] == file_id

    created_protocol = await executors.execute_create_protocol(
        pool,
        mock_enums,
        json.dumps({"name": "proto-1", "title": "P1", "content": "steps"}),
    )
    assert created_protocol["name"] == "proto-1"

    updated_protocol = await executors.execute_update_protocol(
        pool,
        mock_enums,
        json.dumps({"name": "proto-1", "title": "P1-updated"}),
    )
    assert updated_protocol["name"] == "proto-1"

    created_log = await executors.execute_create_log(
        pool,
        mock_enums,
        json.dumps({"log_type": "event"}),
    )
    assert created_log["id"] == log_id

    updated_log = await executors.execute_update_log(
        pool,
        mock_enums,
        json.dumps({"id": log_id}),
    )
    assert updated_log["id"] == log_id


@pytest.mark.asyncio
async def test_execute_bulk_update_and_import_string_payload_wrappers(monkeypatch, mock_enums):
    """String payload wrappers should forward decoded dicts to underlying executors."""

    captured: dict[str, dict] = {}

    async def _create_entity(_pool, _enums, details):
        captured["entity"] = details
        return {"ok": "entity"}

    async def _create_context(_pool, _enums, details):
        captured["context"] = details
        return {"ok": "context"}

    async def _create_relationship(_pool, _enums, details):
        captured["relationship"] = details
        return {"ok": "relationship"}

    async def _create_job(_pool, _enums, details):
        captured["job"] = details
        return {"ok": "job"}

    monkeypatch.setattr(executors, "execute_create_entity", _create_entity)
    monkeypatch.setattr(executors, "execute_create_context", _create_context)
    monkeypatch.setattr(executors, "execute_create_relationship", _create_relationship)
    monkeypatch.setattr(executors, "execute_create_job", _create_job)

    pool = _PoolStub(fetch_rows=[[{"id": "ent-1"}], [{"id": "ent-2"}]])
    tags_result = await executors.execute_bulk_update_entity_tags(
        pool,
        mock_enums,
        json.dumps({"entity_ids": ["ent-1"], "op": "add", "tags": ["x"]}),
    )
    scopes_result = await executors.execute_bulk_update_entity_scopes(
        pool,
        mock_enums,
        json.dumps({"entity_ids": ["ent-2"], "op": "remove", "scopes": ["public"]}),
    )
    assert tags_result == {"updated": 1, "entity_ids": ["ent-1"]}
    assert scopes_result == {"updated": 1, "entity_ids": ["ent-2"]}

    ent = await executors.execute_bulk_import_entities(pool, mock_enums, {"name": "alpha"})
    ctx = await executors.execute_bulk_import_context(pool, mock_enums, {"title": "ctx"})
    rel = await executors.execute_bulk_import_relationships(
        pool, mock_enums, {"relationship_type": "related-to"}
    )
    job = await executors.execute_bulk_import_jobs(pool, mock_enums, {"title": "job"})

    assert ent == {"ok": "entity"}
    assert ctx == {"ok": "context"}
    assert rel == {"ok": "relationship"}
    assert job == {"ok": "job"}
    assert captured["entity"] == {"name": "alpha"}
    assert captured["context"] == {"title": "ctx"}
    assert captured["relationship"] == {"relationship_type": "related-to"}
    assert captured["job"] == {"title": "job"}


@pytest.mark.asyncio
async def test_execute_register_agent_parses_string_inputs_and_revert_entity_branches(
    monkeypatch, mock_enums
):
    """Register/revert should parse string payloads and validate required ids."""

    agent_id = str(uuid4())
    pool = _PoolStub(
        fetchrow_rows=[
            {"requires_approval": True},
            {"id": uuid4(), "name": "agent-string"},
        ]
    )

    result = await executors.execute_register_agent(
        pool,
        mock_enums,
        json.dumps({"agent_id": agent_id, "requested_scopes": ["public"]}),
        review_details='{"grant_scopes":["public"],"grant_requires_approval":false}',
    )
    assert result["name"] == "agent-string"
    assert result["requires_approval"] is False

    with pytest.raises(ValueError, match="entity_id and audit_id are required"):
        await executors.execute_revert_entity(pool, mock_enums, {"entity_id": ""})

    async def _fake_revert(_pool, entity_id, audit_id):
        return {"entity_id": entity_id, "audit_id": audit_id}

    monkeypatch.setattr("nebula_mcp.helpers.revert_entity", _fake_revert)
    reverted = await executors.execute_revert_entity(
        pool,
        mock_enums,
        json.dumps({"entity_id": str(uuid4()), "audit_id": str(uuid4())}),
    )
    assert "entity_id" in reverted
    assert "audit_id" in reverted

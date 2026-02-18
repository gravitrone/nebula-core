"""Unit coverage for relationship route helper logic."""

# Standard Library
from types import SimpleNamespace

# Third-Party
from fastapi import HTTPException
import pytest

# Local
from nebula_api.routes.relationships import (
    _has_write_scopes,
    _job_visible,
    _normalize_relationship_lookup,
    _validate_relationship_node,
)


pytestmark = pytest.mark.unit


class _DummyPool:
    """Minimal async pool stub keyed by node id."""

    def __init__(self, rows: dict[str, dict]):
        self._rows = rows
        self.calls = 0

    async def fetchrow(self, _query, *args):
        self.calls += 1
        node_id = str(args[0]) if args else ""
        return self._rows.get(node_id)


@pytest.fixture
def enums():
    """Build minimal enum registry shape used by route helpers."""

    scopes = SimpleNamespace(
        name_to_id={
            "admin": "scope-admin",
            "public": "scope-public",
            "private": "scope-private",
            "sensitive": "scope-sensitive",
        }
    )
    return SimpleNamespace(scopes=scopes)


def _auth(*, caller_type: str = "agent", scopes: list[str] | None = None) -> dict:
    """Create minimal auth context for helper tests."""

    return {
        "caller_type": caller_type,
        "scopes": scopes or [],
    }


def test_has_write_scopes_happy_and_edge_cases():
    """Write-scope helper should handle empty and subset cases predictably."""

    assert _has_write_scopes(["scope-public"], []) is True
    assert _has_write_scopes([], ["scope-public"]) is False
    assert _has_write_scopes(["scope-public", "scope-private"], ["scope-public"]) is True
    assert _has_write_scopes(["scope-public"], ["scope-private"]) is False


def test_normalize_relationship_lookup_rejects_invalid_source_type():
    """Unknown source type should fail fast with API error."""

    with pytest.raises(HTTPException) as exc:
        _normalize_relationship_lookup("weird", "123")
    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


def test_normalize_relationship_lookup_handles_job_ids():
    """Job lookup should normalize case and validate canonical format."""

    kind, job_id = _normalize_relationship_lookup("job", "2026q1-abcd")
    assert kind == "job"
    assert job_id == "2026Q1-ABCD"

    with pytest.raises(HTTPException) as exc:
        _normalize_relationship_lookup("job", "bad-job-id")
    assert exc.value.status_code == 400


@pytest.mark.asyncio
async def test_job_visible_respects_admin_and_scope_overlap(enums):
    """Job visibility helper should enforce scope checks for non-admin agents."""

    pool = _DummyPool(
        {
            "job-public": {"privacy_scope_ids": ["scope-public"]},
            "job-private": {"privacy_scope_ids": ["scope-private"]},
            "job-open": {"privacy_scope_ids": []},
        }
    )

    assert await _job_visible(
        pool,
        _auth(scopes=["scope-admin"]),
        enums,
        "job-public",
    )
    assert pool.calls == 0  # admin path bypasses fetch

    assert await _job_visible(
        pool,
        _auth(scopes=["scope-public"]),
        enums,
        "job-public",
    )
    assert await _job_visible(
        pool,
        _auth(scopes=["scope-public"]),
        enums,
        "job-open",
    )
    assert not await _job_visible(
        pool,
        _auth(scopes=["scope-public"]),
        enums,
        "job-private",
    )
    assert not await _job_visible(
        pool,
        _auth(scopes=["scope-public"]),
        enums,
        "job-missing",
    )


@pytest.mark.asyncio
async def test_validate_relationship_node_entity_and_context_guards(enums):
    """Node validation should reject missing/forbidden entity/context nodes."""

    rows = {
        "entity-open": {"privacy_scope_ids": []},
        "entity-private": {"privacy_scope_ids": ["scope-private"]},
        "context-open": {"privacy_scope_ids": []},
    }
    pool = _DummyPool(rows)

    # Non-agent and admin callers bypass privacy checks.
    await _validate_relationship_node(
        pool, enums, _auth(caller_type="entity"), "entity", "entity-private"
    )
    await _validate_relationship_node(
        pool, enums, _auth(scopes=["scope-admin"]), "entity", "entity-private"
    )

    await _validate_relationship_node(
        pool, enums, _auth(scopes=["scope-public"]), "entity", "entity-open"
    )
    await _validate_relationship_node(
        pool, enums, _auth(scopes=["scope-public"]), "context", "context-open"
    )

    with pytest.raises(HTTPException) as missing_entity:
        await _validate_relationship_node(
            pool, enums, _auth(scopes=["scope-public"]), "entity", "entity-missing"
        )
    assert missing_entity.value.status_code == 404

    with pytest.raises(HTTPException) as forbidden_entity:
        await _validate_relationship_node(
            pool, enums, _auth(scopes=["scope-public"]), "entity", "entity-private"
        )
    assert forbidden_entity.value.status_code == 403

    with pytest.raises(HTTPException) as missing_context:
        await _validate_relationship_node(
            pool, enums, _auth(scopes=["scope-public"]), "context", "context-missing"
        )
    assert missing_context.value.status_code == 404


@pytest.mark.asyncio
async def test_validate_relationship_node_job_visibility(enums):
    """Job nodes should use visibility checks for non-admin agents."""

    pool = _DummyPool(
        {
            "job-private": {"privacy_scope_ids": ["scope-private"]},
            "job-public": {"privacy_scope_ids": ["scope-public"]},
        }
    )

    await _validate_relationship_node(
        pool, enums, _auth(scopes=["scope-public"]), "job", "job-public"
    )

    with pytest.raises(HTTPException) as forbidden_job:
        await _validate_relationship_node(
            pool, enums, _auth(scopes=["scope-public"]), "job", "job-private"
        )
    assert forbidden_job.value.status_code == 403

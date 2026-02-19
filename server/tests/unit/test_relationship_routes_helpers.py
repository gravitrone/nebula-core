"""Unit coverage for relationship route helper logic."""

# Standard Library
from types import SimpleNamespace

# Third-Party
from fastapi import HTTPException
import pytest

# Local
from nebula_api.routes.relationships import (
    CreateRelationshipBody,
    UpdateRelationshipBody,
    _has_write_scopes,
    _job_visible,
    _normalize_relationship_lookup,
    _validate_relationship_node,
    create_relationship,
    get_relationships,
    query_relationships,
    update_relationship,
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


class _RoutePool:
    """Route-focused pool stub with queued fetchrow/fetch responses."""

    def __init__(self, *, fetchrows: list[dict | None] | None = None, fetch_rows=None):
        self._fetchrows = list(fetchrows or [])
        self._fetch_rows = list(fetch_rows or [])
        self.fetchrow_calls = []
        self.fetch_calls = []

    async def fetchrow(self, query, *args):
        self.fetchrow_calls.append((query, args))
        if not self._fetchrows:
            return None
        return self._fetchrows.pop(0)

    async def fetch(self, query, *args):
        self.fetch_calls.append((query, args))
        if not self._fetch_rows:
            return []
        return self._fetch_rows.pop(0)


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
    statuses = SimpleNamespace(name_to_id={"active": "status-active"})
    relationship_types = SimpleNamespace(name_to_id={"related-to": "rel-related"})
    return SimpleNamespace(
        scopes=scopes,
        statuses=statuses,
        relationship_types=relationship_types,
    )


def _auth(*, caller_type: str = "agent", scopes: list[str] | None = None) -> dict:
    """Create minimal auth context for helper tests."""

    return {
        "caller_type": caller_type,
        "agent": {"requires_approval": False},
        "scopes": scopes or [],
    }


def _request(pool, enums):
    """Build a minimal request-like object for direct route invocation."""

    return SimpleNamespace(
        app=SimpleNamespace(
            state=SimpleNamespace(
                pool=pool,
                enums=enums,
            )
        )
    )


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

    # Admin callers bypass privacy checks.
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

    with pytest.raises(HTTPException) as forbidden_user_entity:
        await _validate_relationship_node(
            pool,
            enums,
            _auth(caller_type="user", scopes=["scope-public"]),
            "entity",
            "entity-private",
        )
    assert forbidden_user_entity.value.status_code == 403

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


@pytest.mark.asyncio
async def test_validate_relationship_node_ignores_other_node_types(enums):
    """Non-guarded node types should pass through validation."""

    pool = _DummyPool({})
    await _validate_relationship_node(pool, enums, _auth(scopes=["scope-public"]), "file", "x")


@pytest.mark.asyncio
async def test_create_relationship_returns_pending_approval_payload(enums, monkeypatch):
    """create_relationship should short-circuit when approval response is returned."""

    pool = _RoutePool(
        fetchrows=[
            {"privacy_scope_ids": ["scope-public"]},  # source entity
            {"privacy_scope_ids": ["scope-public"]},  # target entity
        ]
    )
    auth = _auth(scopes=["scope-public"])
    req = _request(pool, enums)
    payload = CreateRelationshipBody(
        source_type="entity",
        source_id="f7bf1f24-4ebd-4ea7-9b5b-71f4a5229421",
        target_type="entity",
        target_id="0c894f8e-a4e8-4012-af2f-0f59d5efc7e8",
        relationship_type="related-to",
        properties={"note": "x"},
    )

    async def _approval(*_args, **_kwargs):
        return {"requires_approval": True, "approval_id": "appr-1"}

    monkeypatch.setattr("nebula_api.routes.relationships.maybe_check_agent_approval", _approval)
    result = await create_relationship(payload, req, auth=auth)
    assert result["requires_approval"] is True
    assert result["approval_id"] == "appr-1"


@pytest.mark.asyncio
async def test_create_relationship_maps_value_error_to_invalid_input(enums, monkeypatch):
    """create_relationship should convert executor ValueError to INVALID_INPUT."""

    pool = _RoutePool(
        fetchrows=[
            {"privacy_scope_ids": ["scope-public"]},  # source entity
            {"privacy_scope_ids": ["scope-public"]},  # target entity
        ]
    )
    auth = _auth(scopes=["scope-public"])
    req = _request(pool, enums)
    payload = CreateRelationshipBody(
        source_type="entity",
        source_id="7eb21153-fb90-4e67-b8af-e8f4f00b1f1c",
        target_type="entity",
        target_id="0e5dbf9d-f40e-4813-8131-cc12d9e223f7",
        relationship_type="related-to",
    )

    async def _approval(*_args, **_kwargs):
        return None

    async def _raise(*_args, **_kwargs):
        raise ValueError("bad relationship payload")

    monkeypatch.setattr("nebula_api.routes.relationships.maybe_check_agent_approval", _approval)
    monkeypatch.setattr("nebula_api.routes.relationships.execute_create_relationship", _raise)

    with pytest.raises(HTTPException) as exc:
        await create_relationship(payload, req, auth=auth)
    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_create_relationship_success_payload(enums, monkeypatch):
    """create_relationship should return success payload when executor succeeds."""

    pool = _RoutePool(
        fetchrows=[
            {"privacy_scope_ids": ["scope-public"]},
            {"privacy_scope_ids": ["scope-public"]},
        ]
    )
    auth = _auth(scopes=["scope-public"])
    req = _request(pool, enums)
    payload = CreateRelationshipBody(
        source_type="entity",
        source_id="7eb21153-fb90-4e67-b8af-e8f4f00b1f1c",
        target_type="entity",
        target_id="0e5dbf9d-f40e-4813-8131-cc12d9e223f7",
        relationship_type="related-to",
    )

    async def _approval(*_args, **_kwargs):
        return None

    async def _ok(*_args, **_kwargs):
        return {"id": "rel-1", "source_type": "entity"}

    monkeypatch.setattr("nebula_api.routes.relationships.maybe_check_agent_approval", _approval)
    monkeypatch.setattr("nebula_api.routes.relationships.execute_create_relationship", _ok)
    result = await create_relationship(payload, req, auth=auth)
    assert result["data"]["id"] == "rel-1"


@pytest.mark.asyncio
async def test_create_relationship_rejects_unknown_relationship_type(enums):
    """Unknown relationship type should return INVALID_INPUT before approval checks."""

    pool = _RoutePool(
        fetchrows=[
            {"privacy_scope_ids": ["scope-public"]},
            {"privacy_scope_ids": ["scope-public"]},
        ]
    )
    auth = _auth(scopes=["scope-public"])
    req = _request(pool, enums)
    payload = CreateRelationshipBody(
        source_type="entity",
        source_id="7eb21153-fb90-4e67-b8af-e8f4f00b1f1c",
        target_type="entity",
        target_id="0e5dbf9d-f40e-4813-8131-cc12d9e223f7",
        relationship_type="missing-type",
    )
    with pytest.raises(HTTPException) as exc:
        await create_relationship(payload, req, auth=auth)
    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_update_relationship_not_found_paths(enums):
    """update_relationship should return NOT_FOUND for missing relationships."""

    # Missing on initial lookup.
    req = _request(_RoutePool(fetchrows=[None]), enums)
    with pytest.raises(HTTPException) as missing_initial:
        await update_relationship(
            "d3964596-2b5f-4f1c-9f47-c66dc4fd1856",
            UpdateRelationshipBody(properties={"a": 1}),
            req,
            auth=_auth(scopes=["scope-public"]),
        )
    assert missing_initial.value.status_code == 404

    # Missing on update result.
    req2 = _request(
        _RoutePool(
            fetchrows=[
                {
                    "source_type": "entity",
                    "source_id": "3a6f69b8-f6ff-47c6-8f95-212c1f1f8338",
                    "target_type": "entity",
                    "target_id": "85b3f4e8-cf08-48dc-9ef9-663f6d71da53",
                },
                {"privacy_scope_ids": ["scope-public"]},
                {"privacy_scope_ids": ["scope-public"]},
                None,  # update query returns no row
            ]
        ),
        enums,
    )
    with pytest.raises(HTTPException) as missing_update:
        await update_relationship(
            "6d822f58-94a0-4ab2-9f8c-eb1b7e8c8f77",
            UpdateRelationshipBody(properties={"a": 1}, status="active"),
            req2,
            auth=_auth(scopes=["scope-public"]),
        )
    assert missing_update.value.status_code == 404


@pytest.mark.asyncio
async def test_update_relationship_returns_pending_approval_payload(enums, monkeypatch):
    """update_relationship should short-circuit when approval is required."""

    pool = _RoutePool(
        fetchrows=[
            {
                "source_type": "entity",
                "source_id": "3a6f69b8-f6ff-47c6-8f95-212c1f1f8338",
                "target_type": "entity",
                "target_id": "85b3f4e8-cf08-48dc-9ef9-663f6d71da53",
            },
            {"privacy_scope_ids": ["scope-public"]},
            {"privacy_scope_ids": ["scope-public"]},
        ]
    )
    req = _request(pool, enums)

    async def _approval(*_args, **_kwargs):
        return {"requires_approval": True, "approval_id": "appr-update"}

    monkeypatch.setattr("nebula_api.routes.relationships.maybe_check_agent_approval", _approval)
    result = await update_relationship(
        "5a898df8-7f2f-478f-b796-bf013dc5172a",
        UpdateRelationshipBody(properties={"note": "ok"}, status="active"),
        req,
        auth=_auth(scopes=["scope-public"]),
    )
    assert result["requires_approval"] is True
    assert result["approval_id"] == "appr-update"


@pytest.mark.asyncio
async def test_update_relationship_success_payload(enums, monkeypatch):
    """update_relationship should return updated row payload on success."""

    pool = _RoutePool(
        fetchrows=[
            {
                "source_type": "entity",
                "source_id": "3a6f69b8-f6ff-47c6-8f95-212c1f1f8338",
                "target_type": "entity",
                "target_id": "85b3f4e8-cf08-48dc-9ef9-663f6d71da53",
            },
            {"privacy_scope_ids": ["scope-public"]},
            {"privacy_scope_ids": ["scope-public"]},
            {"id": "rel-updated", "status": "active"},
        ]
    )
    req = _request(pool, enums)

    async def _approval(*_args, **_kwargs):
        return None

    monkeypatch.setattr("nebula_api.routes.relationships.maybe_check_agent_approval", _approval)
    result = await update_relationship(
        "5a898df8-7f2f-478f-b796-bf013dc5172a",
        UpdateRelationshipBody(properties={"note": "ok"}, status="active"),
        req,
        auth=_auth(scopes=["scope-public"]),
    )
    assert result["data"]["id"] == "rel-updated"


@pytest.mark.asyncio
async def test_update_relationship_rejects_unknown_status(enums):
    """Unknown relationship status should fail with INVALID_INPUT."""

    pool = _RoutePool(
        fetchrows=[
            {
                "source_type": "entity",
                "source_id": "3a6f69b8-f6ff-47c6-8f95-212c1f1f8338",
                "target_type": "entity",
                "target_id": "85b3f4e8-cf08-48dc-9ef9-663f6d71da53",
            },
            {"privacy_scope_ids": ["scope-public"]},
            {"privacy_scope_ids": ["scope-public"]},
        ]
    )
    req = _request(pool, enums)
    with pytest.raises(HTTPException) as exc:
        await update_relationship(
            "5a898df8-7f2f-478f-b796-bf013dc5172a",
            UpdateRelationshipBody(status="not-a-status"),
            req,
            auth=_auth(scopes=["scope-public"]),
        )
    assert exc.value.status_code == 400
    assert exc.value.detail["error"]["code"] == "INVALID_INPUT"


@pytest.mark.asyncio
async def test_get_and_query_relationships_filter_hidden_jobs_for_agents(enums):
    """Route list handlers should skip out-of-scope job links for agents."""

    job_hidden = {
        "id": "rel-hidden",
        "source_type": "job",
        "source_id": "job-private",
        "target_type": "entity",
        "target_id": "8f6afcf9-bb22-4e79-b129-4328922e4a45",
    }
    job_visible = {
        "id": "rel-visible",
        "source_type": "entity",
        "source_id": "8f6afcf9-bb22-4e79-b129-4328922e4a45",
        "target_type": "job",
        "target_id": "job-public",
    }
    pool = _RoutePool(
        fetchrows=[
            {"privacy_scope_ids": ["scope-private"]},  # job-private lookup
            {"privacy_scope_ids": ["scope-public"]},  # job-public lookup
            {"privacy_scope_ids": ["scope-private"]},  # query job-private lookup
            {"privacy_scope_ids": ["scope-public"]},  # query job-public lookup
        ],
        fetch_rows=[[job_hidden, job_visible], [job_hidden, job_visible]],
    )
    req = _request(pool, enums)
    auth = _auth(scopes=["scope-public"])

    list_result = await get_relationships(
        "entity",
        "8f6afcf9-bb22-4e79-b129-4328922e4a45",
        req,
        auth=auth,
    )
    query_result = await query_relationships(req, auth=auth)

    assert [row["id"] for row in list_result["data"]] == ["rel-visible"]
    assert [row["id"] for row in query_result["data"]] == ["rel-visible"]


@pytest.mark.asyncio
async def test_query_relationships_filters_hidden_jobs_for_user(enums):
    """User callers should also be filtered from out-of-scope job links."""

    row = {
        "id": "rel-admin",
        "source_type": "job",
        "source_id": "job-private",
        "target_type": "entity",
        "target_id": "8f6afcf9-bb22-4e79-b129-4328922e4a45",
    }
    pool = _RoutePool(fetch_rows=[[row]])
    req = _request(pool, enums)
    result = await query_relationships(req, auth=_auth(caller_type="user"))
    assert result["data"] == []


@pytest.mark.asyncio
async def test_query_relationships_filters_hidden_target_job(enums):
    """Query should skip rows when target job is outside caller scopes."""

    row = {
        "id": "rel-hidden-target",
        "source_type": "entity",
        "source_id": "8f6afcf9-bb22-4e79-b129-4328922e4a45",
        "target_type": "job",
        "target_id": "job-private",
    }
    pool = _RoutePool(
        fetchrows=[{"privacy_scope_ids": ["scope-private"]}],
        fetch_rows=[[row]],
    )
    req = _request(pool, enums)
    result = await query_relationships(req, auth=_auth(scopes=["scope-public"]))
    assert result["data"] == []

"""Bulk import route tests."""

# Standard Library
import json

import pytest

# Third-Party
from httpx import ASGITransport, AsyncClient

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


def _untrusted_auth_override(agent_row: dict, enums: object, scopes: list[str]):
    """Override require_auth to simulate an untrusted agent caller."""

    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]
    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_row["id"],
        "agent": {**agent_row, "requires_approval": True},
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Handle mock auth.

        Returns:
            Result value from the operation.
        """

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_import_entities_json(api):
    """Bulk import entities from json."""

    payload = {
        "format": "json",
        "items": [
            {
                "name": "Import Entity",
                "type": "person",
                "scopes": ["public"],
                "tags": ["import"],
            }
        ],
    }
    r = await api.post("/api/import/entities", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 1
    assert data["failed"] == 0


@pytest.mark.asyncio
async def test_import_entities_invalid_format_returns_400(api):
    """Invalid import format should return validation error."""

    r = await api.post(
        "/api/import/entities",
        json={"format": "xml", "items": [{"name": "x", "type": "person"}]},
    )
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_import_entities_json_without_items_returns_400(api):
    """JSON import should require items."""

    r = await api.post("/api/import/entities", json={"format": "json"})
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_import_entities_csv_without_data_returns_400(api):
    """CSV import should require raw CSV data."""

    r = await api.post("/api/import/entities", json={"format": "csv"})
    assert r.status_code == 400
    assert r.json()["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_import_entities_csv(api):
    """Bulk import entities from csv."""

    payload = {
        "format": "csv",
        "data": "name,type,scopes,tags\nCSV Entity,person,public,import",
    }
    r = await api.post("/api/import/entities", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 1


@pytest.mark.asyncio
async def test_import_entities_missing_required_fields_returns_failed_row(api):
    """Rows missing required fields should be reported as failures, not 500s."""

    payload = {
        "format": "json",
        "items": [{"status": "active"}],
    }
    r = await api.post("/api/import/entities", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 0
    assert data["failed"] == 1
    assert "required" in data["errors"][0]["error"].lower()


@pytest.mark.asyncio
async def test_import_context_json(api):
    """Bulk import context from json."""

    payload = {
        "format": "json",
        "items": [
            {
                "title": "Import Context",
                "source_type": "note",
                "scopes": ["public"],
                "content": "test",
            }
        ],
    }
    r = await api.post("/api/import/context", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 1


@pytest.mark.asyncio
async def test_import_context_invalid_url_returns_failed_row(api):
    """Context rows with invalid URL should fail cleanly."""

    payload = {
        "format": "json",
        "items": [
            {
                "title": "Bad URL",
                "source_type": "note",
                "scopes": ["public"],
                "url": "ftp://example.com",
            }
        ],
    }
    r = await api.post("/api/import/context", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 0
    assert data["failed"] == 1
    assert "http://" in data["errors"][0]["error"] or "https://" in data["errors"][0]["error"]


@pytest.mark.asyncio
async def test_import_relationships_json(api):
    """Bulk import relationships from json."""

    r1 = await api.post(
        "/api/entities",
        json={"name": "ImportSource", "type": "person", "scopes": ["public"]},
    )
    r2 = await api.post(
        "/api/entities",
        json={"name": "ImportTarget", "type": "person", "scopes": ["public"]},
    )
    source_id = r1.json()["data"]["id"]
    target_id = r2.json()["data"]["id"]

    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "entity",
                "source_id": source_id,
                "target_type": "entity",
                "target_id": target_id,
                "relationship_type": "related-to",
            }
        ],
    }
    r = await api.post("/api/import/relationships", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 1


@pytest.mark.asyncio
async def test_import_relationships_invalid_type_returns_failed_row(api):
    """Relationship import should report unknown relationship types."""

    r1 = await api.post(
        "/api/entities",
        json={"name": "BadRelSource", "type": "person", "scopes": ["public"]},
    )
    r2 = await api.post(
        "/api/entities",
        json={"name": "BadRelTarget", "type": "person", "scopes": ["public"]},
    )
    payload = {
        "format": "json",
        "items": [
            {
                "source_type": "entity",
                "source_id": r1.json()["data"]["id"],
                "target_type": "entity",
                "target_id": r2.json()["data"]["id"],
                "relationship_type": "does-not-exist",
            }
        ],
    }
    r = await api.post("/api/import/relationships", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 0
    assert data["failed"] == 1


@pytest.mark.asyncio
async def test_import_jobs_json(api):
    """Bulk import jobs from json."""

    payload = {
        "format": "json",
        "items": [
            {
                "title": "Import Job",
                "description": "test",
                "priority": "high",
            }
        ],
    }
    r = await api.post("/api/import/jobs", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 1


@pytest.mark.asyncio
async def test_import_jobs_invalid_priority_returns_failed_row(api):
    """Job import should report invalid priority values."""

    payload = {
        "format": "json",
        "items": [{"title": "Bad Priority Job", "priority": "urgent"}],
    }
    r = await api.post("/api/import/jobs", json=payload)
    assert r.status_code == 200
    data = r.json()["data"]
    assert data["created"] == 0
    assert data["failed"] == 1


@pytest.mark.asyncio
async def test_import_entities_untrusted_agent_invalid_type_rejected_preapproval(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted entity imports should validate type before queueing approvals."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/entities",
            json={
                "format": "json",
                "items": [{"name": "Bad Type Queue", "type": "made-up", "scopes": ["public"]}],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert body["status"] == "approval_required"
    assert len(body["approvals"]) == 0
    assert len(body["errors"]) == 1


@pytest.mark.asyncio
async def test_import_entities_untrusted_agent_invalid_status_rejected_preapproval(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted entity imports should validate status before queueing approvals."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/entities",
            json={
                "format": "json",
                "items": [
                    {
                        "name": "Bad Status Queue",
                        "type": "person",
                        "status": "todo",
                        "scopes": ["public"],
                    }
                ],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert body["status"] == "approval_required"
    assert len(body["approvals"]) == 0
    assert len(body["errors"]) == 1


@pytest.mark.asyncio
async def test_import_entities_untrusted_agent_success_queues_approval(
    db_pool, enums, untrusted_agent_row
):
    """Valid untrusted entity imports should create approval rows."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/entities",
            json={
                "format": "json",
                "items": [{"name": "Queue Entity", "type": "person", "scopes": ["public"]}],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert len(body["approvals"]) == 1
    approval_id = body["approvals"][0]["approval_id"]
    row = await db_pool.fetchrow(
        "SELECT id FROM approval_requests WHERE id = $1::uuid", approval_id
    )
    assert row is not None


@pytest.mark.asyncio
async def test_import_context_untrusted_agent_invalid_scope_rejected_preapproval(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted context imports should enforce scope subset before queueing."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/context",
            json={
                "format": "json",
                "items": [
                    {
                        "title": "Scope Leak",
                        "source_type": "note",
                        "scopes": ["admin"],
                    }
                ],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert body["status"] == "approval_required"
    assert len(body["approvals"]) == 0
    assert len(body["errors"]) == 1


@pytest.mark.asyncio
async def test_import_context_untrusted_agent_success_queues_approval(
    db_pool, enums, untrusted_agent_row
):
    """Valid untrusted context imports should create approval rows."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/context",
            json={
                "format": "json",
                "items": [
                    {
                        "title": "Queued Context",
                        "source_type": "note",
                        "scopes": ["public"],
                    }
                ],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert len(body["approvals"]) == 1


@pytest.mark.asyncio
async def test_import_relationships_untrusted_agent_rejects_foreign_job_node(
    api, db_pool, enums, untrusted_agent_row
):
    """Untrusted relationship imports should reject job nodes not owned by agent."""

    job = await api.post("/api/jobs", json={"title": "Foreign Parent"})
    job_id = job.json()["data"]["id"]
    entity = await api.post(
        "/api/entities",
        json={"name": "Rel Target", "type": "person", "scopes": ["public"]},
    )
    entity_id = entity.json()["data"]["id"]

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/relationships",
            json={
                "format": "json",
                "items": [
                    {
                        "source_type": "job",
                        "source_id": job_id,
                        "target_type": "entity",
                        "target_id": entity_id,
                        "relationship_type": "references",
                    }
                ],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert body["status"] == "approval_required"
    assert len(body["approvals"]) == 0
    assert len(body["errors"]) == 1


@pytest.mark.asyncio
async def test_import_relationships_untrusted_agent_success_queues_approval(
    api, db_pool, enums, untrusted_agent_row
):
    """Valid untrusted relationship imports should create approval rows."""

    source = await api.post(
        "/api/entities",
        json={"name": "Queued Rel Source", "type": "person", "scopes": ["public"]},
    )
    target = await api.post(
        "/api/entities",
        json={"name": "Queued Rel Target", "type": "person", "scopes": ["public"]},
    )
    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/relationships",
            json={
                "format": "json",
                "items": [
                    {
                        "source_type": "entity",
                        "source_id": source.json()["data"]["id"],
                        "target_type": "entity",
                        "target_id": target.json()["data"]["id"],
                        "relationship_type": "related-to",
                    }
                ],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert len(body["approvals"]) == 1


@pytest.mark.asyncio
async def test_import_relationships_untrusted_agent_missing_node_reports_error(
    api, db_pool, enums, untrusted_agent_row
):
    """Missing relationship nodes should be reported as row errors."""

    source = await api.post(
        "/api/entities",
        json={"name": "Missing Target Source", "type": "person", "scopes": ["public"]},
    )
    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/relationships",
            json={
                "format": "json",
                "items": [
                    {
                        "source_type": "entity",
                        "source_id": source.json()["data"]["id"],
                        "target_type": "entity",
                        "target_id": "00000000-0000-0000-0000-000000000001",
                        "relationship_type": "related-to",
                    }
                ],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert len(body["approvals"]) == 0
    assert len(body["errors"]) == 1


@pytest.mark.asyncio
async def test_import_jobs_untrusted_agent_invalid_priority_rejected_preapproval(
    db_pool, enums, untrusted_agent_row
):
    """Untrusted job imports should validate priority before queueing approvals."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/jobs",
            json={
                "format": "json",
                "items": [{"title": "Bad Queue Priority", "priority": "urgent"}],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert body["status"] == "approval_required"
    assert len(body["approvals"]) == 0
    assert len(body["errors"]) == 1


@pytest.mark.asyncio
async def test_import_jobs_untrusted_agent_success_queues_and_sets_agent_id(
    db_pool, enums, untrusted_agent_row
):
    """Valid untrusted job imports should queue approvals with caller agent_id."""

    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/jobs",
            json={
                "format": "json",
                "items": [{"title": "Queued Job", "priority": "high"}],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 202
    body = resp.json()
    assert len(body["approvals"]) == 1
    approval_id = body["approvals"][0]["approval_id"]
    row = await db_pool.fetchrow(
        "SELECT change_details FROM approval_requests WHERE id = $1::uuid",
        approval_id,
    )
    assert row is not None
    change_details = row["change_details"]
    if isinstance(change_details, str):
        change_details = json.loads(change_details)
    assert change_details["agent_id"] == str(untrusted_agent_row["id"])


@pytest.mark.asyncio
async def test_import_untrusted_agent_rate_limited_returns_429(
    db_pool, enums, untrusted_agent_row, monkeypatch
):
    """Import should return 429 when approval queue capacity check fails."""

    async def _raise_capacity(*_args, **_kwargs):
        """Handle raise capacity.

        Args:
            *_args: Input parameter for _raise_capacity.
            **_kwargs: Input parameter for _raise_capacity.
        """

        raise ValueError("approval queue full")

    from nebula_api.routes import imports as imports_routes

    monkeypatch.setattr(imports_routes, "ensure_approval_capacity", _raise_capacity)
    app.state.pool = db_pool
    app.state.enums = enums
    app.dependency_overrides[require_auth] = _untrusted_auth_override(
        untrusted_agent_row, enums, ["public"]
    )
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/import/entities",
            json={
                "format": "json",
                "items": [{"name": "Rate Limited", "type": "person", "scopes": ["public"]}],
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 429
    assert resp.json()["status"] == "rate_limited"


@pytest.mark.asyncio
async def test_import_entities_trusted_agent_runs_direct_write_path(api_agent_auth):
    """Trusted agent imports should use direct write path and return created items."""

    resp = await api_agent_auth.post(
        "/api/import/entities",
        json={
            "format": "json",
            "items": [{"name": "Trusted Import", "type": "person", "scopes": ["public"]}],
        },
    )
    assert resp.status_code == 200
    data = resp.json()["data"]
    assert data["created"] == 1
    assert data["failed"] == 0

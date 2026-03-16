"""Red team API tests for export access controls."""

# Standard Library
import json

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_entity(db_pool, enums, name, scopes):
    """Insert an entity for export access tests."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["test"],
    )
    return dict(row)


async def _make_agent(db_pool, enums, name):
    """Insert an agent for export access tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO agents (name, description, scopes, requires_approval, status_id)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *
        """,
        name,
        "redteam agent",
        scope_ids,
        False,
        status_id,
    )
    return dict(row)


async def _make_job(db_pool, enums, agent_id, scopes):
    """Insert a job for export access tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, privacy_scope_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        "Export Private Job",
        status_id,
        agent_id,
        scope_ids,
    )
    return dict(row)


async def _make_context(db_pool, enums, title, scopes):
    """Insert context for export access tests."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, privacy_scope_ids, status_id, tags)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING *
        """,
        title,
        "note",
        "secret",
        scope_ids,
        status_id,
        ["test"],
    )
    return dict(row)


async def _make_relationship(db_pool, enums, source_type, source_id, target_type, target_id):
    """Insert a relationship for export access tests."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.relationship_types.name_to_id["related-to"]

    row = await db_pool.fetchrow(
        """
        INSERT INTO relationships (source_type, source_id, target_type, target_id, type_id, status_id, properties)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
        RETURNING *
        """,
        source_type,
        source_id,
        target_type,
        target_id,
        type_id,
        status_id,
        json.dumps({"note": "link"}),
    )
    return dict(row)


def _auth_override(agent_id, enums):
    """Build auth override for public agent."""

    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_id,
        "agent": {"id": agent_id},
        "scopes": [enums.scopes.name_to_id["public"]],
    }

    async def mock_auth():
        """Mock auth for public agent."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_export_entities_denies_scope_override(db_pool, enums):
    """Export entities should not allow requesting scopes outside caller access."""

    await _make_entity(db_pool, enums, "Sensitive", ["private"])

    viewer = await _make_agent(db_pool, enums, "export-scope-viewer")
    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/export/entities?scopes=private")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400
    body = resp.json()
    assert body["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_export_snapshot_filters_jobs_by_agent(db_pool, enums):
    """Snapshot export should filter jobs by scopes."""

    owner = await _make_agent(db_pool, enums, "job-owner-export")
    viewer = await _make_agent(db_pool, enums, "job-viewer-export")
    public_job = await _make_job(db_pool, enums, owner["id"], ["public"])
    private_job = await _make_job(db_pool, enums, owner["id"], ["private"])

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/export/snapshot")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    jobs = resp.json()["data"]["jobs"]
    ids = {row["id"] for row in jobs}
    assert public_job["id"] in ids
    assert private_job["id"] not in ids


@pytest.mark.asyncio
async def test_export_snapshot_filters_job_relationships(db_pool, enums):
    """Snapshot export should filter relationships tied to jobs by job scopes."""

    owner = await _make_agent(db_pool, enums, "job-owner-export-rel")
    viewer = await _make_agent(db_pool, enums, "job-viewer-export-rel")
    entity = await _make_entity(db_pool, enums, "Public Link", ["public"])
    public_job = await _make_job(db_pool, enums, owner["id"], ["public"])
    private_job = await _make_job(db_pool, enums, owner["id"], ["private"])
    public_rel = await _make_relationship(
        db_pool, enums, "job", public_job["id"], "entity", str(entity["id"])
    )
    private_rel = await _make_relationship(
        db_pool, enums, "job", private_job["id"], "entity", str(entity["id"])
    )

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/export/snapshot")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    rels = resp.json()["data"]["relationships"]
    ids = {row["id"] for row in rels}
    assert str(public_rel["id"]) in ids
    assert str(private_rel["id"]) not in ids


@pytest.mark.asyncio
async def test_export_jobs_filters_by_agent(db_pool, enums):
    """Job exports should filter by scopes."""

    owner = await _make_agent(db_pool, enums, "job-owner-export-jobs")
    viewer = await _make_agent(db_pool, enums, "job-viewer-export-jobs")
    public_job = await _make_job(db_pool, enums, owner["id"], ["public"])
    private_job = await _make_job(db_pool, enums, owner["id"], ["private"])

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/export/jobs")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    items = resp.json()["data"]["items"]
    ids = {row["id"] for row in items}
    assert public_job["id"] in ids
    assert private_job["id"] not in ids


@pytest.mark.asyncio
async def test_export_context_denies_scope_override(db_pool, enums):
    """Export context should not allow requesting scopes outside caller access."""

    await _make_context(db_pool, enums, "Sensitive Context", ["private"])

    viewer = await _make_agent(db_pool, enums, "context-scope-viewer")
    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/export/context?scopes=private")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 400
    body = resp.json()
    assert body["detail"]["error"]["code"] == "VALIDATION_ERROR"


@pytest.mark.asyncio
async def test_export_relationships_filters_job_ownership(db_pool, enums):
    """Relationship exports should filter relationships tied to jobs by scopes."""

    owner = await _make_agent(db_pool, enums, "rel-job-owner")
    viewer = await _make_agent(db_pool, enums, "rel-job-viewer")
    public_job = await _make_job(db_pool, enums, owner["id"], ["public"])
    private_job = await _make_job(db_pool, enums, owner["id"], ["private"])
    entity = await _make_entity(db_pool, enums, "Public Link", ["public"])

    public_rel = await _make_relationship(
        db_pool, enums, "job", public_job["id"], "entity", str(entity["id"])
    )
    private_rel = await _make_relationship(
        db_pool, enums, "job", private_job["id"], "entity", str(entity["id"])
    )

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/export/relationships")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    items = resp.json()["data"]["items"]
    ids = {row["id"] for row in items}
    assert str(public_rel["id"]) in ids
    assert str(private_rel["id"]) not in ids

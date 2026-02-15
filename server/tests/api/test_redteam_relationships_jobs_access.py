"""Red team API tests for relationships involving jobs."""

# Standard Library
import json

# Third-Party
from httpx import ASGITransport, AsyncClient
import pytest

# Local
from nebula_api.app import app
from nebula_api.auth import require_auth


async def _make_agent(db_pool, enums, name):
    """Insert a test agent for relationships API scenarios."""

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


async def _make_entity(db_pool, enums, name):
    """Insert a test entity for relationships API scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id["public"]]

    row = await db_pool.fetchrow(
        """
        INSERT INTO entities (name, type_id, status_id, privacy_scope_ids, tags, metadata)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb)
        RETURNING *
        """,
        name,
        type_id,
        status_id,
        scope_ids,
        ["test"],
        json.dumps({"note": "public"}),
    )
    return dict(row)


async def _make_job(db_pool, enums, title, agent_id, scopes):
    """Insert a test job for relationships API scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, metadata, privacy_scope_ids)
        VALUES ($1, $2, $3, $4::jsonb, $5)
        RETURNING *
        """,
        title,
        status_id,
        agent_id,
        json.dumps({"secret": "job"}),
        scope_ids,
    )
    return dict(row)


async def _make_relationship(
    db_pool, enums, source_type, source_id, target_type, target_id
):
    """Insert a relationship linking entities/jobs for API access checks."""

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
        json.dumps({"note": "job-link"}),
    )
    return dict(row)


def _auth_override(agent_id, enums):
    """Build an auth override for public agent API requests."""

    scope_ids = [enums.scopes.name_to_id["public"]]
    auth_dict = {
        "key_id": None,
        "caller_type": "agent",
        "entity_id": None,
        "entity": None,
        "agent_id": agent_id,
        "agent": {"id": agent_id},
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth for public agent."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_get_relationships_hides_foreign_job_links(db_pool, enums):
    """Relationships API should filter job links by job scopes."""

    owner = await _make_agent(db_pool, enums, "rel-owner-api")
    viewer = await _make_agent(db_pool, enums, "rel-viewer-api")
    entity = await _make_entity(db_pool, enums, "Public Node")
    public_job = await _make_job(db_pool, enums, "Public Job", owner["id"], ["public"])
    private_job = await _make_job(
        db_pool, enums, "Private Job", owner["id"], ["personal"]
    )
    public_rel = await _make_relationship(
        db_pool, enums, "entity", str(entity["id"]), "job", public_job["id"]
    )
    private_rel = await _make_relationship(
        db_pool, enums, "entity", str(entity["id"]), "job", private_job["id"]
    )

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get(f"/api/relationships/entity/{entity['id']}")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {row["id"] for row in resp.json()["data"]}
    assert str(public_rel["id"]) in ids
    assert str(private_rel["id"]) not in ids


@pytest.mark.asyncio
async def test_query_relationships_hides_foreign_job_links(db_pool, enums):
    """Query relationships should filter job relationships by job scopes."""

    owner = await _make_agent(db_pool, enums, "rel-owner-api-2")
    viewer = await _make_agent(db_pool, enums, "rel-viewer-api-2")
    entity = await _make_entity(db_pool, enums, "Public Node 2")
    public_job = await _make_job(db_pool, enums, "Public Job 2", owner["id"], ["public"])
    private_job = await _make_job(
        db_pool, enums, "Private Job 2", owner["id"], ["personal"]
    )
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
    async with AsyncClient(
        transport=transport, base_url="http://test", follow_redirects=True
    ) as client:
        resp = await client.get("/api/relationships/")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {row["id"] for row in resp.json()["data"]}
    assert str(public_rel["id"]) in ids
    assert str(private_rel["id"]) not in ids

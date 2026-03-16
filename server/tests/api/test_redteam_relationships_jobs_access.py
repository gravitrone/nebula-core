"""Red team API tests for relationships involving jobs."""

# Standard Library
import json

# Third-Party
import pytest
from httpx import ASGITransport, AsyncClient

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


async def _make_entity(db_pool, enums, name, scopes=None):
    """Insert a test entity for relationships API scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    type_id = enums.entity_types.name_to_id["person"]
    scope_ids = [enums.scopes.name_to_id[s] for s in (scopes or ["public"])]

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


async def _make_context(db_pool, enums, title, scopes=None):
    """Insert a test context item for relationships API scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in (scopes or ["public"])]

    row = await db_pool.fetchrow(
        """
        INSERT INTO context_items (title, source_type, content, privacy_scope_ids, status_id, tags)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING *
        """,
        title,
        "note",
        "relationship context",
        scope_ids,
        status_id,
        ["test"],
    )
    return dict(row)


async def _make_job(db_pool, enums, title, agent_id, scopes):
    """Insert a test job for relationships API scenarios."""

    status_id = enums.statuses.name_to_id["active"]
    scope_ids = [enums.scopes.name_to_id[s] for s in scopes]

    row = await db_pool.fetchrow(
        """
        INSERT INTO jobs (title, status_id, agent_id, privacy_scope_ids)
        VALUES ($1, $2, $3, $4)
        RETURNING *
        """,
        title,
        status_id,
        agent_id,
        scope_ids,
    )
    return dict(row)


async def _make_relationship(db_pool, enums, source_type, source_id, target_type, target_id):
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


def _auth_override(agent_id, enums, scopes=None):
    """Build an auth override for public agent API requests."""

    scope_ids = [enums.scopes.name_to_id[s] for s in (scopes or ["public"])]
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


def _user_auth_override(entity_row, enums, scopes=None):
    """Build an auth override for public user API requests."""

    scope_ids = [enums.scopes.name_to_id[s] for s in (scopes or ["public"])]
    auth_dict = {
        "key_id": None,
        "caller_type": "user",
        "entity_id": entity_row["id"],
        "entity": entity_row,
        "agent_id": None,
        "agent": None,
        "scopes": scope_ids,
    }

    async def mock_auth():
        """Mock auth for public user caller."""

        return auth_dict

    return mock_auth


@pytest.mark.asyncio
async def test_get_relationships_hides_foreign_job_links(db_pool, enums):
    """Relationships API should filter job links by job scopes."""

    owner = await _make_agent(db_pool, enums, "rel-owner-api")
    viewer = await _make_agent(db_pool, enums, "rel-viewer-api")
    entity = await _make_entity(db_pool, enums, "Public Node")
    public_job = await _make_job(db_pool, enums, "Public Job", owner["id"], ["public"])
    private_job = await _make_job(db_pool, enums, "Private Job", owner["id"], ["private"])
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
async def test_create_relationship_denies_private_entity_for_public_agent(db_pool, enums):
    """Public agents should not create links from private entities."""

    viewer = await _make_agent(db_pool, enums, "rel-viewer-api-private-entity")
    private_entity = await _make_entity(db_pool, enums, "Sensitive Node", scopes=["sensitive"])
    public_entity = await _make_entity(db_pool, enums, "Public Node 3")

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/relationships/",
            json={
                "source_type": "entity",
                "source_id": str(private_entity["id"]),
                "target_type": "entity",
                "target_id": str(public_entity["id"]),
                "relationship_type": "related-to",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_create_relationship_denies_private_context_for_public_agent(db_pool, enums):
    """Public agents should not create links from private context items."""

    viewer = await _make_agent(db_pool, enums, "rel-viewer-api-private-context")
    private_context = await _make_context(db_pool, enums, "Sensitive Context", scopes=["sensitive"])
    public_entity = await _make_entity(db_pool, enums, "Public Node 4")

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.post(
            "/api/relationships/",
            json={
                "source_type": "context",
                "source_id": str(private_context["id"]),
                "target_type": "entity",
                "target_id": str(public_entity["id"]),
                "relationship_type": "references",
            },
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_update_relationship_denies_private_source_for_public_agent(db_pool, enums):
    """Public agents should not update links attached to private entities."""

    viewer = await _make_agent(db_pool, enums, "rel-viewer-api-update-private")
    private_entity = await _make_entity(db_pool, enums, "Sensitive Node 2", scopes=["sensitive"])
    public_entity = await _make_entity(db_pool, enums, "Public Node 5")
    relationship = await _make_relationship(
        db_pool,
        enums,
        "entity",
        str(private_entity["id"]),
        "entity",
        str(public_entity["id"]),
    )

    app.dependency_overrides[require_auth] = _auth_override(viewer["id"], enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.patch(
            f"/api/relationships/{relationship['id']}",
            json={"properties": {"note": "hijack"}},
        )
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_query_relationships_hides_foreign_job_links(db_pool, enums):
    """Query relationships should filter job relationships by job scopes."""

    owner = await _make_agent(db_pool, enums, "rel-owner-api-2")
    viewer = await _make_agent(db_pool, enums, "rel-viewer-api-2")
    entity = await _make_entity(db_pool, enums, "Public Node 2")
    public_job = await _make_job(db_pool, enums, "Public Job 2", owner["id"], ["public"])
    private_job = await _make_job(db_pool, enums, "Private Job 2", owner["id"], ["private"])
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


@pytest.mark.asyncio
async def test_get_relationships_hides_foreign_job_links_for_user(db_pool, enums):
    """User callers should not see relationships to private jobs."""

    owner = await _make_agent(db_pool, enums, "rel-owner-api-user-get")
    entity = await _make_entity(db_pool, enums, "Public Node User Get")
    private_job = await _make_job(db_pool, enums, "Private Job User Get", owner["id"], ["private"])
    rel = await _make_relationship(
        db_pool, enums, "entity", str(entity["id"]), "job", private_job["id"]
    )

    app.dependency_overrides[require_auth] = _user_auth_override(entity, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get(f"/api/relationships/entity/{entity['id']}")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {row["id"] for row in resp.json()["data"]}
    assert str(rel["id"]) not in ids


@pytest.mark.asyncio
async def test_query_relationships_hides_foreign_job_links_for_user(db_pool, enums):
    """User callers should not see query results linked to private jobs."""

    owner = await _make_agent(db_pool, enums, "rel-owner-api-user-query")
    entity = await _make_entity(db_pool, enums, "Public Node User Query")
    private_job = await _make_job(
        db_pool, enums, "Private Job User Query", owner["id"], ["private"]
    )
    rel = await _make_relationship(
        db_pool, enums, "job", private_job["id"], "entity", str(entity["id"])
    )

    app.dependency_overrides[require_auth] = _user_auth_override(entity, enums)
    app.state.pool = db_pool
    app.state.enums = enums
    transport = ASGITransport(app=app)
    async with AsyncClient(transport=transport, base_url="http://test") as client:
        resp = await client.get("/api/relationships/")
    app.dependency_overrides.pop(require_auth, None)

    assert resp.status_code == 200
    ids = {row["id"] for row in resp.json()["data"]}
    assert str(rel["id"]) not in ids
